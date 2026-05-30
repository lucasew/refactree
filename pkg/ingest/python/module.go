package python

import (
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/python"
	"github.com/lucasew/refactree/pkg/ingest"
)

func init() {
	ingest.RegisterLanguageDriver("python", languageDriver{})
	ingest.RegisterLanguageRules("python", ingest.LanguageRules{
		Extensions:      []string{".py"},
		DirectoryModule: false,
	})
	ingest.RegisterReferenceProvider("python", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "python" }

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

type referenceProvider struct{}

func (referenceProvider) Name() string { return "python" }

func (referenceProvider) Resolve(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if ctx.KnownFiles[spec+".py"] {
		return ingest.FileRef("./" + spec + ".py"), true
	}
	if ctx.KnownFiles[spec+"/__init__.py"] {
		return ingest.FileRef("./" + spec), true
	}
	return "python:" + spec, true
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
		}
	}

	return fe
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

	if body := ingest.ChildByField(n, "body"); body != nil {
		for i := uint32(0); i < body.ChildCount(); i++ {
			child := body.Child(i)
			if child.Type() != "function_definition" {
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

			if methodBody := ingest.ChildByField(child, "body"); methodBody != nil {
				walkPythonUsages(fe, methodBody, source, methodName)
			}
		}

		walkPythonUsages(fe, body, source, className)
	}
}

func extractPythonImportFrom(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	modNode := ingest.ChildByField(n, "module_name")
	if modNode == nil {
		return
	}
	moduleName := ingest.NodeText(modNode, source)

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
			if nameNode.Type() == "dotted_name" {
				if id := ingest.ChildByType(nameNode, "identifier"); id != nil {
					moduleName = ingest.NodeText(id, source)
				}
			}

			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:       ingest.NodeText(aliasNode, source),
				SourcePath:      moduleName,
				StartByte:       aliasNode.StartByte(),
				EndByte:         aliasNode.EndByte(),
				HasAliasBinding: true,
			})

		case "dotted_name":
			if id := ingest.ChildByType(child, "identifier"); id != nil {
				name := ingest.NodeText(id, source)
				fe.Imports = append(fe.Imports, ingest.ImportDef{
					LocalName:  name,
					SourcePath: name,
					StartByte:  id.StartByte(),
					EndByte:    id.EndByte(),
				})
			}
		}
	}
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
	if n.Type() == "call" {
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
			case "attribute":
				obj := ingest.ChildByField(funcNode, "object")
				attr := ingest.ChildByField(funcNode, "attribute")
				if obj != nil && attr != nil {
					fe.Usages = append(fe.Usages, ingest.UsageDef{
						Scope:         scope,
						Name:          ingest.NodeText(attr, source),
						StartByte:     attr.StartByte(),
						EndByte:       attr.EndByte(),
						Qualifier:     ingest.NodeText(obj, source),
						QualStartByte: obj.StartByte(),
						QualEndByte:   obj.EndByte(),
					})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walkPythonUsages(fe, n.Child(i), source, scope)
		}
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
