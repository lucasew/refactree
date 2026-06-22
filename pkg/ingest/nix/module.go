package nix

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/nix"
	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
	nixref "github.com/lucasew/refactree/pkg/reference/nix"
)

func init() {
	ingest.RegisterLanguageDriver("nix", languageDriver{})
	ingest.RegisterLanguageRules("nix", ingest.LanguageRules{
		Extensions:      []string{".nix"},
		DirectoryModule: false,
	})
	ingest.RegisterReferenceProvider("nix", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "nix" }

func (languageDriver) TreeSitterGrammar(filename string) (grammar.Language, bool) {
	if lang, ok := grammar.GetByExtension(filename); ok {
		return lang, true
	}
	return grammar.Get("nix")
}

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	return extractNix(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	if ref, ok := resolveNixPathImport(sourcePath, ctx); ok {
		return ref
	}
	if ref, ok := (referenceProvider{}).Resolve(sourcePath, ctx); ok {
		return ref
	}
	return "nix:" + sourcePath
}

func (languageDriver) AllowListSymbol(name string, opts ingest.SymbolListOptions) bool {
	if opts.IncludeHidden {
		return true
	}
	return !(len(name) > 0 && name[0] == '_')
}

func (languageDriver) DestinationFileInDirectory(string, ingest.Reference) string {
	return ""
}

// ResolveDirectoryModule maps a Nix directory scope to default.nix when present.
func (languageDriver) ResolveDirectoryModule(absDir string) (string, bool) {
	def := filepath.Join(absDir, "default.nix")
	st, err := os.Stat(def)
	if err != nil || st.IsDir() {
		return "", false
	}
	return "default.nix", true
}

type referenceProvider struct{}

func (referenceProvider) Name() string { return "nix" }

func (referenceProvider) Resolve(spec string, _ ingest.ImportResolveContext) (string, bool) {
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") {
		return "", false
	}
	spec = normalizeProviderSpec(spec)
	if spec == "" {
		return "", false
	}
	return "nix:" + spec, true
}

func (referenceProvider) ResolveScopeTarget(ref ingest.Reference, _ string) (ingest.ProviderScopeTarget, bool, error) {
	if ref.Path == "" {
		return ingest.ProviderScopeTarget{}, false, nil
	}
	target, err := nixref.ResolveTarget(ref.Path)
	if err != nil {
		return ingest.ProviderScopeTarget{}, true, err
	}
	canDescend := target.IsDir && (target.File == "" || nixProviderRootScope(ref.Path))
	return ingest.ProviderScopeTarget{Dir: target.Dir, CanDescend: &canDescend}, true, nil
}

func (referenceProvider) ResolveSymbolTarget(ref ingest.Reference, _ string) (ingest.ProviderSymbolTarget, bool, error) {
	target, ok, err := nixref.ResolveSymbolTarget(ref.Path, ref.Symbol)
	if !ok || err != nil {
		return ingest.ProviderSymbolTarget{}, ok, err
	}
	return ingest.ProviderSymbolTarget{Dir: target.Dir, Symbol: target.Symbol}, true, nil
}

func (referenceProvider) ListScopeChildren(ref ingest.Reference, _ string, includeHidden bool) ([]refpkg.ScopeChild, bool, error) {
	if ref.Path == "" {
		return nil, false, nil
	}
	target, err := nixref.ResolveTarget(ref.Path)
	if err != nil {
		return nil, true, err
	}
	if !target.IsDir {
		return nil, true, nil
	}

	entries, err := os.ReadDir(target.Dir)
	if err != nil {
		return nil, true, err
	}

	children := make([]refpkg.ScopeChild, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue
		}

		childRef := ingest.Reference{Provider: "nix", Path: joinProviderPath(ref.Path, name)}
		if entry.IsDir() {
			children = append(children, refpkg.ScopeChild{
				Ref:  childRef,
				Kind: refpkg.ScopeChildDir,
			})
			continue
		}
		if !isNixSourceFile(name) {
			continue
		}
		children = append(children, refpkg.ScopeChild{
			Ref:  childRef,
			Kind: refpkg.ScopeChildFile,
		})
	}

	sort.Slice(children, func(i, j int) bool {
		if children[i].Kind != children[j].Kind {
			return children[i].Kind < children[j].Kind
		}
		return children[i].Ref.Path < children[j].Ref.Path
	})
	return children, true, nil
}

func (referenceProvider) ListIngestRecursive(_ ingest.Reference, opts ingest.ListOptions) bool {
	return opts.Recursive
}

func (referenceProvider) AllowListEntity(ref ingest.Reference, _ ingest.Reference, entPath, language string, _ ingest.ListOptions) bool {
	if language != "nix" {
		return false
	}
	target, err := nixref.ResolveTarget(ref.Path)
	if err != nil {
		return false
	}
	return nixref.MatchesEntityPath(target, entPath)
}

func (referenceProvider) ListOutputReference(ref ingest.Reference, entRef ingest.Reference) ingest.Reference {
	return ingest.Reference{Provider: ref.Provider, Path: ref.Path, Symbol: entRef.Symbol}
}

func (referenceProvider) DocIngestRecursive(ingest.Reference) bool { return false }

func (referenceProvider) AllowDocEntity(ref ingest.Reference, _ ingest.Reference, entPath, language string) bool {
	if language != "nix" {
		return false
	}
	target, err := nixref.ResolveTarget(ref.Path)
	if err != nil {
		return false
	}
	return nixref.MatchesEntityPath(target, entPath)
}

func extractNix(root *grammar.Node, source []byte, filePath string) *ingest.FileExtract {
	fe := &ingest.FileExtract{Language: "nix", Path: filePath}

	expr := nixTopLevelExpression(root)
	if expr == nil {
		return fe
	}

	switch expr.Type() {
	case "attrset_expression":
		if bindings := ingest.ChildByType(expr, "binding_set"); bindings != nil {
			extractNixBindingSet(fe, bindings, source, true)
		}
	case "let_expression":
		if bindings := ingest.ChildByType(expr, "binding_set"); bindings != nil {
			extractNixBindingSet(fe, bindings, source, false)
		}
		if body := ingest.ChildByField(expr, "body"); body != nil {
			walkNixUsages(fe, body, source, "")
		}
	default:
		walkNixUsages(fe, expr, source, "")
	}

	return fe
}

func extractNixBindingSet(fe *ingest.FileExtract, bindings *grammar.Node, source []byte, exportBindings bool) {
	for i := uint32(0); i < bindings.ChildCount(); i++ {
		binding := bindings.Child(i)
		if binding.Type() != "binding" {
			continue
		}

		attrpath := ingest.ChildByField(binding, "attrpath")
		if attrpath == nil {
			continue
		}
		name := nixAttrPathName(attrpath, source)
		if name == "" {
			continue
		}

		expr := ingest.ChildByField(binding, "expression")
		if expr == nil {
			continue
		}

		if spec, ok := nixImportSpec(expr, source); ok {
			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:  name,
				SourcePath: spec,
				StartByte:  attrpath.StartByte(),
				EndByte:    attrpath.EndByte(),
			})
			continue
		}

		scope := ""
		if exportBindings {
			scope = name
			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      name,
				StartByte: attrpath.StartByte(),
				EndByte:   attrpath.EndByte(),
				Exported:  languageDriver{}.AllowListSymbol(name, ingest.SymbolListOptions{}),
			})
		}

		walkNixUsages(fe, expr, source, scope)
	}
}

func walkNixUsages(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	switch n.Type() {
	case "select_expression":
		qualNode, qualifier := nixSelectQualifier(ingest.ChildByField(n, "expression"), source)
		memberNode, member := nixAttrPathLast(ingest.ChildByField(n, "attrpath"), source)
		if qualNode != nil && memberNode != nil && qualifier != "" && member != "" {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          member,
				StartByte:     memberNode.StartByte(),
				EndByte:       memberNode.EndByte(),
				Qualifier:     qualifier,
				QualStartByte: qualNode.StartByte(),
				QualEndByte:   qualNode.EndByte(),
			})
		}
		return
	case "variable_expression":
		nameNode := ingest.ChildByField(n, "name")
		if nameNode != nil {
			name := ingest.NodeText(nameNode, source)
			if name != "" && name != "import" {
				fe.Usages = append(fe.Usages, ingest.UsageDef{
					Scope:     scope,
					Name:      name,
					StartByte: nameNode.StartByte(),
					EndByte:   nameNode.EndByte(),
				})
			}
		}
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkNixUsages(fe, n.Child(i), source, scope)
	}
}

func nixTopLevelExpression(root *grammar.Node) *grammar.Node {
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if root.FieldNameForChild(i) == "expression" && child.IsNamed() {
			return child
		}
	}
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.IsNamed() {
			return child
		}
	}
	return nil
}

func nixImportSpec(n *grammar.Node, source []byte) (string, bool) {
	if n == nil || n.Type() != "apply_expression" {
		return "", false
	}

	functionNode := ingest.ChildByField(n, "function")
	if functionNode == nil {
		return "", false
	}

	if nixIsImportFunction(functionNode, source) {
		return nixImportArgumentSpec(ingest.ChildByField(n, "argument"), source)
	}

	return nixImportSpec(functionNode, source)
}

func nixIsImportFunction(n *grammar.Node, source []byte) bool {
	if n == nil || n.Type() != "variable_expression" {
		return false
	}
	nameNode := ingest.ChildByField(n, "name")
	return nameNode != nil && ingest.NodeText(nameNode, source) == "import"
}

func nixImportArgumentSpec(n *grammar.Node, source []byte) (string, bool) {
	if n == nil {
		return "", false
	}

	switch n.Type() {
	case "spath_expression":
		return normalizeProviderSpec(ingest.NodeText(n, source)), true
	case "path_expression":
		return ingest.NodeText(n, source), true
	default:
		return "", false
	}
}

func nixSelectQualifier(n *grammar.Node, source []byte) (*grammar.Node, string) {
	if n == nil || n.Type() != "variable_expression" {
		return nil, ""
	}
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return nil, ""
	}
	return nameNode, ingest.NodeText(nameNode, source)
}

func nixAttrPathName(n *grammar.Node, source []byte) string {
	parts := []string{}
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() == "identifier" {
			parts = append(parts, ingest.NodeText(child, source))
		}
	}
	return strings.Join(parts, ".")
}

func nixAttrPathLast(n *grammar.Node, source []byte) (*grammar.Node, string) {
	if n == nil {
		return nil, ""
	}
	for i := int(n.ChildCount()) - 1; i >= 0; i-- {
		child := n.Child(uint32(i))
		if child.Type() == "identifier" {
			return child, ingest.NodeText(child, source)
		}
	}
	return nil, ""
}

func resolveNixPathImport(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if !(strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/")) {
		return "", false
	}

	rel := relImportPath(ctx.ImporterPath, spec)
	if rel == "" {
		return "", false
	}

	if ref, ok := resolveKnownNixPath(rel, ctx.KnownFiles); ok {
		return ref, true
	}

	rootAbs, err := filepath.Abs(ctx.RootDir)
	if err != nil {
		return ingest.FileRef("./" + rel), true
	}
	candidate := filepath.Join(rootAbs, filepath.FromSlash(rel))
	if resolved, ok := resolveNixPathOnDisk(candidate); ok {
		return pathRefForAbs(rootAbs, resolved), true
	}

	return ingest.FileRef("./" + rel), true
}

func resolveKnownNixPath(rel string, knownFiles map[string]bool) (string, bool) {
	if knownFiles[rel] {
		return ingest.FileRef("./" + rel), true
	}
	if knownFiles[path.Join(rel, "default.nix")] {
		return ingest.FileRef("./" + rel), true
	}
	return "", false
}

func resolveNixPathOnDisk(baseAbs string) (string, bool) {
	st, err := osStat(baseAbs)
	if err == nil {
		if st.IsDir() {
			if _, err := osStat(filepath.Join(baseAbs, "default.nix")); err == nil {
				return baseAbs, true
			}
			return baseAbs, true
		}
		return baseAbs, true
	}
	return "", false
}

var osStat = os.Stat

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

func normalizeProviderSpec(spec string) string {
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "<") && strings.HasSuffix(spec, ">") {
		spec = strings.TrimPrefix(strings.TrimSuffix(spec, ">"), "<")
	}
	return strings.Trim(spec, "/")
}

func nixProviderRootScope(spec string) bool {
	spec = normalizeProviderSpec(spec)
	return spec != "" && !strings.Contains(spec, "/")
}

func joinProviderPath(base, name string) string {
	base = strings.Trim(base, "/")
	name = strings.Trim(name, "/")
	if base == "" {
		return name
	}
	if name == "" {
		return base
	}
	return base + "/" + name
}

func isNixSourceFile(name string) bool {
	return strings.HasSuffix(name, ".nix")
}
