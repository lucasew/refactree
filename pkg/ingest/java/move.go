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

// ExtraRenameEdits covers two cases relation-based rename misses for Type.member:
//  1. Interface/override method names on implementors and related types
//     (Task.work when Worker.work renames).
//  2. Instance method_invocation / field_access name spans (m.member) via AST walk.
func (moveDriver) ExtraRenameEdits(rootDir string, result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) []ingest.Edit {
	if oldLeaf == "" || oldLeaf == newLeaf || len(sourceRefs) == 0 || rootDir == "" || result == nil {
		return nil
	}
	src := ingest.ParseReference(sourceRefs[0])
	if !strings.Contains(src.Symbol, ".") {
		return nil
	}

	sourceSet := map[string]bool{}
	ourTypes := map[string]bool{} // types whose Type.oldLeaf is in sourceRefs
	ourReceivers := map[string]bool{}
	for _, s := range sourceRefs {
		sourceSet[s] = true
		ref := ingest.ParseReference(s)
		if t, m, ok := javaSplitTypeMethod(ref.Symbol); ok && m == oldLeaf {
			ourTypes[t] = true
			if i := strings.LastIndex(t, "."); i >= 0 {
				ourTypes[t[i+1:]] = true
			}
		}
		if recv, ok := javaMemberReceiver(ref.Symbol); ok {
			ourReceivers[recv] = true
		}
	}
	if len(ourTypes) == 0 && len(ourReceivers) == 0 {
		return nil
	}

	occupied := ingest.MarkEntityRelationSpans(result, sourceSet)
	markOccupied := func(file string, start, end uint32) {
		file = strings.TrimPrefix(file, "./")
		if occupied[file] == nil {
			occupied[file] = map[[2]uint32]bool{}
		}
		occupied[file][[2]uint32{start, end}] = true
	}

	var edits []ingest.Edit

	// (1) Interface / override method declaration renames on related types.
	if len(ourTypes) > 0 {
		implementsEdges := javaImplementsEdges(rootDir, result)
		// alsoTypes: implementors of our types and interfaces/supertypes our types implement.
		alsoTypes := map[string]bool{}
		for t := range ourTypes {
			for iface := range implementsEdges[t] {
				alsoTypes[iface] = true
			}
			for impl, ifaces := range implementsEdges {
				if ifaces[t] {
					alsoTypes[impl] = true
				}
			}
		}
		for _, ent := range result.Entities {
			if sourceSet[ent.Reference] {
				continue
			}
			ref := ingest.ParseReference(ent.Reference)
			t, m, ok := javaSplitTypeMethod(ref.Symbol)
			if !ok || m != oldLeaf {
				continue
			}
			tLeaf := t
			if i := strings.LastIndex(t, "."); i >= 0 {
				tLeaf = t[i+1:]
			}
			if !alsoTypes[t] && !alsoTypes[tLeaf] {
				continue
			}
			file := strings.TrimPrefix(ref.Path, "./")
			if ingest.SpanOccupied(occupied[file], ent.StartByte, ent.EndByte) {
				continue
			}
			edits = append(edits, ingest.Edit{
				File:      file,
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
				NewText:   newLeaf,
			})
			markOccupied(file, ent.StartByte, ent.EndByte)
		}
	}

	// (2) Instance member access: m.oldLeaf method_invocation / field_access.
	if len(ourReceivers) > 0 {
		foreignReceivers := map[string]bool{}
		for _, ent := range result.Entities {
			if sourceSet[ent.Reference] {
				continue
			}
			ref := ingest.ParseReference(ent.Reference)
			if ingest.SymbolLeaf(ref.Symbol) != oldLeaf {
				continue
			}
			if recv, ok := javaMemberReceiver(ref.Symbol); ok && !ourReceivers[recv] {
				foreignReceivers[recv] = true
			}
		}
		for _, f := range result.Files {
			if f.Language != "java" {
				continue
			}
			rel := strings.TrimPrefix(f.Path, "./")
			content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
			if err != nil {
				continue
			}
			occ := occupied[rel]
			for _, e := range javaMemberAccessEdits(rel, content, oldLeaf, newLeaf, ourReceivers, foreignReceivers) {
				if ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
					continue
				}
				edits = append(edits, e)
			}
		}
	}

	return edits
}

func javaSplitTypeMethod(symbol string) (typeName, method string, ok bool) {
	i := strings.LastIndex(symbol, ".")
	if i <= 0 || i+1 >= len(symbol) {
		return "", "", false
	}
	return symbol[:i], symbol[i+1:], true
}

// javaImplementsEdges returns map[typeSimpleName]set[ifaceOrSuperSimpleName]
// from implements / extends clauses across Java files in result.
func javaImplementsEdges(rootDir string, result *ingest.Result) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if result == nil {
		return out
	}
	for _, f := range result.Files {
		if f.Language != "java" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		pf, err := ingest.ParseSource(content, rel, "java")
		if err != nil {
			continue
		}
		collectJavaImplementsEdges(pf.Root, content, out)
		pf.Close()
	}
	return out
}

func collectJavaImplementsEdges(n *grammar.Node, source []byte, out map[string]map[string]bool) {
	if n == nil || n.IsNull() {
		return
	}
	switch n.Type() {
	case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
		nameN := ingest.ChildByField(n, "name")
		if nameN == nil {
			break
		}
		typeName := ingest.NodeText(nameN, source)
		add := func(clause *grammar.Node) {
			if clause == nil {
				return
			}
			for _, id := range javaTypeNamesInClause(clause, source) {
				if out[typeName] == nil {
					out[typeName] = map[string]bool{}
				}
				out[typeName][id] = true
			}
		}
		add(ingest.ChildByField(n, "interfaces"))
		add(ingest.ChildByField(n, "superclass"))
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		collectJavaImplementsEdges(n.Child(i), source, out)
	}
}

func javaTypeNamesInClause(n *grammar.Node, source []byte) []string {
	if n == nil {
		return nil
	}
	var names []string
	var walk func(*grammar.Node)
	walk = func(node *grammar.Node) {
		if node == nil || node.IsNull() {
			return
		}
		switch node.Type() {
		case "type_identifier", "identifier":
			names = append(names, ingest.NodeText(node, source))
			return
		case "scoped_type_identifier":
			// Use leaf type name (Outer.Inner → Inner) for matching simple entities.
			if name := ingest.ChildByField(node, "name"); name != nil {
				names = append(names, ingest.NodeText(name, source))
				return
			}
		}
		for i := uint32(0); i < node.ChildCount(); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return names
}

func (moveDriver) ExtractDecl(filePath string, entity ingest.Entity) (ingest.DeclExtract, error) {
	pf, err := ingest.ParseSourceFile(filePath, "java")
	if err != nil {
		return ingest.DeclExtract{}, err
	}
	defer pf.Close()
	source, root := pf.Source, pf.Root

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
				NewText:   ingest.AppendDeclText(merged, decl.DeclText),
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
		NewText:   ingest.AppendDeclText(body, decl.DeclText),
	}
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
		if pos > 0 && ingest.IsIdentCharJava(text[pos-1]) {
			off = end
			continue
		}
		if end < len(text) && ingest.IsIdentCharJava(text[end]) {
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
		if start > 0 && ingest.IsIdentCharJava(text[start-1]) {
			off = end
			continue
		}
		if end < len(text) && ingest.IsIdentCharJava(text[end]) {
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

// javaMemberReceiver returns the type qualifier for "Type.member" symbols.
func javaMemberReceiver(symbol string) (string, bool) {
	if symbol == "" || !strings.Contains(symbol, ".") {
		return "", false
	}
	parts := strings.Split(symbol, ".")
	if len(parts) < 2 {
		return "", false
	}
	// Nested: Outer.Inner.method → receiver Outer.Inner
	recv := strings.Join(parts[:len(parts)-1], ".")
	return recv, recv != ""
}

// javaMemberAccessEdits finds m.oldLeaf method_invocation name nodes and
// field_access field nodes to rewrite.
func javaMemberAccessEdits(fileRel string, content []byte, oldLeaf, newLeaf string, ourReceivers, foreignReceivers map[string]bool) []ingest.Edit {
	pf, err := ingest.ParseSource(content, fileRel, "java")
	if err != nil {
		return nil
	}
	defer pf.Close()

	ourSimple := map[string]bool{}
	for r := range ourReceivers {
		ourSimple[r] = true
		if i := strings.LastIndex(r, "."); i >= 0 {
			ourSimple[r[i+1:]] = true
		}
	}
	foreignSimple := map[string]bool{}
	for r := range foreignReceivers {
		foreignSimple[r] = true
		if i := strings.LastIndex(r, "."); i >= 0 {
			foreignSimple[r[i+1:]] = true
		}
	}

	typedLocals := javaTypedLocals(pf.Root, content, ourSimple)

	var edits []ingest.Edit
	var walk func(n *grammar.Node, enclosingClass string)
	walk = func(n *grammar.Node, enclosingClass string) {
		if n == nil || n.IsNull() {
			return
		}
		classHere := enclosingClass
		if n.Type() == "class_declaration" || n.Type() == "interface_declaration" || n.Type() == "enum_declaration" || n.Type() == "record_declaration" {
			if nameN := ingest.ChildByField(n, "name"); nameN != nil {
				classHere = ingest.NodeText(nameN, content)
			}
		}
		switch n.Type() {
		case "method_invocation":
			obj := ingest.ChildByField(n, "object")
			name := ingest.ChildByField(n, "name")
			if name != nil && ingest.NodeText(name, content) == oldLeaf {
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals) {
					edits = append(edits, ingest.Edit{
						File:      fileRel,
						StartByte: name.StartByte(),
						EndByte:   name.EndByte(),
						NewText:   newLeaf,
					})
				}
			}
		case "field_access":
			obj := ingest.ChildByField(n, "object")
			field := ingest.ChildByField(n, "field")
			if field == nil {
				field = ingest.ChildByType(n, "identifier")
			}
			if field != nil && ingest.NodeText(field, content) == oldLeaf {
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals) {
					edits = append(edits, ingest.Edit{
						File:      fileRel,
						StartByte: field.StartByte(),
						EndByte:   field.EndByte(),
						NewText:   newLeaf,
					})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), classHere)
		}
	}
	walk(pf.Root, "")
	return edits
}

// javaShouldRenameMemberAccess decides whether obj.oldLeaf targets our receiver type.
// obj may be nil for bare calls (relations usually cover those; ExtraRename still
// can rewrite unique-leaf sites, occupied spans skip already-covered ones).
func javaShouldRenameMemberAccess(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool) bool {
	if obj == nil {
		if enclosingClass != "" && ourReceivers[enclosingClass] {
			return true
		}
		return len(foreignReceivers) == 0
	}
	if obj.Type() == "this" || obj.Type() == "super" {
		if enclosingClass == "" {
			return len(foreignReceivers) == 0
		}
		if ourReceivers[enclosingClass] {
			return true
		}
		if foreignReceivers[enclosingClass] {
			return false
		}
		return len(foreignReceivers) == 0
	}
	// Nested type / enum-constant receivers: Outer.Nested.m(), Color.RED.m()
	if obj.Type() == "field_access" {
		text := ingest.NodeText(obj, content)
		if ourReceivers[text] {
			return true
		}
		if foreignReceivers[text] {
			return false
		}
		// Color.RED.method → root type Color
		if root := javaFieldAccessRoot(obj, content); root != "" {
			if ourReceivers[root] {
				return true
			}
			if foreignReceivers[root] {
				return false
			}
		}
		return len(foreignReceivers) == 0
	}
	if obj.Type() != "identifier" {
		return false
	}
	name := ingest.NodeText(obj, content)
	if ourReceivers[name] {
		return true
	}
	if foreignReceivers[name] {
		return false
	}
	if typedLocals[name] {
		return true
	}
	return len(foreignReceivers) == 0
}

// javaFieldAccessRoot returns the leftmost identifier of a field_access chain
// (Color.RED → Color, Outer.Nested → Outer).
func javaFieldAccessRoot(obj *grammar.Node, content []byte) string {
	for obj != nil && !obj.IsNull() {
		if obj.Type() == "identifier" || obj.Type() == "type_identifier" {
			return ingest.NodeText(obj, content)
		}
		if obj.Type() != "field_access" {
			return ""
		}
		next := ingest.ChildByField(obj, "object")
		if next == nil {
			return ""
		}
		obj = next
	}
	return ""
}

// javaTypedLocals maps locals/params declared with our receiver types.
func javaTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) map[string]bool {
	out := map[string]bool{}
	if root == nil || len(ourReceivers) == 0 {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "formal_parameter", "spread_parameter":
			typeN := ingest.ChildByField(n, "type")
			nameN := ingest.ChildByField(n, "name")
			if nameN == nil {
				nameN = ingest.ChildByType(n, "identifier")
			}
			if typeN != nil && nameN != nil {
				if tn := javaTypeName(typeN, content); ourReceivers[tn] {
					out[ingest.NodeText(nameN, content)] = true
				}
			}
		case "local_variable_declaration", "field_declaration":
			typeN := ingest.ChildByField(n, "type")
			if typeN == nil {
				break
			}
			tn := javaTypeName(typeN, content)
			if !ourReceivers[tn] {
				break
			}
			for i := uint32(0); i < n.ChildCount(); i++ {
				c := n.Child(i)
				if c.Type() == "variable_declarator" {
					nameN := ingest.ChildByField(c, "name")
					if nameN == nil {
						nameN = ingest.ChildByType(c, "identifier")
					}
					if nameN != nil {
						out[ingest.NodeText(nameN, content)] = true
					}
				}
			}
		case "enhanced_for_statement":
			typeN := ingest.ChildByField(n, "type")
			nameN := ingest.ChildByField(n, "name")
			if typeN != nil && nameN != nil {
				if tn := javaTypeName(typeN, content); ourReceivers[tn] {
					out[ingest.NodeText(nameN, content)] = true
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

// javaTypeName extracts a simple type identifier from a type node.
func javaTypeName(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	switch typeN.Type() {
	case "type_identifier", "identifier":
		return ingest.NodeText(typeN, content)
	case "generic_type":
		if name := ingest.ChildByField(typeN, "type"); name != nil {
			return javaTypeName(name, content)
		}
		if name := ingest.ChildByType(typeN, "type_identifier"); name != nil {
			return ingest.NodeText(name, content)
		}
	case "scoped_type_identifier":
		if name := ingest.ChildByField(typeN, "name"); name != nil {
			return ingest.NodeText(name, content)
		}
	case "array_type":
		if elem := ingest.ChildByField(typeN, "element"); elem != nil {
			return javaTypeName(elem, content)
		}
	}
	if id := ingest.ChildByType(typeN, "type_identifier"); id != nil {
		return ingest.NodeText(id, content)
	}
	return ""
}
