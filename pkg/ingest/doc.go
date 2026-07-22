package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// DocResult holds documentation extracted for a symbol.
type DocResult struct {
	Name      string
	Signature string
	DocString string
}

// DocFor locates the given reference and extracts its signature and docstring.
// Path docs materialize only the target file (Seed) or package directory (Dir),
// not the whole project. Provider docs use a scoped DirResult.
func DocFor(dir, reference string) (*DocResult, error) {
	rawRef := ParseReference(reference)
	if rawRef.Provider != "" && rawRef.Provider != "path" {
		if rawRef.Name != "" {
			target, ok, err := NewResolver("").ResolveSymbolTarget(rawRef)
			if err != nil {
				return nil, err
			}
			if ok {
				return docForProviderSymbol(rawRef, target)
			}
		}
		return nil, fmt.Errorf("provider doc reference requires symbol (::name): %s", reference)
	}

	rawRef = normalizePathReference(rawRef)
	absPath := rawRef.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(dir, strings.TrimPrefix(rawRef.Path, "./"))
	}

	var result *Result
	st, err := os.Stat(absPath)
	if err != nil {
		// Fall back to hop canonicalize + seed (path may need directory-module rewrite).
		canon := CanonicalizeReference(dir, rawRef)
		cAbs := canon.Path
		if !filepath.IsAbs(cAbs) {
			cAbs = filepath.Join(dir, strings.TrimPrefix(canon.Path, "./"))
		}
		result, err = SeedResult(dir, cAbs)
		if err != nil {
			return nil, err
		}
		return docForResultEntity(dir, result, canon)
	}

	if st.IsDir() {
		// Package-style dir: walk only that subtree under project Root (paths stay project-relative).
		// ExpandImports off — docs do not need a full project graph.
		result, err = MaterializeSource(ExtractSource{
			Kind:      ExtractDir,
			Root:      dir,
			Dir:       absPath,
			Recursive: true,
		}, MaterializeOptions{ExpandImports: false})
		if err != nil {
			return nil, err
		}
		ref, err := canonicalSourceReference(dir, result, rawRef)
		if err != nil {
			return nil, err
		}
		return docForResultEntity(dir, result, ref)
	}

	// File path: hop barrels then seed the concrete file neighborhood.
	canon := CanonicalizeReference(dir, rawRef)
	cAbs := canon.Path
	if !filepath.IsAbs(cAbs) {
		cAbs = filepath.Join(dir, strings.TrimPrefix(canon.Path, "./"))
	}
	result, err = SeedResult(dir, cAbs)
	if err != nil {
		return nil, err
	}
	return docForResultEntity(dir, result, canon)
}

func docForResultEntity(dir string, result *Result, ref Reference) (*DocResult, error) {
	reference := ref.String()
	var entity *Atom
	for i := range result.Atoms {
		if result.Atoms[i].Reference == reference {
			entity = &result.Atoms[i]
			break
		}
	}
	if entity == nil && ref.Name != "" {
		for i := range result.Atoms {
			er := ParseReference(result.Atoms[i].Reference)
			if er.Name == ref.Name && sameScopePath(ref, er) {
				entity = &result.Atoms[i]
				ref = er
				break
			}
		}
	}
	if entity == nil {
		return nil, fmt.Errorf("entity not found: %s", reference)
	}
	return docForEntity(dir, result, ref, entity)
}

func docForProviderSymbol(ref Reference, target ProviderSymbolTarget) (*DocResult, error) {
	result, err := DirResult(target.Dir, providerDocIngestRecursive(ref))
	if err != nil {
		return nil, err
	}

	langOf := map[string]string{}
	for _, f := range result.Files {
		langOf[f.Path] = f.Language
	}

	symbol := target.Name
	if symbol == "" {
		symbol = ref.Name
	}

	matches := make([]int, 0, 1)
	for i := range result.Atoms {
		entityRef := ParseReference(result.Atoms[i].Reference)
		entPath := strings.TrimPrefix(entityRef.Path, "./")
		if !providerAllowDocEntity(ref, entityRef, entPath, langOf[entPath]) {
			continue
		}
		if entityRef.Name == symbol {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("entity not found: %s", ref.String())
	}
	if len(matches) > 1 {
		candidates := make([]string, 0, len(matches))
		for _, idx := range matches {
			candidates = append(candidates, result.Atoms[idx].Reference)
		}
		return nil, fmt.Errorf("ambiguous symbol %q in %q package %q, matches: %s", symbol, ref.Provider, ref.Path, strings.Join(candidates, ", "))
	}

	entity := &result.Atoms[matches[0]]
	entityRef := ParseReference(entity.Reference)
	return docForEntity(target.Dir, result, entityRef, entity)
}

func docForEntity(dir string, result *Result, ref Reference, entity *Atom) (*DocResult, error) {
	relPath := strings.TrimPrefix(ref.Path, "./")
	filePath := filepath.Join(dir, relPath)

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

	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	pf, err := ParseSourceLanguage(source, lang, filePath)
	if err != nil {
		return nil, err
	}
	defer pf.Close()

	return extractDocFromAST(pf.Root, pf.Source, entity.StartByte, ref.Name, language)
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
	case "java":
		declTypes["class_declaration"] = true
		declTypes["interface_declaration"] = true
		declTypes["enum_declaration"] = true
		declTypes["record_declaration"] = true
		declTypes["annotation_type_declaration"] = true
		declTypes["method_declaration"] = true
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if declTypes[child.Type()] {
			if n := ChildByField(child, "name"); n != nil && n.StartByte() == nameStart {
				return child
			}
		}
		// JS export wrapping
		if child.Type() == "export_statement" {
			for j := uint32(0); j < child.ChildCount(); j++ {
				inner := child.Child(j)
				if declTypes[inner.Type()] {
					if n := ChildByField(inner, "name"); n != nil && n.StartByte() == nameStart {
						return inner
					}
				}
			}
		}
		// Java members nested in type bodies.
		if language == "java" {
			if found := findJavaMemberDecl(child, nameStart, declTypes); found != nil {
				return found
			}
		}
	}
	return nil
}

func findJavaMemberDecl(n *grammar.Node, nameStart uint32, declTypes map[string]bool) *grammar.Node {
	if n == nil {
		return nil
	}
	if declTypes[n.Type()] {
		if name := ChildByField(n, "name"); name != nil && name.StartByte() == nameStart {
			return n
		}
	}
	body := ChildByField(n, "body")
	if body == nil {
		return nil
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		if found := findJavaMemberDecl(body.Child(i), nameStart, declTypes); found != nil {
			return found
		}
	}
	return nil
}

// extractSignature returns a one-line function signature.
func extractSignature(funcNode *grammar.Node, source []byte, language string) string {
	bodyNode := ChildByField(funcNode, "body")
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
	body := ChildByField(funcNode, "body")
	if body == nil {
		return ""
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child.Type() == "string" || child.Type() == "concatenated_string" {
			return strings.Trim(NodeText(child, source), "\"'")
		}
		if child.Type() == "expression_statement" {
			if child.NamedChildCount() > 0 {
				expr := child.NamedChild(0)
				if expr.Type() == "string" || expr.Type() == "concatenated_string" {
					return strings.Trim(NodeText(expr, source), "\"'")
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
		case "javascript", "java":
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
