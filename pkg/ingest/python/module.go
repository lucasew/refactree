package python

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/python"
	"github.com/lucasew/refactree/pkg/ingest"
	pythonref "github.com/lucasew/refactree/pkg/reference/python"
)

func init() {
	ingest.RegisterLanguageDriver("python", languageDriver{})
	ingest.RegisterLanguageRules("python", ingest.LanguageRules{
		Extensions:      []string{".py"},
		DirectoryModule: false,
		Family:          ingest.FamilyPython,
	})
	ingest.RegisterReferenceProvider("python", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "python" }

func (languageDriver) TreeSitterGrammar(filename string) (grammar.Language, bool) {
	if lang, ok := grammar.GetByExtension(filename); ok {
		return lang, true
	}
	return grammar.Get("python")
}

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	return extractPython(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	if ref, ok := (referenceProvider{}).Resolve(sourcePath, ctx); ok {
		return ref
	}
	return "python:" + sourcePath
}

func (languageDriver) AllowListSymbol(name string, opts ingest.SymbolListOptions) bool {
	if opts.IncludeHidden {
		return true
	}
	check := pythonVisibilityName(name)
	return !(len(check) > 0 && check[0] == '_')
}

func (languageDriver) DestinationFileInDirectory(dstDirRel string, _ ingest.Reference) string {
	_ = dstDirRel
	return ""
}

// ResolveDirectoryModule maps a Python package directory to __init__.py.
func (languageDriver) ResolveDirectoryModule(absDir string) (string, bool) {
	initPy := filepath.Join(absDir, "__init__.py")
	st, err := os.Stat(initPy)
	if err != nil || st.IsDir() {
		return "", false
	}
	return "__init__.py", true
}

type referenceProvider struct{}

func (referenceProvider) Name() string { return "python" }

func (referenceProvider) Resolve(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if ref, ok := resolvePythonImportSpec(spec, ctx); ok {
		return ref, true
	}
	return "python:" + spec, true
}

func (referenceProvider) ResolveScopeTarget(ref ingest.Reference, rootDir string) (ingest.ProviderScopeTarget, bool, error) {
	if ref.Path == "" {
		return ingest.ProviderScopeTarget{}, false, nil
	}
	target, err := pythonref.ResolveModuleTarget(ref.Path, rootDir)
	if err != nil {
		return ingest.ProviderScopeTarget{}, true, err
	}
	canDescend := target.IsPackage
	return ingest.ProviderScopeTarget{Dir: target.Dir, CanDescend: &canDescend}, true, nil
}

func (referenceProvider) ResolveSymbolTarget(ref ingest.Reference, rootDir string) (ingest.ProviderSymbolTarget, bool, error) {
	target, ok, err := pythonref.ResolveSymbolTarget(ref.Path, ref.Symbol, rootDir)
	if !ok || err != nil {
		return ingest.ProviderSymbolTarget{}, ok, err
	}
	return ingest.ProviderSymbolTarget{Dir: target.Dir, Symbol: target.Symbol}, true, nil
}

func (referenceProvider) ListIngestRecursive(_ ingest.Reference, opts ingest.ListOptions) bool {
	return opts.Recursive
}

func (referenceProvider) AllowListEntity(ref ingest.Reference, _ ingest.Reference, entPath, language string, _ ingest.ListOptions) bool {
	if language != "python" {
		return false
	}
	// Listing/doc filter without workDir still works for stdlib; callers with
	// project context go through Resolver which passes rootDir at scope resolve.
	target, err := pythonref.ResolveModuleTarget(ref.Path, "")
	if err != nil {
		return false
	}
	return pythonref.MatchesEntityPath(target, entPath)
}

func (referenceProvider) ListOutputReference(ref ingest.Reference, entRef ingest.Reference) ingest.Reference {
	return ingest.Reference{Provider: ref.Provider, Path: ref.Path, Symbol: entRef.Symbol}
}

func (referenceProvider) DocIngestRecursive(ingest.Reference) bool { return false }

func (referenceProvider) AllowDocEntity(ref ingest.Reference, _ ingest.Reference, entPath, language string) bool {
	if language != "python" {
		return false
	}
	target, err := pythonref.ResolveModuleTarget(ref.Path, "")
	if err != nil {
		return false
	}
	return pythonref.MatchesEntityPath(target, entPath)
}

func extractPython(root *grammar.Node, source []byte, path string) *ingest.FileExtract {
	fe := &ingest.FileExtract{Language: "python", Path: path}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_definition":
			extractPythonFunc(fe, child, source)
		case "class_definition":
			extractPythonClass(fe, child, source)
		case "import_from_statement":
			extractPythonImportFrom(fe, child, source)
		case "import_statement":
			extractPythonImport(fe, child, source)
		case "assignment", "augmented_assignment":
			extractPythonAssign(fe, child, source, "")
		case "expression_statement":
			// Sometimes assignments are wrapped; accept either shape.
			if child.ChildCount() > 0 {
				inner := child.Child(0)
				if inner.Type() == "assignment" || inner.Type() == "augmented_assignment" {
					extractPythonAssign(fe, inner, source, "")
				}
			}
		}
	}

	return fe
}

func extractPythonAssign(fe *ingest.FileExtract, assign *grammar.Node, source []byte, scope string) {
	if assign == nil {
		return
	}
	left := ingest.ChildByField(assign, "left")
	if left != nil && left.Type() == "identifier" {
		appendPythonEntity(fe, left, source)
	}
	// Type/value sides may reference imports (logger = logging.getLogger(...)).
	if right := ingest.ChildByField(assign, "right"); right != nil {
		walkPythonUsages(fe, right, source, scope)
	}
	if typ := ingest.ChildByField(assign, "type"); typ != nil {
		walkPythonUsages(fe, typ, source, scope)
	}
}

func appendPythonEntity(fe *ingest.FileExtract, nameNode *grammar.Node, source []byte) {
	name := ingest.NodeText(nameNode, source)
	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  len(name) == 0 || name[0] != '_',
	})
}

func extractPythonFunc(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	name := ingest.NodeText(nameNode, source)

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  len(name) == 0 || name[0] != '_',
	})

	// Decorators, params, return type — not the function name itself.
	if dec := ingest.ChildByField(n, "superclasses"); dec != nil {
		walkPythonUsages(fe, dec, source, name)
	}
	for _, field := range []string{"parameters", "return_type"} {
		if part := ingest.ChildByField(n, field); part != nil {
			walkPythonUsages(fe, part, source, name)
		}
	}
	if body := ingest.ChildByField(n, "body"); body != nil {
		walkPythonUsages(fe, body, source, name)
	}
}

func extractPythonClass(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	className := ingest.NodeText(nameNode, source)

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      className,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  len(className) == 0 || className[0] != '_',
	})

	if bases := ingest.ChildByField(n, "superclasses"); bases != nil {
		walkPythonUsages(fe, bases, source, className)
	}

	if body := ingest.ChildByField(n, "body"); body != nil {
		for i := uint32(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			if child.Type() != "function_definition" {
				// Class-level assignments / attributes.
				if child.Type() == "assignment" || child.Type() == "augmented_assignment" {
					extractPythonAssign(fe, child, source, className)
				} else if child.Type() == "expression_statement" && child.ChildCount() > 0 {
					inner := child.Child(0)
					if inner.Type() == "assignment" || inner.Type() == "augmented_assignment" {
						extractPythonAssign(fe, inner, source, className)
					}
				}
				continue
			}

			methodNameNode := ingest.ChildByField(child, "name")
			if methodNameNode == nil {
				continue
			}
			methodShort := ingest.NodeText(methodNameNode, source)
			methodName := className + "." + methodShort

			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      methodName,
				StartByte: methodNameNode.StartByte(),
				EndByte:   methodNameNode.EndByte(),
				Exported:  len(methodShort) == 0 || methodShort[0] != '_',
			})

			for _, field := range []string{"parameters", "return_type"} {
				if part := ingest.ChildByField(child, field); part != nil {
					walkPythonUsages(fe, part, source, methodName)
				}
			}
			if methodBody := ingest.ChildByField(child, "body"); methodBody != nil {
				walkPythonUsages(fe, methodBody, source, methodName)
			}
		}
	}
}

func extractPythonImportFrom(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	modNode := ingest.ChildByField(n, "module_name")
	if modNode == nil {
		return
	}
	moduleName := pythonModuleSpec(modNode, source)

	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.FieldNameForChild(i) != "name" {
			continue
		}
		child := n.Child(i)

		switch child.Type() {
		case "aliased_import":
			nameNode := ingest.ChildByField(child, "name")
			aliasNode := ingest.ChildByField(child, "alias")
			importedNode := pythonImportNameNode(nameNode)
			if importedNode == nil || aliasNode == nil {
				continue
			}
			importedName := ingest.NodeText(importedNode, source)

			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:       ingest.NodeText(aliasNode, source),
				SourcePath:      moduleName,
				MemberName:      importedName,
				StartByte:       aliasNode.StartByte(),
				EndByte:         aliasNode.EndByte(),
				TargetStartByte: importedNode.StartByte(),
				TargetEndByte:   importedNode.EndByte(),
				HasAliasBinding: true,
			})
		default:
			importedNode := pythonImportNameNode(child)
			if importedNode == nil {
				continue
			}
			importedName := ingest.NodeText(importedNode, source)

			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:       importedName,
				SourcePath:      moduleName,
				MemberName:      importedName,
				StartByte:       importedNode.StartByte(),
				EndByte:         importedNode.EndByte(),
				TargetStartByte: importedNode.StartByte(),
				TargetEndByte:   importedNode.EndByte(),
			})
		}
	}
}

func extractPythonImport(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.FieldNameForChild(i) != "name" {
			continue
		}
		child := n.Child(i)

		switch child.Type() {
		case "aliased_import":
			nameNode := ingest.ChildByField(child, "name")
			aliasNode := ingest.ChildByField(child, "alias")
			if nameNode == nil || aliasNode == nil {
				continue
			}
			moduleName := ingest.NodeText(nameNode, source)

			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:       ingest.NodeText(aliasNode, source),
				SourcePath:      moduleName,
				StartByte:       aliasNode.StartByte(),
				EndByte:         aliasNode.EndByte(),
				HasAliasBinding: true,
			})

		case "dotted_name":
			// import os.path → local name is the first segment ("os"), full module is dotted text.
			first := pythonFirstIdentifier(child)
			if first == nil {
				continue
			}
			full := ingest.NodeText(child, source)
			local := ingest.NodeText(first, source)
			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:  local,
				SourcePath: full,
				StartByte:  first.StartByte(),
				EndByte:    first.EndByte(),
			})
		}
	}
}

func pythonFirstIdentifier(n *grammar.Node) *grammar.Node {
	if n == nil {
		return nil
	}
	if n.Type() == "identifier" {
		return n
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() == "identifier" {
			return child
		}
	}
	return nil
}

func pythonImportNameNode(n *grammar.Node) *grammar.Node {
	if n == nil {
		return nil
	}
	if n.Type() == "identifier" {
		return n
	}
	if n.Type() != "dotted_name" {
		return nil
	}

	var last *grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() == "identifier" {
			last = child
		}
	}
	return last
}

func walkPythonUsages(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if n == nil || n.IsNull() {
		return
	}

	switch n.Type() {
	case "attribute":
		obj := ingest.ChildByField(n, "object")
		attr := ingest.ChildByField(n, "attribute")
		// Only simple obj.attr (obj is identifier) — matches import/local entity resolution.
		if obj != nil && attr != nil && obj.Type() == "identifier" {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          ingest.NodeText(attr, source),
				StartByte:     attr.StartByte(),
				EndByte:       attr.EndByte(),
				Qualifier:     ingest.NodeText(obj, source),
				QualStartByte: obj.StartByte(),
				QualEndByte:   obj.EndByte(),
			})
			return
		}
	case "identifier":
		fe.Usages = append(fe.Usages, ingest.UsageDef{
			Scope:     scope,
			Name:      ingest.NodeText(n, source),
			StartByte: n.StartByte(),
			EndByte:   n.EndByte(),
		})
		return
	case "string", "integer", "float", "true", "false", "none", "comment":
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkPythonUsages(fe, n.Child(i), source, scope)
	}
}

func pythonVisibilityName(name string) string {
	if name == "" {
		return ""
	}
	if i := strings.LastIndex(name, "."); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return name
}

func resolvePythonImportSpec(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if strings.HasPrefix(spec, ".") {
		if ref, ok := resolvePythonRelativeImport(spec, ctx); ok {
			return ref, true
		}
		return "", false
	}
	if ref, ok := resolvePythonAbsoluteImport(spec, ctx.KnownFiles); ok {
		return ref, true
	}
	// Outside the ingest slice: resolve via project .venv / importlib (ctx.RootDir = serve --dir).
	if _, err := pythonref.ResolveModuleTarget(spec, ctx.RootDir); err == nil {
		return "python:" + spec, true
	}
	return "", false
}

func resolvePythonAbsoluteImport(spec string, knownFiles map[string]bool) (string, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", false
	}

	candidates := []string{strings.ReplaceAll(spec, ".", "/")}
	if !strings.Contains(spec, "/") {
		candidates = append(candidates, spec)
	}

	for _, modulePath := range candidates {
		if modulePath == "" {
			continue
		}
		if knownFiles[modulePath+".py"] {
			return ingest.FileRef("./" + modulePath + ".py"), true
		}
		if knownFiles[modulePath+"/__init__.py"] {
			// Point at the package file, not the directory (web/annotate need a file path).
			return ingest.FileRef("./" + modulePath + "/__init__.py"), true
		}
	}
	return "", false
}

func resolvePythonRelativeImport(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	level := 0
	for level < len(spec) && spec[level] == '.' {
		level++
	}
	if level == 0 {
		return "", false
	}

	tail := strings.TrimPrefix(spec, strings.Repeat(".", level))
	tail = strings.ReplaceAll(tail, ".", "/")

	baseDir := filepath.ToSlash(filepath.Dir(ctx.ImporterPath))
	if baseDir == "." {
		baseDir = ""
	}
	for i := 1; i < level; i++ {
		baseDir = path.Dir(baseDir)
		if baseDir == "." {
			baseDir = ""
		}
	}

	modulePath := baseDir
	if tail != "" {
		if modulePath == "" {
			modulePath = tail
		} else {
			modulePath = path.Join(modulePath, tail)
		}
	}
	if modulePath == "" {
		return "", false
	}

	if ctx.KnownFiles[modulePath+".py"] {
		return ingest.FileRef("./" + modulePath + ".py"), true
	}
	if ctx.KnownFiles[modulePath+"/__init__.py"] {
		return ingest.FileRef("./" + modulePath + "/__init__.py"), true
	}
	return "", false
}

func pythonModuleSpec(n *grammar.Node, source []byte) string {
	if n == nil {
		return ""
	}
	if n.Type() != "relative_import" {
		return ingest.NodeText(n, source)
	}

	prefix := ""
	if p := ingest.ChildByType(n, "import_prefix"); p != nil {
		for _, r := range ingest.NodeText(p, source) {
			if r == '.' {
				prefix += "."
			}
		}
	}

	if dotted := ingest.ChildByType(n, "dotted_name"); dotted != nil {
		return prefix + ingest.NodeText(dotted, source)
	}
	return prefix
}
