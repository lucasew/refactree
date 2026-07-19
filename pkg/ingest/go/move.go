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

func (moveDriver) ExpandRenameSources(rootDir string, result *ingest.Result, sourceRef string) []string {
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
	// Interface method → expand to same-leaf methods (implementors + sibling ifaces)
	// in the package tree so concrete defs and call sites rename together.
	ifaceExpand := isMethod && goTypeIsInterface(rootDir, result, srcPkgDir, recv)
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
				if ifaceExpand {
					// Any Type.leaf in the package / sibling scope under the iface tree.
					if entPkgDir != srcPkgDir &&
						(scopePrefix == "" || (entPkgDir != scopePrefix && !strings.HasPrefix(rel, scopePrefix+"/"))) {
						continue
					}
				} else if entRecv != recv {
					continue
				} else if entPkgDir != srcPkgDir &&
					(scopePrefix == "" || (entPkgDir != scopePrefix && !strings.HasPrefix(rel, scopePrefix+"/"))) {
					// Same impl package or sibling packages under the interface tree.
					continue
				}
			} else if ref.Symbol == leaf {
				// Facade free function in the interface package (e.g. wallpaper.SetStatic).
				// Bare path entities also cover types/vars/consts with the same leaf
				// (type Helper vs Box.Helper) — only free funcs may co-rename.
				if entPkgDir != scopePrefix {
					continue
				}
				if !goEntityIsFreeFunc(rootDir, ent) {
					continue
				}
				// Concrete methods only absorb facades in a *parent* package
				// (implementor in feh → facade in wallpaper). Same-package bare
				// names stay for interface-method renames so we do not rename
				// unrelated funcs that share the method leaf.
				if !ifaceExpand && entPkgDir == srcPkgDir {
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

// goEntityIsFreeFunc reports whether ent is a package-level function (no receiver),
// as opposed to a type/var/const/method that shares a bare symbol leaf.
func goEntityIsFreeFunc(rootDir string, ent ingest.Entity) bool {
	ref := ingest.ParseReference(ent.Reference)
	if ref.Symbol == "" || strings.Contains(ref.Symbol, ".") {
		return false
	}
	rel := strings.TrimPrefix(ref.Path, "./")
	content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	pf, err := ingest.ParseSource(content, rel, "go")
	if err != nil {
		return false
	}
	defer pf.Close()
	d := findGoDecl(pf.Root, ent.StartByte)
	if d == nil || d.Node == nil {
		return false
	}
	// tree-sitter-go uses function_declaration for both funcs and methods;
	// methods carry a receiver field.
	if d.Node.Type() != "function_declaration" && d.Node.Type() != "method_declaration" {
		return false
	}
	return ingest.ChildByField(d.Node, "receiver") == nil
}

// goTypeIsInterface reports whether typeName is declared as an interface in pkgDir.
func goTypeIsInterface(rootDir string, result *ingest.Result, pkgDir, typeName string) bool {
	if typeName == "" || result == nil {
		return false
	}
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		if dirOf(rel) != pkgDir {
			continue
		}
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if typeIsInterfaceInFile(content, typeName) {
			return true
		}
	}
	return false
}

// typeIsInterfaceInFile reports whether typeName is declared as interface in content.
// Covers both `type T interface {…}` (type_spec) and `type T = interface {…}` (type_alias).
func typeIsInterfaceInFile(content []byte, typeName string) bool {
	if typeName == "" {
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
		switch n.Type() {
		case "type_spec":
			id := ingest.ChildByType(n, "type_identifier")
			if id != nil && ingest.NodeText(id, content) == typeName {
				if t := ingest.ChildByField(n, "type"); t != nil && t.Type() == "interface_type" {
					found = true
					return
				}
			}
		case "type_alias":
			// First type_identifier is the alias name; RHS may be interface_type.
			var nameNode *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "type_identifier" {
					nameNode = ch
					break
				}
			}
			if nameNode != nil && ingest.NodeText(nameNode, content) == typeName {
				for i := uint32(0); i < n.ChildCount(); i++ {
					ch := n.Child(i)
					if ch == nameNode {
						continue
					}
					if ch.Type() == "interface_type" {
						found = true
						return
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(pf.Root)
	return found
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
	// Entity symbols for generic methods use Box[T] / Map[K,V]; AST type
	// analysis and local bindings use the bare name Box / Map. Normalize so
	// ExtraRename does not treat the same receiver as foreign.
	recv = stripGoTypeParams(recv)
	if recv == "" {
		return "", false
	}
	return recv, true
}

// stripGoTypeParams removes a trailing [type-args] suffix: "Box[T]" → "Box",
// "Map[K,V]" → "Map". Nested brackets are balanced; no-bracket names pass through.
func stripGoTypeParams(name string) string {
	i := strings.IndexByte(name, '[')
	if i < 0 {
		return name
	}
	return name[:i]
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
	ingest.MaskNonNewlinesInPlace(masked, int(decl.RemoveStart), int(decl.RemoveEnd))
	seenBlocks := map[int]bool{}
	for _, spec := range specs {
		if spec.blockStart >= 0 && spec.blockEnd > 0 {
			if !seenBlocks[spec.blockStart] {
				ingest.MaskNonNewlinesInPlace(masked, spec.blockStart, spec.blockEnd)
				seenBlocks[spec.blockStart] = true
			}
			continue
		}
		ingest.MaskNonNewlinesInPlace(masked, spec.lineStart, spec.lineEnd)
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
	// Interface method entities (Type.Method on interface types) are co-renamed
	// via findInterfaceMethodEdits, not competing foreign receivers — treating
	// them as foreign would skip interface-typed selectors (var d Driver; d.M()).
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
			entPkg := dirOf(strings.TrimPrefix(ref.Path, "./"))
			if goTypeIsInterface(rootDir, result, entPkg, recv) {
				continue
			}
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

	occupied := ingest.MarkEntityRelationSpans(result, sourceSet)
	relatedFiles := map[string]bool{}
	for _, rel := range result.Relations {
		if !sourceSet[rel.Target] {
			continue
		}
		ref := ingest.ParseReference(rel.Reference)
		relatedFiles[strings.TrimPrefix(ref.Path, "./")] = true
	}
	for _, s := range sourceRefs {
		ref := ingest.ParseReference(s)
		relatedFiles[strings.TrimPrefix(ref.Path, "./")] = true
	}

	// Receivers among our rename targets that do not also declare a method with
	// this leaf — composite literal keys only apply to pure field renames.
	fieldReceivers := map[string]bool{}
	for recv := range ourReceivers {
		fieldReceivers[recv] = true
	}
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		if !ourPkgDirs[dirOf(rel)] {
			continue
		}
		c, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, r := range methodReceiversWithLeaf(c, oldLeaf) {
			delete(fieldReceivers, r)
		}
	}

	var edits []ingest.Edit
	for _, f := range result.Files {
		if f.Language != "go" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		entPkg := dirOf(rel)
		inOurPkg := ourPkgDirs[entPkg]
		samePkg := inOurPkg || relatedFiles[rel]
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		importsRelated := fileImportsAnyPackage(content, pkgDirs)
		if !samePkg && !importsRelated {
			continue
		}
		occ := occupied[rel]
		var selectorEdits []ingest.Edit
		if samePkg {
			// Same package tree: rename ident.Leaf only (not Type{}.Leaf).
			selectorEdits = findSelectorLeafEdits(rel, content, oldLeaf, newLeaf, nil)
		} else {
			// External importers: package qualifiers for our pkgs plus variables
			// typed as imported interfaces that carry this method (var d a.Driver)
			// or as our concrete receiver types (b pkga.Box / b := pkga.Box{}).
			allowed := importAllowedReceivers(content, pkgDirs, ourPkgDirs, ourReceivers, rootDir, result, oldLeaf)
			selectorEdits = findSelectorLeafEdits(rel, content, oldLeaf, newLeaf, allowed)
		}
		// Filter same-package renames by the *call target* type (selector receiver),
		// not the enclosing method's receiver — so package-level helpers and
		// cross-type calls inside foreign methods are handled correctly.
		var callTargetType func(leafStart uint32) (string, bool)
		if len(foreignReceivers) > 0 && samePkg {
			callTargetType = selectorCallTargetTypeFunc(content)
		}
		for _, e := range selectorEdits {
			if ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
				continue
			}
			if callTargetType != nil && goSelectorTargetsForeign(content, e.StartByte, callTargetType, ourReceivers, foreignReceivers) {
				continue
			}
			edits = appendUnoccupied(edits, occ, e)
		}
		if samePkg {
			// Struct field renames: Type{OldLeaf: …} composite literal keys.
			if len(fieldReceivers) > 0 {
				edits = appendUnoccupiedAll(edits, occ, findCompositeFieldKeyEdits(rel, content, oldLeaf, newLeaf, fieldReceivers))
			}
			// Non-identifier receivers: Type{}.M, v.(T).M, xs[i].M, Make().M, (*T).M.
			// Identifier receivers are handled by findSelectorLeafEdits.
			edits = appendUnoccupiedAll(edits, occ, findComplexOperandSelectorEdits(rel, content, oldLeaf, newLeaf, ourReceivers, foreignReceivers))
		}
		if inOurPkg {
			// Interface method decls: do not mark occupied (matches prior behavior).
			for _, e := range findInterfaceMethodEdits(rel, content, oldLeaf, newLeaf) {
				if !ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
					edits = append(edits, e)
				}
			}
		}
	}
	return edits
}

// appendUnoccupied appends e when its span is free, and marks the span occupied.
func appendUnoccupied(edits []ingest.Edit, occ map[[2]uint32]bool, e ingest.Edit) []ingest.Edit {
	if ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
		return edits
	}
	if occ != nil {
		occ[[2]uint32{e.StartByte, e.EndByte}] = true
	}
	return append(edits, e)
}

func appendUnoccupiedAll(edits []ingest.Edit, occ map[[2]uint32]bool, candidates []ingest.Edit) []ingest.Edit {
	for _, e := range candidates {
		edits = appendUnoccupied(edits, occ, e)
	}
	return edits
}

// goSelectorTargetsForeign reports whether the selector at leafStart targets a
// foreign concrete receiver (skip rewrite).
func goSelectorTargetsForeign(content []byte, leafStart uint32, callTargetType func(uint32) (string, bool), ourReceivers, foreignReceivers map[string]bool) bool {
	if typ, ok := callTargetType(leafStart); ok {
		typ = stripGoTypeParams(strings.TrimPrefix(typ, "*"))
		return foreignReceivers[typ] && !ourReceivers[typ]
	}
	if recv := selectorReceiverIdent(content, leafStart); recv != "" {
		return foreignReceivers[recv] && !ourReceivers[recv]
	}
	return false
}

// importAllowedReceivers builds the allowed selector-receiver set for a file
// that imports our packages: import locals, vars typed as importLocal.Iface /
// importLocal.Recv, and short composite assigns (b := pkga.Box{}).
func importAllowedReceivers(content []byte, pkgDirs, ourPkgDirs, ourReceivers map[string]bool, rootDir string, result *ingest.Result, oldLeaf string) map[string]bool {
	importLocals := importLocalsForPackages(content, pkgDirs)
	allowed := make(map[string]bool, len(importLocals))
	for local := range importLocals {
		allowed[local] = true
	}
	ifaceNames := interfaceNamesWithMethod(rootDir, result, ourPkgDirs, oldLeaf)
	for local := range importLocals {
		for _, iface := range ifaceNames {
			for name := range varsTypedAsImported(content, local, iface) {
				allowed[name] = true
			}
		}
		for recv := range ourReceivers {
			recv = strings.TrimPrefix(recv, "*")
			if recv == "" {
				continue
			}
			for name := range varsTypedAsImported(content, local, recv) {
				allowed[name] = true
			}
			for name := range varsAssignedImportedComposite(content, local, recv) {
				allowed[name] = true
			}
		}
	}
	return allowed
}

// findCompositeFieldKeyEdits rewrites OldLeaf keys in composite literals whose
// type is one of ourReceivers (e.g. Box{Helper: 1} when renaming Box.Helper).
func findCompositeFieldKeyEdits(file string, content []byte, oldLeaf, newLeaf string, ourReceivers map[string]bool) []ingest.Edit {
	if oldLeaf == "" || len(ourReceivers) == 0 {
		return nil
	}
	pf, err := ingest.ParseSource(content, file, "go")
	if err != nil {
		return nil
	}
	defer pf.Close()

	var edits []ingest.Edit
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "composite_literal" {
			typ := compositeLiteralTypeName(n, content)
			if typ != "" && ourReceivers[typ] {
				var body *grammar.Node
				for i := uint32(0); i < n.ChildCount(); i++ {
					c := n.Child(i)
					if c.Type() == "literal_value" {
						body = c
						break
					}
				}
				if body != nil {
					collectCompositeKeyEdits(body, content, file, oldLeaf, newLeaf, &edits)
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(pf.Root)
	return edits
}

func collectCompositeKeyEdits(n *grammar.Node, content []byte, file, oldLeaf, newLeaf string, edits *[]ingest.Edit) {
	if n == nil || n.IsNull() {
		return
	}
	if n.Type() == "keyed_element" {
		// key: value — key is field_identifier, identifier, or literal_element wrapping one.
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c.Type() == ":" {
				break
			}
			keyNode := compositeKeyIdent(c)
			if keyNode == nil {
				continue
			}
			if ingest.NodeText(keyNode, content) == oldLeaf {
				*edits = append(*edits, ingest.Edit{
					File:      file,
					StartByte: keyNode.StartByte(),
					EndByte:   keyNode.EndByte(),
					NewText:   newLeaf,
				})
			}
			break
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		collectCompositeKeyEdits(n.Child(i), content, file, oldLeaf, newLeaf, edits)
	}
}

func compositeKeyIdent(n *grammar.Node) *grammar.Node {
	if n == nil {
		return nil
	}
	switch n.Type() {
	case "field_identifier", "identifier":
		return n
	case "literal_element":
		for i := uint32(0); i < n.ChildCount(); i++ {
			if id := compositeKeyIdent(n.Child(i)); id != nil {
				return id
			}
		}
	}
	return nil
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
	occupied := ingest.MarkEntityRelationSpans(result, sourceSet)

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
			if !ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
				edits = append(edits, e)
			}
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
// '.' in recv.Leaf). Resolves method receiver params, function/method params,
// range variables, and simple local bindings (t := &T{}, var t *T, …).
// ok=false when the type cannot be determined.
func selectorCallTargetTypeFunc(content []byte) func(leafStart uint32) (string, bool) {
	var bindings []identTypeBinding
	pf, err := ingest.ParseSource(content, ".go", "")
	if err != nil {
		return func(uint32) (string, bool) { return "", false }
	}
	defer pf.Close()

	// Same-file function result types for multi-return short-var typing:
	// a, b := makeAB() with makeAB() (*A, *B) binds a→A, b→B so foreign same-leaf
	// methods on b are not fail-open rewritten.
	funcResults := sameFileFuncResultTypes(pf.Root, content)
	genericPeels := sameFileGenericPeelFuncs(pf.Root, content)
	// Named collection types: type AS []*A / type AM = map[K]*A — param peels.
	namedColl := sameFileNamedCollectionResults(pf.Root, content)

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
			rangeSrc := map[string]rangeSourceInfo{}
			if params := ingest.ChildByField(n, "parameters"); params != nil {
				collectParameterListBindings(params, content, n.StartByte(), n.EndByte(), &bindings, rangeSrc, namedColl)
			}
			// Named results: func (r *T) M() (a *A, b *B) — a/b used in body.
			collectResultParameterBindings(n, content, &bindings, rangeSrc, namedColl)
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalTypeBindings(body, content, &bindings, rangeSrc, funcResults, genericPeels, namedColl)
			}
		case "function_declaration":
			rangeSrc := map[string]rangeSourceInfo{}
			if params := ingest.ChildByField(n, "parameters"); params != nil {
				collectParameterListBindings(params, content, n.StartByte(), n.EndByte(), &bindings, rangeSrc, namedColl)
			}
			// Named results: func Use() (a *A, b *B) { a.Run(); b.Run() }.
			// Without this, same-leaf foreign methods are fail-open rewritten.
			collectResultParameterBindings(n, content, &bindings, rangeSrc, namedColl)
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalTypeBindings(body, content, &bindings, rangeSrc, funcResults, genericPeels, namedColl)
			}
		case "func_literal":
			// Nested / package-level func literals: func(a *A, b *B) { a.Run(); b.Run() }.
			// Without param bindings, foreign same-leaf call sites are rewritten (fail-open).
			rangeSrc := map[string]rangeSourceInfo{}
			if params := ingest.ChildByField(n, "parameters"); params != nil {
				collectParameterListBindings(params, content, n.StartByte(), n.EndByte(), &bindings, rangeSrc, namedColl)
			}
			collectResultParameterBindings(n, content, &bindings, rangeSrc, namedColl)
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalTypeBindings(body, content, &bindings, rangeSrc, funcResults, genericPeels, namedColl)
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

// rangeSourceInfo describes how to type range variables and channel receives
// over a named collection.
type rangeSourceInfo struct {
	elemType string // element (slice/array/chan) or map value named type
	keyType  string // map key named type (only when isMap)
	isMap    bool
	// isArray is true when the collection is (or is a named alias of) an array
	// type. Used so new(AA) peels under foreign same-leaf while new(AS) for a
	// named slice does not (result is *[]T, not indexable without deref).
	isArray bool
}

// collectResultParameterBindings records named result params from a
// function/method/func-literal declaration (result field is a parameter_list).
// Unnamed results (func f() *A) have no identifiers and are skipped.
// namedColl may be nil (see collectParameterListBindings).
func collectResultParameterBindings(decl *grammar.Node, content []byte, bindings *[]identTypeBinding, rangeSrc map[string]rangeSourceInfo, namedColl map[string]rangeSourceInfo) {
	if decl == nil {
		return
	}
	result := ingest.ChildByField(decl, "result")
	if result == nil || result.Type() != "parameter_list" {
		return
	}
	collectParameterListBindings(result, content, decl.StartByte(), decl.EndByte(), bindings, rangeSrc, namedColl)
}

// collectParameterListBindings records concrete-typed params (a *A, a, b *A)
// and container params usable as range sources (as []*A, m map[K]*B, c ...T).
// namedColl maps same-file type names (type AS []*A) to collection element
// info so params typed as AS / *AS / AM peel under foreign same-leaf (may be nil).
func collectParameterListBindings(paramList *grammar.Node, content []byte, scopeStart, scopeEnd uint32, bindings *[]identTypeBinding, rangeSrc map[string]rangeSourceInfo, namedColl map[string]rangeSourceInfo) {
	if paramList == nil {
		return
	}
	for i := uint32(0); i < paramList.ChildCount(); i++ {
		p := paramList.Child(i)
		if p == nil {
			continue
		}
		switch p.Type() {
		case "parameter_declaration":
			names := parameterDeclNames(p, content)
			typeN := ingest.ChildByField(p, "type")
			if typ := concreteNamedType(typeN, content); typ != "" {
				for _, name := range names {
					if name == "" || name == "_" {
						continue
					}
					*bindings = append(*bindings, identTypeBinding{scopeStart, scopeEnd, name, typ})
				}
			}
			if info, ok := rangeSourceFromTypeNode(typeN, content); ok {
				for _, name := range names {
					if name == "" || name == "_" {
						continue
					}
					rangeSrc[name] = info
				}
			} else if info, ok := collectionTypeOrNamedResult(typeN, content, namedColl); ok {
				// as AS / pas *AS / am AM with type AS []*A / AM map[K]*A.
				for _, name := range names {
					if name == "" || name == "_" {
						continue
					}
					rangeSrc[name] = info
				}
			}
		case "variadic_parameter_declaration":
			// c ...*T — c is a []T-like range source; element type is T.
			nameN := ingest.ChildByField(p, "name")
			if nameN == nil {
				nameN = ingest.ChildByType(p, "identifier")
			}
			if nameN == nil {
				continue
			}
			name := ingest.NodeText(nameN, content)
			if name == "" || name == "_" {
				continue
			}
			typeN := ingest.ChildByField(p, "type")
			if typ := typeNameFromTypeNode(typeN, content); typ != "" {
				rangeSrc[name] = rangeSourceInfo{elemType: typ, isMap: false}
			} else if info, ok := collectionTypeOrNamedResult(typeN, content, namedColl); ok {
				// c ...AS with type AS []*A — element type is A (same as ...*A).
				rangeSrc[name] = info
			}
		}
	}
}

// parameterDeclNames returns all name identifiers on a parameter_declaration
// (supports `a, b *T` multi-name form).
func parameterDeclNames(p *grammar.Node, content []byte) []string {
	if p == nil {
		return nil
	}
	var names []string
	for i := uint32(0); i < p.ChildCount(); i++ {
		if p.FieldNameForChild(i) != "name" {
			continue
		}
		c := p.Child(i)
		if c != nil && c.Type() == "identifier" {
			names = append(names, ingest.NodeText(c, content))
		}
	}
	if len(names) == 0 {
		if nm := ingest.ChildByField(p, "name"); nm != nil && nm.Type() == "identifier" {
			names = append(names, ingest.NodeText(nm, content))
		}
	}
	return names
}

// varSpecNames returns all name identifiers on a var_spec (supports `var a, b = …`).
// ChildByField("name") only yields the first repeated name field.
func varSpecNames(spec *grammar.Node, content []byte) []string {
	if spec == nil {
		return nil
	}
	var names []string
	for i := uint32(0); i < spec.ChildCount(); i++ {
		if spec.FieldNameForChild(i) != "name" {
			continue
		}
		c := spec.Child(i)
		if c == nil {
			continue
		}
		if c.Type() == "identifier" {
			names = append(names, ingest.NodeText(c, content))
			continue
		}
		// Some grammars wrap multi-names; fall back to walking identifiers.
		names = append(names, identListNames(c, content)...)
	}
	return names
}

// concreteNamedType returns T for T / *T / pkg.T / T[K], empty for slice/map/chan.
func concreteNamedType(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "type_identifier", "pointer_type", "generic_type", "qualified_type":
		return typeNameFromTypeNode(n, content)
	default:
		return ""
	}
}

// rangeSourceFromTypeNode reports element/value (and map key) type for ranging
// over a typed collection, or for receiving from a typed channel.
func rangeSourceFromTypeNode(n *grammar.Node, content []byte) (rangeSourceInfo, bool) {
	if n == nil {
		return rangeSourceInfo{}, false
	}
	switch n.Type() {
	case "parenthesized_type":
		// ([]*T) in type conversions — peel parens to the inner collection type.
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "(" || c.Type() == ")" {
				continue
			}
			return rangeSourceFromTypeNode(c, content)
		}
	case "slice_type":
		if el := ingest.ChildByField(n, "element"); el != nil {
			if typ := typeNameFromTypeNode(el, content); typ != "" {
				return rangeSourceInfo{elemType: typ, isMap: false}, true
			}
		}
	case "array_type":
		if el := ingest.ChildByField(n, "element"); el != nil {
			if typ := typeNameFromTypeNode(el, content); typ != "" {
				return rangeSourceInfo{elemType: typ, isMap: false, isArray: true}, true
			}
		}
	case "map_type":
		info := rangeSourceInfo{isMap: true}
		if v := ingest.ChildByField(n, "value"); v != nil {
			info.elemType = typeNameFromTypeNode(v, content)
		}
		if k := ingest.ChildByField(n, "key"); k != nil {
			info.keyType = typeNameFromTypeNode(k, content)
		}
		if info.elemType != "" || info.keyType != "" {
			return info, true
		}
	case "channel_type":
		// chan T / <-chan T — prefer the value field; else first non-keyword child.
		if v := ingest.ChildByField(n, "value"); v != nil {
			if typ := typeNameFromTypeNode(v, content); typ != "" {
				return rangeSourceInfo{elemType: typ, isMap: false}, true
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "chan" || c.Type() == "<-" {
				continue
			}
			if typ := typeNameFromTypeNode(c, content); typ != "" {
				return rangeSourceInfo{elemType: typ, isMap: false}, true
			}
		}
	case "pointer_type":
		// *[]T / *map[K]V
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "*" {
				continue
			}
			return rangeSourceFromTypeNode(c, content)
		}
	}
	return rangeSourceInfo{}, false
}

// functionTypeSingleCollectionResult reports element/value type when typeN is a
// function_type with a single collection result:
//
//	func() []*T
//	func() map[K]*T
//	func() ([]*T) / func() (as []*T)  — parenthesized single result
//
// Multi-result signatures (func() ([]*T, error)) fail closed — same policy as
// multi-result same-file helpers used as lone call values. Parameter types of
// the function type are ignored (only the result matters for fa() peels).
func functionTypeSingleCollectionResult(typeN *grammar.Node, content []byte) (rangeSourceInfo, bool) {
	if typeN == nil || typeN.Type() != "function_type" {
		return rangeSourceInfo{}, false
	}
	result := ingest.ChildByField(typeN, "result")
	if result == nil {
		return rangeSourceInfo{}, false
	}
	if result.Type() == "parameter_list" {
		// Parenthesized results: ( []*T ) / ( as []*T ) / ( []*T, error ).
		var only rangeSourceInfo
		count := 0
		for i := uint32(0); i < result.ChildCount(); i++ {
			p := result.Child(i)
			if p == nil || (p.Type() != "parameter_declaration" && p.Type() != "variadic_parameter_declaration") {
				continue
			}
			count++
			typeN := ingest.ChildByField(p, "type")
			info, ok := rangeSourceFromTypeNode(typeN, content)
			if !ok || info.elemType == "" {
				// Non-collection slot (error, bool, …) or multi mixed — fail closed.
				return rangeSourceInfo{}, false
			}
			only = info
		}
		if count != 1 {
			return rangeSourceInfo{}, false
		}
		return only, true
	}
	return rangeSourceFromTypeNode(result, content)
}

// sameFileNamedFuncCollectionResults maps same-file type names whose underlying
// type is a function type with a single collection result:
//
//	type FA func() []*A
//	type FMA = func() map[K]*A
//	type FPA = func() ([]*A)
//	type FA = FA0          // chained alias of a named func type
//	type FA FA0            // defined type whose base is a named func type
//
// Multi-result function types fail closed (same as inline function_type peels).
// Chained aliases / defined-of-named resolve via fixed-point over type_identifier
// RHS edges; cycles and unresolved targets stay absent (fail closed).
func sameFileNamedFuncCollectionResults(root *grammar.Node, content []byte) map[string]rangeSourceInfo {
	out := map[string]rangeSourceInfo{}
	if root == nil {
		return out
	}
	edges := map[string]string{}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "type_spec", "type_alias":
			nameN := ingest.ChildByField(n, "name")
			typeN := ingest.ChildByField(n, "type")
			if nameN == nil || typeN == nil {
				break
			}
			name := ingest.NodeText(nameN, content)
			if name == "" {
				break
			}
			if info, ok := functionTypeSingleCollectionResult(typeN, content); ok && info.elemType != "" {
				out[name] = info
			} else if tgt := typeIdentifierName(typeN, content); tgt != "" && tgt != name {
				// type FA = FA0 / type FA FA0 — resolve after direct peels.
				edges[name] = tgt
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	resolveNamedTypeAliasEdges(out, edges)
	return out
}

// sameFileNamedCollectionResults maps same-file type names whose underlying
// type is a slice/array/map/channel (or pointer-to-those):
//
//	type AS []*A
//	type AM = map[string]*A
//	type CA chan *A
//	type AA [1]*A
//	type PAS *[]*A
//	type AS = AS0          // chained alias of a named collection type
//	type AS AS0            // defined type whose base is a named collection
//	type PAS *AS0          // pointer-to-named collection (param (*pas)[0])
//
// Chained aliases / defined-of-named resolve via fixed-point over type_identifier
// RHS edges; pointer-to-named (*AS0) peels after the base is known. Cycles and
// unresolved targets stay absent (fail closed).
// isArray is set only when the declared type is (paren of) array_type, not when
// reached only via pointer peel (type PAS *[n]T) so new(AA) peels but new(PAS) does not.
// Chained aliases of an array-named type inherit isArray from the resolved base;
// pointer-to-named always clears isArray (new(PAS) is not indexable).
func sameFileNamedCollectionResults(root *grammar.Node, content []byte) map[string]rangeSourceInfo {
	out := map[string]rangeSourceInfo{}
	if root == nil {
		return out
	}
	edges := map[string]string{}
	ptrEdges := map[string]string{} // name → target for type PAS *AS0
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "type_spec", "type_alias":
			nameN := ingest.ChildByField(n, "name")
			typeN := ingest.ChildByField(n, "type")
			if nameN == nil || typeN == nil {
				break
			}
			name := ingest.NodeText(nameN, content)
			if name == "" {
				break
			}
			if info, ok := rangeSourceFromTypeNode(typeN, content); ok && info.elemType != "" {
				// Clear isArray unless the declared type is directly an array
				// (possibly parenthesized). Pointer-to-array named types stay
				// indexable via (*pas) but not via new(PAS).
				info.isArray = typeNodeIsDirectArray(typeN)
				out[name] = info
			} else if tgt := typeIdentifierName(typeN, content); tgt != "" && tgt != name {
				// type AS = AS0 / type AS AS0 — resolve after direct peels.
				edges[name] = tgt
			} else if tgt := pointerToTypeIdentifierName(typeN, content); tgt != "" && tgt != name {
				// type PAS *AS0 — resolve after AS0 (or chained AS) is known.
				ptrEdges[name] = tgt
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	// Alias edges first (AS = AS0), then pointer-to-named (PAS *AS0 / PAS *AS),
	// then alias edges again so type PAS2 = PAS resolves after PAS.
	resolveNamedTypeAliasEdges(out, edges)
	resolveNamedPointerToCollectionEdges(out, ptrEdges)
	resolveNamedTypeAliasEdges(out, edges)
	return out
}

// typeIdentifierName returns the text of a type_identifier node (after peeling
// parenthesized_type wrappers), or "" when n is not a bare type name.
func typeIdentifierName(n *grammar.Node, content []byte) string {
	for n != nil && n.Type() == "parenthesized_type" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "(" || c.Type() == ")" {
				continue
			}
			inner = c
			break
		}
		n = inner
	}
	if n == nil || n.Type() != "type_identifier" {
		return ""
	}
	return ingest.NodeText(n, content)
}

// pointerToTypeIdentifierName returns T for *T / (*T) when T is a type_identifier,
// or "" when n is not a single-level pointer to a named type.
func pointerToTypeIdentifierName(n *grammar.Node, content []byte) string {
	for n != nil && n.Type() == "parenthesized_type" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "(" || c.Type() == ")" {
				continue
			}
			inner = c
			break
		}
		n = inner
	}
	if n == nil || n.Type() != "pointer_type" {
		return ""
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil || c.Type() == "*" {
			continue
		}
		return typeIdentifierName(c, content)
	}
	return ""
}

// resolveNamedTypeAliasEdges follows type-identifier RHS chains into out using
// fixed-point iteration (type AS = AS0 where AS0 already peels; multi-hop
// type AS2 = AS). Cycles and unresolved targets stay absent (fail closed).
func resolveNamedTypeAliasEdges(out map[string]rangeSourceInfo, edges map[string]string) {
	if len(edges) == 0 {
		return
	}
	for {
		progress := false
		for name, target := range edges {
			if name == "" || target == "" || name == target {
				continue
			}
			if _, ok := out[name]; ok {
				continue
			}
			info, ok := out[target]
			if !ok || info.elemType == "" {
				continue
			}
			out[name] = info
			progress = true
		}
		if !progress {
			return
		}
	}
}

// resolveNamedPointerToCollectionEdges peels type PAS *AS0 once AS0 (or a chain
// to a collection) is in out. isArray is cleared so new(PAS) stays fail-closed.
func resolveNamedPointerToCollectionEdges(out map[string]rangeSourceInfo, ptrEdges map[string]string) {
	if len(ptrEdges) == 0 {
		return
	}
	for {
		progress := false
		for name, target := range ptrEdges {
			if name == "" || target == "" || name == target {
				continue
			}
			if _, ok := out[name]; ok {
				continue
			}
			info, ok := out[target]
			if !ok || info.elemType == "" {
				continue
			}
			info.isArray = false
			out[name] = info
			progress = true
		}
		if !progress {
			return
		}
	}
}

// typeNodeIsDirectArray reports whether n is (or is parenthesized) an array_type.
func typeNodeIsDirectArray(n *grammar.Node) bool {
	for n != nil && n.Type() == "parenthesized_type" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "(" || c.Type() == ")" {
				continue
			}
			inner = c
			break
		}
		n = inner
	}
	return n != nil && n.Type() == "array_type"
}

// functionTypeOrNamedCollectionResult peels an inline function_type or a
// same-file named type alias/def whose RHS is such a function type
// (type FA func() []*A / type FA = func() map[K]*A).
func functionTypeOrNamedCollectionResult(typeN *grammar.Node, content []byte, named map[string]rangeSourceInfo) (rangeSourceInfo, bool) {
	if info, ok := functionTypeSingleCollectionResult(typeN, content); ok {
		return info, true
	}
	if typeN == nil || typeN.Type() != "type_identifier" || len(named) == 0 {
		return rangeSourceInfo{}, false
	}
	info, ok := named[ingest.NodeText(typeN, content)]
	if !ok || info.elemType == "" {
		return rangeSourceInfo{}, false
	}
	return info, true
}

// sameFileFuncReturningFuncCollection maps same-file function names whose
// single result is a function type with a single collection result:
//
//	func getFA() FA          // type FA func() []*A
//	func getFA() func() []*A // inline function type
//
// Multi-result signatures (func getFA() (FA, error)) fail closed. Used so
// fa := getFA(); fa()[0].M and getFA()()[0].M resolve under foreign same-leaf.
func sameFileFuncReturningFuncCollection(root *grammar.Node, content []byte) map[string]rangeSourceInfo {
	out := map[string]rangeSourceInfo{}
	if root == nil {
		return out
	}
	namedFunc := sameFileNamedFuncCollectionResults(root, content)
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "function_declaration" {
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.Type() == "identifier" {
				name := ingest.NodeText(nameN, content)
				if name != "" {
					if info, ok := functionDeclSingleFuncCollectionResult(n, content, namedFunc); ok && info.elemType != "" {
						out[name] = info
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out
}

// functionDeclSingleFuncCollectionResult reports the collection element of a
// function's single result when that result is itself a function type with a
// single collection result (named FA or inline func() []*T). Multi-result
// parameter_list fails closed.
func functionDeclSingleFuncCollectionResult(decl *grammar.Node, content []byte, namedFunc map[string]rangeSourceInfo) (rangeSourceInfo, bool) {
	if decl == nil {
		return rangeSourceInfo{}, false
	}
	result := ingest.ChildByField(decl, "result")
	if result == nil {
		return rangeSourceInfo{}, false
	}
	if result.Type() == "parameter_list" {
		var only rangeSourceInfo
		count := 0
		for i := uint32(0); i < result.ChildCount(); i++ {
			p := result.Child(i)
			if p == nil || (p.Type() != "parameter_declaration" && p.Type() != "variadic_parameter_declaration") {
				continue
			}
			count++
			typeN := ingest.ChildByField(p, "type")
			info, ok := functionTypeOrNamedCollectionResult(typeN, content, namedFunc)
			if !ok || info.elemType == "" {
				return rangeSourceInfo{}, false
			}
			only = info
		}
		if count != 1 {
			return rangeSourceInfo{}, false
		}
		return only, true
	}
	return functionTypeOrNamedCollectionResult(result, content, namedFunc)
}

// sameFileStructNamedFuncFields maps same-file struct type → field name →
// collection result of a function-typed field (type Box struct { Fa FA } with
// type FA func() []*A). Only direct same-file named/inline function types;
// multi-result function types fail closed. Embedded fields without names skipped.
func sameFileStructNamedFuncFields(root *grammar.Node, content []byte) map[string]map[string]rangeSourceInfo {
	out := map[string]map[string]rangeSourceInfo{}
	if root == nil {
		return out
	}
	namedFunc := sameFileNamedFuncCollectionResults(root, content)
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "type_spec" {
			nameN := ingest.ChildByField(n, "name")
			typeN := ingest.ChildByField(n, "type")
			if nameN != nil && typeN != nil && typeN.Type() == "struct_type" {
				structName := ingest.NodeText(nameN, content)
				if structName != "" {
					fields := collectStructNamedFuncFields(typeN, content, namedFunc)
					if len(fields) > 0 {
						out[structName] = fields
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out
}

// callReturningFuncCollection reports collection result element info when n is
// a call to a same-file helper that returns a function type with a single
// collection result (getFA() FA / getFA() func() []*A). Used to bind
// fa := getFA() so fa()[0].M peels under foreign same-leaf.
func callReturningFuncCollection(n *grammar.Node, content []byte, funcRetFunc map[string]rangeSourceInfo) (rangeSourceInfo, bool) {
	if n == nil || len(funcRetFunc) == 0 {
		return rangeSourceInfo{}, false
	}
	if n.Type() == "expression_list" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			return callReturningFuncCollection(c, content, funcRetFunc)
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() != "call_expression" {
		return rangeSourceInfo{}, false
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "identifier" {
		return rangeSourceInfo{}, false
	}
	info, ok := funcRetFunc[ingest.NodeText(fn, content)]
	if !ok || info.elemType == "" {
		return rangeSourceInfo{}, false
	}
	return info, true
}

// selectorNamedFuncField reports collection result element info when n is a
// selector xa.Fa and xa has a known same-file struct type with function-typed
// field Fa (type Box struct { Fa FA } with type FA func() []*A).
func selectorNamedFuncField(n *grammar.Node, content []byte, valueType func(string, uint32) (string, bool), structFields map[string]map[string]rangeSourceInfo, at uint32) (rangeSourceInfo, bool) {
	if n == nil || valueType == nil || len(structFields) == 0 {
		return rangeSourceInfo{}, false
	}
	if n.Type() == "expression_list" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			return selectorNamedFuncField(c, content, valueType, structFields, at)
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() != "selector_expression" {
		return rangeSourceInfo{}, false
	}
	operand := ingest.ChildByField(n, "operand")
	field := ingest.ChildByField(n, "field")
	if operand == nil || field == nil || operand.Type() != "identifier" {
		return rangeSourceInfo{}, false
	}
	recvType, ok := valueType(ingest.NodeText(operand, content), at)
	if !ok || recvType == "" {
		return rangeSourceInfo{}, false
	}
	recvType = strings.TrimPrefix(recvType, "*")
	fields := structFields[recvType]
	if len(fields) == 0 {
		return rangeSourceInfo{}, false
	}
	info, ok := fields[ingest.NodeText(field, content)]
	if !ok || info.elemType == "" {
		return rangeSourceInfo{}, false
	}
	return info, true
}

// collectStructNamedFuncFields walks a struct_type for named fields whose type
// is a function type with a single collection result.
func collectStructNamedFuncFields(structType *grammar.Node, content []byte, namedFunc map[string]rangeSourceInfo) map[string]rangeSourceInfo {
	fields := map[string]rangeSourceInfo{}
	if structType == nil {
		return fields
	}
	var walk func(*grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "field_declaration" {
			typeField := ingest.ChildByField(n, "type")
			info, ok := functionTypeOrNamedCollectionResult(typeField, content, namedFunc)
			if !ok || info.elemType == "" {
				return
			}
			// Collect all field_identifier names (Fa FA / X, Y FA). Skip
			// embedded fields (type only, no field_identifier).
			anyName := false
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == nil || ch.Type() != "field_identifier" {
					continue
				}
				fname := ingest.NodeText(ch, content)
				if fname == "" || fname == "_" {
					continue
				}
				fields[fname] = info
				anyName = true
			}
			_ = anyName
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(structType)
	return fields
}

// collectionTypeOrNamedResult peels an inline collection type or a same-file
// named type alias/def whose RHS is a collection (type AS []*A / type AM = map[K]*A).
// Also peels *AS when AS is a named collection.
func collectionTypeOrNamedResult(typeN *grammar.Node, content []byte, named map[string]rangeSourceInfo) (rangeSourceInfo, bool) {
	if info, ok := rangeSourceFromTypeNode(typeN, content); ok && info.elemType != "" {
		return info, true
	}
	if typeN == nil || len(named) == 0 {
		return rangeSourceInfo{}, false
	}
	// *AS / *AM when AS/AM is a named collection.
	if typeN.Type() == "pointer_type" {
		for i := uint32(0); i < typeN.ChildCount(); i++ {
			c := typeN.Child(i)
			if c == nil || c.Type() == "*" {
				continue
			}
			return collectionTypeOrNamedResult(c, content, named)
		}
		return rangeSourceInfo{}, false
	}
	if typeN.Type() != "type_identifier" {
		return rangeSourceInfo{}, false
	}
	info, ok := named[ingest.NodeText(typeN, content)]
	if !ok || info.elemType == "" {
		return rangeSourceInfo{}, false
	}
	return info, true
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
// (short_var_declaration / var_spec / type-switch case arms / range vars /
// channel receives / select receive cases).
// short_var / var_spec use the enclosing body's end as scope; type-switch
// aliases and select receive vars are scoped to each arm so same-leaf methods
// on foreign arms are not rewritten. rangeSrc maps collection names
// (params/vars) to their range element/value/key types and channel payloads.
// funcResults maps same-file function names to positional concrete result types
// for multi-return call binding (a, b := makeAB()).
// namedColl maps same-file type AS []*A / AM map[K]*A (may be nil).
func collectLocalTypeBindings(node *grammar.Node, content []byte, bindings *[]identTypeBinding, rangeSrc map[string]rangeSourceInfo, funcResults map[string][]string, genericPeels map[string]string, namedColl map[string]rangeSourceInfo) {
	if node == nil {
		return
	}
	if rangeSrc == nil {
		rangeSrc = map[string]rangeSourceInfo{}
	}
	// rangeSrc-backed element lookup for First(xs) assign peels.
	rangeElem := func(name string, at uint32) (string, bool) {
		if info, ok := rangeSrc[name]; ok && info.elemType != "" {
			return strings.TrimPrefix(info.elemType, "*"), true
		}
		return "", false
	}
	scopeEnd := node.EndByte()
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "short_var_declaration":
			// left := right — extract names from left, type from right (&T{} / T{} / <-ch).
			// Multi-value RHS is position-wise: a, b := &A{}, &B{} binds a→A, b→B
			// so foreign same-leaf methods on b are not rewritten.
			// Multi-return call: a, b := makeAB() with makeAB() (*A, *B) likewise.
			// Func-literal result: f := func() *A { … } binds f→A so f().Run renames.
			names := identListNames(ingest.ChildByField(n, "left"), content)
			right := ingest.ChildByField(n, "right")
			if chTyp, ok := channelReceiveElemType(right, content, rangeSrc); ok {
				// a := <-ch  /  a, ok := <-ch — only the value (first name) is T.
				if len(names) > 0 && names[0] != "" && names[0] != "_" {
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, names[0], chTyp})
				}
			} else if types := typeNamesFromMultiRHS(right, content); len(types) > 0 {
				for i, name := range names {
					if name == "" || name == "_" {
						continue
					}
					if i < len(types) && types[i] != "" {
						*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, types[i]})
					}
				}
			} else if types := typeNamesFromCallResults(right, content, funcResults, genericPeels, rangeElem); len(types) > 0 {
				for i, name := range names {
					if name == "" || name == "_" {
						continue
					}
					if i < len(types) && types[i] != "" {
						*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, types[i]})
					}
				}
			} else if types := typeNamesFromFuncLiteralResults(right, content); len(types) > 0 {
				// f := func() *A { … } / f, g := func() *A {…}, func() *B {…}
				for i, name := range names {
					if name == "" || name == "_" {
						continue
					}
					if i < len(types) && types[i] != "" {
						*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, types[i]})
					}
				}
			}
		case "var_spec":
			// var a, b T / var a, b = … — name is a repeated field; ChildByField
			// only returns the first, so collect every name field (like params).
			names := varSpecNames(n, content)
			typeN := ingest.ChildByField(n, "type")
			valueN := ingest.ChildByField(n, "value")
			typ := concreteNamedType(typeN, content)
			if typ == "" {
				if chTyp, ok := channelReceiveElemType(valueN, content, rangeSrc); ok {
					typ = chTyp
				} else if len(names) > 1 {
					// var a, b = makeAB() / var a, b = &A{}, &B{} — bind positionally.
					types := typeNamesFromMultiRHS(valueN, content)
					if len(types) == 0 {
						types = typeNamesFromCallResults(valueN, content, funcResults, genericPeels, rangeElem)
					}
					if len(types) == 0 {
						types = typeNamesFromFuncLiteralResults(valueN, content)
					}
					if len(types) > 0 {
						for i, name := range names {
							if name == "" || name == "_" {
								continue
							}
							if i < len(types) && types[i] != "" {
								*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, types[i]})
							}
						}
						break
					}
					typ = typeNameFromRHS(valueN, content)
				} else if types := typeNamesFromFuncLiteralResults(valueN, content); len(types) == 1 && types[0] != "" {
					// var f = func() *A { … } — single-name func-literal result.
					typ = types[0]
				} else {
					typ = typeNameFromRHS(valueN, content)
				}
			}
			if typ != "" {
				for _, name := range names {
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, typ})
				}
			}
			if info, ok := rangeSourceFromTypeNode(typeN, content); ok {
				for _, name := range names {
					if name != "" && name != "_" {
						rangeSrc[name] = info
					}
				}
			} else if info, ok := collectionTypeOrNamedResult(typeN, content, namedColl); ok {
				// var xa AS / var pas *AS with type AS []*A.
				for _, name := range names {
					if name != "" && name != "_" {
						rangeSrc[name] = info
					}
				}
			}
		case "type_switch_statement":
			// switch v := x.(type) { case *T: v.M() } — bind v to T only
			// inside each single-type case arm (multi-type cases leave v as
			// interface-typed and unbound).
			aliasNames := identListNames(ingest.ChildByField(n, "alias"), content)
			if len(aliasNames) == 1 {
				alias := aliasNames[0]
				for i := uint32(0); i < n.ChildCount(); i++ {
					c := n.Child(i)
					if c == nil || c.Type() != "type_case" {
						continue
					}
					if typ, ok := typeCaseSingleType(c, content); ok {
						*bindings = append(*bindings, identTypeBinding{c.StartByte(), c.EndByte(), alias, typ})
					}
				}
			}
		case "for_statement":
			// for _, v := range xs { v.M() } — bind v from range source type.
			collectRangeClauseBindings(n, content, bindings, rangeSrc)
		case "communication_case":
			// case x := <-ch: / case x, ok := <-ch: — bind x to channel payload,
			// scoped to this case so same-leaf methods on other arms stay put.
			if comm := ingest.ChildByField(n, "communication"); comm != nil && comm.Type() == "receive_statement" {
				bindReceiveStatement(comm, content, n.StartByte(), n.EndByte(), bindings, rangeSrc)
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

// bindReceiveStatement binds the value name of `x := <-ch` / `x, ok := <-ch`
// when ch is a known channel source.
func bindReceiveStatement(recv *grammar.Node, content []byte, scopeStart, scopeEnd uint32, bindings *[]identTypeBinding, rangeSrc map[string]rangeSourceInfo) {
	if recv == nil || recv.Type() != "receive_statement" {
		return
	}
	names := identListNames(ingest.ChildByField(recv, "left"), content)
	if len(names) == 0 {
		return
	}
	chTyp, ok := channelReceiveElemType(ingest.ChildByField(recv, "right"), content, rangeSrc)
	if !ok {
		return
	}
	name := names[0]
	if name == "" || name == "_" {
		return
	}
	*bindings = append(*bindings, identTypeBinding{scopeStart, scopeEnd, name, chTyp})
}

// channelReceiveElemType returns the payload type for <-ch when ch is a known
// channel (or channel-like) source in rangeSrc.
func channelReceiveElemType(right *grammar.Node, content []byte, rangeSrc map[string]rangeSourceInfo) (string, bool) {
	if right == nil || rangeSrc == nil {
		return "", false
	}
	// expression_list → first expression
	if right.Type() == "expression_list" {
		for i := uint32(0); i < right.ChildCount(); i++ {
			c := right.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			return channelReceiveElemType(c, content, rangeSrc)
		}
		return "", false
	}
	if right.Type() != "unary_expression" {
		return "", false
	}
	op := ingest.ChildByField(right, "operator")
	if op == nil || ingest.NodeText(op, content) != "<-" {
		return "", false
	}
	operand := ingest.ChildByField(right, "operand")
	if operand == nil || operand.Type() != "identifier" {
		return "", false
	}
	info, ok := rangeSrc[ingest.NodeText(operand, content)]
	if !ok || info.isMap || info.elemType == "" {
		return "", false
	}
	return info.elemType, true
}

// collectRangeClauseBindings binds range variables inside a for_statement when
// the range source is a known collection (param/var with slice/array/map/chan type).
// Two-var form binds key (maps) + value; one-var form binds the element for
// non-map sources and the key for maps.
func collectRangeClauseBindings(forStmt *grammar.Node, content []byte, bindings *[]identTypeBinding, rangeSrc map[string]rangeSourceInfo) {
	if forStmt == nil {
		return
	}
	var clause *grammar.Node
	for i := uint32(0); i < forStmt.ChildCount(); i++ {
		c := forStmt.Child(i)
		if c != nil && c.Type() == "range_clause" {
			clause = c
			break
		}
	}
	if clause == nil {
		return
	}
	names := identListNames(ingest.ChildByField(clause, "left"), content)
	right := ingest.ChildByField(clause, "right")
	if right == nil || len(names) == 0 {
		return
	}
	info, ok := rangeSourceInfo{}, false
	if right.Type() == "identifier" {
		info, ok = rangeSrc[ingest.NodeText(right, content)]
	}
	if !ok {
		// Composite / typed RHS: range over []T{...} / make([]*T, n) — best-effort.
		if typ := typeNameFromRHS(right, content); typ != "" {
			// typeNameFromRHS peels composites to named type; treat as element type
			// only when RHS itself looks like a composite/make of a collection —
			// for bare identifiers we already consulted rangeSrc.
			if right.Type() != "identifier" {
				info, ok = rangeSourceInfo{elemType: typ, isMap: false}, typ != ""
			}
		}
	}
	if !ok || (info.elemType == "" && info.keyType == "") {
		return
	}
	scopeStart, scopeEnd := forStmt.StartByte(), forStmt.EndByte()
	switch {
	case len(names) >= 2:
		// for k, v := range m — k is key (maps), v is element/value.
		if info.isMap && info.keyType != "" {
			k := names[0]
			if k != "" && k != "_" {
				*bindings = append(*bindings, identTypeBinding{scopeStart, scopeEnd, k, info.keyType})
			}
		}
		if info.elemType != "" {
			v := names[1]
			if v != "" && v != "_" {
				*bindings = append(*bindings, identTypeBinding{scopeStart, scopeEnd, v, info.elemType})
			}
		}
	case len(names) == 1 && info.isMap:
		// for k := range m — keys for maps.
		if info.keyType != "" {
			k := names[0]
			if k != "" && k != "_" {
				*bindings = append(*bindings, identTypeBinding{scopeStart, scopeEnd, k, info.keyType})
			}
		}
	case len(names) == 1 && !info.isMap:
		// for v := range xs — element for slice/array/chan.
		if info.elemType != "" {
			v := names[0]
			if v != "" && v != "_" {
				*bindings = append(*bindings, identTypeBinding{scopeStart, scopeEnd, v, info.elemType})
			}
		}
	}
}

// typeCaseSingleType returns the concrete type name when a type_case lists
// exactly one type (pointer star stripped). Multi-type arms (case A, B:)
// return ok=false because the alias is not a single concrete type there.
func typeCaseSingleType(typeCase *grammar.Node, content []byte) (string, bool) {
	if typeCase == nil {
		return "", false
	}
	var types []string
	for i := uint32(0); i < typeCase.ChildCount(); i++ {
		if typeCase.FieldNameForChild(i) != "type" {
			continue
		}
		c := typeCase.Child(i)
		if c == nil || c.IsNull() {
			continue
		}
		// Comma separators may appear as type-field children in some grammars.
		if c.Type() == "," {
			continue
		}
		if typ := typeNameFromTypeNode(c, content); typ != "" {
			types = append(types, typ)
		}
	}
	if len(types) != 1 {
		return "", false
	}
	return types[0], true
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

// typeNamesFromMultiRHS returns position-wise concrete types for a short-var /
// multi-assign RHS. expression_list yields one type per expression (empty string
// when unknown); a single expression yields a one-element slice when typed.
// Used so a, b := &A{}, &B{} does not type both names as A (first only).
// Multi-return calls (a, b := makeAB()) are handled by typeNamesFromCallResults.
func typeNamesFromMultiRHS(n *grammar.Node, content []byte) []string {
	if n == nil {
		return nil
	}
	if n.Type() == "expression_list" {
		var out []string
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			out = append(out, typeNameFromRHS(c, content))
		}
		// Drop trailing empties only when nothing was typed at all.
		any := false
		for _, t := range out {
			if t != "" {
				any = true
				break
			}
		}
		if !any {
			return nil
		}
		return out
	}
	if typ := typeNameFromRHS(n, content); typ != "" {
		return []string{typ}
	}
	return nil
}

// sameFileFuncResultTypes maps function names in root to positional concrete
// result type names (pointer star stripped). Empty slots for non-concrete
// results (bool, error, slices, …) keep multi-return indices aligned.
func sameFileFuncResultTypes(root *grammar.Node, content []byte) map[string][]string {
	if root == nil {
		return nil
	}
	out := map[string][]string{}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "function_declaration" {
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.Type() == "identifier" {
				name := ingest.NodeText(nameN, content)
				if name != "" {
					if types := functionResultTypes(n, content); len(types) > 0 {
						out[name] = types
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out
}

// sameFileGenericPeelFuncs maps same-file generic helpers whose call result
// peels from the first argument under foreign same-leaf methods:
//
//	"arg"  — func Id[T any](v T) T / First2[T, U any](a T, b U) T:
//	         peel first-arg concrete type (first value param type = result
//	         type param; extra type/value params allowed)
//	         (Id(A{}).Run / First2(A{}, B{}).Run / a := Id(A{}); a.Run)
//	"elem" — func First[T any](xs []T) T / At[T](xs []T, i int) T /
//	         Get[K comparable, V any](m map[K]V, k K) V:
//	         peel first-arg slice/array element or map value type
//	         (First([]A{{}}).Run / Get(map[string]A{…}, k).Run /
//	         a := First(xs); a.Run with xs []A)
//
// Only these shapes are recognized; other generics fail closed.
func sameFileGenericPeelFuncs(root *grammar.Node, content []byte) map[string]string {
	if root == nil {
		return nil
	}
	out := map[string]string{}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "function_declaration" {
			if name := goGenericIdentityFuncName(n, content); name != "" {
				out[name] = "arg"
			} else if name := goGenericSliceElemFuncName(n, content); name != "" {
				out[name] = "elem"
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	if len(out) == 0 {
		return nil
	}
	return out
}

// goGenericTypeParamNames returns the ordered type-parameter names of a
// generic function_declaration (including multi-name decls like [T, U any]).
// Empty when the decl is non-generic or malformed.
func goGenericTypeParamNames(decl *grammar.Node, content []byte) []string {
	if decl == nil {
		return nil
	}
	tpList := ingest.ChildByField(decl, "type_parameters")
	if tpList == nil {
		// Some grammars expose type_parameter_list as a child without field name.
		for i := uint32(0); i < decl.ChildCount(); i++ {
			ch := decl.Child(i)
			if ch != nil && ch.Type() == "type_parameter_list" {
				tpList = ch
				break
			}
		}
	}
	if tpList == nil || tpList.Type() != "type_parameter_list" {
		return nil
	}
	var typeParams []string
	for i := uint32(0); i < tpList.ChildCount(); i++ {
		ch := tpList.Child(i)
		if ch == nil || ch.Type() != "type_parameter_declaration" {
			continue
		}
		// Collect every identifier before the constraint (T / T, U any).
		sawName := false
		for j := uint32(0); j < ch.ChildCount(); j++ {
			c := ch.Child(j)
			if c == nil {
				continue
			}
			switch c.Type() {
			case "identifier":
				pname := ingest.NodeText(c, content)
				if pname != "" {
					typeParams = append(typeParams, pname)
					sawName = true
				}
			case ",", "comment":
				continue
			default:
				// type_constraint / other — stop name harvest for this decl.
				if sawName {
					break
				}
			}
		}
		if !sawName {
			return nil
		}
	}
	return typeParams
}

// goGenericResultTypeParam returns the bare type-param name of the function
// result when it is a type_identifier matching one of typeParams.
func goGenericResultTypeParam(decl *grammar.Node, content []byte, typeParams []string) string {
	if decl == nil || len(typeParams) == 0 {
		return ""
	}
	paramSet := map[string]bool{}
	for _, p := range typeParams {
		paramSet[p] = true
	}
	result := ingest.ChildByField(decl, "result")
	if result == nil {
		for i := uint32(0); i < decl.ChildCount(); i++ {
			ch := decl.Child(i)
			if ch != nil && ch.Type() == "type_identifier" {
				// Last type_identifier before block is typically the result.
				result = ch
			}
		}
	}
	if result == nil || result.Type() != "type_identifier" {
		return ""
	}
	r := ingest.NodeText(result, content)
	if !paramSet[r] {
		return ""
	}
	return r
}

// goGenericIdentityFuncName returns the function name when decl is
// func Name[T …](v T[, …]) T (or multi-param First2[T, U any](a T, b U) T):
// result is a type param Ti and the first value param's type is bare Ti.
// Extra type params and extra value params are allowed. Other shapes fail closed.
func goGenericIdentityFuncName(decl *grammar.Node, content []byte) string {
	if decl == nil || decl.Type() != "function_declaration" {
		return ""
	}
	nameN := ingest.ChildByField(decl, "name")
	if nameN == nil || nameN.Type() != "identifier" {
		return ""
	}
	name := ingest.NodeText(nameN, content)
	if name == "" {
		return ""
	}
	typeParams := goGenericTypeParamNames(decl, content)
	if len(typeParams) == 0 {
		return ""
	}
	tParam := goGenericResultTypeParam(decl, content, typeParams)
	if tParam == "" {
		return ""
	}

	// First value parameter of type Ti (bare type_identifier). Extra params OK.
	params := ingest.ChildByField(decl, "parameters")
	if params == nil || params.Type() != "parameter_list" {
		return ""
	}
	var firstParamType *grammar.Node
	for i := uint32(0); i < params.ChildCount(); i++ {
		p := params.Child(i)
		if p == nil || p.Type() != "parameter_declaration" {
			continue
		}
		firstParamType = ingest.ChildByField(p, "type")
		// Multi-name first param (a, b T) — still one type slot; allow only if
		// single name so dual-class First2(a T, b U) stays first-arg peel.
		names := parameterDeclNames(p, content)
		if len(names) > 1 {
			return ""
		}
		break
	}
	if firstParamType == nil || firstParamType.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(firstParamType, content) != tParam {
		return ""
	}
	return name
}

// goGenericSliceElemFuncName returns the function name when decl is
// func Name[T …](xs []T[, …]) T / ([N]T) / map[K]T value peel:
//
//	First[T](xs []T) T / At[T](xs []T, i int) T — slice/array element
//	Get[K comparable, V any](m map[K]V, k K) V — map value
//
// Result is type param Ti; first value param is []Ti / [N]Ti / map[K]Ti.
// Extra type/value params allowed. Other shapes fail closed.
func goGenericSliceElemFuncName(decl *grammar.Node, content []byte) string {
	if decl == nil || decl.Type() != "function_declaration" {
		return ""
	}
	nameN := ingest.ChildByField(decl, "name")
	if nameN == nil || nameN.Type() != "identifier" {
		return ""
	}
	name := ingest.NodeText(nameN, content)
	if name == "" {
		return ""
	}
	typeParams := goGenericTypeParamNames(decl, content)
	if len(typeParams) == 0 {
		return ""
	}
	tParam := goGenericResultTypeParam(decl, content, typeParams)
	if tParam == "" {
		return ""
	}
	paramSet := map[string]bool{}
	for _, p := range typeParams {
		paramSet[p] = true
	}

	// First value parameter is []T / [N]T / map[K]T; further params allowed.
	params := ingest.ChildByField(decl, "parameters")
	if params == nil || params.Type() != "parameter_list" {
		return ""
	}
	var firstParamType *grammar.Node
	paramCount := 0
	for i := uint32(0); i < params.ChildCount(); i++ {
		p := params.Child(i)
		if p == nil || p.Type() != "parameter_declaration" {
			continue
		}
		paramCount++
		if firstParamType == nil {
			firstParamType = ingest.ChildByField(p, "type")
		}
		// Multi-name first param (a, b []T) is still one type slot — allow.
	}
	if paramCount < 1 || firstParamType == nil {
		return ""
	}
	// Peel []T / [N]T / map[K]T (and parenthesized).
	pt := firstParamType
	for pt != nil && pt.Type() == "parenthesized_type" {
		var inner *grammar.Node
		for i := uint32(0); i < pt.ChildCount(); i++ {
			c := pt.Child(i)
			if c == nil || c.Type() == "(" || c.Type() == ")" {
				continue
			}
			inner = c
			break
		}
		pt = inner
	}
	if pt == nil {
		return ""
	}
	switch pt.Type() {
	case "slice_type", "array_type":
		el := ingest.ChildByField(pt, "element")
		if el == nil || el.Type() != "type_identifier" || ingest.NodeText(el, content) != tParam {
			return ""
		}
	case "map_type":
		// map[K]V with V == result type param; K should be a type param too
		// (Get[K, V]) so non-generic map helpers fail closed.
		val := ingest.ChildByField(pt, "value")
		if val == nil || val.Type() != "type_identifier" || ingest.NodeText(val, content) != tParam {
			return ""
		}
		key := ingest.ChildByField(pt, "key")
		if key == nil || key.Type() != "type_identifier" || !paramSet[ingest.NodeText(key, content)] {
			return ""
		}
	default:
		return ""
	}
	return name
}

// goCallFirstArgNode returns the first positional argument of a call_expression.
func goCallFirstArgNode(call *grammar.Node) *grammar.Node {
	if call == nil || call.Type() != "call_expression" {
		return nil
	}
	args := ingest.ChildByField(call, "arguments")
	if args == nil {
		return nil
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		c := args.Child(i)
		if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
			continue
		}
		return c
	}
	return nil
}

// goCallFirstArgType recovers the concrete named type of the first positional
// argument of a call_expression (A{} / &A{} / new(A) / nested peels). Used by
// generic identity Id(arg) and multi-param First2(a, b) peels (first value
// param is the result type param; extra args ignored).
func goCallFirstArgType(call *grammar.Node, content []byte, indexElemType, valueType func(name string, at uint32) (string, bool), funcColl map[string][]rangeSourceInfo, funcResults map[string][]string, genericPeels map[string]string, namedColl map[string]rangeSourceInfo, funcRetFunc map[string]rangeSourceInfo, structFields map[string]map[string]rangeSourceInfo) string {
	if call == nil || call.Type() != "call_expression" {
		return ""
	}
	first := goCallFirstArgNode(call)
	if first == nil {
		return ""
	}
	// Prefer composite / unary peels (A{} / &A{}) before full complex (avoids
	// re-entering identity on the same shape awkwardly).
	if t := compositeLiteralTypeName(first, content); t != "" {
		return t
	}
	if t := typeNameFromRHS(first, content); t != "" {
		return strings.TrimPrefix(t, "*")
	}
	return goComplexOperandType(first, content, indexElemType, valueType, funcColl, funcResults, genericPeels, namedColl, funcRetFunc, structFields)
}

// goCallFirstArgElemType recovers the element type of the first positional
// argument when it is a slice/array collection (composite, make, typed local,
// append, …). Used by generic First[T](xs []T) T / At[T](xs []T, i int) T peels.
// Extra args after the collection are ignored (At index). Missing first arg
// fails closed.
func goCallFirstArgElemType(call *grammar.Node, content []byte, indexElemType func(name string, at uint32) (string, bool), funcColl map[string][]rangeSourceInfo, namedColl map[string]rangeSourceInfo, funcRetFunc map[string]rangeSourceInfo, structFields map[string]map[string]rangeSourceInfo, valueType func(string, uint32) (string, bool)) string {
	first := goCallFirstArgNode(call)
	if first == nil {
		return ""
	}
	at := call.StartByte()
	if info, ok := rangeSourceFromCollectionExprIdent(first, content, indexElemType, at, funcColl, namedColl, funcRetFunc, structFields, valueType); ok && info.elemType != "" {
		return strings.TrimPrefix(info.elemType, "*")
	}
	return ""
}

// sameFileFuncCollectionResults maps same-file function names to positional
// collection element/value info for each result slot
// (func getA() []*A / func getA() ([]*A, error) / func getM() map[K]*A /
// named (as []*A, err error) / func getAS() AS with type AS []*A). Empty-elem
// slots keep multi-return indices aligned so as, err := getA(); as[0].M and
// getA()[0].M (single-result only) resolve under foreign same-leaf methods.
func sameFileFuncCollectionResults(root *grammar.Node, content []byte) map[string][]rangeSourceInfo {
	if root == nil {
		return nil
	}
	namedColl := sameFileNamedCollectionResults(root, content)
	out := map[string][]rangeSourceInfo{}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "function_declaration" {
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.Type() == "identifier" {
				name := ingest.NodeText(nameN, content)
				if name != "" {
					if infos := functionCollectionResults(n, content, namedColl); len(infos) > 0 {
						out[name] = infos
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	if len(out) == 0 {
		return nil
	}
	return out
}

// functionCollectionResults returns positional collection element/value info
// from a function_declaration's (or func_literal's) result list. Non-collection
// slots (error, bool, scalars) keep empty elemType so multi-return indices stay
// aligned (e.g. ([]*A, error) → [{A}, {}]). namedColl peels same-file named
// collection result types (func getAS() AS with type AS []*A; may be nil).
// Returns nil when no result is a collection.
func functionCollectionResults(decl *grammar.Node, content []byte, namedColl map[string]rangeSourceInfo) []rangeSourceInfo {
	if decl == nil {
		return nil
	}
	result := ingest.ChildByField(decl, "result")
	if result == nil {
		return nil
	}
	if result.Type() == "parameter_list" {
		var infos []rangeSourceInfo
		any := false
		for i := uint32(0); i < result.ChildCount(); i++ {
			p := result.Child(i)
			if p == nil || (p.Type() != "parameter_declaration" && p.Type() != "variadic_parameter_declaration") {
				continue
			}
			typeN := ingest.ChildByField(p, "type")
			info, ok := collectionTypeOrNamedResult(typeN, content, namedColl)
			if !ok || info.elemType == "" {
				// Non-collection slot — keep alignment (e.g. error in ([]*A, error)).
				info = rangeSourceInfo{}
			} else {
				any = true
			}
			// Multi-name rare in results; one slot per declaration name, or one
			// if unnamed.
			names := parameterDeclNames(p, content)
			if len(names) == 0 {
				infos = append(infos, info)
			} else {
				for range names {
					infos = append(infos, info)
				}
			}
		}
		if !any {
			return nil
		}
		return infos
	}
	if info, ok := collectionTypeOrNamedResult(result, content, namedColl); ok && info.elemType != "" {
		return []rangeSourceInfo{info}
	}
	return nil
}

// collectionInfosFromCallResults returns positional collection infos when n is
// a call to a same-file function known in funcColl (or expression_list of one
// such call). Used for as, err := getA() / var as, err = getA().
func collectionInfosFromCallResults(n *grammar.Node, content []byte, funcColl map[string][]rangeSourceInfo) []rangeSourceInfo {
	if n == nil || len(funcColl) == 0 {
		return nil
	}
	// expression_list → single call only (multi-return, not multi-expr).
	if n.Type() == "expression_list" {
		var exprs []*grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			exprs = append(exprs, c)
		}
		if len(exprs) != 1 {
			return nil
		}
		n = exprs[0]
	}
	if n.Type() != "call_expression" {
		return nil
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "identifier" {
		return nil
	}
	name := ingest.NodeText(fn, content)
	if name == "" {
		return nil
	}
	infos := funcColl[name]
	if len(infos) == 0 {
		return nil
	}
	out := make([]rangeSourceInfo, len(infos))
	copy(out, infos)
	return out
}

// functionResultTypes returns positional concrete type names from a
// function_declaration's result (parameter_list or single type).
func functionResultTypes(decl *grammar.Node, content []byte) []string {
	if decl == nil {
		return nil
	}
	result := ingest.ChildByField(decl, "result")
	if result == nil {
		return nil
	}
	if result.Type() == "parameter_list" {
		var types []string
		any := false
		for i := uint32(0); i < result.ChildCount(); i++ {
			p := result.Child(i)
			if p == nil || (p.Type() != "parameter_declaration" && p.Type() != "variadic_parameter_declaration") {
				continue
			}
			typeN := ingest.ChildByField(p, "type")
			typ := concreteNamedType(typeN, content)
			if typ == "" {
				// Keep alignment for multi-return (e.g. (*A, bool)).
				typ = ""
			} else {
				any = true
			}
			// Multi-name params in results are rare; one slot per declaration.
			names := parameterDeclNames(p, content)
			if len(names) == 0 {
				types = append(types, typ)
			} else {
				for range names {
					types = append(types, typ)
				}
			}
		}
		if !any {
			return nil
		}
		return types
	}
	if typ := concreteNamedType(result, content); typ != "" {
		return []string{typ}
	}
	return nil
}

// typeNamesFromFuncLiteralResults returns positional concrete result types when
// n is a func_literal (or expression_list of func_literals):
//
//	f := func() *A { return &A{} }     → ["A"]
//	f, g := func() *A {…}, func() *B {…} → ["A","B"]
//
// Multi-result func literals (func() (*A, error)) fail closed (not a lone
// method receiver). Used so f().Run renames under foreign same-leaf methods.
func typeNamesFromFuncLiteralResults(n *grammar.Node, content []byte) []string {
	if n == nil {
		return nil
	}
	if n.Type() == "expression_list" {
		var out []string
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			types := typeNamesFromFuncLiteralResults(c, content)
			if len(types) != 1 || types[0] == "" {
				// Position must be a single-result func literal.
				return nil
			}
			out = append(out, types[0])
		}
		return out
	}
	if n.Type() != "func_literal" {
		return nil
	}
	types := functionResultTypes(n, content)
	if len(types) != 1 || types[0] == "" {
		return nil
	}
	return types
}

// typeNamesFromCallResults returns positional result types when n is a call to
// a same-file function known in funcResults (or expression_list of one such call).
// Used for a, b := makeAB() / var a, b = makeAB().
// indexElemType is optional: enables First(xs) assign peels when xs is a typed
// slice/array param or local (via rangeSrc-backed lookup).
func typeNamesFromCallResults(n *grammar.Node, content []byte, funcResults map[string][]string, genericPeels map[string]string, indexElemType func(name string, at uint32) (string, bool)) []string {
	if n == nil {
		return nil
	}
	// expression_list → single call only (multi-return, not multi-expr).
	if n.Type() == "expression_list" {
		var exprs []*grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			exprs = append(exprs, c)
		}
		if len(exprs) != 1 {
			return nil
		}
		n = exprs[0]
	}
	if n.Type() != "call_expression" {
		return nil
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "identifier" {
		return nil
	}
	name := ingest.NodeText(fn, content)
	if name == "" {
		return nil
	}
	// Generic peels before funcResults so result type-param "T" is not used as a leaf.
	// "arg":  Id(A{}) peels arg type.
	// "elem": First([]A{{}})/First(xs) peels first-arg element type.
	if mode := genericPeels[name]; mode == "arg" {
		args := ingest.ChildByField(n, "arguments")
		if args != nil {
			for i := uint32(0); i < args.ChildCount(); i++ {
				c := args.Child(i)
				if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
					continue
				}
				if t := compositeLiteralTypeName(c, content); t != "" {
					return []string{t}
				}
				if t := typeNameFromRHS(c, content); t != "" {
					return []string{strings.TrimPrefix(t, "*")}
				}
				break
			}
		}
		return nil
	} else if mode == "elem" {
		// Element peel for assign: a := First([]A{{}}) / a := First(xs).
		if t := goCallFirstArgElemType(n, content, indexElemType, nil, nil, nil, nil, nil); t != "" {
			return []string{t}
		}
		return nil
	}
	if len(funcResults) == 0 {
		return nil
	}
	types := funcResults[name]
	if len(types) == 0 {
		return nil
	}
	// Copy so callers cannot mutate the shared map entry.
	out := make([]string, len(types))
	copy(out, types)
	return out
}

// typeNameFromRHS extracts T from &T{}, T{}, new(T), (*T)(nil)-ish forms.
// For expression_list, only the first expression is considered (legacy); prefer
// typeNamesFromMultiRHS when binding multiple names.
func typeNameFromRHS(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	// expression_list → first child
	if n.Type() == "expression_list" && n.ChildCount() > 0 {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			return typeNameFromRHS(c, content)
		}
		return ""
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
		// new(T) — type is the first type argument. Other calls (Get(map[string]A{}, k),
		// First2(A{}, B{}), helpers) must not fall through into argument type nodes:
		// typeNameFromTypeNode would harvest the first type_identifier (e.g. map key
		// "string") and poison short-var bindings ahead of generic peels.
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
		return ""
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
		switch n.Type() {
		case "type_spec":
			if id := ingest.ChildByType(n, "type_identifier"); id != nil {
				nextName = ingest.NodeText(id, content)
			}
			if t := ingest.ChildByField(n, "type"); t != nil && t.Type() == "interface_type" {
				nextIface = true
			}
		case "type_alias":
			var nameNode *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "type_identifier" {
					nameNode = ch
					break
				}
			}
			if nameNode != nil {
				nextName = ingest.NodeText(nameNode, content)
			}
			for i := uint32(0); i < n.ChildCount(); i++ {
				if n.Child(i).Type() == "interface_type" {
					nextIface = true
					break
				}
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

// varsAssignedImportedComposite returns identifiers from short assignments of
// imported composite literals or address-of composites, e.g.:
//
//	b := pkga.Box{}
//	b := &pkga.Box{}
//	b = pkga.Box{N: 1}
func varsAssignedImportedComposite(content []byte, importLocal, typeName string) map[string]bool {
	names := map[string]bool{}
	if importLocal == "" || typeName == "" {
		return names
	}
	text := string(content)
	inString := buildStringLiteralMask(text)
	needle := importLocal + "." + typeName
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
		// After type name: optional space then '{' (composite literal).
		k := pos + len(needle)
		for k < len(text) && (text[k] == ' ' || text[k] == '\t') {
			k++
		}
		if k >= len(text) || text[k] != '{' {
			off = pos + len(needle)
			continue
		}
		// Before type name: optional '&' then ':=' or '=' then identifier.
		j := pos - 1
		for j >= 0 && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n') {
			j--
		}
		if j >= 0 && text[j] == '&' {
			j--
			for j >= 0 && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n') {
				j--
			}
		}
		if j < 0 || text[j] != '=' {
			off = pos + len(needle)
			continue
		}
		j-- // before '='
		if j >= 0 && text[j] == ':' {
			j--
		}
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
		// Complex operands (type assert, index, call, composite, paren) are
		// handled by findComplexOperandSelectorEdits.
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

// compositeLiteralTypeName returns T for T{}, &T{}, (*T){}, or nested unary &.
// Used for both composite field keys (Type{Field: …}) and selector receivers
// (Type{}.Method / &Type{}.Method).
func compositeLiteralTypeName(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "composite_literal":
		if t := ingest.ChildByField(n, "type"); t != nil {
			return strings.TrimPrefix(typeNameFromTypeNode(t, content), "*")
		}
		// tree-sitter-go often exposes the type as a bare type_identifier child.
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			switch c.Type() {
			case "type_identifier", "qualified_type", "generic_type", "pointer_type":
				return strings.TrimPrefix(typeNameFromTypeNode(c, content), "*")
			}
		}
	case "unary_expression":
		// &T{}
		for i := uint32(0); i < n.ChildCount(); i++ {
			if t := compositeLiteralTypeName(n.Child(i), content); t != "" {
				return t
			}
		}
	case "parenthesized_expression":
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return compositeLiteralTypeName(ch, content)
		}
	}
	return ""
}

// findComplexOperandSelectorEdits rewrites recv.Method when recv is not a bare
// identifier: v.(T).M, (v.(T)).M, xs[i].M, Make().M, T{}.M / &T{}.M, (*T).M,
// new(T).M, getA().M, (*pa).M, (<-ch).M (channel receive payload),
// A(x).M / (*A)(p).M (type conversion call forms under foreign same-leaf).
//
// When foreign same-leaf methods exist, only rewrite when the operand type is
// known and not a foreign receiver (same filter as identifier selectors).
// When the leaf is unique package-wide (no foreignReceivers), rewrite all
// complex selectors with that leaf.
func findComplexOperandSelectorEdits(file string, content []byte, oldLeaf, newLeaf string, ourReceivers, foreignReceivers map[string]bool) []ingest.Edit {
	pf, err := ingest.ParseSource(content, ".go", "")
	if err != nil {
		return nil
	}
	defer pf.Close()

	// Slice/map index: as[0].M / m[k].M — resolve element/value type from
	// typed params and locals so foreign same-leaf methods are not fail-open
	// rewritten and ours are not skipped. Same-file func results (getA() []*A)
	// feed short-var / inline index too.
	funcColl := sameFileFuncCollectionResults(pf.Root, content)
	namedColl := sameFileNamedCollectionResults(pf.Root, content)
	funcRetFunc := sameFileFuncReturningFuncCollection(pf.Root, content)
	structFields := sameFileStructNamedFuncFields(pf.Root, content)
	indexElemType := collectionIndexElemTypeFunc(pf.Root, content, funcColl)
	// new(T).M / getA().M / (*pa).M under foreign same-leaf methods.
	funcResults := sameFileFuncResultTypes(pf.Root, content)
	genericPeels := sameFileGenericPeelFuncs(pf.Root, content)
	valueType := goIdentTypeAtFunc(pf.Root, content, funcResults)

	uniqueLeaf := len(foreignReceivers) == 0
	var edits []ingest.Edit
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "selector_expression" {
			operand := ingest.ChildByField(n, "operand")
			field := ingest.ChildByField(n, "field")
			if field != nil && ingest.NodeText(field, content) == oldLeaf && operand != nil && !goOperandIsBareIdent(operand) {
				ok := false
				if typ := goComplexOperandType(operand, content, indexElemType, valueType, funcColl, funcResults, genericPeels, namedColl, funcRetFunc, structFields); typ != "" {
					typ = strings.TrimPrefix(typ, "*")
					if ourReceivers[typ] {
						ok = true
					} else if !foreignReceivers[typ] && uniqueLeaf {
						ok = true
					}
				} else if uniqueLeaf {
					// Index/call/paren-ident/etc. without static type: only when unique.
					ok = true
				}
				if ok {
					edits = append(edits, ingest.Edit{
						File:      file,
						StartByte: field.StartByte(),
						EndByte:   field.EndByte(),
						NewText:   newLeaf,
					})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(pf.Root)
	return edits
}

// goIdentTypeAtFunc returns the concrete type of a named local/param/receiver
// covering byte offset at (innermost scope wins). Same bindings as
// selectorCallTargetTypeFunc; used for (*pa).M when pa is typed.
func goIdentTypeAtFunc(root *grammar.Node, content []byte, funcResults map[string][]string) func(name string, at uint32) (string, bool) {
	if root == nil {
		return func(string, uint32) (string, bool) { return "", false }
	}
	genericPeels := sameFileGenericPeelFuncs(root, content)
	namedColl := sameFileNamedCollectionResults(root, content)
	var bindings []identTypeBinding
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
			rangeSrc := map[string]rangeSourceInfo{}
			if params := ingest.ChildByField(n, "parameters"); params != nil {
				collectParameterListBindings(params, content, n.StartByte(), n.EndByte(), &bindings, rangeSrc, namedColl)
			}
			collectResultParameterBindings(n, content, &bindings, rangeSrc, namedColl)
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalTypeBindings(body, content, &bindings, rangeSrc, funcResults, genericPeels, namedColl)
			}
		case "function_declaration", "func_literal":
			rangeSrc := map[string]rangeSourceInfo{}
			if params := ingest.ChildByField(n, "parameters"); params != nil {
				collectParameterListBindings(params, content, n.StartByte(), n.EndByte(), &bindings, rangeSrc, namedColl)
			}
			collectResultParameterBindings(n, content, &bindings, rangeSrc, namedColl)
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalTypeBindings(body, content, &bindings, rangeSrc, funcResults, genericPeels, namedColl)
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return func(name string, at uint32) (string, bool) {
		if name == "" || name == "_" {
			return "", false
		}
		var best *identTypeBinding
		for i := range bindings {
			b := &bindings[i]
			if b.name != name {
				continue
			}
			if at < b.start || at >= b.end {
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

// goOperandIsBareIdent reports whether n is a plain identifier. Parenthesized
// and other complex operands are handled by findComplexOperandSelectorEdits
// (findSelectorLeafEdits only matches identifier characters immediately before '.').
func goOperandIsBareIdent(n *grammar.Node) bool {
	return n != nil && n.Type() == "identifier"
}

// collectionIndexElemTypeFunc returns the element/value named type for a
// collection identifier at a byte offset (as in as[0] / m[k]). Built from
// function/method/func-literal params, typed local var_specs (slice, array,
// map), and collection short-var / var initializers (make, new([n]T), append,
// composite []*T{…} / map[K]*T{…}, same-file getA() []*A /
// as, err := getA() with getA() ([]*A, error)). ok=false when unknown.
// funcColl is same-file function → positional collection result element types
// (may be nil).
func collectionIndexElemTypeFunc(root *grammar.Node, content []byte, funcColl map[string][]rangeSourceInfo) func(name string, at uint32) (string, bool) {
	var bindings []identTypeBinding
	// Named function types: type FA func() []*A / type FA = func() map[K]*A.
	namedFunc := sameFileNamedFuncCollectionResults(root, content)
	// Named collection types: type AS []*A / type AM = map[K]*A.
	namedColl := sameFileNamedCollectionResults(root, content)
	// func getFA() FA / func getFA() func() []*A — result is callable with collection result.
	funcRetFunc := sameFileFuncReturningFuncCollection(root, content)
	// type Box struct { Fa FA } — field call peels.
	structFields := sameFileStructNamedFuncFields(root, content)
	// Concrete types for xa.Fa peels (params/locals like xa BoxA).
	valueType := goIdentTypeAtFunc(root, content, sameFileFuncResultTypes(root, content))
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "method_declaration", "function_declaration", "func_literal":
			rangeSrc := map[string]rangeSourceInfo{}
			var dummy []identTypeBinding
			if params := ingest.ChildByField(n, "parameters"); params != nil {
				collectParameterListBindings(params, content, n.StartByte(), n.EndByte(), &dummy, rangeSrc, namedColl)
			}
			// Receiver is not a collection; skip. Bind collection params to
			// the whole decl so as[0] inside the body resolves.
			for name, info := range rangeSrc {
				if name == "" || name == "_" || info.elemType == "" {
					continue
				}
				bindings = append(bindings, identTypeBinding{n.StartByte(), n.EndByte(), name, info.elemType})
			}
			// Function-typed params with a single collection result:
			// fa func() []*A / fm func() map[K]*A / fa FA with type FA func() []*A —
			// bind fa→A so fa()[0].M and as := fa(); as[0].M resolve under foreign
			// same-leaf methods. Multi-result func types fail closed.
			if params := ingest.ChildByField(n, "parameters"); params != nil {
				for i := uint32(0); i < params.ChildCount(); i++ {
					p := params.Child(i)
					if p == nil || p.Type() != "parameter_declaration" {
						continue
					}
					typeN := ingest.ChildByField(p, "type")
					info, ok := functionTypeOrNamedCollectionResult(typeN, content, namedFunc)
					if !ok || info.elemType == "" {
						continue
					}
					for _, name := range parameterDeclNames(p, content) {
						if name == "" || name == "_" {
							continue
						}
						bindings = append(bindings, identTypeBinding{n.StartByte(), n.EndByte(), name, info.elemType})
					}
				}
			}
			if body := ingest.ChildByField(n, "body"); body != nil {
				collectLocalCollectionElemBindings(body, content, &bindings, funcColl, namedFunc, namedColl, funcRetFunc, structFields, valueType)
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)

	return func(name string, at uint32) (string, bool) {
		if name == "" {
			return "", false
		}
		var best *identTypeBinding
		for i := range bindings {
			b := &bindings[i]
			if b.name != name {
				continue
			}
			if at < b.start || at >= b.end {
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

// rangeSourceFromMakeCall reports element/value (and map key) type for
// make(T, n[, cap]) when T is a slice, array, map, channel, or same-file named
// collection type (type AS []*A / type AM map[K]*A). Len/cap args are ignored.
// expression_list peels to its first expression only. namedColl may be nil.
func rangeSourceFromMakeCall(n *grammar.Node, content []byte, namedColl map[string]rangeSourceInfo) (rangeSourceInfo, bool) {
	if n == nil {
		return rangeSourceInfo{}, false
	}
	if n.Type() == "expression_list" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			return rangeSourceFromMakeCall(c, content, namedColl)
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() != "call_expression" {
		return rangeSourceInfo{}, false
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || ingest.NodeText(fn, content) != "make" {
		return rangeSourceInfo{}, false
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return rangeSourceInfo{}, false
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		c := args.Child(i)
		if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
			continue
		}
		// First argument is the type: make([]*T, n) / make(map[K]*T) / make(AS).
		return collectionTypeOrNamedResult(c, content, namedColl)
	}
	return rangeSourceInfo{}, false
}

// rangeSourceFromCollectionExpr reports element/value (and map key) type for
// expressions that construct or retype a slice/array/map: composite literals
// ([]*T{…} / map[K]*T{…}), make(T, …), new([n]T) (pointer-to-array is
// indexable), append(slice, …) when the first argument is itself such an
// expression, type assert/convert to a collection type (x.([]*T) / ([]*T)(x) /
// x.(map[K]*T)), slice expressions (as[:n] / as[i:] / as[:]) which preserve
// the operand's element type, and same-file calls with a single collection
// result (getA() []*A). Used for short-var / untyped-var collection locals
// and inline index operands (append([]*A{}, x)[0] / x.([]*A)[0] / as[:1][0] /
// new([1]*A)[0] / getA()[0]).
//
// Prefer rangeSourceFromCollectionExprIdent when the first append argument may
// be a known collection param/local (append(as, x) where as []*A).
func rangeSourceFromCollectionExpr(n *grammar.Node, content []byte) (rangeSourceInfo, bool) {
	return rangeSourceFromCollectionExprIdent(n, content, nil, 0, nil, nil, nil, nil, nil)
}

// rangeSourceFromCollectionExprIdent is rangeSourceFromCollectionExpr plus
// optional resolution of bare collection identifiers via identElem (params and
// prior short-var/var bindings), function-typed params/vars with a single
// collection result (fa func() []*A → fa() peels via identElem), and same-file
// function collection results via funcColl. That covers append(as, …) /
// append(append(as, …), …) when as is a typed slice/map local or parameter,
// as[:n] / s := as[:n] when as is known, getA() / as := getA() when getA
// returns a single slice/array/map, and fa() / as := fa() when fa is
// func() []*A / func() map[K]*A (param or var).
// Multi-result helpers (getA() ([]*A, error)) are not valid as a lone call
// expression value — those bind via collectLocalCollectionElemBindings.
func rangeSourceFromCollectionExprIdent(n *grammar.Node, content []byte, identElem func(name string, at uint32) (string, bool), at uint32, funcColl map[string][]rangeSourceInfo, namedColl map[string]rangeSourceInfo, funcRetFunc map[string]rangeSourceInfo, structFields map[string]map[string]rangeSourceInfo, valueType func(string, uint32) (string, bool)) (rangeSourceInfo, bool) {
	if n == nil {
		return rangeSourceInfo{}, false
	}
	if n.Type() == "expression_list" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			return rangeSourceFromCollectionExprIdent(c, content, identElem, at, funcColl, namedColl, funcRetFunc, structFields, valueType)
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() == "parenthesized_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return rangeSourceFromCollectionExprIdent(ch, content, identElem, at, funcColl, namedColl, funcRetFunc, structFields, valueType)
		}
		return rangeSourceInfo{}, false
	}
	// (*pas) / (*pam) — pointer-to-collection param/local (pas *[]*A / *map[K]*A).
	// indexElemType peels *[]T / *map to element T; enable (*pas)[1:][0] and
	// range over *pas under foreign same-leaf (same leaf as (*pas)[0]).
	// (*&as)[0] / (*&aa)[0] — address-of then deref of a collection local/param:
	// peel unary & so the subsequent * (or pointer-to-array index) reaches the
	// identifier (dual-class under foreign same-leaf).
	if n.Type() == "unary_expression" {
		opText := ""
		var inner *grammar.Node
		if opField := ingest.ChildByField(n, "operator"); opField != nil {
			opText = ingest.NodeText(opField, content)
		}
		if opField := ingest.ChildByField(n, "operand"); opField != nil {
			inner = opField
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch == nil {
				continue
			}
			if ch.Type() == "*" || ch.Type() == "&" {
				if opText == "" {
					opText = ch.Type()
				}
				continue
			}
			if ch.Type() == "unary_operator" {
				t := ingest.NodeText(ch, content)
				if t == "*" || t == "&" {
					if opText == "" {
						opText = t
					}
					continue
				}
			}
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			if inner == nil {
				inner = ch
			}
		}
		if (opText == "*" || opText == "&") && inner != nil {
			for inner != nil && inner.Type() == "parenthesized_expression" {
				var in2 *grammar.Node
				for i := uint32(0); i < inner.ChildCount(); i++ {
					ch := inner.Child(i)
					if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					in2 = ch
					break
				}
				inner = in2
			}
			return rangeSourceFromCollectionExprIdent(inner, content, identElem, at, funcColl, namedColl, funcRetFunc, structFields, valueType)
		}
	}
	// as[:n] / as[i:] / as[i:j] / as[:] — result has the same element type as
	// the sliced collection (param, local, make/append/composite, nested slice).
	if n.Type() == "slice_expression" {
		if op := ingest.ChildByField(n, "operand"); op != nil {
			return rangeSourceFromCollectionExprIdent(op, content, identElem, at, funcColl, namedColl, funcRetFunc, structFields, valueType)
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() == "identifier" {
		if identElem != nil {
			if el, ok := identElem(ingest.NodeText(n, content), at); ok && el != "" {
				return rangeSourceInfo{elemType: el}, true
			}
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() == "composite_literal" {
		if t := ingest.ChildByField(n, "type"); t != nil {
			// []*T{…} / map[K]*T{…} / AS{…} / AM{…} with type AS []*A.
			return collectionTypeOrNamedResult(t, content, namedColl)
		}
		return rangeSourceInfo{}, false
	}
	// x.([]*T) / x.(AS) / x.(map[K]*T) / ([]*T)(x) / (AS)(x) — type field.
	if n.Type() == "type_assertion_expression" || n.Type() == "type_conversion_expression" {
		if t := ingest.ChildByField(n, "type"); t != nil {
			return collectionTypeOrNamedResult(t, content, namedColl)
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() != "call_expression" {
		return rangeSourceInfo{}, false
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return rangeSourceInfo{}, false
	}
	switch ingest.NodeText(fn, content) {
	case "make":
		return rangeSourceFromMakeCall(n, content, namedColl)
	case "new":
		// new([n]*T) returns *[n]*T; Go allows indexing the pointer-to-array
		// without dereference. new([]*T) / new(map[…]) return non-indexable
		// pointers — only array type args count as collection sources.
		return rangeSourceFromNewArrayCall(n, content, namedColl)
	case "append":
		args := ingest.ChildByField(n, "arguments")
		if args == nil {
			return rangeSourceInfo{}, false
		}
		for i := uint32(0); i < args.ChildCount(); i++ {
			c := args.Child(i)
			if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
				continue
			}
			// First argument is the slice: append([]*T{}, …) / append(as, …) /
			// append(make(...), …) / append(getA(), …).
			return rangeSourceFromCollectionExprIdent(c, content, identElem, at, funcColl, namedColl, funcRetFunc, structFields, valueType)
		}
		return rangeSourceInfo{}, false
	}
	// Callable with a single collection result:
	//  1. Function-typed params/vars in scope: fa func() []*A / var fa func() map[K]*A
	//     (registered in identElem with the *result* element type). Prefer these
	//     over same-file helpers so params shadow file-level functions.
	//  2. Same-file helper: getA() []*A / getM() map[K]*A / getAS() AS.
	// Multi-result signatures cannot appear as a lone call value (compile
	// error); those bind positionally via short-var / var.
	// Note: bare collection idents (as []*A) also live in identElem; as() is
	// invalid Go so a peel here is harmless on real code.
	if fn.Type() == "identifier" {
		name := ingest.NodeText(fn, content)
		if identElem != nil {
			if el, ok := identElem(name, at); ok && el != "" {
				return rangeSourceInfo{elemType: el}, true
			}
		}
		if funcColl != nil {
			if infos := funcColl[name]; len(infos) == 1 && infos[0].elemType != "" {
				return infos[0], true
			}
		}
		// AS(x) — named type conversion parsed as call_expression (not
		// type_conversion_expression). Prefer real funcs above; only peel when
		// name is a same-file named collection type.
		if info, ok := namedColl[name]; ok && info.elemType != "" {
			return info, true
		}
	}
	// (AS)(x) / (AM)(x) — parenthesized named type as call callee (tree-sitter-go).
	// Inline ([]*A)(x) is type_conversion_expression (handled above).
	if fn.Type() == "parenthesized_expression" && len(namedColl) > 0 {
		for i := uint32(0); i < fn.ChildCount(); i++ {
			ch := fn.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			// (AS) → identifier / type_identifier; (*AS) not a conversion form here.
			if ch.Type() == "identifier" || ch.Type() == "type_identifier" {
				if info, ok := namedColl[ingest.NodeText(ch, content)]; ok && info.elemType != "" {
					return info, true
				}
			}
			break
		}
	}
	// getFA()() — nested call when getFA returns a function type with a single
	// collection result (type FA func() []*A / inline func() []*A). Multi-result
	// helpers fail closed (not in funcRetFunc).
	if fn.Type() == "call_expression" && len(funcRetFunc) > 0 {
		innerFn := ingest.ChildByField(fn, "function")
		if innerFn != nil && innerFn.Type() == "identifier" {
			if info, ok := funcRetFunc[ingest.NodeText(innerFn, content)]; ok && info.elemType != "" {
				return info, true
			}
		}
	}
	// xa.Fa() — struct field of named/inline function type with collection result.
	if fn.Type() == "selector_expression" {
		if info, ok := selectorNamedFuncField(fn, content, valueType, structFields, at); ok && info.elemType != "" {
			return info, true
		}
	}
	return rangeSourceInfo{}, false
}

// rangeSourceFromNewArrayCall reports element type for new([n]T) / new(AA) when
// the type argument is an array (possibly parenthesized) or a same-file named
// array type (type AA [n]*T with isArray). Len is ignored. new([]*T) / new(AS)
// and other non-array type args return false (result is not indexable).
// namedColl may be nil.
func rangeSourceFromNewArrayCall(n *grammar.Node, content []byte, namedColl map[string]rangeSourceInfo) (rangeSourceInfo, bool) {
	if n == nil {
		return rangeSourceInfo{}, false
	}
	if n.Type() == "expression_list" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			return rangeSourceFromNewArrayCall(c, content, namedColl)
		}
		return rangeSourceInfo{}, false
	}
	if n.Type() != "call_expression" {
		return rangeSourceInfo{}, false
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || ingest.NodeText(fn, content) != "new" {
		return rangeSourceInfo{}, false
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return rangeSourceInfo{}, false
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		c := args.Child(i)
		if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
			continue
		}
		// Peel new(([n]*T)).
		for c != nil && c.Type() == "parenthesized_type" {
			var inner *grammar.Node
			for j := uint32(0); j < c.ChildCount(); j++ {
				ch := c.Child(j)
				if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
			c = inner
		}
		if c == nil {
			return rangeSourceInfo{}, false
		}
		if c.Type() == "array_type" {
			return rangeSourceFromTypeNode(c, content)
		}
		// new(AA) with type AA [n]*T — only named arrays (isArray), not slices.
		if c.Type() == "type_identifier" && len(namedColl) > 0 {
			info, ok := namedColl[ingest.NodeText(c, content)]
			if ok && info.elemType != "" && info.isArray {
				return info, true
			}
		}
		return rangeSourceInfo{}, false
	}
	return rangeSourceInfo{}, false
}

// collectLocalCollectionElemBindings records slice/array/map locals from
// var_spec with an explicit type (var as []*A / var m map[K]*B), function-typed
// vars with a single collection result (var fa func() []*A / var fa FA with
// type FA func() []*A), and from collection short-var / untyped var initializers:
// make(T, …), new([n]T), append([]*T{}, …) / append(ident, …), []*T{…} /
// map[K]*T{…}, x.([]*T) / ([]*T)(x), as[:n] / as[i:], same-file getA() []*A /
// multi-return as, err := getA() with getA() ([]*A, error), fa() when fa is a
// function-typed param/var with a single collection result, and
// fa := func() []*A { … } / var fa = func() map[K]*A { … } (func-literal with
// a single collection result; multi-result func literals fail closed).
// append(ident, …) and slice of ident resolve against params and earlier
// collection bindings already recorded in *bindings. funcColl maps same-file
// function names to positional collection result element types (may be nil).
// namedFunc maps same-file type names (type FA func() []*A) to collection
// result element info (may be nil). namedColl maps type AS []*A / AM map[K]*A
// (may be nil).
func collectLocalCollectionElemBindings(body *grammar.Node, content []byte, bindings *[]identTypeBinding, funcColl map[string][]rangeSourceInfo, namedFunc map[string]rangeSourceInfo, namedColl map[string]rangeSourceInfo, funcRetFunc map[string]rangeSourceInfo, structFields map[string]map[string]rangeSourceInfo, valueType func(string, uint32) (string, bool)) {
	if body == nil {
		return
	}
	scopeEnd := body.EndByte()
	// Lookup collection element type for a name in already-recorded bindings
	// (params bound by the caller, plus locals bound earlier in this walk).
	lookupElem := func(name string, at uint32) (string, bool) {
		if name == "" {
			return "", false
		}
		var best *identTypeBinding
		for i := range *bindings {
			b := &(*bindings)[i]
			if b.name != name {
				continue
			}
			if at < b.start || at >= b.end {
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
	bindElem := func(start uint32, names []string, info rangeSourceInfo) {
		if info.elemType == "" {
			return
		}
		for _, name := range names {
			if name == "" || name == "_" {
				continue
			}
			*bindings = append(*bindings, identTypeBinding{start, scopeEnd, name, info.elemType})
		}
	}
	// Positional RHS expressions for multi-value short-var / var_spec.
	rhsExprs := func(right *grammar.Node) []*grammar.Node {
		if right == nil {
			return nil
		}
		if right.Type() != "expression_list" {
			return []*grammar.Node{right}
		}
		var out []*grammar.Node
		for i := uint32(0); i < right.ChildCount(); i++ {
			c := right.Child(i)
			if c == nil || c.Type() == "," {
				continue
			}
			out = append(out, c)
		}
		return out
	}
	// Bind names from a multi-return same-file collection call
	// (as, err := getA() / var as, err = getA()). Returns true when the RHS
	// was such a call (even if no collection slot bound — skip expr walk).
	bindMultiReturnCall := func(start uint32, names []string, right *grammar.Node) bool {
		infos := collectionInfosFromCallResults(right, content, funcColl)
		if len(infos) == 0 {
			return false
		}
		// Only multi-return needs positional slots here; single-result
		// as := getA() still goes through the per-expr path below so
		// getA()[0]-style inline stays consistent.
		if len(infos) < 2 {
			return false
		}
		for i, name := range names {
			if name == "" || name == "_" || i >= len(infos) {
				continue
			}
			if infos[i].elemType == "" {
				continue
			}
			*bindings = append(*bindings, identTypeBinding{start, scopeEnd, name, infos[i].elemType})
		}
		return true
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "var_spec":
			names := varSpecNames(n, content)
			typeN := ingest.ChildByField(n, "type")
			if info, ok := rangeSourceFromTypeNode(typeN, content); ok && info.elemType != "" {
				bindElem(n.StartByte(), names, info)
			} else if info, ok := collectionTypeOrNamedResult(typeN, content, namedColl); ok && info.elemType != "" {
				// var xa AS / var pas *AS with type AS []*A.
				bindElem(n.StartByte(), names, info)
			} else if info, ok := functionTypeOrNamedCollectionResult(typeN, content, namedFunc); ok && info.elemType != "" {
				// var fa func() []*A / var fa FA (type FA func() []*A) —
				// fa() peels to result elem.
				bindElem(n.StartByte(), names, info)
			} else {
				valueN := ingest.ChildByField(n, "value")
				// var as, err = getA() — multi-return collection + non-collection.
				if bindMultiReturnCall(n.StartByte(), names, valueN) {
					break
				}
				// var as = make/append/getA()/[]*A{…}/append(ident,…) — bind element type positionally.
				// var fa = func() []*A { … } — single collection-result func literal.
				exprs := rhsExprs(valueN)
				for i, name := range names {
					if name == "" || name == "_" || i >= len(exprs) {
						continue
					}
					if info, ok := rangeSourceFromCollectionExprIdent(exprs[i], content, lookupElem, n.StartByte(), funcColl, namedColl, funcRetFunc, structFields, valueType); ok && info.elemType != "" {
						*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, info.elemType})
					} else if infos := functionCollectionResults(exprs[i], content, namedColl); len(infos) == 1 && infos[0].elemType != "" {
						// var fa = func() []*A { … } / var fm = func() map[K]*A { … }.
						// Multi-result func literals fail closed (len != 1).
						*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, infos[0].elemType})
					} else if info, ok := callReturningFuncCollection(exprs[i], content, funcRetFunc); ok {
						// var fa = getFA() with getFA() FA / func() []*A.
						*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, info.elemType})
					} else if info, ok := selectorNamedFuncField(exprs[i], content, valueType, structFields, n.StartByte()); ok {
						// var fa = xa.Fa with type Box struct { Fa FA }.
						*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, info.elemType})
					}
				}
			}
		case "short_var_declaration":
			// as := make/append/getA()/[]*A{…}/append(ident,…) — same positional binding.
			// as, err := getA() with getA() ([]*A, error) — multi-return slots.
			// fa := func() []*A { … } / fa, fb := func() []*A{…}, func() []*B{…}.
			// fa := getFA() / fa := xa.Fa — named func type returns / struct fields.
			names := identListNames(ingest.ChildByField(n, "left"), content)
			right := ingest.ChildByField(n, "right")
			if bindMultiReturnCall(n.StartByte(), names, right) {
				break
			}
			exprs := rhsExprs(right)
			for i, name := range names {
				if name == "" || name == "_" || i >= len(exprs) {
					continue
				}
				if info, ok := rangeSourceFromCollectionExprIdent(exprs[i], content, lookupElem, n.StartByte(), funcColl, namedColl, funcRetFunc, structFields, valueType); ok && info.elemType != "" {
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, info.elemType})
				} else if infos := functionCollectionResults(exprs[i], content, namedColl); len(infos) == 1 && infos[0].elemType != "" {
					// fa := func() []*A { … } / fa := func() map[K]*A { … }.
					// Single collection result only; multi-result func literals fail closed.
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, infos[0].elemType})
				} else if info, ok := callReturningFuncCollection(exprs[i], content, funcRetFunc); ok {
					// fa := getFA() with getFA() FA / func() []*A.
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, info.elemType})
				} else if info, ok := selectorNamedFuncField(exprs[i], content, valueType, structFields, n.StartByte()); ok {
					// fa := xa.Fa with type Box struct { Fa FA }.
					*bindings = append(*bindings, identTypeBinding{n.StartByte(), scopeEnd, name, info.elemType})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(body)
}

// goComplexOperandType returns a type name for operands we can resolve without
// full type inference: type assert/convert, composite lit, &T{}, (*T), paren,
// new(T), same-file getA(), (*pa) via valueType, and index expressions
// (as[0] / m[k]) when indexElemType knows the collection.
// indexElemType / valueType / funcColl / funcResults may be nil when unavailable.
func goComplexOperandType(n *grammar.Node, content []byte, indexElemType, valueType func(name string, at uint32) (string, bool), funcColl map[string][]rangeSourceInfo, funcResults map[string][]string, genericPeels map[string]string, namedColl map[string]rangeSourceInfo, funcRetFunc map[string]rangeSourceInfo, structFields map[string]map[string]rangeSourceInfo) string {
	if n == nil {
		return ""
	}
	// Prefer full composite resolution (T{}, &T{}, bare type_identifier children).
	// Skip for index_expression — compositeLiteralTypeName does not apply, and
	// we must not peel the operand into a bare type by accident.
	if n.Type() != "index_expression" {
		if t := compositeLiteralTypeName(n, content); t != "" {
			return t
		}
	}
	switch n.Type() {
	case "parenthesized_expression":
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return goComplexOperandType(ch, content, indexElemType, valueType, funcColl, funcResults, genericPeels, namedColl, funcRetFunc, structFields)
		}
	case "type_assertion_expression", "type_conversion_expression":
		if t := ingest.ChildByField(n, "type"); t != nil {
			return typeNameFromTypeNode(t, content)
		}
	case "call_expression":
		// new(T).M — allocated value is *T; methods promote, type name is T.
		// Under foreign same-leaf methods, a := new(T); a.M already peels via
		// typeNameFromRHS; inline new(T).M must recover T from the type arg.
		fn := ingest.ChildByField(n, "function")
		if fn != nil && ingest.NodeText(fn, content) == "new" {
			if args := ingest.ChildByField(n, "arguments"); args != nil {
				for i := uint32(0); i < args.ChildCount(); i++ {
					c := args.Child(i)
					if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
						continue
					}
					if t := concreteNamedType(c, content); t != "" {
						return t
					}
					// new((*T)) / parenthesized type args.
					if t := typeNameFromTypeNode(c, content); t != "" {
						return t
					}
				}
			}
			return ""
		}
		// getA().M — same-file helper with a single concrete result (*A / A).
		// Collection results (getA() []*A) are not method receivers here;
		// those go through index (getA()[0].M). Multi-result calls cannot
		// appear as a lone value (compile error).
		if fn != nil && fn.Type() == "identifier" {
			name := ingest.NodeText(fn, content)
			// Generic peels before funcResults so result type-param "T" is not used as a leaf.
			// "arg":  Id(A{}).M — identity peels arg type.
			// "elem": First([]A{{}}).M / At(xs, i).M — peels first-arg element type.
			if mode := genericPeels[name]; mode == "arg" {
				if t := goCallFirstArgType(n, content, indexElemType, valueType, funcColl, funcResults, genericPeels, namedColl, funcRetFunc, structFields); t != "" {
					return t
				}
				return ""
			} else if mode == "elem" {
				if t := goCallFirstArgElemType(n, content, indexElemType, funcColl, namedColl, funcRetFunc, structFields, valueType); t != "" {
					return t
				}
				return ""
			}
			if len(funcResults) > 0 {
				if types := funcResults[name]; len(types) == 1 && types[0] != "" {
					return types[0]
				}
				// Multi-result / empty-concrete same-file func: not a conversion.
				if _, isFunc := funcResults[name]; isFunc {
					return ""
				}
			}
			// f().M — local func var f := func() *A { … } bound via valueType
			// (func-literal result leaf). Enables f().Run under foreign same-leaf.
			if valueType != nil {
				if t, ok := valueType(name, n.StartByte()); ok && t != "" {
					return t
				}
			}
			// A(x).M — type conversion parsed as call_expression by tree-sitter-go
			// (not type_conversion_expression). When the callee is not a same-file
			// function / typed func var, treat the identifier as the conversion
			// type name. Under foreign same-leaf methods, caller filters via
			// ourReceivers so B(x).Run is preserved when renaming A.Run.
			if name != "" {
				return name
			}
		}
		// (*A)(p).M / (A)(p).M — parenthesized conversion type as call callee.
		// tree-sitter-go parses these as call_expression too; peel the type from
		// the parenthesized operand (unary *A / type_identifier A).
		if fn != nil && fn.Type() == "parenthesized_expression" {
			if t := goComplexOperandType(fn, content, indexElemType, valueType, funcColl, funcResults, genericPeels, namedColl, funcRetFunc, structFields); t != "" {
				return t
			}
		}
	case "unary_expression":
		// (<-ch).M — channel receive payload type (same leaf as a := <-ch).
		// Under foreign same-leaf methods, assigned receive already renames via
		// typed locals; inline (<-ch).M must recover T from the channel param.
		if op := ingest.ChildByField(n, "operator"); op != nil && ingest.NodeText(op, content) == "<-" {
			operand := ingest.ChildByField(n, "operand")
			for operand != nil && operand.Type() == "parenthesized_expression" {
				var inner *grammar.Node
				for i := uint32(0); i < operand.ChildCount(); i++ {
					ch := operand.Child(i)
					if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					inner = ch
					break
				}
				operand = inner
			}
			if operand != nil && operand.Type() == "identifier" && indexElemType != nil {
				if el, ok := indexElemType(ingest.NodeText(operand, content), n.StartByte()); ok {
					return el
				}
			}
			return ""
		}
		// (*pa).M — typed local/param; (*T).M method expression when unbound.
		var starIdent string
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "identifier" {
				starIdent = ingest.NodeText(ch, content)
				continue
			}
			if t := goComplexOperandType(ch, content, indexElemType, valueType, funcColl, funcResults, genericPeels, namedColl, funcRetFunc, structFields); t != "" {
				return t
			}
		}
		if starIdent != "" {
			if valueType != nil {
				if t, ok := valueType(starIdent, n.StartByte()); ok {
					return t
				}
			}
			// (*T).M method expression — identifier is the type name.
			return strings.TrimPrefix(starIdent, "*")
		}
	case "pointer_type", "type_identifier":
		return typeNameFromTypeNode(n, content)
	case "index_expression":
		// as[0].M / m[k].M — element/value type of the collection operand.
		// (*pas)[0].M — pointer-to-slice/map param (pas *[]*A / *map[K]*A):
		// indexElemType already peels *[]T / *map to element T via
		// rangeSourceFromTypeNode pointer_type; only the unary * operand needs
		// unwrapping here (dual-class under foreign same-leaf).
		op := ingest.ChildByField(n, "operand")
		if op == nil {
			return ""
		}
		// Peel (as)[0] / (*pas)[0] outer parens.
		for op != nil && op.Type() == "parenthesized_expression" {
			var inner *grammar.Node
			for i := uint32(0); i < op.ChildCount(); i++ {
				ch := op.Child(i)
				if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
			op = inner
		}
		if op == nil {
			return ""
		}
		// (*pas)[0] — unary * of collection pointer local/param.
		if op.Type() == "unary_expression" {
			star := false
			var inner *grammar.Node
			for i := uint32(0); i < op.ChildCount(); i++ {
				ch := op.Child(i)
				if ch == nil {
					continue
				}
				if ch.Type() == "*" || (ch.Type() == "unary_operator" && ingest.NodeText(ch, content) == "*") {
					star = true
					continue
				}
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				// Prefer field-named operand when present.
				inner = ch
			}
			if opField := ingest.ChildByField(op, "operand"); opField != nil {
				inner = opField
			}
			if opField := ingest.ChildByField(op, "operator"); opField != nil && ingest.NodeText(opField, content) == "*" {
				star = true
			}
			if star && inner != nil {
				// Peel (*pas) inner parens if any.
				for inner != nil && inner.Type() == "parenthesized_expression" {
					var in2 *grammar.Node
					for i := uint32(0); i < inner.ChildCount(); i++ {
						ch := inner.Child(i)
						if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
							continue
						}
						in2 = ch
						break
					}
					inner = in2
				}
				if inner != nil && inner.Type() == "identifier" && indexElemType != nil {
					if el, ok := indexElemType(ingest.NodeText(inner, content), n.StartByte()); ok {
						return el
					}
				}
				if info, ok := rangeSourceFromCollectionExprIdent(inner, content, indexElemType, n.StartByte(), funcColl, namedColl, funcRetFunc, structFields, valueType); ok {
					return info.elemType
				}
			}
		}
		if op.Type() == "identifier" && indexElemType != nil {
			if el, ok := indexElemType(ingest.NodeText(op, content), n.StartByte()); ok {
				return el
			}
		}
		// Typed collection index: []*A{…}[0] / make([]*A,n)[0] /
		// append([]*A{}, x)[0] / append(as, x)[0] / map[string]*A{…}["k"] /
		// x.([]*A)[0] / ([]*A)(x)[0] / as[:1][0] / make([]*A,n)[:1][0] /
		// new([1]*A)[0] / getA()[0].
		if info, ok := rangeSourceFromCollectionExprIdent(op, content, indexElemType, n.StartByte(), funcColl, namedColl, funcRetFunc, structFields, valueType); ok {
			return info.elemType
		}
	}
	return ""
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
// type in the file: named declarations (type Worker interface { M() }) and
// inline/anonymous interfaces (func Call[T interface{ M() }](...),
// var t interface{ M() }, parameters, returns).
// Includes both `type T interface` and `type T = interface` forms.
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
		case "interface_type":
			// Bare interface types (params, returns, type params, vars) as well as
			// the body under type_spec. Enter method_elem children with inIface.
			nextIface = true
		case "type_spec":
			if t := ingest.ChildByField(n, "type"); t != nil && t.Type() == "interface_type" {
				nextIface = true
			}
		case "type_alias":
			for i := uint32(0); i < n.ChildCount(); i++ {
				if n.Child(i).Type() == "interface_type" {
					nextIface = true
					break
				}
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
