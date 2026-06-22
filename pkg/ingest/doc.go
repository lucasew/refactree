package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

// DocResult holds documentation extracted for a symbol.
type DocResult struct {
	Name      string
	Signature string
	DocString string
}

// DocFor ingests dir, locates the given reference, and extracts its
// signature and docstring from the source file.
func DocFor(dir, reference string) (*DocResult, error) {
	rawRef := ParseReference(reference)
	if rawRef.Provider != "" && rawRef.Provider != "path" {
		if rawRef.Symbol != "" {
			target, ok, err := resolveProviderSymbolTarget(rawRef)
			if err != nil {
				return nil, err
			}
			if ok {
				return docForProviderSymbol(rawRef, target)
			}
		}
		return nil, fmt.Errorf("provider doc reference requires symbol (::name): %s", reference)
	}

	result, err := Ingest(dir)
	if err != nil {
		return nil, err
	}

	ref, err := canonicalSourceReference(dir, result, ParseReference(reference))
	if err != nil {
		return nil, err
	}
	reference = ref.String()

	var entity *Entity
	for i := range result.Entities {
		if result.Entities[i].Reference == reference {
			entity = &result.Entities[i]
			break
		}
	}
	if entity == nil {
		return nil, fmt.Errorf("entity not found: %s", reference)
	}

	return docForEntity(dir, result, ref, entity)
}

func docForProviderSymbol(ref Reference, target ProviderSymbolTarget) (*DocResult, error) {
	result, err := IngestWithRecursion(target.Dir, providerDocIngestRecursive(ref))
	if err != nil {
		return nil, err
	}

	langOf := map[string]string{}
	for _, f := range result.Files {
		langOf[f.Path] = f.Language
	}

	symbol := target.Symbol
	if symbol == "" {
		symbol = ref.Symbol
	}

	matches := make([]int, 0, 1)
	for i := range result.Entities {
		entityRef := ParseReference(result.Entities[i].Reference)
		entPath := strings.TrimPrefix(entityRef.Path, "./")
		if !providerAllowDocEntity(ref, entityRef, entPath, langOf[entPath]) {
			continue
		}
		if entityRef.Symbol == symbol {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("entity not found: %s", ref.String())
	}
	if len(matches) > 1 {
		candidates := make([]string, 0, len(matches))
		for _, idx := range matches {
			candidates = append(candidates, result.Entities[idx].Reference)
		}
		return nil, fmt.Errorf("ambiguous symbol %q in %q package %q, matches: %s", symbol, ref.Provider, ref.Path, strings.Join(candidates, ", "))
	}

	entity := &result.Entities[matches[0]]
	entityRef := ParseReference(entity.Reference)
	return docForEntity(target.Dir, result, entityRef, entity)
}

func docForEntity(dir string, result *Result, ref Reference, entity *Entity) (*DocResult, error) {
	relPath := strings.TrimPrefix(ref.Path, "./")
	filePath := filepath.Join(dir, relPath)

	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var language string
	for _, f := range result.Files {
		if f.Path == relPath {
			language = f.Language
			break
		}
	}

	driver, ok := languageDriverForFile(filePath)
	if !ok {
		return nil, fmt.Errorf("unsupported language for %s", filePath)
	}
	lang, ok := driver.TreeSitterGrammar(filePath)
	if !ok {
		return nil, fmt.Errorf("unsupported language for %s", filePath)
	}

	parser := grammar.NewParser()
	defer parser.Delete()
	parser.SetLanguage(lang)

	tree := parser.ParseString(string(source))
	defer tree.Delete()

	root := tree.RootNode()

	return extractDocFromAST(root, source, entity.StartByte, ref.Symbol, language)
}

func extractDocFromAST(root *grammar.Node, source []byte, nameStart uint32, name, language string) (*DocResult, error) {
	declNode := findDeclContaining(root, nameStart, language)
	if declNode == nil {
		return &DocResult{Name: name}, nil
	}

	sig := extractSignature(declNode, source, language)
	doc := extractDocstring(root, declNode, source, language)

	return &DocResult{
		Name:      name,
		Signature: sig,
		DocString: doc,
	}, nil
}

// findDeclContaining returns the top-level declaration node whose name starts
// at nameStart.
func findDeclContaining(root *grammar.Node, nameStart uint32, language string) *grammar.Node {
	declTypes := map[string]bool{}
	switch language {
	case "go":
		declTypes["function_declaration"] = true
		declTypes["method_declaration"] = true
	case "python":
		declTypes["function_definition"] = true
		declTypes["class_definition"] = true
	case "javascript":
		declTypes["function_declaration"] = true
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if declTypes[child.Type()] {
			if n := childByField(child, "name"); n != nil && n.StartByte() == nameStart {
				return child
			}
		}
		// JS export wrapping
		if child.Type() == "export_statement" {
			for j := uint32(0); j < child.ChildCount(); j++ {
				inner := child.Child(j)
				if declTypes[inner.Type()] {
					if n := childByField(inner, "name"); n != nil && n.StartByte() == nameStart {
						return inner
					}
				}
			}
		}
	}
	return nil
}

// extractSignature returns a one-line function signature.
func extractSignature(funcNode *grammar.Node, source []byte, language string) string {
	bodyNode := childByField(funcNode, "body")
	if bodyNode == nil {
		return ""
	}
	// Signature is everything from the function node start up to the body start.
	start := funcNode.StartByte()
	end := bodyNode.StartByte()
	if int(end) > len(source) {
		return ""
	}
	sig := strings.TrimSpace(string(source[start:end]))
	// Trim trailing colon for Python.
	sig = strings.TrimSuffix(sig, ":")
	return strings.TrimSpace(sig)
}

// extractDocstring tries to find a docstring/comment attached to the function.
func extractDocstring(root *grammar.Node, funcNode *grammar.Node, source []byte, language string) string {
	switch language {
	case "python":
		return extractPythonDocstring(funcNode, source)
	default:
		return extractCommentBefore(root, funcNode, source, language)
	}
}

// extractPythonDocstring looks for a string expression as the first
// statement in the function body.
func extractPythonDocstring(funcNode *grammar.Node, source []byte) string {
	body := childByField(funcNode, "body")
	if body == nil {
		return ""
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child.Type() == "string" || child.Type() == "concatenated_string" {
			return strings.Trim(nodeText(child, source), "\"'")
		}
		if child.Type() == "expression_statement" {
			if child.NamedChildCount() > 0 {
				expr := child.NamedChild(0)
				if expr.Type() == "string" || expr.Type() == "concatenated_string" {
					return strings.Trim(nodeText(expr, source), "\"'")
				}
			}
		}
		// Only the very first non-whitespace statement counts.
		if child.IsNamed() {
			break
		}
	}
	return ""
}

// extractCommentBefore scans backward from the function node to collect
// adjacent comment lines.
func extractCommentBefore(root *grammar.Node, funcNode *grammar.Node, source []byte, language string) string {
	funcStart := funcNode.StartByte()
	// Walk backward through the source to find comment lines.
	pos := int(funcStart) - 1
	for pos >= 0 && (source[pos] == ' ' || source[pos] == '\t' || source[pos] == '\n' || source[pos] == '\r') {
		pos--
	}
	if pos < 0 {
		return ""
	}

	// Find the line containing pos.
	lineEnd := pos + 1
	lineStart := pos
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}

	var lines []string
	for {
		line := strings.TrimSpace(string(source[lineStart:lineEnd]))
		isComment := false
		switch language {
		case "go":
			isComment = strings.HasPrefix(line, "//")
		case "javascript":
			isComment = strings.HasPrefix(line, "//") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*/")
		}
		if !isComment {
			break
		}
		// Strip comment prefix.
		cleaned := line
		cleaned = strings.TrimPrefix(cleaned, "//")
		cleaned = strings.TrimPrefix(cleaned, "/*")
		cleaned = strings.TrimSuffix(cleaned, "*/")
		cleaned = strings.TrimPrefix(cleaned, "*")
		cleaned = strings.TrimSpace(cleaned)
		lines = append([]string{cleaned}, lines...)

		// Move to previous line.
		lineEnd = lineStart
		if lineEnd <= 1 {
			break
		}
		lineEnd-- // skip the \n
		lineStart = lineEnd
		for lineStart > 0 && source[lineStart-1] != '\n' {
			lineStart--
		}
	}

	return strings.Join(lines, "\n")
}
