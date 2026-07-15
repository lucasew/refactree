package ingestgo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

func init() {
	ingest.RegisterMoveDriver("go", moveDriver{})
}

type moveDriver struct{}

func (moveDriver) Language() string { return "go" }

func (moveDriver) ExtractDecl(filePath string, entity ingest.Entity) (ingest.DeclExtract, error) {
	pf, err := ingest.ParseSourceFile(filePath, "")
	if err != nil {
		return ingest.DeclExtract{}, err
	}
	defer pf.Close()
	source, root := pf.Source, pf.Root

	// Extract package name.
	pkg := ""
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Type() == "package_clause" {
			if id := ingest.ChildByType(child, "package_identifier"); id != nil {
				pkg = ingest.NodeText(id, source)
			}
		}
	}

	result := findGoDecl(root, entity.StartByte)
	if result == nil {
		return ingest.DeclExtract{}, fmt.Errorf("declaration not found in %s", filePath)
	}

	var declText string
	var removeStart, removeEnd uint32

	if result.Grouped {
		// Grouped type/var/const: extract just the matching spec as a standalone
		// "type|var|const X ..." declaration. Dedent one tab level from the group.
		spec := result.Spec
		keyword := result.Keyword
		if keyword == "" {
			keyword = "type"
		}
		specText := string(source[spec.StartByte():spec.EndByte()])
		declText = keyword + " " + dedentOnce(specText)
		removeStart = spec.StartByte()
		removeEnd = spec.EndByte()
		// Remove trailing whitespace/newlines up to the next spec or ')'.
		for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r' || source[removeEnd] == '\t' || source[removeEnd] == ' ') {
			removeEnd++
		}
	} else {
		start := result.Node.StartByte()
		end := result.Node.EndByte()
		declText = string(source[start:end])
		removeStart = start
		removeEnd = end
		// Remove up to two trailing newlines.
		for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
			removeEnd++
			if removeEnd-end >= 2 {
				break
			}
		}
	}

	imports := goImportsNeededByDecl(source, declText)

	return ingest.DeclExtract{
		Preamble:    pkg,
		DeclText:    declText,
		Imports:     imports,
		RemoveStart: removeStart,
		RemoveEnd:   removeEnd,
	}, nil
}

func (moveDriver) InsertDecl(dstRelPath string, dstContent []byte, decl ingest.DeclExtract) ingest.Edit {
	insertAt := uint32(0)
	insertText := ""

	if dstContent != nil {
		merged := ensureGoImports(string(dstContent), decl.Imports)
		if merged != string(dstContent) {
			return ingest.Edit{
				File:      dstRelPath,
				StartByte: 0,
				EndByte:   uint32(len(dstContent)),
				NewText:   ingest.AppendDeclText(merged, decl.DeclText),
			}
		}
		insertAt = uint32(len(dstContent))
		if len(dstContent) > 0 && dstContent[len(dstContent)-1] != '\n' {
			insertText += "\n"
		}
		if len(dstContent) > 0 {
			insertText += "\n"
		}
		insertText += decl.DeclText + "\n"
	} else {
		pkgName := decl.Preamble
		if pkgName == "" {
			pkgName = ingest.LastPathComponent(strings.TrimSuffix(dstRelPath, ".go"))
		}
		body := fmt.Sprintf("package %s\n", pkgName)
		if len(decl.Imports) > 0 {
			body = ensureGoImports(body, decl.Imports)
		}
		insertText = ingest.AppendDeclText(body, decl.DeclText)
	}

	return ingest.Edit{
		File:      dstRelPath,
		StartByte: insertAt,
		EndByte:   insertAt,
		NewText:   insertText,
	}
}

// goImportsNeededByDecl returns import paths from the source file whose local
// names appear in declText (selectors or unqualified idents).
func goImportsNeededByDecl(source []byte, declText string) []string {
	specs := parseGoImportSpecs(source)
	if len(specs) == 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, spec := range specs {
		if spec.path == "" || seen[spec.path] {
			continue
		}
		local := spec.local
		if local == "" || local == "." || local == "_" {
			continue
		}
		if goIdentUsed(declText, local) {
			seen[spec.path] = true
			out = append(out, spec.path)
		}
	}
	return out
}

type goImportSpec struct {
	local      string
	path       string
	lineStart  int
	lineEnd    int // exclusive, includes trailing newline when present
	blockStart int // start of "import (" line, or -1 for single-line import
	blockEnd   int // end of closing ")" line exclusive, or -1
}

func parseGoImportSpecs(source []byte) []goImportSpec {
	text := string(source)
	var specs []goImportSpec
	lines := strings.Split(text, "\n")
	inBlock := false
	blockStart := -1
	offset := 0
	for _, line := range lines {
		lineLen := len(line)
		lineEnd := offset + lineLen
		if lineEnd < len(text) {
			lineEnd++ // newline
		}
		trim := strings.TrimSpace(line)
		if !inBlock {
			if trim == "import (" {
				inBlock = true
				blockStart = offset
				offset = lineEnd
				continue
			}
			if strings.HasPrefix(trim, "import ") {
				if spec, ok := parseGoImportLine(strings.TrimSpace(strings.TrimPrefix(trim, "import "))); ok {
					spec.lineStart = offset
					spec.lineEnd = lineEnd
					spec.blockStart = -1
					spec.blockEnd = -1
					specs = append(specs, spec)
				}
			}
			offset = lineEnd
			continue
		}
		if trim == ")" {
			for i := range specs {
				if specs[i].blockStart == blockStart && specs[i].blockEnd == 0 {
					specs[i].blockEnd = lineEnd
				}
			}
			inBlock = false
			blockStart = -1
			offset = lineEnd
			continue
		}
		if trim == "" || strings.HasPrefix(trim, "//") {
			offset = lineEnd
			continue
		}
		if spec, ok := parseGoImportLine(trim); ok {
			spec.lineStart = offset
			spec.lineEnd = lineEnd
			spec.blockStart = blockStart
			spec.blockEnd = 0
			specs = append(specs, spec)
		}
		offset = lineEnd
	}
	return specs
}

func parseGoImportLine(s string) (goImportSpec, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return goImportSpec{}, false
	}
	local := ""
	pathPart := s
	if s[0] == '"' || s[0] == '`' {
		pathPart = s
	} else {
		parts := strings.Fields(s)
		if len(parts) < 2 {
			return goImportSpec{}, false
		}
		local = parts[0]
		pathPart = parts[1]
	}
	pathPart = strings.TrimSpace(pathPart)
	if len(pathPart) < 2 {
		return goImportSpec{}, false
	}
	quote := pathPart[0]
	if quote != '"' && quote != '`' {
		return goImportSpec{}, false
	}
	end := strings.IndexByte(pathPart[1:], quote)
	if end < 0 {
		return goImportSpec{}, false
	}
	p := pathPart[1 : 1+end]
	if local == "" {
		local = ingest.LastPathComponent(p)
	}
	return goImportSpec{local: local, path: p}, true
}

// goIdentUsed reports whether ident appears as a Go identifier in text,
// ignoring comments and string/rune literals so comments/docs do not count
// as real uses (imports, package-local deps, etc.).
func goIdentUsed(text, ident string) bool {
	if ident == "" {
		return false
	}
	for i := 0; i < len(text); {
		// Line comment.
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '/' {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		}
		// Block comment.
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '*' {
			i += 2
			for i+1 < len(text) && !(text[i] == '*' && text[i+1] == '/') {
				i++
			}
			if i+1 < len(text) {
				i += 2
			}
			continue
		}
		// Interpreted string.
		if text[i] == '"' {
			i++
			for i < len(text) && text[i] != '"' {
				if text[i] == '\\' && i+1 < len(text) {
					i += 2
					continue
				}
				i++
			}
			if i < len(text) {
				i++
			}
			continue
		}
		// Raw string.
		if text[i] == '`' {
			i++
			for i < len(text) && text[i] != '`' {
				i++
			}
			if i < len(text) {
				i++
			}
			continue
		}
		// Rune literal.
		if text[i] == '\'' {
			i++
			for i < len(text) && text[i] != '\'' {
				if text[i] == '\\' && i+1 < len(text) {
					i += 2
					continue
				}
				i++
			}
			if i < len(text) {
				i++
			}
			continue
		}
		// Identifier token: match only at identifier boundaries.
		if isIdentStart(text[i]) {
			start := i
			i++
			for i < len(text) && ingest.IsIdentChar(text[i]) {
				i++
			}
			if text[start:i] == ident {
				return true
			}
			continue
		}
		i++
	}
	return false
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func ensureGoImports(content string, paths []string) string {
	if len(paths) == 0 {
		return content
	}
	existing := map[string]bool{}
	for _, spec := range parseGoImportSpecs([]byte(content)) {
		existing[spec.path] = true
	}
	var missing []string
	for _, p := range paths {
		if p == "" || existing[p] {
			continue
		}
		existing[p] = true
		missing = append(missing, p)
	}
	if len(missing) == 0 {
		return content
	}
	block := formatGoImportBlock(missing)
	// Insert after package clause.
	lines := strings.Split(content, "\n")
	insertAt := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			insertAt = i + 1
			break
		}
	}
	// Skip blank lines after package.
	for insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
		insertAt++
	}
	if insertAt < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[insertAt]), "import ") {
		// Merge into existing import section by rewriting whole file imports.
		return mergeIntoExistingGoImports(content, missing)
	}
	out := make([]string, 0, len(lines)+len(missing)+3)
	out = append(out, lines[:insertAt]...)
	if insertAt > 0 && insertAt <= len(lines) {
		out = append(out, "")
	}
	out = append(out, strings.Split(strings.TrimSuffix(block, "\n"), "\n")...)
	out = append(out, "")
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n")
}

func formatGoImportBlock(paths []string) string {
	if len(paths) == 1 {
		return fmt.Sprintf("import %q", paths[0])
	}
	var b strings.Builder
	b.WriteString("import (\n")
	for _, p := range paths {
		b.WriteString(fmt.Sprintf("\t%q\n", p))
	}
	b.WriteString(")")
	return b.String()
}

func mergeIntoExistingGoImports(content string, missing []string) string {
	specs := parseGoImportSpecs([]byte(content))
	have := map[string]bool{}
	for _, s := range specs {
		have[s.path] = true
	}
	var add []string
	for _, p := range missing {
		if !have[p] {
			have[p] = true
			add = append(add, p)
		}
	}
	if len(add) == 0 {
		return content
	}
	text := content
	// Prefer inserting into an import ( ... ) block.
	if idx := strings.Index(text, "import ("); idx >= 0 {
		end := strings.Index(text[idx:], "\n)")
		if end >= 0 {
			insertPos := idx + end + 1
			var b strings.Builder
			for _, p := range add {
				b.WriteString(fmt.Sprintf("\t%q\n", p))
			}
			return text[:insertPos] + b.String() + text[insertPos:]
		}
	}
	// Single-line import: convert by appending a new import line after it.
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "import ") {
			var extra []string
			for _, p := range add {
				extra = append(extra, fmt.Sprintf("import %q", p))
			}
			out := append([]string{}, lines[:i+1]...)
			out = append(out, extra...)
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n")
		}
	}
	return ensureGoImports(content, add)
}

func (moveDriver) RewriteImports(fileRelPath string, content []byte, result *ingest.Result, oldRef, newRef ingest.Reference) []ingest.Edit {
	// Determine old and new directory paths.
	oldPath := strings.TrimPrefix(oldRef.Path, "./")
	newPath := strings.TrimPrefix(newRef.Path, "./")

	// For symbol-level moves (cross-file), use the parent directories.
	oldDir := oldPath
	newDir := newPath
	if oldRef.Symbol != "" {
		oldDir = dirOf(oldPath)
		newDir = dirOf(newPath)
	}

	if oldDir == "" || newDir == "" || oldDir == newDir {
		return nil
	}

	var edits []ingest.Edit

	// Rewrite import path strings on path-segment boundaries so that moving
	// "pkg/api" does not rewrite "pkg/palette/api", and moving "pkg/cas" does
	// not corrupt "case" or "lucas" inside string literals.
	edits = findPathSegmentOccurrencesInStrings(fileRelPath, content, oldDir, newDir)

	if len(edits) == 0 {
		oldBase := ingest.LastPathComponent(oldDir)
		newBase := ingest.LastPathComponent(newDir)
		if cp := ingest.CommonPathPrefix(oldDir, newDir); cp != "" {
			if rel := strings.Trim(strings.TrimPrefix(newDir, cp), "/"); rel != "" {
				newBase = rel
			}
		}
		if oldBase != newBase {
			parent := dirOf(oldDir)
			edits = findPathSegmentOccurrencesInStringsWithParent(
				fileRelPath, content, oldBase, newBase, parent,
			)
		}
	}

	// For cross-file symbol moves, also update the qualifier (e.g. pkga.Helper -> pkgb.Helper).
	// For package moves, the qualifier comes from the `package` directive and is handled
	// separately by planPackageMove's declaredName logic.
	if oldRef.Symbol != "" {
		oldQual := ingest.LastPathComponent(oldDir)
		newQual := ingest.LastPathComponent(newDir)
		if oldQual != newQual {
			qualEdits := findQualifierDotOccurrences(fileRelPath, content, oldQual, newQual)
			edits = append(edits, qualEdits...)
		}
	}

	return edits
}

// findQualifierDotOccurrences rewrites ident. selections where ident is a
// whole word (package qualifier), avoiding substring hits like mypkga.X.
func findQualifierDotOccurrences(file string, content []byte, oldQual, newQual string) []ingest.Edit {
	if oldQual == "" || oldQual == newQual {
		return nil
	}
	text := string(content)
	needle := oldQual + "."
	var edits []ingest.Edit
	off := 0
	for {
		idx := strings.Index(text[off:], needle)
		if idx < 0 {
			break
		}
		pos := off + idx
		endPos := pos + len(oldQual)
		if pos > 0 && ingest.IsIdentChar(text[pos-1]) {
			off = pos + len(needle)
			continue
		}
		edits = append(edits, ingest.Edit{
			File:      file,
			StartByte: uint32(pos),
			EndByte:   uint32(endPos),
			NewText:   newQual,
		})
		off = pos + len(needle)
	}
	return edits
}

// dedentOnce removes one leading tab from each line of s.
func dedentOnce(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if len(line) > 0 && line[0] == '\t' {
			lines[i] = line[1:]
		}
	}
	return strings.Join(lines, "\n")
}

func dirOf(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i]
}

// goDeclResult holds the matched declaration node and, for grouped type/var/const
// declarations, the individual spec that matched (when the declaration contains
// multiple specs).
type goDeclResult struct {
	Node    *grammar.Node // the top-level declaration
	Grouped bool          // true when part of a type|var|const (...) group
	Spec    *grammar.Node // non-nil for grouped type/var/const declarations
	Keyword string        // "type", "var", or "const" when Grouped
}

// findGoDecl returns the declaration containing the entity whose name starts at nameStart.
func findGoDecl(root *grammar.Node, nameStart uint32) *goDeclResult {
	declTypes := map[string]bool{
		"function_declaration": true,
		"method_declaration":   true,
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if declTypes[child.Type()] {
			if n := ingest.ChildByField(child, "name"); n != nil && n.StartByte() == nameStart {
				return &goDeclResult{Node: child}
			}
		}
		if child.Type() == "type_declaration" {
			specCount := 0
			var matchedSpec *grammar.Node
			for j := uint32(0); j < child.ChildCount(); j++ {
				spec := child.Child(j)
				if spec.Type() == "type_spec" {
					specCount++
					if id := ingest.ChildByType(spec, "type_identifier"); id != nil && id.StartByte() == nameStart {
						matchedSpec = spec
					}
				}
			}
			if matchedSpec != nil {
				if specCount > 1 {
					return &goDeclResult{Node: child, Grouped: true, Spec: matchedSpec, Keyword: "type"}
				}
				return &goDeclResult{Node: child}
			}
		}
		if child.Type() == "var_declaration" || child.Type() == "const_declaration" {
			keyword := "var"
			if child.Type() == "const_declaration" {
				keyword = "const"
			}
			specs := goVarConstSpecs(child)
			var matchedSpec *grammar.Node
			for _, spec := range specs {
				if goSpecNameStartsAt(spec, nameStart) {
					matchedSpec = spec
					break
				}
			}
			if matchedSpec != nil {
				if len(specs) > 1 {
					return &goDeclResult{Node: child, Grouped: true, Spec: matchedSpec, Keyword: keyword}
				}
				return &goDeclResult{Node: child}
			}
		}
	}
	return nil
}

// goVarConstSpecs returns var_spec/const_spec children of a var_declaration or
// const_declaration, including those nested under var_spec_list/const_spec_list.
func goVarConstSpecs(decl *grammar.Node) []*grammar.Node {
	specType := "var_spec"
	listType := "var_spec_list"
	if decl.Type() == "const_declaration" {
		specType = "const_spec"
		listType = "const_spec_list"
	}
	var specs []*grammar.Node
	for i := uint32(0); i < decl.ChildCount(); i++ {
		child := decl.Child(i)
		switch child.Type() {
		case specType:
			specs = append(specs, child)
		case listType:
			for j := uint32(0); j < child.ChildCount(); j++ {
				spec := child.Child(j)
				if spec.Type() == specType {
					specs = append(specs, spec)
				}
			}
		}
	}
	return specs
}

// goSpecNameStartsAt reports whether a var_spec/const_spec declares a name whose
// identifier starts at nameStart (single name or identifier_list entry).
func goSpecNameStartsAt(spec *grammar.Node, nameStart uint32) bool {
	nameNode := ingest.ChildByField(spec, "name")
	if nameNode == nil {
		return false
	}
	if nameNode.Type() == "identifier" {
		return nameNode.StartByte() == nameStart
	}
	for i := uint32(0); i < nameNode.ChildCount(); i++ {
		c := nameNode.Child(i)
		if c.Type() == "identifier" && c.StartByte() == nameStart {
			return true
		}
	}
	return false
}

func (moveDriver) ExpandRenameSources(result *ingest.Result, sourceRef string) []string {
	src := ingest.ParseReference(sourceRef)
	if src.Symbol == "" {
		return nil
	}
	leaf := ingest.SymbolLeaf(src.Symbol)
	recv, isMethod := receiverTypeName(src.Symbol)
	if leaf == "" {
		return nil
	}
	srcRel := strings.TrimPrefix(src.Path, "./")
	srcPkgDir := dirOf(srcRel)
	scopePrefix := methodRenameScopePrefix(srcRel)
	var extra []string
	for _, ent := range result.Entities {
		ref := ingest.ParseReference(ent.Reference)
		if ref.Provider != "path" || ref.Symbol == "" {
			continue
		}
		rel := strings.TrimPrefix(ref.Path, "./")
		entPkgDir := dirOf(rel)
		entLeaf := ingest.SymbolLeaf(ref.Symbol)
		if entLeaf != leaf {
			continue
		}
		if isMethod {
			entRecv, entMethod := receiverTypeName(ref.Symbol)
			if entMethod {
				if entRecv != recv {
					continue
				}
				// Same impl package or sibling packages under the interface tree.
				if entPkgDir != srcPkgDir &&
					(scopePrefix == "" || (entPkgDir != scopePrefix && !strings.HasPrefix(rel, scopePrefix+"/"))) {
					continue
				}
			} else if ref.Symbol == leaf {
				// Facade function only in the *true* parent package dir
				// (e.g. pkg/driver/wallpaper.SetStatic when renaming
				// *Driver.SetStatic under wallpaper/feh).
				// When scopePrefix is empty or equals the method's package
				// (root / single-segment dirs), do not expand bare leaves —
				// that wrongly pulled same-package *types* named like the
				// method leaf into the rename set (type Helper vs Box.Helper).
				if scopePrefix == "" || scopePrefix == srcPkgDir || entPkgDir != scopePrefix {
					continue
				}
			} else {
				continue
			}
		} else if ref.Symbol != src.Symbol && ref.Symbol != leaf {
			continue
		} else if ref.Symbol == leaf && entPkgDir != srcPkgDir && entPkgDir != scopePrefix {
			continue
		}
		if ent.Reference != sourceRef {
			extra = append(extra, ent.Reference)
		}
	}
	return extra
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}
	r := name[0]
	return r >= 'A' && r <= 'Z'
}

func receiverTypeName(symbol string) (string, bool) {
	i := strings.LastIndex(symbol, ".")
	if i < 0 {
		return "", false
	}
	recv := strings.TrimPrefix(symbol[:i], "*")
	if recv == "" {
		return "", false
	}
	return recv, true
}

// methodRenameScopePrefix returns the parent package directory of the file's
// package dir (e.g. pkg/driver/wallpaper/feh/driver.go -> pkg/driver/wallpaper)
// so interface methods and sibling implementations rename together without
// touching unrelated Driver types in other package trees.
func methodRenameScopePrefix(fileRel string) string {
	dir := dirOf(fileRel)
	if dir == "" {
		return ""
	}
	parent := dirOf(dir)
	if parent == "" {
		return dir
	}
	return parent
}

func (moveDriver) FinishCrossFileMove(rootDir string, result *ingest.Result, src, dst ingest.Reference, decl ingest.DeclExtract) ([]ingest.Edit, error) {
	srcRel := strings.TrimPrefix(src.Path, "./")
	dstRel := strings.TrimPrefix(dst.Path, "./")
	oldDir := dirOf(srcRel)
	newDir := dirOf(dstRel)

	// Same-package layout moves still need unused import cleanup on the source file.
	if oldDir == newDir {
		if srcContent, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(srcRel))); err == nil {
			return stripUnusedSourceImports(srcRel, srcContent, decl), nil
		}
		return nil, nil
	}
	if oldDir == "" || newDir == "" {
		return nil, nil
	}

	leaf := ingest.SymbolLeaf(src.Symbol)
	if leaf == "" {
		return nil, nil
	}
	if !isExportedIdent(leaf) && leaf != "init" {
		return nil, fmt.Errorf("cross-package move of unexported symbol %s is not supported", leaf)
	}
	if strings.HasPrefix(leaf, "Test") && strings.HasSuffix(srcRel, "_test.go") && !strings.HasSuffix(dstRel, "_test.go") {
		return nil, fmt.Errorf("moving test function %s into non-test file %s is not supported", leaf, dstRel)
	}
	// Go methods must live in the type's package; moving the type alone breaks them.
	if packageHasMethodsOf(result, oldDir, leaf) {
		return nil, fmt.Errorf("cross-package move of type %s with methods in package is not supported", leaf)
	}
	// Declaration bodies that still reference same-package symbols would leave
	// undefined names at the destination (imports only cover other packages).
	if dep := packageLocalDepInDecl(result, oldDir, leaf, decl.DeclText); dep != "" {
		return nil, fmt.Errorf("cross-package move of %s still depends on same-package symbol %s is not supported", leaf, dep)
	}

	var edits []ingest.Edit
	if srcContent, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(srcRel))); err == nil {
		edits = append(edits, stripUnusedSourceImports(srcRel, srcContent, decl)...)
	}
	if leaf == "init" {
		return edits, nil
	}

	newQual := ingest.LastPathComponent(newDir)
	modPath, err := readGoModulePath(rootDir)
	if err != nil || modPath == "" {
		return edits, nil
	}
	newImportPath := modPath + "/" + newDir

	srcRef := src.String()
	seenFiles := map[string]bool{dstRel: true}
	needImport := map[string]bool{}

	for _, rel := range result.Relations {
		if rel.Target != srcRef || rel.ViaImportAlias {
			continue
		}
		ref := ingest.ParseReference(rel.Reference)
		fileRel := strings.TrimPrefix(ref.Path, "./")
		if fileRel == dstRel {
			continue
		}
		if dirOf(fileRel) != oldDir {
			continue
		}
		// Skip spans inside the declaration being removed from the source file.
		if fileRel == srcRel && rel.StartByte >= decl.RemoveStart && rel.EndByte <= decl.RemoveEnd {
			continue
		}
		// Same-package calls are bare identifiers; qualify them for the new package.
		edits = append(edits, ingest.Edit{
			File:      fileRel,
			StartByte: rel.StartByte,
			EndByte:   rel.EndByte,
			NewText:   newQual + "." + leaf,
		})
		needImport[fileRel] = true
	}
	for fileRel := range needImport {
		if seenFiles[fileRel] {
			continue
		}
		seenFiles[fileRel] = true
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(fileRel)))
		if err != nil {
			continue
		}
		edits = append(edits, goImportInsertEdits(fileRel, content, []string{newImportPath})...)
	}
	return edits, nil
}

// packageHasMethodsOf reports whether pkgDir declares methods whose receiver
// type leaf is typeName (e.g. *Session.Close or Session.Group).
func packageHasMethodsOf(result *ingest.Result, pkgDir, typeName string) bool {
	if result == nil || typeName == "" {
		return false
	}
	for _, ent := range result.Entities {
		ref := ingest.ParseReference(ent.Reference)
		rel := strings.TrimPrefix(ref.Path, "./")
		if dirOf(rel) != pkgDir {
			continue
		}
		if methodReceiverType(ref.Symbol) == typeName {
			return true
		}
	}
	return false
}

// methodReceiverType returns the base type name for a method symbol like
// "*Session.Close", "Session.Group", or "*Set[T].Add", or "" if not a method.
// Generic receivers keep type args in the entity name (*Set[T].Add); type
// entities are named by the identifier only (Set), so strip […].
func methodReceiverType(symbol string) string {
	recv, ok := receiverTypeName(symbol)
	if !ok {
		return ""
	}
	if i := strings.IndexByte(recv, '['); i >= 0 {
		recv = recv[:i]
	}
	return recv
}

// packageLocalDepInDecl returns the first package-scope symbol in pkgDir (other
// than movedLeaf) that the moved declaration depends on, or "".
// Prefer resolved same-package relations (precise uses); fall back to a
// comment/string-aware identifier scan of declText for cases without usage
// edges (e.g. type field types not recorded as relations).
func packageLocalDepInDecl(result *ingest.Result, pkgDir, movedLeaf, declText string) string {
	if result == nil {
		return ""
	}
	if dep := packageLocalDepFromRelations(result, pkgDir, movedLeaf); dep != "" {
		return dep
	}
	if declText == "" {
		return ""
	}
	// Stable order: scan entities as listed.
	seen := map[string]bool{movedLeaf: true}
	for _, ent := range result.Entities {
		ref := ingest.ParseReference(ent.Reference)
		rel := strings.TrimPrefix(ref.Path, "./")
		if dirOf(rel) != pkgDir {
			continue
		}
		sym := ref.Symbol
		if sym == "" || strings.Contains(sym, ".") {
			continue // skip empty and method symbols
		}
		name := strings.TrimPrefix(sym, "*")
		if seen[name] {
			continue
		}
		seen[name] = true
		if goIdentUsed(declText, name) {
			return name
		}
	}
	return ""
}

// packageLocalDepFromRelations finds a same-package dependency via usage
// relations scoped to movedLeaf (e.g. Load -> Helper).
func packageLocalDepFromRelations(result *ingest.Result, pkgDir, movedLeaf string) string {
	if result == nil || movedLeaf == "" {
		return ""
	}
	for _, rel := range result.Relations {
		src := ingest.ParseReference(rel.Reference)
		if ingest.SymbolLeaf(src.Symbol) != movedLeaf {
			continue
		}
		srcRel := strings.TrimPrefix(src.Path, "./")
		if dirOf(srcRel) != pkgDir {
			continue
		}
		tgt := ingest.ParseReference(rel.Target)
		if tgt.Symbol == "" || strings.Contains(tgt.Symbol, ".") {
			continue
		}
		tgtRel := strings.TrimPrefix(tgt.Path, "./")
		if dirOf(tgtRel) != pkgDir {
			continue
		}
		name := strings.TrimPrefix(tgt.Symbol, "*")
		if name == "" || name == movedLeaf {
			continue
		}
		return name
	}
	return ""
}

func readGoModulePath(rootDir string) (string, error) {
	dir := rootDir
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
				}
			}
			return "", fmt.Errorf("no module line in go.mod")
		}
		parent := filepath.Dir(dir)
		if parent == "" || parent == dir {
			return "", err
		}
		dir = parent
	}
}

func goImportInsertEdits(file string, content []byte, paths []string) []ingest.Edit {
	text := string(content)
	existing := map[string]bool{}
	for _, spec := range parseGoImportSpecs(content) {
		existing[spec.path] = true
	}
	var missing []string
	for _, p := range paths {
		if p != "" && !existing[p] {
			existing[p] = true
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if idx := strings.Index(text, "import ("); idx >= 0 {
		end := strings.Index(text[idx:], "\n)")
		if end >= 0 {
			insertPos := idx + end + 1
			var b strings.Builder
			for _, p := range missing {
				b.WriteString(fmt.Sprintf("\t%q\n", p))
			}
			return []ingest.Edit{{
				File:      file,
				StartByte: uint32(insertPos),
				EndByte:   uint32(insertPos),
				NewText:   b.String(),
			}}
		}
	}
	lines := strings.Split(text, "\n")
	offset := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "import ") {
			insertPos := offset + len(line) + 1
			var b strings.Builder
			for _, p := range missing {
				b.WriteString(fmt.Sprintf("import %q\n", p))
			}
			return []ingest.Edit{{
				File:      file,
				StartByte: uint32(insertPos),
				EndByte:   uint32(insertPos),
				NewText:   b.String(),
			}}
		}
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			insertPos := offset + len(line) + 1
			// Preserve a single blank line between import and the next decl.
			return []ingest.Edit{{
				File:      file,
				StartByte: uint32(insertPos),
				EndByte:   uint32(insertPos),
				NewText:   "\n" + formatGoImportBlock(missing) + "\n",
			}}
		}
		offset += len(line) + 1
	}
	return nil
}

func stripUnusedSourceImports(file string, content []byte, decl ingest.DeclExtract) []ingest.Edit {
	if len(decl.Imports) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, p := range decl.Imports {
		want[p] = true
	}
	specs := parseGoImportSpecs(content)
	masked := append([]byte(nil), content...)
	mask := func(start, end int) {
		if start < 0 {
			start = 0
		}
		if end > len(masked) {
			end = len(masked)
		}
		for i := start; i < end; i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	mask(int(decl.RemoveStart), int(decl.RemoveEnd))
	seenBlocks := map[int]bool{}
	for _, spec := range specs {
		if spec.blockStart >= 0 && spec.blockEnd > 0 {
			if !seenBlocks[spec.blockStart] {
				mask(spec.blockStart, spec.blockEnd)
				seenBlocks[spec.blockStart] = true
			}
			continue
		}
		mask(spec.lineStart, spec.lineEnd)
	}
	bodyText := string(masked)
	var edits []ingest.Edit
	blockCounts := map[int]int{}
	blockRemove := map[int]int{}
	for _, spec := range specs {
		if spec.blockStart >= 0 {
			blockCounts[spec.blockStart]++
		}
	}
	for _, spec := range specs {
		if !want[spec.path] || spec.local == "" || spec.local == "." || spec.local == "_" {
			continue
		}
		if goIdentUsed(bodyText, spec.local) {
			continue
		}
		edits = append(edits, ingest.Edit{
			File:      file,
			StartByte: uint32(spec.lineStart),
			EndByte:   uint32(spec.lineEnd),
			NewText:   "",
		})
		if spec.blockStart >= 0 {
			blockRemove[spec.blockStart]++
		}
	}
	for blockStart, removed := range blockRemove {
		if removed == 0 || removed < blockCounts[blockStart] {
			continue
		}
		blockEnd := 0
		for _, spec := range specs {
			if spec.blockStart == blockStart && spec.blockEnd > 0 {
				blockEnd = spec.blockEnd
				break
			}
		}
		if blockEnd <= blockStart {
			continue
		}
		filtered := edits[:0]
		for _, e := range edits {
			if int(e.StartByte) >= blockStart && int(e.EndByte) <= blockEnd && e.NewText == "" {
				continue
			}
			filtered = append(filtered, e)
		}
		edits = filtered
		start, end := blockStart, blockEnd
		if start > 0 && content[start-1] == '\n' {
			start--
		}
		edits = append(edits, ingest.Edit{
			File:      file,
			StartByte: uint32(start),
			EndByte:   uint32(end),
			NewText:   "",
		})
	}
	return edits
}

func (moveDriver) ExtraRenameEdits(rootDir string, result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) []ingest.Edit {
	if oldLeaf == "" || oldLeaf == newLeaf || len(sourceRefs) == 0 || rootDir == "" {
		return nil
	}
	src := ingest.ParseReference(sourceRefs[0])
	_, isMethod := receiverTypeName(src.Symbol)
	if !isMethod {
		// Type (or other non-method) rename: rewrite embedded-field selectors
		// (struct { Base }; b.Base) when the package embeds oldLeaf.
		return embeddedTypeFieldSelectorEdits(rootDir, result, sourceRefs, oldLeaf, newLeaf)
	}
	scopePrefix := methodRenameScopePrefix(strings.TrimPrefix(src.Path, "./"))
	sourceSet := map[string]bool{}
	pkgDirs := map[string]bool{}
	ourPkgDirs := map[string]bool{}
	// Include the source package even when it is the module root (dir "").
	// Skipping "" left ExtraRename unable to run findInterfaceMethodEdits for
	// package main / root packages, so interface-typed selectors were rewritten
	// without renaming the interface method (broken go build).
	srcPkgDir := dirOf(strings.TrimPrefix(src.Path, "./"))
	ourPkgDirs[srcPkgDir] = true
	pkgDirs[srcPkgDir] = true
	if scopePrefix != "" {
		ourPkgDirs[scopePrefix] = true
	}
	// Receiver types we are renaming (*T.m / T.m). Same-package ExtraRename
	// must not rewrite t.m on a different concrete type that also defines m.
	ourReceivers := map[string]bool{}
	for _, s := range sourceRefs {
		sourceSet[s] = true
		ref := ingest.ParseReference(s)
		rel := strings.TrimPrefix(ref.Path, "./")
		// Always record package dir, including "" for files at module root.
		d := dirOf(rel)
		pkgDirs[d] = true
		ourPkgDirs[d] = true
		if recv, ok := receiverTypeName(ref.Symbol); ok {
			ourReceivers[recv] = true
		}
	}
	// Other concrete types that define a method with the same leaf (entity graph
	// first; AST scan below fills gaps when ingest misses a method entity).
	foreignReceivers := map[string]bool{}
	for _, ent := range result.Entities {
		if sourceSet[ent.Reference] {
			continue
		}
		ref := ingest.ParseReference(ent.Reference)
		if ingest.SymbolLeaf(ref.Symbol) != oldLeaf {
			continue
		}
		if recv, ok := receiverTypeName(ref.Symbol); ok && !ourReceivers[recv] {
			foreignReceivers[recv] = true
		}
	}
	// Cheap fail-closed assist: scan package AST for same-leaf methods not in ourReceivers.
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		if !ourPkgDirs[dirOf(rel)] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, recv := range methodReceiversWithLeaf(content, oldLeaf) {
			if !ourReceivers[recv] {
				foreignReceivers[recv] = true
			}
		}
	}
	occupied := map[string]map[[2]uint32]bool{}
	mark := func(file string, start, end uint32) {
		file = strings.TrimPrefix(file, "./")
		if occupied[file] == nil {
			occupied[file] = map[[2]uint32]bool{}
		}
		occupied[file][[2]uint32{start, end}] = true
	}
	for _, ent := range result.Entities {
		if !sourceSet[ent.Reference] {
			continue
		}
		ref := ingest.ParseReference(ent.Reference)
		mark(ref.Path, ent.StartByte, ent.EndByte)
	}
	relatedFiles := map[string]bool{}
	for _, rel := range result.Relations {
		if !sourceSet[rel.Target] {
			continue
		}
		ref := ingest.ParseReference(rel.Reference)
		fileRel := strings.TrimPrefix(ref.Path, "./")
		relatedFiles[fileRel] = true
		mark(ref.Path, rel.StartByte, rel.EndByte)
	}
	for _, s := range sourceRefs {
		ref := ingest.ParseReference(s)
		relatedFiles[strings.TrimPrefix(ref.Path, "./")] = true
	}

	var edits []ingest.Edit
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		entPkg := dirOf(rel)
		inOurPkg := ourPkgDirs[entPkg]
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		importsRelated := fileImportsAnyPackage(content, pkgDirs)
		if !inOurPkg && !relatedFiles[rel] && !importsRelated {
			continue
		}
		occ := occupied[rel]
		var selectorEdits []ingest.Edit
		if inOurPkg || relatedFiles[rel] {
			// Same package tree: rename ident.Leaf only (not Type{}.Leaf).
			selectorEdits = findSelectorLeafEdits(rel, content, oldLeaf, newLeaf, nil)
		} else if importsRelated {
			// External importers: package qualifiers for our pkgs plus variables
			// typed as imported interfaces that carry this method (var d a.Driver).
			// Never b.Unrelated{}.WriteImage or b.WriteImage on foreign packages.
			allowed := importLocalsForPackages(content, pkgDirs)
			ifaceNames := interfaceNamesWithMethod(rootDir, result, ourPkgDirs, oldLeaf)
			for local := range importLocalsForPackages(content, pkgDirs) {
				for _, iface := range ifaceNames {
					for name := range varsTypedAsImported(content, local, iface) {
						allowed[name] = true
					}
				}
			}
			selectorEdits = findSelectorLeafEdits(rel, content, oldLeaf, newLeaf, allowed)
		}
		// Filter same-package renames by the *call target* type (selector receiver),
		// not the enclosing method's receiver — so package-level helpers and
		// cross-type calls inside foreign methods are handled correctly.
		var callTargetType func(leafStart uint32) (string, bool)
		if len(foreignReceivers) > 0 && (inOurPkg || relatedFiles[rel]) {
			callTargetType = selectorCallTargetTypeFunc(content)
		}
		for _, e := range selectorEdits {
			if occ[[2]uint32{e.StartByte, e.EndByte}] {
				continue
			}
			if callTargetType != nil {
				if typ, ok := callTargetType(e.StartByte); ok {
					typ = strings.TrimPrefix(typ, "*")
					if foreignReceivers[typ] && !ourReceivers[typ] {
						continue
					}
				}
			}
			edits = append(edits, e)
		}
		if inOurPkg {
			for _, e := range findInterfaceMethodEdits(rel, content, oldLeaf, newLeaf) {
				if occ[[2]uint32{e.StartByte, e.EndByte}] {
					continue
				}
				edits = append(edits, e)
			}
		}
	}
	return edits
}

// embeddedTypeFieldSelectorEdits rewrites `.OldType` selectors when renaming a
// type that is embedded in a struct in the same package (promoted field name
// equals the type name in Go).
func embeddedTypeFieldSelectorEdits(rootDir string, result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) []ingest.Edit {
	src := ingest.ParseReference(sourceRefs[0])
	// Only plain type/func-like leaves (no dots); skip package moves.
	if src.Symbol == "" || strings.Contains(src.Symbol, ".") {
		return nil
	}
	srcPkgDir := dirOf(strings.TrimPrefix(src.Path, "./"))
	ourPkgDirs := map[string]bool{srcPkgDir: true}

	// Confirm the package embeds oldLeaf somewhere; otherwise skip (avoids
	// rewriting arbitrary .OldLeaf method/field selectors for non-embed renames).
	embeds := false
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		if dirOf(rel) != srcPkgDir {
			continue
		}
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if fileEmbedsType(content, oldLeaf) {
			embeds = true
			break
		}
	}
	if !embeds {
		return nil
	}

	sourceSet := map[string]bool{}
	for _, s := range sourceRefs {
		sourceSet[s] = true
	}
	occupied := map[string]map[[2]uint32]bool{}
	mark := func(file string, start, end uint32) {
		file = strings.TrimPrefix(file, "./")
		if occupied[file] == nil {
			occupied[file] = map[[2]uint32]bool{}
		}
		occupied[file][[2]uint32{start, end}] = true
	}
	for _, ent := range result.Entities {
		if sourceSet[ent.Reference] {
			ref := ingest.ParseReference(ent.Reference)
			mark(ref.Path, ent.StartByte, ent.EndByte)
		}
	}
	for _, reln := range result.Relations {
		if sourceSet[reln.Target] {
			ref := ingest.ParseReference(reln.Reference)
			mark(ref.Path, reln.StartByte, reln.EndByte)
		}
	}

	var edits []ingest.Edit
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		if !ourPkgDirs[dirOf(rel)] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		occ := occupied[rel]
		for _, e := range findSelectorLeafEdits(rel, content, oldLeaf, newLeaf, nil) {
			if occ[[2]uint32{e.StartByte, e.EndByte}] {
				continue
			}
			edits = append(edits, e)
		}
	}
	return edits
}

// fileEmbedsType reports whether content has a struct field_declaration that is
// an embedded type named leaf (type_identifier only, no field name).
func fileEmbedsType(content []byte, leaf string) bool {
	if leaf == "" {
		return false
	}
	pf, err := ingest.ParseSource(content, ".go", "")
	if err != nil {
		return false
	}
	defer pf.Close()
	var found bool
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || found {
			return
		}
		if n.Type() == "field_declaration" {
			// Embedded: has type_identifier child and no field_identifier name.
			var hasFieldName bool
			var embedsLeaf bool
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				switch ch.Type() {
				case "field_identifier":
					hasFieldName = true
				case "type_identifier":
					if ingest.NodeText(ch, content) == leaf {
						embedsLeaf = true
					}
				}
			}
			if embedsLeaf && !hasFieldName {
				found = true
				return
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(pf.Root)
	return found
}

// methodReceiversWithLeaf returns receiver type names (star-stripped) of methods
// named leaf in content. Used to augment foreignReceivers when the entity graph
// is incomplete.
func methodReceiversWithLeaf(content []byte, leaf string) []string {
	if leaf == "" {
		return nil
	}
	pf, err := ingest.ParseSource(content, ".go", "")
	if err != nil {
		return nil
	}
	defer pf.Close()
	var out []string
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "method_declaration" {
			if name := ingest.ChildByField(n, "name"); name != nil && ingest.NodeText(name, content) == leaf {
				if recv := methodDeclReceiverType(n, content); recv != "" {
					out = append(out, recv)
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(pf.Root)
	return out
}

// identTypeBinding maps an identifier to a concrete type within a byte span.
type identTypeBinding struct {
	start, end uint32 // scope of the binding
	name       string
	typ        string
}

// selectorCallTargetTypeFunc returns a lookup for the concrete type of the
// selector receiver expression at a leaf-identifier start byte (the char after
// '.' in recv.Leaf). Resolves method receiver params and simple local bindings
// (t := &T{}, var t *T, …). ok=false when the type cannot be determined.
func selectorCallTargetTypeFunc(content []byte) func(leafStart uint32) (string, bool) {
	var bindings []identTypeBinding
	pf, err := ingest.ParseSource(content, ".go", "")
	if err != nil {
		return func(uint32) (string, bool) { return "", false }
	}
	defer pf.Close()

	// Collect method/function scopes and local typed bindings.
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "method_declaration":
			recvType := methodDeclReceiverType(n, content)
			recvName := methodDeclReceiverName(n, content)
			if recvType != "" && recvName != "" {
				bindings = append(bindings, identTypeBinding{n.StartByte(), n.EndByte(), recvName, recvType})
			}
			// Locals inside the method body.
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalTypeBindings(body, content, &bindings)
			}
		case "function_declaration":
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalTypeBindings(body, content, &bindings)
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(pf.Root)

	return func(leafStart uint32) (string, bool) {
		recvName := selectorReceiverIdent(content, leafStart)
		if recvName == "" {
			return "", false
		}
		// Prefer the innermost binding covering leafStart.
		var best *identTypeBinding
		for i := range bindings {
			b := &bindings[i]
			if b.name != recvName {
				continue
			}
			if leafStart < b.start || leafStart >= b.end {
				continue
			}
			if best == nil || (b.end-b.start) < (best.end-best.start) {
				best = b
			}
		}
		if best == nil {
			return "", false
		}
		return best.typ, true
	}
}

// selectorReceiverIdent returns the identifier immediately before '.' at leafStart.
func selectorReceiverIdent(content []byte, leafStart uint32) string {
	if leafStart == 0 {
		return ""
	}
	text := string(content)
	dot := int(leafStart) - 1
	if dot < 0 || text[dot] != '.' {
		return ""
	}
	recvEnd := dot
	recvStart := dot - 1
	if recvStart < 0 || !ingest.IsIdentChar(text[recvStart]) {
		return ""
	}
	for recvStart > 0 && ingest.IsIdentChar(text[recvStart-1]) {
		recvStart--
	}
	return text[recvStart:recvEnd]
}

// methodDeclReceiverName returns the receiver parameter identifier, if any.
func methodDeclReceiverName(method *grammar.Node, content []byte) string {
	recv := ingest.ChildByField(method, "receiver")
	if recv == nil {
		return ""
	}
	var nameNode *grammar.Node
	var findName func(n *grammar.Node)
	findName = func(n *grammar.Node) {
		if n == nil || nameNode != nil {
			return
		}
		// parameter_declaration name field, or first identifier before the type.
		if n.Type() == "parameter_declaration" {
			if nm := ingest.ChildByField(n, "name"); nm != nil {
				nameNode = nm
				return
			}
			// Some grammars expose name as identifier child.
			for i := uint32(0); i < n.ChildCount(); i++ {
				c := n.Child(i)
				if c.Type() == "identifier" {
					nameNode = c
					return
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			findName(n.Child(i))
		}
	}
	findName(recv)
	if nameNode == nil {
		return ""
	}
	return ingest.NodeText(nameNode, content)
}

// methodDeclReceiverType returns the type name of a method_declaration receiver
// (pointer star stripped), e.g. "*claudeTool" / "claudeTool" → "claudeTool".
func methodDeclReceiverType(method *grammar.Node, content []byte) string {
	recv := ingest.ChildByField(method, "receiver")
	if recv == nil {
		return ""
	}
	// receiver is parameter_list → parameter_declaration → type
	var typeNode *grammar.Node
	var findType func(n *grammar.Node)
	findType = func(n *grammar.Node) {
		if n == nil || typeNode != nil {
			return
		}
		if n.Type() == "type_identifier" || n.Type() == "pointer_type" || n.Type() == "generic_type" {
			// Prefer the named type identifier under the type node.
			if n.Type() == "type_identifier" {
				typeNode = n
				return
			}
			for i := uint32(0); i < n.ChildCount(); i++ {
				findType(n.Child(i))
			}
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			findType(n.Child(i))
		}
	}
	findType(recv)
	if typeNode == nil {
		return ""
	}
	return strings.TrimPrefix(ingest.NodeText(typeNode, content), "*")
}

// collectLocalTypeBindings appends simple local typed bindings under node
// (short_var_declaration / var_spec) into bindings with the body's end as scope.
func collectLocalTypeBindings(node *grammar.Node, content []byte, bindings *[]identTypeBinding) {
	if node == nil {
		return
	}
	scopeEnd := node.EndByte()
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "short_var_declaration":
			// left := right — extract names from left, type from right (&T{} / T{}).
			names := identListNames(ingest.ChildByField(n, "left"), content)
			typ := typeNameFromRHS(ingest.ChildByField(n, "right"), content)
			if typ != "" {
				for _, name := range names {
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, typ})
				}
			}
		case "var_spec":
			names := identListNames(ingest.ChildByField(n, "name"), content)
			if len(names) == 0 {
				// name may be a single identifier field.
				if nm := ingest.ChildByField(n, "name"); nm != nil && nm.Type() == "identifier" {
					names = []string{ingest.NodeText(nm, content)}
				}
			}
			typ := typeNameFromTypeNode(ingest.ChildByField(n, "type"), content)
			if typ == "" {
				typ = typeNameFromRHS(ingest.ChildByField(n, "value"), content)
			}
			if typ != "" {
				for _, name := range names {
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, typ})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

func identListNames(n *grammar.Node, content []byte) []string {
	if n == nil {
		return nil
	}
	var names []string
	if n.Type() == "identifier" {
		return []string{ingest.NodeText(n, content)}
	}
	var walk func(x *grammar.Node)
	walk = func(x *grammar.Node) {
		if x == nil {
			return
		}
		if x.Type() == "identifier" {
			names = append(names, ingest.NodeText(x, content))
			return
		}
		for i := uint32(0); i < x.ChildCount(); i++ {
			walk(x.Child(i))
		}
	}
	walk(n)
	return names
}

// typeNameFromRHS extracts T from &T{}, T{}, new(T), (*T)(nil)-ish forms.
func typeNameFromRHS(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	// expression_list → first child
	if n.Type() == "expression_list" && n.ChildCount() > 0 {
		return typeNameFromRHS(n.Child(0), content)
	}
	switch n.Type() {
	case "unary_expression":
		// &T{}
		for i := uint32(0); i < n.ChildCount(); i++ {
			if t := typeNameFromRHS(n.Child(i), content); t != "" {
				return t
			}
		}
	case "composite_literal":
		if t := ingest.ChildByField(n, "type"); t != nil {
			return typeNameFromTypeNode(t, content)
		}
	case "call_expression":
		// new(T) / make — function is "new"
		if fn := ingest.ChildByField(n, "function"); fn != nil && ingest.NodeText(fn, content) == "new" {
			if args := ingest.ChildByField(n, "arguments"); args != nil && args.ChildCount() > 0 {
				// argument_list children
				for i := uint32(0); i < args.ChildCount(); i++ {
					if t := typeNameFromTypeNode(args.Child(i), content); t != "" {
						return t
					}
				}
			}
		}
	case "type_conversion_expression", "type_assertion_expression":
		if t := ingest.ChildByField(n, "type"); t != nil {
			return typeNameFromTypeNode(t, content)
		}
	}
	return typeNameFromTypeNode(n, content)
}

func typeNameFromTypeNode(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	if n.Type() == "type_identifier" {
		return strings.TrimPrefix(ingest.NodeText(n, content), "*")
	}
	if n.Type() == "pointer_type" || n.Type() == "generic_type" || n.Type() == "qualified_type" {
		var id *grammar.Node
		var find func(x *grammar.Node)
		find = func(x *grammar.Node) {
			if x == nil || id != nil {
				return
			}
			if x.Type() == "type_identifier" {
				id = x
				return
			}
			for i := uint32(0); i < x.ChildCount(); i++ {
				find(x.Child(i))
			}
		}
		find(n)
		if id != nil {
			return strings.TrimPrefix(ingest.NodeText(id, content), "*")
		}
	}
	// Search shallowly for a type_identifier.
	for i := uint32(0); i < n.ChildCount(); i++ {
		if t := typeNameFromTypeNode(n.Child(i), content); t != "" {
			return t
		}
	}
	return ""
}

// interfaceNamesWithMethod returns interface type names in ourPkgDirs that
// declare a method named oldLeaf.
func interfaceNamesWithMethod(rootDir string, result *ingest.Result, ourPkgDirs map[string]bool, oldLeaf string) []string {
	seen := map[string]bool{}
	var names []string
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		if !ourPkgDirs[dirOf(rel)] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, name := range interfaceTypeNamesWithMethod(content, oldLeaf) {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

func interfaceTypeNamesWithMethod(content []byte, methodLeaf string) []string {
	pf, err := ingest.ParseSource(content, ".go", "")
	if err != nil {
		return nil
	}
	defer pf.Close()
	treeRoot := pf.Root
	var names []string
	var walk func(n *grammar.Node, ifaceName string, inIface bool)
	walk = func(n *grammar.Node, ifaceName string, inIface bool) {
		if n == nil {
			return
		}
		nextName, nextIface := ifaceName, inIface
		if n.Type() == "type_spec" {
			if id := ingest.ChildByType(n, "type_identifier"); id != nil {
				nextName = ingest.NodeText(id, content)
			}
			if t := ingest.ChildByField(n, "type"); t != nil && t.Type() == "interface_type" {
				nextIface = true
			}
		}
		if inIface && (n.Type() == "method_elem" || n.Type() == "field_declaration") {
			if name := ingest.ChildByField(n, "name"); name != nil && ingest.NodeText(name, content) == methodLeaf {
				if ifaceName != "" {
					names = append(names, ifaceName)
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), nextName, nextIface)
		}
	}
	walk(treeRoot, "", false)
	return names
}

// varsTypedAsImported returns identifiers in content whose type is written as
// importLocal.TypeName or *importLocal.TypeName (e.g. var d a.Driver).
func varsTypedAsImported(content []byte, importLocal, typeName string) map[string]bool {
	names := map[string]bool{}
	if importLocal == "" || typeName == "" {
		return names
	}
	text := string(content)
	inString := buildStringLiteralMask(text)
	for _, needle := range []string{importLocal + "." + typeName, "*" + importLocal + "." + typeName} {
		off := 0
		for {
			idx := strings.Index(text[off:], needle)
			if idx < 0 {
				break
			}
			pos := off + idx
			if inString[pos] {
				off = pos + len(needle)
				continue
			}
			j := pos - 1
			for j >= 0 && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n') {
				j--
			}
			end := j + 1
			for j >= 0 && ingest.IsIdentChar(text[j]) {
				j--
			}
			name := text[j+1 : end]
			switch name {
			case "", "var", "const", "type", "func", "return", "range", "map", "chan", "struct", "interface":
			default:
				names[name] = true
			}
			off = pos + len(needle)
		}
	}
	return names
}

func fileImportsAnyPackage(content []byte, pkgDirs map[string]bool) bool {
	return len(importLocalsForPackages(content, pkgDirs)) > 0
}

func importLocalsForPackages(content []byte, pkgDirs map[string]bool) map[string]bool {
	locals := map[string]bool{}
	if len(pkgDirs) == 0 {
		return locals
	}
	for _, spec := range parseGoImportSpecs(content) {
		if spec.local == "" || spec.local == "." || spec.local == "_" {
			continue
		}
		for dir := range pkgDirs {
			if spec.path == dir || strings.HasSuffix(spec.path, "/"+dir) {
				locals[spec.local] = true
				break
			}
		}
	}
	return locals
}

func findSelectorLeafEdits(file string, content []byte, oldLeaf, newLeaf string, allowedReceivers map[string]bool) []ingest.Edit {
	text := string(content)
	inString := buildStringLiteralMask(text)
	needle := "." + oldLeaf
	var edits []ingest.Edit
	off := 0
	for {
		idx := strings.Index(text[off:], needle)
		if idx < 0 {
			break
		}
		dot := off + idx
		start := dot + 1
		end := start + len(oldLeaf)
		if end < len(text) && ingest.IsIdentChar(text[end]) {
			off = end
			continue
		}
		if inString[dot] || inString[start] {
			off = end
			continue
		}
		// Receiver must be an identifier (pkg.Leaf / recv.Leaf), not Type{}.Leaf.
		recvStart := dot - 1
		if recvStart < 0 || !ingest.IsIdentChar(text[recvStart]) {
			off = end
			continue
		}
		for recvStart > 0 && ingest.IsIdentChar(text[recvStart-1]) {
			recvStart--
		}
		receiver := text[recvStart:dot]
		if allowedReceivers != nil && !allowedReceivers[receiver] {
			off = end
			continue
		}
		edits = append(edits, ingest.Edit{
			File:      file,
			StartByte: uint32(start),
			EndByte:   uint32(end),
			NewText:   newLeaf,
		})
		off = end
	}
	return edits
}

func buildStringLiteralMask(text string) []bool {
	mask := make([]bool, len(text))
	i := 0
	for i < len(text) {
		// Skip comments so apostrophes in prose (doesn't) are not char literals.
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '/' {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '*' {
			i += 2
			for i+1 < len(text) && !(text[i] == '*' && text[i+1] == '/') {
				i++
			}
			if i+1 < len(text) {
				i += 2
			}
			continue
		}
		switch text[i] {
		case '"':
			mask[i] = true
			i++
			for i < len(text) {
				mask[i] = true
				if text[i] == '\\' && i+1 < len(text) {
					mask[i+1] = true
					i += 2
					continue
				}
				if text[i] == '"' {
					i++
					break
				}
				i++
			}
		case '`':
			mask[i] = true
			i++
			for i < len(text) {
				mask[i] = true
				if text[i] == '`' {
					i++
					break
				}
				i++
			}
		case '\'':
			// Go character literals are short; bound scan to avoid comment apostrophes
			// that slipped through.
			start := i
			mask[i] = true
			i++
			if i < len(text) && text[i] == '\\' && i+1 < len(text) {
				mask[i] = true
				mask[i+1] = true
				i += 2
			} else if i < len(text) {
				mask[i] = true
				i++
			}
			if i < len(text) && text[i] == '\'' {
				mask[i] = true
				i++
			} else {
				// Invalid/incomplete rune; do not treat following code as a string.
				for j := start; j < i && j < len(mask); j++ {
					mask[j] = false
				}
			}
		default:
			i++
		}
	}
	return mask
}

// findInterfaceMethodEdits renames method oldLeaf→newLeaf on every interface
// declaration in the file (all in-scope interfaces that declare the method).
func findInterfaceMethodEdits(file string, content []byte, oldLeaf, newLeaf string) []ingest.Edit {
	pf, err := ingest.ParseSource(content, file, "")
	if err != nil {
		return nil
	}
	defer pf.Close()
	treeRoot := pf.Root
	var edits []ingest.Edit
	var walk func(n *grammar.Node, inIface bool)
	walk = func(n *grammar.Node, inIface bool) {
		if n == nil {
			return
		}
		nextIface := inIface
		switch n.Type() {
		case "type_spec":
			if t := ingest.ChildByField(n, "type"); t != nil && t.Type() == "interface_type" {
				nextIface = true
			}
		case "method_elem", "field_declaration":
			if inIface {
				if name := ingest.ChildByField(n, "name"); name != nil {
					if ingest.NodeText(name, content) == oldLeaf {
						edits = append(edits, ingest.Edit{
							File:      file,
							StartByte: name.StartByte(),
							EndByte:   name.EndByte(),
							NewText:   newLeaf,
						})
					}
				}
				for i := uint32(0); i < n.ChildCount(); i++ {
					ch := n.Child(i)
					if ch == nil {
						continue
					}
					if (ch.Type() == "field_identifier" || ch.Type() == "identifier") &&
						ingest.NodeText(ch, content) == oldLeaf {
						edits = append(edits, ingest.Edit{
							File:      file,
							StartByte: ch.StartByte(),
							EndByte:   ch.EndByte(),
							NewText:   newLeaf,
						})
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), nextIface)
		}
	}
	walk(treeRoot, false)
	return edits
}
