package ingest

import (
	"path"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

type pythonLanguageDriver struct{}

func (pythonLanguageDriver) Language() string { return "python" }

func (pythonLanguageDriver) Extract(root *grammar.Node, source []byte, relPath string) *fileExtract {
	return extractPython(root, source, relPath)
}

func (pythonLanguageDriver) ResolveImport(sourcePath string, ctx ImportResolveContext) string {
	if p, ok := referenceProviderForName("python"); ok {
		if ref, ok := p.Resolve(sourcePath, ctx); ok {
			return ref
		}
	}
	return "python:" + sourcePath
}

func (pythonLanguageDriver) DirectoryEntryFile(dirRel string) string {
	return path.Join(dirRel, "__init__.py")
}

func (pythonLanguageDriver) DestinationFileInDirectory(dstDirRel string, _ Reference) string {
	return path.Join(dstDirRel, "__init__.py")
}

// extractPython walks a Python module AST and produces a fileExtract.
func extractPython(root *grammar.Node, source []byte, path string) *fileExtract {
	fe := &fileExtract{
		language: "python",
		path:     path,
	}

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

func extractPythonFunc(fe *fileExtract, n *grammar.Node, source []byte) {
	nameNode := childByField(n, "name")
	if nameNode == nil {
		return
	}
	name := nodeText(nameNode, source)

	fe.entities = append(fe.entities, entityDef{
		name:      name,
		startByte: nameNode.StartByte(),
		endByte:   nameNode.EndByte(),
		exported:  len(name) == 0 || name[0] != '_',
	})

	if body := childByField(n, "body"); body != nil {
		walkPythonUsages(fe, body, source, name)
	}
}

func extractPythonClass(fe *fileExtract, n *grammar.Node, source []byte) {
	nameNode := childByField(n, "name")
	if nameNode == nil {
		return
	}
	name := nodeText(nameNode, source)

	fe.entities = append(fe.entities, entityDef{
		name:      name,
		startByte: nameNode.StartByte(),
		endByte:   nameNode.EndByte(),
		exported:  len(name) == 0 || name[0] != '_',
	})

	if body := childByField(n, "body"); body != nil {
		walkPythonUsages(fe, body, source, name)
	}
}

// extractPythonImportFrom handles: from X import Y [, Z]
func extractPythonImportFrom(fe *fileExtract, n *grammar.Node, source []byte) {
	modNode := childByField(n, "module_name")
	if modNode == nil {
		return
	}
	moduleName := nodeText(modNode, source)

	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.FieldNameForChild(i) != "name" {
			continue
		}
		child := n.Child(i)

		switch child.Type() {
		case "aliased_import":
			nameNode := childByField(child, "name")
			aliasNode := childByField(child, "alias")
			importedNode := pythonImportNameNode(nameNode)
			if importedNode == nil || aliasNode == nil {
				continue
			}
			importedName := nodeText(importedNode, source)

			fe.imports = append(fe.imports, importDef{
				localName:       nodeText(aliasNode, source),
				sourcePath:      moduleName,
				memberName:      importedName,
				startByte:       aliasNode.StartByte(),
				endByte:         aliasNode.EndByte(),
				targetStartByte: importedNode.StartByte(),
				targetEndByte:   importedNode.EndByte(),
				hasAliasBinding: true,
			})
		default:
			importedNode := pythonImportNameNode(child)
			if importedNode == nil {
				continue
			}
			importedName := nodeText(importedNode, source)

			fe.imports = append(fe.imports, importDef{
				localName:       importedName,
				sourcePath:      moduleName,
				memberName:      importedName,
				startByte:       importedNode.StartByte(),
				endByte:         importedNode.EndByte(),
				targetStartByte: importedNode.StartByte(),
				targetEndByte:   importedNode.EndByte(),
			})
		}
	}
}

// extractPythonImport handles: import X / import X as Y
func extractPythonImport(fe *fileExtract, n *grammar.Node, source []byte) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.FieldNameForChild(i) != "name" {
			continue
		}
		child := n.Child(i)

		switch child.Type() {
		case "aliased_import":
			nameNode := childByField(child, "name")
			aliasNode := childByField(child, "alias")
			if nameNode == nil || aliasNode == nil {
				continue
			}
			moduleName := nodeText(nameNode, source)
			if nameNode.Type() == "dotted_name" {
				if id := childByType(nameNode, "identifier"); id != nil {
					moduleName = nodeText(id, source)
				}
			}

			fe.imports = append(fe.imports, importDef{
				localName:       nodeText(aliasNode, source),
				sourcePath:      moduleName,
				startByte:       aliasNode.StartByte(),
				endByte:         aliasNode.EndByte(),
				hasAliasBinding: true,
			})

		case "dotted_name":
			if id := childByType(child, "identifier"); id != nil {
				name := nodeText(id, source)
				fe.imports = append(fe.imports, importDef{
					localName:  name,
					sourcePath: name,
					startByte:  id.StartByte(),
					endByte:    id.EndByte(),
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

// walkPythonUsages recursively finds call nodes inside a Python function body.
func walkPythonUsages(fe *fileExtract, n *grammar.Node, source []byte, scope string) {
	if n.Type() == "call" {
		funcNode := childByField(n, "function")
		if funcNode != nil {
			switch funcNode.Type() {
			case "identifier":
				fe.usages = append(fe.usages, usageDef{
					scope:     scope,
					name:      nodeText(funcNode, source),
					startByte: funcNode.StartByte(),
					endByte:   funcNode.EndByte(),
				})
			case "attribute":
				obj := childByField(funcNode, "object")
				attr := childByField(funcNode, "attribute")
				if obj != nil && attr != nil {
					fe.usages = append(fe.usages, usageDef{
						scope:         scope,
						name:          nodeText(attr, source),
						startByte:     attr.StartByte(),
						endByte:       attr.EndByte(),
						qualifier:     nodeText(obj, source),
						qualStartByte: obj.StartByte(),
						qualEndByte:   obj.EndByte(),
					})
				}
			}
		}
		if args := childByField(n, "arguments"); args != nil {
			walkPythonUsages(fe, args, source, scope)
		}
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkPythonUsages(fe, n.Child(i), source, scope)
	}
}
