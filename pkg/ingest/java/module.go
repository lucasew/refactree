package java

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/java"
	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
	javaref "github.com/lucasew/refactree/pkg/reference/java"
)

func init() {
	ingest.RegisterLanguageDriver("java", languageDriver{})
	ingest.RegisterLanguageRules("java", ingest.LanguageRules{
		Extensions:      []string{".java"},
		DirectoryModule: true,
	})
	ingest.RegisterReferenceProvider("java", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "java" }

func (languageDriver) TreeSitterGrammar(filename string) (grammar.Language, bool) {
	if lang, ok := grammar.GetByExtension(filename); ok {
		return lang, true
	}
	return grammar.Get("java")
}

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	return extractJava(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	if ref, ok := (referenceProvider{}).Resolve(sourcePath, ctx); ok {
		return ref
	}
	return "java:" + sourcePath
}

func (languageDriver) AllowListSymbol(string, ingest.SymbolListOptions) bool { return true }

func (languageDriver) UseExportedFlag() bool { return true }

func (languageDriver) DestinationFileInDirectory(dstDirRel string, srcRef ingest.Reference) string {
	srcPath := strings.TrimPrefix(srcRef.Path, "./")
	base := path.Base(srcPath)
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return path.Join(dstDirRel, base)
}

type referenceProvider struct{}

func (referenceProvider) Name() string { return "java" }

func (referenceProvider) Resolve(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", false
	}
	ref := javaref.ResolveImport(spec, ctx.KnownFiles)
	if ref == "" {
		return "", false
	}
	return ref, true
}

func (referenceProvider) ResolveScopeTarget(ref ingest.Reference, rootDir string) (ingest.ProviderScopeTarget, bool, error) {
	if ref.Path == "" {
		return ingest.ProviderScopeTarget{}, false, nil
	}
	target, err := javaref.ResolveModuleTarget(ref.Path, rootDir)
	if err != nil {
		return ingest.ProviderScopeTarget{}, true, err
	}
	canDescend := target.File == ""
	return ingest.ProviderScopeTarget{Dir: target.Dir, CanDescend: &canDescend}, true, nil
}

func (referenceProvider) ResolveSymbolTarget(ref ingest.Reference, rootDir string) (ingest.ProviderSymbolTarget, bool, error) {
	target, ok, err := javaref.ResolveSymbolTarget(ref.Path, ref.Symbol, rootDir)
	if !ok || err != nil {
		return ingest.ProviderSymbolTarget{}, ok, err
	}
	return ingest.ProviderSymbolTarget{Dir: target.Dir, Symbol: target.Symbol}, true, nil
}

func (referenceProvider) ListScopeChildren(ref ingest.Reference, rootDir string, includeHidden bool) ([]refpkg.ScopeChild, bool, error) {
	if ref.Path == "" {
		return nil, false, nil
	}
	target, err := javaref.ResolveModuleTarget(ref.Path, rootDir)
	if err != nil {
		return nil, true, err
	}
	if target.File != "" {
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
		if entry.IsDir() {
			sub := filepath.Join(target.Dir, name)
			if !dirHasJavaSourcesRecursive(sub) {
				continue
			}
			children = append(children, refpkg.ScopeChild{
				Ref:  ingest.Reference{Provider: "java", Path: joinProviderPath(ref.Path, name)},
				Kind: refpkg.ScopeChildDir,
			})
			continue
		}
		if !strings.HasSuffix(name, ".java") {
			continue
		}
		typeName := strings.TrimSuffix(name, ".java")
		children = append(children, refpkg.ScopeChild{
			Ref:  ingest.Reference{Provider: "java", Path: joinProviderPath(ref.Path, typeName)},
			Kind: refpkg.ScopeChildFile,
		})
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Ref.Path < children[j].Ref.Path
	})
	return children, true, nil
}

func (referenceProvider) ListIngestRecursive(_ ingest.Reference, opts ingest.ListOptions) bool {
	return opts.Recursive
}

func (referenceProvider) AllowListEntity(_ ingest.Reference, _ ingest.Reference, entPath, language string, _ ingest.ListOptions) bool {
	return language == "java" && strings.HasSuffix(entPath, ".java")
}

func (referenceProvider) ListOutputReference(ref ingest.Reference, entRef ingest.Reference) ingest.Reference {
	return ingest.Reference{Provider: ref.Provider, Path: ref.Path, Symbol: entRef.Symbol}
}

func (referenceProvider) DocIngestRecursive(ingest.Reference) bool { return false }

func (referenceProvider) AllowDocEntity(_ ingest.Reference, _ ingest.Reference, entPath, language string) bool {
	return language == "java" && strings.HasSuffix(entPath, ".java")
}

func joinProviderPath(base, name string) string {
	base = strings.Trim(base, "/")
	name = strings.Trim(name, "/")
	base = strings.ReplaceAll(base, "/", ".")
	name = strings.ReplaceAll(name, "/", ".")
	if base == "" {
		return name
	}
	if name == "" {
		return base
	}
	return base + "." + name
}

func dirHasJavaSourcesRecursive(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if dirHasJavaSourcesRecursive(filepath.Join(dir, entry.Name())) {
				return true
			}
			continue
		}
		if strings.HasSuffix(entry.Name(), ".java") {
			return true
		}
	}
	return false
}

func extractJava(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	fe := &ingest.FileExtract{Language: "java", Path: relPath}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "package_declaration":
			if name := javaPackageNameNode(child); name != nil {
				fe.Package = ingest.NodeText(name, source)
			}
		case "import_declaration":
			extractJavaImport(fe, child, source)
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			extractJavaType(fe, child, source)
		}
	}

	return fe
}

func extractJavaImport(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	staticImport := false
	var nameNode *grammar.Node
	asterisk := false

	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch child.Type() {
		case "static":
			staticImport = true
		case "asterisk":
			asterisk = true
		case "scoped_identifier", "identifier":
			nameNode = child
		}
	}
	if nameNode == nil {
		return
	}

	full := ingest.NodeText(nameNode, source)
	if asterisk {
		// package.* — forward the package scope without a local binding name.
		fe.Reexports = append(fe.Reexports, ingest.ReexportDef{
			SourcePath: full,
			Star:       true,
		})
		return
	}

	member, pkgOrType := splitJavaImport(full)
	if member == "" {
		return
	}

	localNode := javaImportNameNode(nameNode)
	if localNode == nil {
		return
	}

	imp := ingest.ImportDef{
		LocalName:       member,
		SourcePath:      full,
		MemberName:      member,
		StartByte:       localNode.StartByte(),
		EndByte:         localNode.EndByte(),
		TargetStartByte: localNode.StartByte(),
		TargetEndByte:   localNode.EndByte(),
	}
	if staticImport {
		imp.SourcePath = pkgOrType
	}
	fe.Imports = append(fe.Imports, imp)
}

func splitJavaImport(full string) (member, prefix string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	if i := strings.LastIndex(full, "."); i >= 0 {
		return full[i+1:], full[:i]
	}
	return full, ""
}

func javaImportNameNode(n *grammar.Node) *grammar.Node {
	if n == nil {
		return nil
	}
	if n.Type() == "identifier" {
		return n
	}
	if name := ingest.ChildByField(n, "name"); name != nil {
		return name
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

func javaPackageNameNode(n *grammar.Node) *grammar.Node {
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch child.Type() {
		case "scoped_identifier", "identifier":
			return child
		}
	}
	return nil
}

func extractJavaType(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	typeName := ingest.NodeText(nameNode, source)
	exported := javaNodeIsPublic(n)

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      typeName,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  exported,
	})

	for _, field := range []string{"superclass", "interfaces", "type_parameters"} {
		if part := ingest.ChildByField(n, field); part != nil {
			walkJavaUsages(fe, part, source, typeName)
		}
	}

	body := ingest.ChildByField(n, "body")
	if body == nil {
		return
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Type() {
		case "method_declaration":
			extractJavaMethod(fe, child, source, typeName)
		case "constructor_declaration":
			extractJavaConstructor(fe, child, source, typeName)
		case "field_declaration":
			extractJavaField(fe, child, source, typeName)
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			// Nested types: qualify with outer name.
			extractJavaNestedType(fe, child, source, typeName)
		case "constant_declaration":
			extractJavaConstant(fe, child, source, typeName)
		case "enum_constant":
			extractJavaEnumConstant(fe, child, source, typeName)
		}
	}
}

func extractJavaNestedType(fe *ingest.FileExtract, n *grammar.Node, source []byte, outer string) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	short := ingest.NodeText(nameNode, source)
	full := outer + "." + short
	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      full,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  javaNodeIsPublic(n),
	})
	for _, field := range []string{"superclass", "interfaces", "type_parameters"} {
		if part := ingest.ChildByField(n, field); part != nil {
			walkJavaUsages(fe, part, source, full)
		}
	}
	if body := ingest.ChildByField(n, "body"); body != nil {
		for i := uint32(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			switch child.Type() {
			case "method_declaration":
				extractJavaMethod(fe, child, source, full)
			case "constructor_declaration":
				extractJavaConstructor(fe, child, source, full)
			case "field_declaration":
				extractJavaField(fe, child, source, full)
			}
		}
	}
}

func extractJavaMethod(fe *ingest.FileExtract, n *grammar.Node, source []byte, typeName string) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	short := ingest.NodeText(nameNode, source)
	full := typeName + "." + short
	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      full,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  javaNodeIsPublic(n),
	})
	for _, field := range []string{"type_parameters", "parameters", "type", "dimensions"} {
		if part := ingest.ChildByField(n, field); part != nil {
			walkJavaUsages(fe, part, source, full)
		}
	}
	// throws clause is not a named field consistently; walk unnamed throws children.
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() == "throws" {
			walkJavaUsages(fe, child, source, full)
		}
	}
	if body := ingest.ChildByField(n, "body"); body != nil {
		walkJavaUsages(fe, body, source, full)
	}
}

func extractJavaConstructor(fe *ingest.FileExtract, n *grammar.Node, source []byte, typeName string) {
	scope := typeName
	if params := ingest.ChildByField(n, "parameters"); params != nil {
		walkJavaUsages(fe, params, source, scope)
	}
	if body := ingest.ChildByField(n, "body"); body != nil {
		walkJavaUsages(fe, body, source, scope)
	}
}

func extractJavaField(fe *ingest.FileExtract, n *grammar.Node, source []byte, typeName string) {
	exported := javaNodeIsPublic(n)
	if typ := ingest.ChildByField(n, "type"); typ != nil {
		walkJavaUsages(fe, typ, source, typeName)
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() != "variable_declarator" {
			continue
		}
		nameNode := ingest.ChildByField(child, "name")
		if nameNode == nil {
			continue
		}
		short := ingest.NodeText(nameNode, source)
		fe.Entities = append(fe.Entities, ingest.EntityDef{
			Name:      typeName + "." + short,
			StartByte: nameNode.StartByte(),
			EndByte:   nameNode.EndByte(),
			Exported:  exported,
		})
		if val := ingest.ChildByField(child, "value"); val != nil {
			walkJavaUsages(fe, val, source, typeName)
		}
	}
}

func extractJavaConstant(fe *ingest.FileExtract, n *grammar.Node, source []byte, typeName string) {
	exported := javaNodeIsPublic(n)
	if typ := ingest.ChildByField(n, "type"); typ != nil {
		walkJavaUsages(fe, typ, source, typeName)
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() != "variable_declarator" {
			continue
		}
		nameNode := ingest.ChildByField(child, "name")
		if nameNode == nil {
			continue
		}
		short := ingest.NodeText(nameNode, source)
		fe.Entities = append(fe.Entities, ingest.EntityDef{
			Name:      typeName + "." + short,
			StartByte: nameNode.StartByte(),
			EndByte:   nameNode.EndByte(),
			Exported:  exported,
		})
		if val := ingest.ChildByField(child, "value"); val != nil {
			walkJavaUsages(fe, val, source, typeName)
		}
	}
}

func extractJavaEnumConstant(fe *ingest.FileExtract, n *grammar.Node, source []byte, typeName string) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		for i := uint32(0); i < n.ChildCount(); i++ {
			if n.Child(i).Type() == "identifier" {
				nameNode = n.Child(i)
				break
			}
		}
	}
	if nameNode == nil {
		return
	}
	short := ingest.NodeText(nameNode, source)
	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      typeName + "." + short,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  true,
	})
	if args := ingest.ChildByField(n, "arguments"); args != nil {
		walkJavaUsages(fe, args, source, typeName)
	}
}

func javaNodeIsPublic(n *grammar.Node) bool {
	if n == nil {
		return false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() != "modifiers" {
			continue
		}
		for j := uint32(0); j < child.ChildCount(); j++ {
			if child.Child(j).Type() == "public" {
				return true
			}
		}
	}
	return false
}

func walkJavaUsages(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if n == nil || n.IsNull() {
		return
	}

	switch n.Type() {
	case "method_invocation":
		obj := ingest.ChildByField(n, "object")
		name := ingest.ChildByField(n, "name")
		if obj != nil && name != nil && obj.Type() == "identifier" {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          ingest.NodeText(name, source),
				StartByte:     name.StartByte(),
				EndByte:       name.EndByte(),
				Qualifier:     ingest.NodeText(obj, source),
				QualStartByte: obj.StartByte(),
				QualEndByte:   obj.EndByte(),
			})
			if args := ingest.ChildByField(n, "arguments"); args != nil {
				walkJavaUsages(fe, args, source, scope)
			}
			if targs := ingest.ChildByField(n, "type_arguments"); targs != nil {
				walkJavaUsages(fe, targs, source, scope)
			}
			return
		}
	case "field_access":
		obj := ingest.ChildByField(n, "object")
		field := ingest.ChildByField(n, "field")
		if field == nil {
			field = ingest.ChildByType(n, "identifier")
		}
		if obj != nil && field != nil && obj.Type() == "identifier" {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          ingest.NodeText(field, source),
				StartByte:     field.StartByte(),
				EndByte:       field.EndByte(),
				Qualifier:     ingest.NodeText(obj, source),
				QualStartByte: obj.StartByte(),
				QualEndByte:   obj.EndByte(),
			})
			return
		}
	case "object_creation_expression":
		if typ := ingest.ChildByField(n, "type"); typ != nil {
			walkJavaUsages(fe, typ, source, scope)
		}
		if args := ingest.ChildByField(n, "arguments"); args != nil {
			walkJavaUsages(fe, args, source, scope)
		}
		return
	case "scoped_type_identifier", "scoped_identifier":
		scopeNode := ingest.ChildByField(n, "scope")
		nameNode := ingest.ChildByField(n, "name")
		if scopeNode != nil && nameNode != nil && scopeNode.Type() == "identifier" {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          ingest.NodeText(nameNode, source),
				StartByte:     nameNode.StartByte(),
				EndByte:       nameNode.EndByte(),
				Qualifier:     ingest.NodeText(scopeNode, source),
				QualStartByte: scopeNode.StartByte(),
				QualEndByte:   scopeNode.EndByte(),
			})
			return
		}
		if scopeNode != nil {
			walkJavaUsages(fe, scopeNode, source, scope)
		}
		if nameNode != nil {
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:     scope,
				Name:      ingest.NodeText(nameNode, source),
				StartByte: nameNode.StartByte(),
				EndByte:   nameNode.EndByte(),
			})
		}
		return
	case "identifier", "type_identifier":
		fe.Usages = append(fe.Usages, ingest.UsageDef{
			Scope:     scope,
			Name:      ingest.NodeText(n, source),
			StartByte: n.StartByte(),
			EndByte:   n.EndByte(),
		})
		return
	case "string_literal", "character_literal", "decimal_integer_literal", "hex_integer_literal",
		"octal_integer_literal", "binary_integer_literal", "decimal_floating_point_literal",
		"true", "false", "null_literal", "line_comment", "block_comment", "modifiers":
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkJavaUsages(fe, n.Child(i), source, scope)
	}
}
