package js

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/javascript"
	"github.com/lucasew/refactree/pkg/ingest"
)

func init() {
	ingest.RegisterLanguageDriver("javascript", languageDriver{})
	ingest.RegisterLanguageRules("javascript", ingest.LanguageRules{
		Extensions:      []string{".js", ".mjs", ".cjs"},
		DirectoryModule: false,
	})
	ingest.RegisterReferenceProvider("node", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "javascript" }

// TreeSitterGrammar maps .js/.mjs/.cjs (and grammar.GetByExtension misses) to
// the registered "javascript" grammar.
func (languageDriver) TreeSitterGrammar(filename string) (grammar.Language, bool) {
	if lang, ok := grammar.GetByExtension(filename); ok {
		return lang, true
	}
	return grammar.Get("javascript")
}

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	return extractJavaScript(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	if ref, ok := resolvePathImport(sourcePath, ctx); ok {
		return ref
	}
	if ref, ok := resolveNodeImport(sourcePath, ctx); ok {
		return ref
	}
	if strings.HasPrefix(sourcePath, "node:") {
		return sourcePath
	}
	return "node:" + sourcePath
}

func (languageDriver) AllowListSymbol(string, ingest.SymbolListOptions) bool { return true }

func (languageDriver) DestinationFileInDirectory(dstDirRel string, _ ingest.Reference) string {
	_ = dstDirRel
	return ""
}

// ResolveDirectoryModule maps a JS/Node package directory to package.json main
// or index.{js,mjs,cjs} (same convention as import resolution).
func (languageDriver) ResolveDirectoryModule(absDir string) (string, bool) {
	resolved, ok := resolveJSFileOnDisk(absDir, true)
	if !ok {
		return "", false
	}
	st, err := os.Stat(absDir)
	if err != nil || !st.IsDir() {
		return "", false
	}
	rel, err := filepath.Rel(absDir, resolved)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

type referenceProvider struct{}

func (referenceProvider) Name() string { return "node" }

func (referenceProvider) Resolve(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	return resolveNodeImport(spec, ctx)
}

func resolvePathImport(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if !(strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/")) {
		return "", false
	}

	rel := relImportPath(ctx.ImporterPath, spec)
	if rel == "" {
		return "", false
	}

	if ref, ok := resolveKnownJSPath(rel, ctx.KnownFiles); ok {
		return ref, true
	}

	rootAbs, err := filepath.Abs(ctx.RootDir)
	if err != nil {
		return ingest.FileRef("./" + rel), true
	}
	candidate := filepath.Join(rootAbs, filepath.FromSlash(rel))
	resolved, ok := resolveJSFileOnDisk(candidate, false)
	if ok {
		return pathRefForAbs(rootAbs, resolved), true
	}

	return ingest.FileRef("./" + rel), true
}

func resolveNodeImport(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if strings.HasPrefix(spec, "node:") {
		return spec, true
	}
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") {
		return "", false
	}

	pkgName, subpath := splitNodePackageSpecifier(spec)
	if pkgName == "" {
		return "node:" + spec, true
	}

	rootAbs, err := filepath.Abs(ctx.RootDir)
	if err != nil {
		rootAbs = ctx.RootDir
	}
	importerAbs := filepath.Join(rootAbs, filepath.FromSlash(filepath.Dir(ctx.ImporterPath)))

	for _, pkgRoot := range nodeModuleCandidates(importerAbs, pkgName) {
		targetBase := pkgRoot
		preferPackageMain := subpath == ""
		if subpath != "" {
			targetBase = filepath.Join(pkgRoot, filepath.FromSlash(subpath))
			preferPackageMain = false
		}

		if resolved, ok := resolveJSFileOnDisk(targetBase, preferPackageMain); ok {
			return pathRefForAbs(rootAbs, resolved), true
		}
		if st, err := os.Stat(targetBase); err == nil && st.IsDir() {
			return pathRefForAbs(rootAbs, targetBase), true
		}
	}

	return "node:" + spec, true
}

func extractJavaScript(root *grammar.Node, source []byte, path string) *ingest.FileExtract {
	fe := &ingest.FileExtract{Language: "javascript", Path: path}

	for i := uint32(0); i < root.ChildCount(); i++ {
		extractJSTopLevel(fe, root.Child(i), source)
	}

	return fe
}

func extractJSTopLevel(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	if n == nil {
		return
	}
	switch n.Type() {
	case "function_declaration":
		extractJSFunc(fe, n, source)
	case "class_declaration":
		extractJSClass(fe, n, source)
	case "lexical_declaration", "variable_declaration":
		extractJSVarDecl(fe, n, source, "")
	case "expression_statement":
		walkJSUsages(fe, n, source, "")
	case "export_statement":
		extractJSExport(fe, n, source)
	case "import_statement":
		extractJSImport(fe, n, source)
	}
}

func extractJSExport(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	for j := uint32(0); j < n.ChildCount(); j++ {
		inner := n.Child(j)
		switch inner.Type() {
		case "function_declaration":
			extractJSFunc(fe, inner, source)
		case "class_declaration":
			extractJSClass(fe, inner, source)
		case "lexical_declaration", "variable_declaration":
			extractJSVarDecl(fe, inner, source, "")
		default:
			// export default defineConfig({...}) — usages inside the expression.
			if inner.IsNamed() {
				walkJSUsages(fe, inner, source, "")
			}
		}
	}
}

func extractJSVarDecl(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() != "variable_declarator" {
			continue
		}
		nameNode := ingest.ChildByField(child, "name")
		if nameNode != nil && nameNode.Type() == "identifier" {
			name := ingest.NodeText(nameNode, source)
			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      name,
				StartByte: nameNode.StartByte(),
				EndByte:   nameNode.EndByte(),
				Exported:  true,
			})
		}
		if value := ingest.ChildByField(child, "value"); value != nil {
			walkJSUsages(fe, value, source, scope)
		}
	}
}

func extractJSFunc(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	name := ingest.NodeText(nameNode, source)

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  true,
	})

	if body := ingest.ChildByField(n, "body"); body != nil {
		walkJSUsages(fe, body, source, name)
	}
}

func extractJSClass(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	className := ingest.NodeText(nameNode, source)

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      className,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  true,
	})

	body := ingest.ChildByField(n, "body")
	if body == nil {
		return
	}

	for i := uint32(0); i < body.ChildCount(); i++ {
		member := body.Child(i)
		if member.Type() != "method_definition" {
			continue
		}
		methodNameNode := ingest.ChildByField(member, "name")
		if methodNameNode == nil {
			continue
		}

		methodShort := ingest.NodeText(methodNameNode, source)
		methodName := className + "." + methodShort
		fe.Entities = append(fe.Entities, ingest.EntityDef{
			Name:      methodName,
			StartByte: methodNameNode.StartByte(),
			EndByte:   methodNameNode.EndByte(),
			Exported:  true,
		})

		if methodBody := ingest.ChildByField(member, "body"); methodBody != nil {
			walkJSUsages(fe, methodBody, source, methodName)
		}
	}
}

func extractJSImport(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	sourceNode := ingest.ChildByField(n, "source")
	if sourceNode == nil {
		return
	}
	fragment := ingest.ChildByType(sourceNode, "string_fragment")
	if fragment == nil {
		return
	}
	sourcePath := ingest.NodeText(fragment, source)

	clause := ingest.ChildByType(n, "import_clause")
	if clause == nil {
		return
	}

	for i := uint32(0); i < clause.ChildCount(); i++ {
		child := clause.Child(i)
		switch child.Type() {
		case "named_imports":
			extractJSNamedImports(fe, child, source, sourcePath)
		case "namespace_import":
			extractJSNamespaceImport(fe, child, source, sourcePath)
		case "identifier":
			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:  ingest.NodeText(child, source),
				SourcePath: sourcePath,
				StartByte:  child.StartByte(),
				EndByte:    child.EndByte(),
			})
		}
	}
}

func extractJSNamedImports(fe *ingest.FileExtract, n *grammar.Node, source []byte, sourcePath string) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		spec := n.Child(i)
		if spec.Type() != "import_specifier" {
			continue
		}
		nameNode := ingest.ChildByField(spec, "name")
		if nameNode == nil {
			continue
		}
		memberName := ingest.NodeText(nameNode, source)

		aliasNode := ingest.ChildByField(spec, "alias")
		localName := memberName
		startByte := nameNode.StartByte()
		endByte := nameNode.EndByte()
		if aliasNode != nil {
			localName = ingest.NodeText(aliasNode, source)
			startByte = aliasNode.StartByte()
			endByte = aliasNode.EndByte()
		}

		fe.Imports = append(fe.Imports, ingest.ImportDef{
			LocalName:       localName,
			SourcePath:      sourcePath,
			MemberName:      memberName,
			StartByte:       startByte,
			EndByte:         endByte,
			TargetStartByte: nameNode.StartByte(),
			TargetEndByte:   nameNode.EndByte(),
			HasAliasBinding: aliasNode != nil,
		})
	}
}

func extractJSNamespaceImport(fe *ingest.FileExtract, n *grammar.Node, source []byte, sourcePath string) {
	var nameNode *grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Type() == "identifier" {
			nameNode = c
		}
	}
	if nameNode == nil {
		return
	}
	fe.Imports = append(fe.Imports, ingest.ImportDef{
		LocalName:  ingest.NodeText(nameNode, source),
		SourcePath: sourcePath,
		StartByte:  nameNode.StartByte(),
		EndByte:    nameNode.EndByte(),
	})
}

func walkJSUsages(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if n.Type() == "call_expression" {
		funcNode := ingest.ChildByField(n, "function")
		if funcNode != nil {
			switch funcNode.Type() {
			case "identifier":
				fe.Usages = append(fe.Usages, ingest.UsageDef{
					Scope:     scope,
					Name:      ingest.NodeText(funcNode, source),
					StartByte: funcNode.StartByte(),
					EndByte:   funcNode.EndByte(),
				})
			case "member_expression":
				obj := ingest.ChildByField(funcNode, "object")
				prop := ingest.ChildByField(funcNode, "property")
				if obj != nil && prop != nil {
					fe.Usages = append(fe.Usages, ingest.UsageDef{
						Scope:         scope,
						Name:          ingest.NodeText(prop, source),
						StartByte:     prop.StartByte(),
						EndByte:       prop.EndByte(),
						Qualifier:     ingest.NodeText(obj, source),
						QualStartByte: obj.StartByte(),
						QualEndByte:   obj.EndByte(),
					})
				}
			}
		}
		if args := ingest.ChildByField(n, "arguments"); args != nil {
			walkJSUsages(fe, args, source, scope)
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		walkJSUsages(fe, n.Child(i), source, scope)
	}
}

func pathRefForAbs(rootDir, absPath string) string {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return ingest.FileRef(filepath.ToSlash(absPath))
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return ingest.FileRef(filepath.ToSlash(absPath))
	}

	rel, err := filepath.Rel(rootAbs, abs)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ingest.FileRef("./" + filepath.ToSlash(rel))
	}
	return ingest.FileRef(filepath.ToSlash(abs))
}

func relImportPath(importerPath, spec string) string {
	importerDir := filepath.ToSlash(filepath.Dir(importerPath))
	if importerDir == "." {
		importerDir = ""
	}
	if strings.HasPrefix(spec, "/") {
		return strings.TrimPrefix(filepath.ToSlash(spec), "/")
	}
	joined := filepath.ToSlash(filepath.Clean(filepath.Join(importerDir, spec)))
	joined = strings.TrimPrefix(joined, "./")
	return joined
}

func resolveKnownJSPath(rel string, knownFiles map[string]bool) (string, bool) {
	if knownFiles[rel] {
		return ingest.FileRef("./" + rel), true
	}
	for _, ext := range []string{".js", ".mjs", ".cjs"} {
		if knownFiles[rel+ext] {
			return ingest.FileRef("./" + rel + ext), true
		}
	}
	for _, indexName := range []string{"index.js", "index.mjs", "index.cjs"} {
		p := filepath.ToSlash(filepath.Join(rel, indexName))
		if knownFiles[p] {
			return ingest.FileRef("./" + p), true
		}
	}
	return "", false
}

func resolveJSFileOnDisk(baseAbs string, preferPackageMain bool) (string, bool) {
	if st, err := os.Stat(baseAbs); err == nil {
		if !st.IsDir() {
			return baseAbs, true
		}

		if preferPackageMain {
			if mainEntry, ok := readPackageMain(filepath.Join(baseAbs, "package.json")); ok {
				mainAbs := filepath.Join(baseAbs, filepath.FromSlash(mainEntry))
				if resolved, ok := resolveJSFileOnDisk(mainAbs, false); ok {
					return resolved, true
				}
			}
		}

		for _, indexName := range []string{"index.js", "index.mjs", "index.cjs"} {
			candidate := filepath.Join(baseAbs, indexName)
			if st2, err := os.Stat(candidate); err == nil && !st2.IsDir() {
				return candidate, true
			}
		}
	}

	for _, ext := range []string{".js", ".mjs", ".cjs"} {
		candidate := baseAbs + ext
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, true
		}
	}

	return "", false
}

func readPackageMain(packageJSONPath string) (string, bool) {
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return "", false
	}
	var pkg struct {
		Main string `json:"main"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false
	}
	if pkg.Main == "" {
		return "", false
	}
	return filepath.ToSlash(pkg.Main), true
}

func splitNodePackageSpecifier(spec string) (pkgName, subpath string) {
	parts := strings.Split(spec, "/")
	if spec == "" {
		return "", ""
	}
	if strings.HasPrefix(spec, "@") {
		if len(parts) < 2 {
			return spec, ""
		}
		pkgName = parts[0] + "/" + parts[1]
		if len(parts) > 2 {
			subpath = strings.Join(parts[2:], "/")
		}
		return pkgName, subpath
	}
	pkgName = parts[0]
	if len(parts) > 1 {
		subpath = strings.Join(parts[1:], "/")
	}
	return pkgName, subpath
}

func nodeModuleCandidates(importerAbsDir, pkgName string) []string {
	seen := map[string]bool{}
	out := []string{}

	for dir := importerAbsDir; ; {
		candidate := filepath.Join(dir, "node_modules", filepath.FromSlash(pkgName))
		if !seen[candidate] {
			seen[candidate] = true
			out = append(out, candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if nodePath := os.Getenv("NODE_PATH"); nodePath != "" {
		for _, entry := range filepath.SplitList(nodePath) {
			if entry == "" {
				continue
			}
			candidate := filepath.Join(entry, filepath.FromSlash(pkgName))
			if !seen[candidate] {
				seen[candidate] = true
				out = append(out, candidate)
			}
		}
	}

	return out
}
