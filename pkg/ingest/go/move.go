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
	source, err := os.ReadFile(filePath)
	if err != nil {
		return ingest.DeclExtract{}, err
	}

	lang, ok := grammar.GetByExtension(filePath)
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
		// Grouped type declaration: extract just the matching type_spec.
		// The output should be a standalone "type X struct {...}" declaration.
		// Dedent by one tab level since the spec was inside type (...).
		spec := result.TypeSpec
		specText := string(source[spec.StartByte():spec.EndByte()])
		declText = "type " + dedentOnce(specText)
		removeStart = spec.StartByte()
		removeEnd = spec.EndByte()
		// Remove trailing whitespace/newlines up to the next type_spec or ')'.
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
				NewText:   appendDeclText(merged, decl.DeclText),
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
			pkgName = lastPathComponent(strings.TrimSuffix(dstRelPath, ".go"))
			if i := strings.LastIndex(pkgName, "/"); i >= 0 {
				pkgName = pkgName[i+1:]
			}
		}
		body := fmt.Sprintf("package %s\n", pkgName)
		if len(decl.Imports) > 0 {
			body = ensureGoImports(body, decl.Imports)
		}
		insertText = appendDeclText(body, decl.DeclText)
	}

	return ingest.Edit{
		File:      dstRelPath,
		StartByte: insertAt,
		EndByte:   insertAt,
		NewText:   insertText,
	}
}

func appendDeclText(content, declText string) string {
	out := content
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out += "\n"
	}
	if len(out) > 0 {
		out += "\n"
	}
	return out + declText + "\n"
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
		local = lastPathComponent(p)
	}
	return goImportSpec{local: local, path: p}, true
}

func goIdentUsed(text, ident string) bool {
	if ident == "" {
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
		if pos > 0 && isIdentChar(text[pos-1]) {
			off = end
			continue
		}
		if end < len(text) && isIdentChar(text[end]) {
			off = end
			continue
		}
		return true
	}
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
		oldBase := lastPathComponent(oldDir)
		newBase := lastPathComponent(newDir)
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
		oldQual := lastPathComponent(oldDir)
		newQual := lastPathComponent(newDir)
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
		if pos > 0 && isIdentChar(text[pos-1]) {
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

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
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

// goDeclResult holds the matched declaration node and, for grouped type
// declarations, the individual type_spec that matched (when the declaration
// contains multiple specs).
type goDeclResult struct {
	Node     *grammar.Node // the top-level declaration or type_spec
	Grouped  bool          // true when part of a type (...) group
	TypeSpec *grammar.Node // non-nil for grouped type declarations
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
					return &goDeclResult{Node: child, Grouped: true, TypeSpec: matchedSpec}
				}
				return &goDeclResult{Node: child}
			}
		}
	}
	return nil
}

func (moveDriver) ExpandRenameSources(result *ingest.Result, sourceRef string) []string {
	src := ingest.ParseReference(sourceRef)
	if src.Symbol == "" {
		return nil
	}
	leaf := symbolLeaf(src.Symbol)
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
		entLeaf := symbolLeaf(ref.Symbol)
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
				// Facade function only in the parent package dir (e.g. wallpaper.SetStatic).
				if entPkgDir != scopePrefix {
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

func symbolLeaf(symbol string) string {
	leaf := symbol
	if i := strings.LastIndex(leaf, "."); i >= 0 {
		leaf = leaf[i+1:]
	}
	return strings.TrimPrefix(leaf, "*")
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

	leaf := symbolLeaf(src.Symbol)
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

	newQual := lastPathComponent(newDir)
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

// methodReceiverType returns the type name for a method symbol like
// "*Session.Close" or "Session.Group", or "" if not a method symbol.
func methodReceiverType(symbol string) string {
	if symbol == "" || !strings.Contains(symbol, ".") {
		return ""
	}
	recv, _, ok := strings.Cut(symbol, ".")
	if !ok || recv == "" {
		return ""
	}
	return strings.TrimPrefix(recv, "*")
}

// packageLocalDepInDecl returns the first package-scope symbol in pkgDir (other
// than movedLeaf) whose identifier appears in declText, or "".
func packageLocalDepInDecl(result *ingest.Result, pkgDir, movedLeaf, declText string) string {
	if result == nil || declText == "" {
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
		return nil
	}
	scopePrefix := methodRenameScopePrefix(strings.TrimPrefix(src.Path, "./"))
	sourceSet := map[string]bool{}
	pkgDirs := map[string]bool{}
	ourPkgDirs := map[string]bool{}
	srcPkgDir := dirOf(strings.TrimPrefix(src.Path, "./"))
	if srcPkgDir != "" {
		ourPkgDirs[srcPkgDir] = true
	}
	if scopePrefix != "" {
		ourPkgDirs[scopePrefix] = true
	}
	for _, s := range sourceRefs {
		sourceSet[s] = true
		ref := ingest.ParseReference(s)
		rel := strings.TrimPrefix(ref.Path, "./")
		if d := dirOf(rel); d != "" {
			pkgDirs[d] = true
			ourPkgDirs[d] = true
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
		for _, e := range selectorEdits {
			if occ[[2]uint32{e.StartByte, e.EndByte}] {
				continue
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
	lang, ok := grammar.GetByExtension(".go")
	if !ok {
		return nil
	}
	parser := grammar.NewParser()
	defer parser.Delete()
	if !parser.SetLanguage(lang) {
		return nil
	}
	tree := parser.ParseString(string(content))
	defer tree.Delete()
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
	walk(tree.RootNode(), "", false)
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
			for j >= 0 && isIdentChar(text[j]) {
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
		if end < len(text) && isIdentChar(text[end]) {
			off = end
			continue
		}
		if inString[dot] || inString[start] {
			off = end
			continue
		}
		// Receiver must be an identifier (pkg.Leaf / recv.Leaf), not Type{}.Leaf.
		recvStart := dot - 1
		if recvStart < 0 || !isIdentChar(text[recvStart]) {
			off = end
			continue
		}
		for recvStart > 0 && isIdentChar(text[recvStart-1]) {
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
	lang, ok := grammar.GetByExtension(file)
	if !ok {
		return nil
	}
	parser := grammar.NewParser()
	defer parser.Delete()
	if !parser.SetLanguage(lang) {
		return nil
	}
	tree := parser.ParseString(string(content))
	defer tree.Delete()
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
	walk(tree.RootNode(), false)
	return edits
}
