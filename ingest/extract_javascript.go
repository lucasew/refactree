package ingest

import (
	"path"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

type javascriptLanguageDriver struct{}

func (javascriptLanguageDriver) Language() string { return "javascript" }

func (javascriptLanguageDriver) Extract(root *grammar.Node, source []byte, relPath string) *fileExtract {
	return extractJavaScript(root, source, relPath)
}

func (javascriptLanguageDriver) DirectoryEntryFile(dirRel string) string {
	return path.Join(dirRel, "index.js")
}

func (javascriptLanguageDriver) DestinationFileInDirectory(dstDirRel string, _ Reference) string {
	return path.Join(dstDirRel, "index.js")
}

// extractJavaScript walks a JS program AST and produces a fileExtract.
func extractJavaScript(root *grammar.Node, source []byte, path string) *fileExtract {
	fe := &fileExtract{
		language: "javascript",
		path:     path,
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_declaration":
			extractJSFunc(fe, child, source)
		case "export_statement":
			// May wrap a function_declaration.
			for j := uint32(0); j < child.ChildCount(); j++ {
				inner := child.Child(j)
				if inner.Type() == "function_declaration" {
					extractJSFunc(fe, inner, source)
				}
			}
		case "import_statement":
			extractJSImport(fe, child, source)
		}
	}

	return fe
}

func extractJSFunc(fe *fileExtract, n *grammar.Node, source []byte) {
	nameNode := childByField(n, "name")
	if nameNode == nil {
		return
	}
	name := nodeText(nameNode, source)

	fe.entities = append(fe.entities, entityDef{
		name:      name,
		startByte: nameNode.StartByte(),
		endByte:   nameNode.EndByte(),
		exported:  true,
	})

	if body := childByField(n, "body"); body != nil {
		walkJSUsages(fe, body, source, name)
	}
}

func extractJSImport(fe *fileExtract, n *grammar.Node, source []byte) {
	sourceNode := childByField(n, "source")
	if sourceNode == nil {
		return
	}
	fragment := childByType(sourceNode, "string_fragment")
	if fragment == nil {
		return
	}
	sourcePath := nodeText(fragment, source)

	clause := childByType(n, "import_clause")
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
			// Default import: import X from "..."
			fe.imports = append(fe.imports, importDef{
				localName:  nodeText(child, source),
				sourcePath: sourcePath,
				startByte:  child.StartByte(),
				endByte:    child.EndByte(),
			})
		}
	}
}

// extractJSNamedImports handles: import { a, b as c } from "..."
func extractJSNamedImports(fe *fileExtract, n *grammar.Node, source []byte, sourcePath string) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		spec := n.Child(i)
		if spec.Type() != "import_specifier" {
			continue
		}
		nameNode := childByField(spec, "name")
		if nameNode == nil {
			continue
		}
		memberName := nodeText(nameNode, source)

		aliasNode := childByField(spec, "alias")
		localName := memberName
		startByte := nameNode.StartByte()
		endByte := nameNode.EndByte()
		if aliasNode != nil {
			localName = nodeText(aliasNode, source)
			startByte = aliasNode.StartByte()
			endByte = aliasNode.EndByte()
		}

		fe.imports = append(fe.imports, importDef{
			localName:       localName,
			sourcePath:      sourcePath,
			memberName:      memberName,
			startByte:       startByte,
			endByte:         endByte,
			targetStartByte: nameNode.StartByte(),
			targetEndByte:   nameNode.EndByte(),
			hasAliasBinding: aliasNode != nil,
		})
	}
}

// extractJSNamespaceImport handles: import * as X from "..."
func extractJSNamespaceImport(fe *fileExtract, n *grammar.Node, source []byte, sourcePath string) {
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
	fe.imports = append(fe.imports, importDef{
		localName:  nodeText(nameNode, source),
		sourcePath: sourcePath,
		startByte:  nameNode.StartByte(),
		endByte:    nameNode.EndByte(),
	})
}

// walkJSUsages recursively finds call_expression nodes inside a JS function body.
func walkJSUsages(fe *fileExtract, n *grammar.Node, source []byte, scope string) {
	if n.Type() == "call_expression" {
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
			case "member_expression":
				obj := childByField(funcNode, "object")
				prop := childByField(funcNode, "property")
				if obj != nil && prop != nil {
					fe.usages = append(fe.usages, usageDef{
						scope:         scope,
						name:          nodeText(prop, source),
						startByte:     prop.StartByte(),
						endByte:       prop.EndByte(),
						qualifier:     nodeText(obj, source),
						qualStartByte: obj.StartByte(),
						qualEndByte:   obj.EndByte(),
					})
				}
			}
		}
		if args := childByField(n, "arguments"); args != nil {
			walkJSUsages(fe, args, source, scope)
		}
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkJSUsages(fe, n.Child(i), source, scope)
	}
}
