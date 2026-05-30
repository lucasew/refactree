package ingestgo

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/go"
	"github.com/lucasew/refactree/pkg/ingest"
	goref "github.com/lucasew/refactree/pkg/reference/go"
)

func init() {
	ingest.RegisterLanguageDriver("go", languageDriver{})
	ingest.RegisterLanguageRules("go", ingest.LanguageRules{
		Extensions:      []string{".go"},
		DirectoryModule: true,
	})
	ingest.RegisterReferenceProvider("go", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "go" }

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	return extractGo(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	return goref.ResolveImport(sourcePath, ctx.KnownDirs)
}

func (languageDriver) AllowListSymbol(name string, opts ingest.SymbolListOptions) bool {
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

func (referenceProvider) ResolveScopeTarget(ref ingest.Reference) (ingest.ProviderScopeTarget, bool, error) {
	if ref.Path == "" {
		return ingest.ProviderScopeTarget{}, false, nil
	}
	dir, err := goref.ResolvePackageDir(ref.Path)
	if err != nil {
		return ingest.ProviderScopeTarget{}, true, err
	}
	return ingest.ProviderScopeTarget{Dir: dir}, true, nil
}

func (referenceProvider) ResolveSymbolTarget(ref ingest.Reference) (ingest.ProviderSymbolTarget, bool, error) {
	target, ok, err := goref.ResolveSymbolTarget(ref.Path, ref.Symbol)
	if !ok || err != nil {
		return ingest.ProviderSymbolTarget{}, ok, err
	}
	return ingest.ProviderSymbolTarget{Dir: target.Dir, Symbol: target.Symbol}, true, nil
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
	return ingest.Reference{Provider: ref.Provider, Path: ref.Path, Symbol: entRef.Symbol}
}

func (referenceProvider) DocIngestRecursive(ingest.Reference) bool { return false }

func (referenceProvider) AllowDocEntity(_ ingest.Reference, _ ingest.Reference, entPath, language string) bool {
	if language != "go" {
		return false
	}
	return !strings.HasSuffix(entPath, "_test.go")
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
		case "import_declaration":
			extractGoImportDecl(fe, child, source)
		}
	}

	return fe
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

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  languageDriver{}.AllowListSymbol(name, ingest.SymbolListOptions{}),
	})

	if body := ingest.ChildByField(n, "body"); body != nil {
		walkGoUsages(fe, body, source, name)
	}
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
		localName = lastPathComponent(importPath)
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
			case "selector_expression":
				operand := ingest.ChildByField(funcNode, "operand")
				field := ingest.ChildByField(funcNode, "field")
				if operand != nil && field != nil {
					fe.Usages = append(fe.Usages, ingest.UsageDef{
						Scope:         scope,
						Name:          ingest.NodeText(field, source),
						StartByte:     field.StartByte(),
						EndByte:       field.EndByte(),
						Qualifier:     ingest.NodeText(operand, source),
						QualStartByte: operand.StartByte(),
						QualEndByte:   operand.EndByte(),
					})
				}
			}
		}
		if args := ingest.ChildByField(n, "arguments"); args != nil {
			walkGoUsages(fe, args, source, scope)
		}
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkGoUsages(fe, n.Child(i), source, scope)
	}
}

func lastPathComponent(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
