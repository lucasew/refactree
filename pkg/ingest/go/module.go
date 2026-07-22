package ingestgo

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
	goref "github.com/lucasew/refactree/pkg/reference/go"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/go"
)

func init() {
	ingest.RegisterLanguageDriver("go", languageDriver{})
	ingest.RegisterLanguageRules("go", ingest.LanguageRules{
		Extensions:      []string{".go"},
		DirectoryModule: true,
		Family:          ingest.FamilyGo,
	})
	ingest.RegisterReferenceProvider("go", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "go" }

func (languageDriver) TreeSitterGrammar(filename string) (grammar.Language, bool) {
	if lang, ok := grammar.GetByExtension(filename); ok {
		return lang, true
	}
	return grammar.Get("go")
}

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	return extractGo(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	return goref.ResolveImport(sourcePath, ctx.KnownDirs)
}

func (languageDriver) AllowListAtom(name string, opts ingest.AtomListOptions) bool {
	if opts.IncludeHidden {
		return true
	}
	exportName := goSymbolExportName(name)
	if exportName == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(exportName)
	return unicode.IsUpper(r)
}

func (languageDriver) DestinationFileInDirectory(dstDirRel string, srcRef ingest.Reference) string {
	srcPath := strings.TrimPrefix(srcRef.Path, "./")
	return path.Join(dstDirRel, path.Base(srcPath))
}

type referenceProvider struct{}

func (referenceProvider) Name() string { return "go" }

func (referenceProvider) Resolve(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	return goref.ResolveImport(spec, ctx.KnownDirs), true
}

func (referenceProvider) ResolveScopeTarget(ref ingest.Reference, rootDir string) (ingest.ProviderScopeTarget, bool, error) {
	if ref.Path == "" {
		return ingest.ProviderScopeTarget{}, false, nil
	}
	dir, err := goref.ResolvePackageDir(ref.Path, rootDir)
	if err != nil {
		return ingest.ProviderScopeTarget{}, true, err
	}
	return ingest.ProviderScopeTarget{Dir: dir}, true, nil
}

func (referenceProvider) ResolveSymbolTarget(ref ingest.Reference, rootDir string) (ingest.ProviderSymbolTarget, bool, error) {
	target, ok, err := goref.ResolveSymbolTarget(ref.Path, ref.Name, rootDir)
	if !ok || err != nil {
		return ingest.ProviderSymbolTarget{}, ok, err
	}
	return ingest.ProviderSymbolTarget{Dir: target.Dir, Name: target.Name}, true, nil
}

func (referenceProvider) ListScopeChildren(ref ingest.Reference, rootDir string, includeHidden bool) ([]refpkg.ScopeChild, bool, error) {
	if ref.Path == "" {
		return nil, false, nil
	}
	dir, err := goref.ResolvePackageDir(ref.Path, rootDir)
	if err != nil {
		return nil, true, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, true, err
	}

	children := make([]refpkg.ScopeChild, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if !dirHasGoSources(filepath.Join(dir, name)) {
			continue
		}
		children = append(children, refpkg.ScopeChild{
			Ref:  ingest.Reference{Provider: "go", Path: refpkg.JoinProviderPath(ref.Path, name)},
			Kind: refpkg.ScopeChildDir,
		})
	}
	refpkg.SortScopeChildrenByPath(children)
	return children, true, nil
}

func (referenceProvider) ListIngestRecursive(_ ingest.Reference, opts ingest.ListOptions) bool {
	return opts.Recursive
}

func (referenceProvider) AllowListEntity(_ ingest.Reference, _ ingest.Reference, entPath, language string, _ ingest.ListOptions) bool {
	if language != "go" {
		return false
	}
	return !strings.HasSuffix(entPath, "_test.go")
}

func (referenceProvider) ListOutputReference(ref ingest.Reference, entRef ingest.Reference) ingest.Reference {
	return ingest.Reference{Provider: ref.Provider, Path: ref.Path, Name: entRef.Name}
}

func (referenceProvider) DocIngestRecursive(ingest.Reference) bool { return false }

func (referenceProvider) AllowDocEntity(_ ingest.Reference, _ ingest.Reference, entPath, language string) bool {
	if language != "go" {
		return false
	}
	return !strings.HasSuffix(entPath, "_test.go")
}

func dirHasGoSources(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			return true
		}
	}
	return false
}

func extractGo(root *grammar.Node, source []byte, path string) *ingest.FileExtract {
	fe := &ingest.FileExtract{Language: "go", Path: path}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "package_clause":
			if id := ingest.ChildByType(child, "package_identifier"); id != nil {
				fe.Package = ingest.NodeText(id, source)
			}
		case "function_declaration", "method_declaration":
			extractGoFunc(fe, child, source)
		case "type_declaration":
			extractGoTypeDecl(fe, child, source)
		case "var_declaration":
			extractGoVarDecl(fe, child, source, "")
		case "const_declaration":
			extractGoConstDecl(fe, child, source, "")
		case "import_declaration":
			extractGoImportDecl(fe, child, source)
		}
	}

	return fe
}

func extractGoTypeDecl(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	// A type_declaration may contain a single type_spec / type_alias or
	// multiple inside a parenthesised group: type ( A struct{} ; B int ).
	// tree-sitter-go uses type_alias for `type Name = RHS` (distinct from type_spec).
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch child.Type() {
		case "type_spec":
			extractGoTypeSpec(fe, child, source)
		case "type_alias":
			extractGoTypeAlias(fe, child, source)
		}
	}
}

func extractGoTypeSpec(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByType(n, "type_identifier")
	if nameNode == nil {
		return
	}
	name := ingest.NodeText(nameNode, source)

	fe.Atoms = append(fe.Atoms, ingest.AtomDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  languageDriver{}.AllowListAtom(name, ingest.AtomListOptions{}),
	})

	// Interface methods as first-class path entities (Worker.Helper) so renames can
	// target the interface method directly; ExpandRenameSources pulls implementors.
	if iface := ingest.ChildByField(n, "type"); iface != nil && iface.Type() == "interface_type" {
		extractGoInterfaceMethods(fe, iface, source, name)
	}

	// Walk the type body so embedded fields, pointer/element types, and other
	// type_identifier sites become usages (struct { Base }, *Box, []T, …).
	// Skip the declared name itself to avoid a self-usage on the definition.
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child == nameNode {
			continue
		}
		// type_spec children: name type_identifier, then the RHS type node(s).
		if child.Type() == "type_identifier" && child.StartByte() == nameNode.StartByte() {
			continue
		}
		if child.Type() == "struct_type" {
			extractGoStructFields(fe, child, source, name)
		}
		walkGoUsages(fe, child, source, name)
	}
}

// extractGoStructFields records named struct fields as Type.Field entities so
// renames rewrite definitions and selector/literal sites (via ExtraRename).
// Embedded types (no field_identifier) are not field entities — they are type
// references already handled as usages.
func extractGoStructFields(fe *ingest.FileExtract, structType *grammar.Node, source []byte, typeName string) {
	if structType == nil || typeName == "" {
		return
	}
	list := ingest.ChildByType(structType, "field_declaration_list")
	if list == nil {
		return
	}
	for i := uint32(0); i < list.ChildCount(); i++ {
		fd := list.Child(i)
		if fd.Type() != "field_declaration" {
			continue
		}
		// Named fields only (field_identifier). Multiple names on one line are rare
		// in modern Go but field_identifier children cover them.
		for j := uint32(0); j < fd.ChildCount(); j++ {
			c := fd.Child(j)
			if c.Type() != "field_identifier" {
				continue
			}
			fieldName := ingest.NodeText(c, source)
			if fieldName == "" {
				continue
			}
			full := typeName + "." + fieldName
			fe.Atoms = append(fe.Atoms, ingest.AtomDef{
				Name:      full,
				StartByte: c.StartByte(),
				EndByte:   c.EndByte(),
				Exported:  languageDriver{}.AllowListAtom(fieldName, ingest.AtomListOptions{}),
			})
		}
	}
}

// extractGoInterfaceMethods records method_elem names as Type.Method entities.
func extractGoInterfaceMethods(fe *ingest.FileExtract, iface *grammar.Node, source []byte, typeName string) {
	if iface == nil || typeName == "" {
		return
	}
	for i := uint32(0); i < iface.ChildCount(); i++ {
		child := iface.Child(i)
		if child.Type() != "method_elem" && child.Type() != "field_declaration" {
			continue
		}
		nameNode := ingest.ChildByField(child, "name")
		if nameNode == nil {
			continue
		}
		// Skip embedded interfaces (type_identifier only, no method name field pattern).
		// method_elem always has name field; field_declaration for embedding has type without name.
		short := ingest.NodeText(nameNode, source)
		if short == "" {
			continue
		}
		methodName := typeName + "." + short
		fe.Atoms = append(fe.Atoms, ingest.AtomDef{
			Name:      methodName,
			StartByte: nameNode.StartByte(),
			EndByte:   nameNode.EndByte(),
			Exported:  languageDriver{}.AllowListAtom(short, ingest.AtomListOptions{}),
		})
	}
}

// extractGoTypeAlias handles `type Alias = RHS` (type_alias node).
func extractGoTypeAlias(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	// Children are two type_identifiers: name then RHS (may also be more complex types).
	var nameNode *grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() == "type_identifier" {
			nameNode = child
			break
		}
	}
	if nameNode == nil {
		return
	}
	name := ingest.NodeText(nameNode, source)
	fe.Atoms = append(fe.Atoms, ingest.AtomDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  languageDriver{}.AllowListAtom(name, ingest.AtomListOptions{}),
	})
	// Usages: everything after the alias name (RHS type, possibly pointer/qualified).
	// Alias-to-interface: `type Worker = interface { Helper() }` — method_elem names
	// are path entities Worker.Helper (same as type_spec interfaces).
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child == nameNode || (child.Type() == "type_identifier" && child.StartByte() == nameNode.StartByte()) {
			continue
		}
		if child.Type() == "interface_type" {
			extractGoInterfaceMethods(fe, child, source, name)
		}
		if child.Type() == "struct_type" {
			extractGoStructFields(fe, child, source, name)
		}
		walkGoUsages(fe, child, source, name)
	}
}

func extractGoFunc(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	name := ingest.NodeText(nameNode, source)
	if receiver := goMethodReceiverType(n, source); receiver != "" {
		name = receiver + "." + name
	}

	fe.Atoms = append(fe.Atoms, ingest.AtomDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  languageDriver{}.AllowListAtom(name, ingest.AtomListOptions{}),
	})

	// Signature types (params/results/receiver) carry package.Type refs.
	// Skip the function/method name itself so we don't emit a self-usage.
	for _, field := range []string{"receiver", "parameters", "result", "type_parameters"} {
		if part := ingest.ChildByField(n, field); part != nil {
			walkGoUsages(fe, part, source, name)
		}
	}
	if body := ingest.ChildByField(n, "body"); body != nil {
		walkGoUsages(fe, body, source, name)
	}
}

func extractGoVarDecl(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	extractGoVarOrConstDecl(fe, n, source, scope, "var_spec", "var_spec_list")
}

func extractGoConstDecl(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	extractGoVarOrConstDecl(fe, n, source, scope, "const_spec", "const_spec_list")
}

func extractGoVarOrConstDecl(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope, specType, listType string) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch child.Type() {
		case specType:
			extractGoVarOrConstSpec(fe, child, source, scope)
		case listType:
			for j := uint32(0); j < child.ChildCount(); j++ {
				spec := child.Child(j)
				if spec.Type() == specType {
					extractGoVarOrConstSpec(fe, spec, source, scope)
				}
			}
		}
	}
}

// extractGoVarOrConstSpec records named entities from a var_spec/const_spec and
// walks type/value as usages. tree-sitter-go uses field "name" for the identifier(s).
func extractGoVarOrConstSpec(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if nameNode := ingest.ChildByField(n, "name"); nameNode != nil {
		if nameNode.Type() == "identifier" {
			appendGoNamedEntity(fe, nameNode, source)
		} else {
			// identifier_list
			for i := uint32(0); i < nameNode.ChildCount(); i++ {
				c := nameNode.Child(i)
				if c.Type() == "identifier" {
					appendGoNamedEntity(fe, c, source)
				}
			}
		}
	}
	if typ := ingest.ChildByField(n, "type"); typ != nil {
		walkGoUsages(fe, typ, source, scope)
	}
	if val := ingest.ChildByField(n, "value"); val != nil {
		walkGoUsages(fe, val, source, scope)
	}
}

func appendGoNamedEntity(fe *ingest.FileExtract, nameNode *grammar.Node, source []byte) {
	name := ingest.NodeText(nameNode, source)
	fe.Atoms = append(fe.Atoms, ingest.AtomDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  languageDriver{}.AllowListAtom(name, ingest.AtomListOptions{}),
	})
}

func goMethodReceiverType(n *grammar.Node, source []byte) string {
	if n == nil || n.Type() != "method_declaration" {
		return ""
	}
	receiver := ingest.ChildByField(n, "receiver")
	if receiver == nil {
		return ""
	}
	for i := uint32(0); i < receiver.ChildCount(); i++ {
		child := receiver.Child(i)
		if child.Type() != "parameter_declaration" {
			continue
		}
		typ := ingest.ChildByField(child, "type")
		if typ == nil {
			continue
		}
		return strings.ReplaceAll(ingest.NodeText(typ, source), " ", "")
	}
	return ""
}

func goSymbolExportName(name string) string {
	if name == "" {
		return ""
	}
	if i := strings.LastIndex(name, "."); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return name
}

func extractGoImportDecl(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch child.Type() {
		case "import_spec":
			extractGoImportSpec(fe, child, source)
		case "import_spec_list":
			for j := uint32(0); j < child.ChildCount(); j++ {
				spec := child.Child(j)
				if spec.Type() == "import_spec" {
					extractGoImportSpec(fe, spec, source)
				}
			}
		}
	}
}

func extractGoImportSpec(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	pathNode := ingest.ChildByField(n, "path")
	if pathNode == nil {
		return
	}
	pathContent := ingest.ChildByType(pathNode, "interpreted_string_literal_content")
	if pathContent == nil {
		return
	}
	importPath := ingest.NodeText(pathContent, source)

	aliasNode := ingest.ChildByField(n, "name")
	localName := ""
	startByte := uint32(0)
	endByte := uint32(0)
	if aliasNode != nil {
		localName = ingest.NodeText(aliasNode, source)
		startByte = aliasNode.StartByte()
		endByte = aliasNode.EndByte()
	} else {
		localName = ingest.LastPathComponent(importPath)
		startByte = pathContent.StartByte()
		endByte = pathContent.EndByte()
	}

	fe.Imports = append(fe.Imports, ingest.ImportDef{
		LocalName:  localName,
		SourcePath: importPath,
		StartByte:  startByte,
		EndByte:    endByte,
	})
}

func walkGoUsages(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if n == nil || n.IsNull() {
		return
	}

	switch n.Type() {
	case "selector_expression":
		// pkg.Member, obj.Field, cobra.Command (type or value).
		operand := ingest.ChildByField(n, "operand")
		field := ingest.ChildByField(n, "field")
		if operand != nil && field != nil && operand.Type() == "identifier" {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          ingest.NodeText(field, source),
				StartByte:     field.StartByte(),
				EndByte:       field.EndByte(),
				Qualifier:     ingest.NodeText(operand, source),
				QualStartByte: operand.StartByte(),
				QualEndByte:   operand.EndByte(),
			})
			// Do not recurse into operand/field — already recorded.
			return
		}
		// Nested selectors (a.b.c): walk children normally.
	case "qualified_type":
		// Some grammars expose package.Type as qualified_type.
		pkg := ingest.ChildByField(n, "package")
		name := ingest.ChildByField(n, "name")
		if pkg == nil {
			pkg = ingest.ChildByType(n, "package_identifier")
		}
		if name == nil {
			name = ingest.ChildByType(n, "type_identifier")
		}
		if pkg != nil && name != nil {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          ingest.NodeText(name, source),
				StartByte:     name.StartByte(),
				EndByte:       name.EndByte(),
				Qualifier:     ingest.NodeText(pkg, source),
				QualStartByte: pkg.StartByte(),
				QualEndByte:   pkg.EndByte(),
			})
			return
		}
	case "type_identifier":
		// Bare type name (same-package or builtin) — treat as direct usage.
		fe.Usages = append(fe.Usages, ingest.UsageDef{
			Scope:     scope,
			Name:      ingest.NodeText(n, source),
			StartByte: n.StartByte(),
			EndByte:   n.EndByte(),
		})
		return
	case "identifier":
		// Direct identifier uses outside of selectors (var refs, func refs).
		// Definitions are walked separately; this may include some noise (range vars)
		// which resolve only when they match an entity/import.
		fe.Usages = append(fe.Usages, ingest.UsageDef{
			Scope:     scope,
			Name:      ingest.NodeText(n, source),
			StartByte: n.StartByte(),
			EndByte:   n.EndByte(),
		})
		return
	case "interpreted_string_literal", "raw_string_literal", "rune_literal",
		"int_literal", "float_literal", "imaginary_literal", "nil", "true", "false",
		"comment", "package_identifier":
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkGoUsages(fe, n.Child(i), source, scope)
	}
}
