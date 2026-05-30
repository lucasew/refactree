package ingest

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

type goLanguageDriver struct{}

func (goLanguageDriver) Language() string { return "go" }

func (goLanguageDriver) Extract(root *grammar.Node, source []byte, relPath string) *fileExtract {
	return extractGo(root, source, relPath)
}

func (goLanguageDriver) ResolveImport(sourcePath string, ctx ImportResolveContext) string {
	if p, ok := referenceProviderForName("go"); ok {
		if ref, ok := p.Resolve(sourcePath, ctx); ok {
			return ref
		}
	}
	return "go:" + lastPathComponent(sourcePath)
}

func (goLanguageDriver) AllowListSymbol(name string, opts SymbolListOptions) bool {
	if opts.IncludeHidden {
		return true
	}
	if name == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func (goLanguageDriver) DirectoryEntryFile(string) string { return "" }

func (goLanguageDriver) DestinationFileInDirectory(dstDirRel string, srcRef Reference) string {
	srcPath := strings.TrimPrefix(srcRef.Path, "./")
	return path.Join(dstDirRel, path.Base(srcPath))
}

// extractGo walks a Go source_file AST and produces a fileExtract.
func extractGo(root *grammar.Node, source []byte, path string) *fileExtract {
	fe := &fileExtract{
		language: "go",
		path:     path,
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "package_clause":
			if id := childByType(child, "package_identifier"); id != nil {
				fe.pkg = nodeText(id, source)
			}
		case "function_declaration":
			extractGoFunc(fe, child, source)
		case "method_declaration":
			extractGoFunc(fe, child, source)
		case "import_declaration":
			extractGoImportDecl(fe, child, source)
		}
	}

	return fe
}

// extractGoFunc extracts an entity from a function/method declaration
// and walks the body for call-site usages.
func extractGoFunc(fe *fileExtract, n *grammar.Node, source []byte) {
	nameNode := childByField(n, "name")
	if nameNode == nil {
		return
	}
	name := nodeText(nameNode, source)

	fe.entities = append(fe.entities, entityDef{
		name:      name,
		startByte: nameNode.StartByte(),
		endByte:   nameNode.EndByte(),
		exported:  len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z',
	})

	if body := childByField(n, "body"); body != nil {
		walkGoUsages(fe, body, source, name)
	}
}

// extractGoImportDecl handles both single and grouped import declarations.
func extractGoImportDecl(fe *fileExtract, n *grammar.Node, source []byte) {
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

// extractGoImportSpec processes a single import_spec node.
func extractGoImportSpec(fe *fileExtract, n *grammar.Node, source []byte) {
	pathNode := childByField(n, "path")
	if pathNode == nil {
		return
	}
	pathContent := childByType(pathNode, "interpreted_string_literal_content")
	if pathContent == nil {
		return
	}
	importPath := nodeText(pathContent, source)

	aliasNode := childByField(n, "name")

	var localName string
	var startByte, endByte uint32

	if aliasNode != nil {
		localName = nodeText(aliasNode, source)
		startByte = aliasNode.StartByte()
		endByte = aliasNode.EndByte()
	} else {
		localName = lastPathComponent(importPath)
		startByte = pathContent.StartByte()
		endByte = pathContent.EndByte()
	}

	fe.imports = append(fe.imports, importDef{
		localName:  localName,
		sourcePath: importPath,
		startByte:  startByte,
		endByte:    endByte,
	})
}

// walkGoUsages recursively finds call_expression nodes inside a Go function body.
func walkGoUsages(fe *fileExtract, n *grammar.Node, source []byte, scope string) {
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
			case "selector_expression":
				operand := childByField(funcNode, "operand")
				field := childByField(funcNode, "field")
				if operand != nil && field != nil {
					fe.usages = append(fe.usages, usageDef{
						scope:         scope,
						name:          nodeText(field, source),
						startByte:     field.StartByte(),
						endByte:       field.EndByte(),
						qualifier:     nodeText(operand, source),
						qualStartByte: operand.StartByte(),
						qualEndByte:   operand.EndByte(),
					})
				}
			}
		}
		// Recurse into arguments for nested calls.
		if args := childByField(n, "arguments"); args != nil {
			walkGoUsages(fe, args, source, scope)
		}
		return
	}

	for i := uint32(0); i < n.ChildCount(); i++ {
		walkGoUsages(fe, n.Child(i), source, scope)
	}
}
