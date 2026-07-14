package java

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/java"
)

func init() {
	ingest.RegisterMoveDriver("java", moveDriver{})
}

type moveDriver struct{}

func (moveDriver) Language() string { return "java" }

func (moveDriver) ExtractDecl(filePath string, entity ingest.Entity) (ingest.DeclExtract, error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return ingest.DeclExtract{}, err
	}

	lang, ok := grammar.GetByExtension(filePath)
	if !ok {
		lang, ok = grammar.Get("java")
	}
	if !ok {
		return ingest.DeclExtract{}, fmt.Errorf("unsupported language for %s", filePath)
	}

	parser := grammar.NewParser()
	defer parser.Delete()
	if !parser.SetLanguage(lang) {
		return ingest.DeclExtract{}, fmt.Errorf("failed to set language for %s", filePath)
	}

	tree := parser.ParseString(string(source))
	defer tree.Delete()
	root := tree.RootNode()

	pkg := ""
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Type() == "package_declaration" {
			if name := javaPackageNameNode(child); name != nil {
				pkg = ingest.NodeText(name, source)
			}
		}
	}

	declNode := findJavaTopLevelDecl(root, entity.StartByte)
	if declNode == nil {
		return ingest.DeclExtract{}, fmt.Errorf("top-level declaration not supported or not found in %s", filePath)
	}

	start := declNode.StartByte()
	end := declNode.EndByte()
	declText := string(source[start:end])
	removeEnd := end
	for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
		removeEnd++
		if removeEnd-end >= 2 {
			break
		}
	}

	return ingest.DeclExtract{
		Preamble:    pkg,
		DeclText:    declText,
		Imports:     javaImportsNeededByDecl(source, declText),
		RemoveStart: start,
		RemoveEnd:   removeEnd,
	}, nil
}

func (moveDriver) InsertDecl(dstRelPath string, dstContent []byte, decl ingest.DeclExtract) ingest.Edit {
	pkg := packageNameForJavaFile(dstRelPath)
	if pkg == "" {
		pkg = decl.Preamble
	}

	if dstContent != nil {
		merged := ensureJavaImports(string(dstContent), decl.Imports)
		if merged != string(dstContent) {
			return ingest.Edit{
				File:      dstRelPath,
				StartByte: 0,
				EndByte:   uint32(len(dstContent)),
				NewText:   appendJavaDeclText(merged, decl.DeclText),
			}
		}
		insertAt := uint32(len(dstContent))
		insertText := ""
		if len(dstContent) > 0 && dstContent[len(dstContent)-1] != '\n' {
			insertText += "\n"
		}
		if len(dstContent) > 0 {
			insertText += "\n"
		}
		insertText += decl.DeclText + "\n"
		return ingest.Edit{
			File:      dstRelPath,
			StartByte: insertAt,
			EndByte:   insertAt,
			NewText:   insertText,
		}
	}

	body := ""
	if pkg != "" {
		body = fmt.Sprintf("package %s;\n", pkg)
	}
	if len(decl.Imports) > 0 {
		body = ensureJavaImports(body, decl.Imports)
	}
	return ingest.Edit{
		File:      dstRelPath,
		StartByte: 0,
		EndByte:   0,
		NewText:   appendJavaDeclText(body, decl.DeclText),
	}
}

func appendJavaDeclText(content, declText string) string {
	out := content
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out += "\n"
	}
	if len(out) > 0 {
		out += "\n"
	}
	return out + declText + "\n"
}

// javaImportSpec is one import/import-static line from a Java source file.
// stmt is the full statement including "import " and trailing ';'.
type javaImportSpec struct {
	stmt      string
	local     string // simple name used by the declaration, or "*" for wildcards
	startByte int    // start of the line in source
	endByte   int    // exclusive end (includes trailing newline when present)
}

// javaImportsNeededByDecl returns full import statements from source whose
// simple names appear in declText.
//
// On-demand / static-star imports (local == "*") are always kept: we cannot
// observe which simple names they supply. That is correct for new-file
// destinations (gson new_module), but when merging into an existing file that
// already has a conflicting on-demand import for the same simple name (e.g.
// dest uses java.awt.List via java.awt.* and we carry java.util.*), the merge
// can make List ambiguous. Prefer single-type imports in moved sources; do not
// try to resolve star conflicts here.
func javaImportsNeededByDecl(source []byte, declText string) []string {
	specs := parseJavaImportSpecs(source)
	if len(specs) == 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, spec := range specs {
		if spec.stmt == "" || seen[spec.stmt] {
			continue
		}
		if spec.local == "*" || javaIdentUsed(declText, spec.local) {
			seen[spec.stmt] = true
			out = append(out, spec.stmt)
		}
	}
	return out
}

func parseJavaImportSpecs(source []byte) []javaImportSpec {
	text := string(source)
	var specs []javaImportSpec
	offset := 0
	for offset <= len(text) {
		nl := strings.IndexByte(text[offset:], '\n')
		lineEnd := len(text)
		next := len(text)
		if nl >= 0 {
			lineEnd = offset + nl
			next = lineEnd + 1
		}
		line := text[offset:lineEnd]
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "import ") {
			// Drop trailing comments for parsing, keep original line for stmt span.
			stmt := trim
			if i := strings.Index(stmt, "//"); i >= 0 {
				stmt = strings.TrimSpace(stmt[:i])
			}
			if strings.HasSuffix(stmt, ";") {
				body := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(stmt, "import ")), ";"))
				if body != "" {
					local := body
					if strings.HasPrefix(local, "static ") {
						local = strings.TrimSpace(strings.TrimPrefix(local, "static "))
					}
					if strings.HasSuffix(local, ".*") {
						local = "*"
					} else if i := strings.LastIndex(local, "."); i >= 0 {
						local = local[i+1:]
					}
					end := next
					if nl < 0 {
						end = len(text)
					}
					specs = append(specs, javaImportSpec{
						stmt:      stmt,
						local:     local,
						startByte: offset,
						endByte:   end,
					})
				}
			}
		}
		if nl < 0 {
			break
		}
		offset = next
	}
	return specs
}

func javaIdentUsed(text, ident string) bool {
	if ident == "" || ident == "*" {
		return false
	}
	off := 0
	for {
		idx := strings.Index(text[off:], ident)
		if idx < 0 {
			return false
		}
		pos := off + idx
		end := pos + len(ident)
		if pos > 0 && isJavaIdentChar(text[pos-1]) {
			off = end
			continue
		}
		if end < len(text) && isJavaIdentChar(text[end]) {
			off = end
			continue
		}
		return true
	}
}

// ensureJavaImports inserts missing import statements after the package clause.
// each entry in imports is a full "import …;" statement.
func ensureJavaImports(content string, imports []string) string {
	if len(imports) == 0 {
		return content
	}
	existing := map[string]bool{}
	for _, spec := range parseJavaImportSpecs([]byte(content)) {
		existing[spec.stmt] = true
	}
	var missing []string
	for _, stmt := range imports {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || existing[stmt] {
			continue
		}
		existing[stmt] = true
		missing = append(missing, stmt)
	}
	if len(missing) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	// Insert after package clause and any existing imports.
	insertAt := 0
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "package ") || strings.HasPrefix(trim, "import ") {
			insertAt = i + 1
			continue
		}
		if insertAt > 0 && trim != "" && !strings.HasPrefix(trim, "//") && !strings.HasPrefix(trim, "/*") && !strings.HasPrefix(trim, "*") {
			// First non-header line: stop so we insert before the body.
			break
		}
	}

	out := make([]string, 0, len(lines)+len(missing)+2)
	out = append(out, lines[:insertAt]...)
	// Blank line after package when there were no imports yet.
	if insertAt > 0 {
		prev := ""
		if len(out) > 0 {
			prev = strings.TrimSpace(out[len(out)-1])
		}
		if strings.HasPrefix(prev, "package ") {
			out = append(out, "")
		}
	}
	out = append(out, missing...)
	// Blank line between imports and body when body follows immediately.
	if insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) != "" {
		out = append(out, "")
	}
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n")
}

// FinishCrossFileMove strips source imports that were only needed by the moved decl.
func (moveDriver) FinishCrossFileMove(rootDir string, result *ingest.Result, src, dst ingest.Reference, decl ingest.DeclExtract) ([]ingest.Edit, error) {
	srcRel := strings.TrimPrefix(src.Path, "./")
	srcContent, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(srcRel)))
	if err != nil {
		return nil, nil
	}
	return stripUnusedJavaImports(srcRel, srcContent, decl), nil
}

// stripUnusedJavaImports removes import statements from the source file that
// were only used by the removed declaration (same idea as Go/JS).
func stripUnusedJavaImports(file string, content []byte, decl ingest.DeclExtract) []ingest.Edit {
	if len(decl.Imports) == 0 {
		return nil
	}
	carried := map[string]bool{}
	for _, stmt := range decl.Imports {
		carried[strings.TrimSpace(stmt)] = true
	}
	// Mask the removed declaration so remaining body usage is visible.
	masked := append([]byte(nil), content...)
	for i := decl.RemoveStart; i < decl.RemoveEnd && int(i) < len(masked); i++ {
		if masked[i] != '\n' {
			masked[i] = ' '
		}
	}
	// Mask import lines themselves so we do not count import idents as body use.
	specs := parseJavaImportSpecs(content)
	for _, spec := range specs {
		for i := spec.startByte; i < spec.endByte && i < len(masked); i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	rest := string(masked)

	var edits []ingest.Edit
	for _, spec := range specs {
		if !carried[spec.stmt] {
			continue
		}
		// Wildcards: cannot prove remaining body still needs them. Only strip
		// when the simple-name path would also strip (never for "*").
		if spec.local == "*" {
			continue
		}
		if javaIdentUsed(rest, spec.local) {
			continue
		}
		edits = append(edits, ingest.Edit{
			File:      file,
			StartByte: uint32(spec.startByte),
			EndByte:   uint32(spec.endByte),
			NewText:   "",
		})
	}
	return edits
}

func (moveDriver) RewriteImports(fileRelPath string, content []byte, result *ingest.Result, oldRef, newRef ingest.Reference) []ingest.Edit {
	oldPath := strings.TrimPrefix(oldRef.Path, "./")
	newPath := strings.TrimPrefix(newRef.Path, "./")
	if oldPath == "" || newPath == "" || oldPath == newPath {
		return nil
	}

	if oldRef.Symbol != "" {
		if strings.Contains(oldRef.Symbol, ".") {
			return nil
		}
		oldType := joinJavaName(packageNameForJavaFile(oldPath), oldRef.Symbol)
		newType := joinJavaName(packageNameForJavaFile(newPath), newRef.Symbol)
		if oldType == "" || newType == "" || oldType == newType {
			return nil
		}
		edits := rewriteJavaPrefixedSpecs(fileRelPath, content, []string{"import static ", "import "}, oldType, newType)
		oldPkg := packageNameForJavaFile(oldPath)
		newPkg := packageNameForJavaFile(newPath)
		if oldPkg != "" && newPkg != "" && oldPkg != newPkg {
			edits = append(edits, rewriteJavaPrefixedSpecs(fileRelPath, content, []string{"import static ", "import "}, oldPkg+".*", newPkg+".*")...)
		}
		return edits
	}

	oldPkg := packageNameForJavaDir(oldPath)
	newPkg := packageNameForJavaDir(newPath)
	if oldPkg == "" || newPkg == "" || oldPkg == newPkg {
		oldBase := ingest.LastPathComponent(oldPath)
		newBase := ingest.LastPathComponent(newPath)
		if oldBase == "" || oldBase == newBase {
			return nil
		}
		return ingest.FindAllWholeWordOccurrences(fileRelPath, content, oldBase, newBase)
	}
	// Replace the fully-qualified package token everywhere it appears as a
	// Java name segment (package/import lines, FQNs, module exports).
	return rewriteJavaNameToken(fileRelPath, content, oldPkg, newPkg)
}

func findJavaTopLevelDecl(root *grammar.Node, nameStart uint32) *grammar.Node {
	declTypes := map[string]bool{
		"class_declaration":           true,
		"interface_declaration":       true,
		"enum_declaration":            true,
		"record_declaration":          true,
		"annotation_type_declaration": true,
	}
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if !declTypes[child.Type()] {
			continue
		}
		if n := ingest.ChildByField(child, "name"); n != nil && n.StartByte() == nameStart {
			return child
		}
	}
	return nil
}

func packageNameForJavaFile(relPath string) string {
	relPath = strings.TrimPrefix(relPath, "./")
	dir := path.Dir(relPath)
	if dir == "." || dir == "" {
		return ""
	}
	return packageNameForJavaDir(dir)
}

func packageNameForJavaDir(relPath string) string {
	relPath = strings.Trim(strings.TrimPrefix(relPath, "./"), "/")
	if relPath == "" || relPath == "." {
		return ""
	}
	if strings.HasSuffix(relPath, ".java") {
		relPath = path.Dir(relPath)
		if relPath == "." {
			return ""
		}
	}
	if pkg, ok := packageNameFromSourceDir(relPath); ok {
		return pkg
	}
	return strings.ReplaceAll(relPath, "/", ".")
}

func joinJavaName(pkg, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

func rewriteJavaPrefixedSpecs(fileRelPath string, content []byte, prefixes []string, oldSpec, newSpec string) []ingest.Edit {
	if oldSpec == "" || oldSpec == newSpec {
		return nil
	}
	text := string(content)
	var edits []ingest.Edit
	for _, pfx := range prefixes {
		off := 0
		for off < len(text) {
			idx := strings.Index(text[off:], pfx)
			if idx < 0 {
				break
			}
			specStart := off + idx + len(pfx)
			for specStart < len(text) && (text[specStart] == ' ' || text[specStart] == '\t') {
				specStart++
			}
			specEnd := specStart
			for specEnd < len(text) && text[specEnd] != ';' && text[specEnd] != '\n' {
				specEnd++
			}
			if specEnd >= len(text) || text[specEnd] != ';' {
				off = specStart
				continue
			}
			spec := text[specStart:specEnd]
			repl := ""
			switch {
			case spec == oldSpec:
				repl = newSpec
			case strings.HasPrefix(spec, oldSpec+"."):
				repl = newSpec + strings.TrimPrefix(spec, oldSpec)
			}
			if repl != "" {
				edits = append(edits, ingest.Edit{
					File:      fileRelPath,
					StartByte: uint32(specStart),
					EndByte:   uint32(specEnd),
					NewText:   repl,
				})
			}
			off = specEnd + 1
		}
	}
	return edits
}

func rewriteJavaNameToken(fileRelPath string, content []byte, oldName, newName string) []ingest.Edit {
	if oldName == "" || oldName == newName {
		return nil
	}
	text := string(content)
	var edits []ingest.Edit
	off := 0
	for off < len(text) {
		idx := strings.Index(text[off:], oldName)
		if idx < 0 {
			break
		}
		start := off + idx
		end := start + len(oldName)
		if start > 0 && isJavaIdentChar(text[start-1]) {
			off = end
			continue
		}
		if end < len(text) && isJavaIdentChar(text[end]) {
			off = end
			continue
		}
		edits = append(edits, ingest.Edit{
			File:      fileRelPath,
			StartByte: uint32(start),
			EndByte:   uint32(end),
			NewText:   newName,
		})
		off = end
	}
	return edits
}

func isJavaIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '$'
}
