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

	// Inheritance edges (implements / extends) used for override decl expansion
	// and for super.member ExtraRename decisions.
	var implementsEdges map[string]map[string]bool
	alsoTypes := map[string]bool{}
	if len(ourTypes) > 0 {
		implementsEdges = javaImplementsEdges(rootDir, result)
		// alsoTypes: implementors of our types and interfaces/supertypes our types implement.
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
		// Related hierarchy types must count as our receivers so call sites on
		// expanded override decls (enum implementors, subclasses) rewrite too.
		// Without this, Worker.helper rename updates Kind's override def but
		// leaves k.helper() stale when k is typed Kind.
		for t := range alsoTypes {
			ourReceivers[t] = true
		}
	}

	// (1) Interface / override method declaration renames on related types.
	// alsoTypes covers named implementors/supertypes. ourTypes alone is enough
	// when the only other Type.method entities are anonymous class overrides
	// (new Type() { m() {} }) which share the constructed type's simple name
	// but live on a different path — no implements edge is recorded for them.
	if len(alsoTypes) > 0 || len(ourTypes) > 0 {
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
			// alsoTypes: named implementors/supertypes (Task.work when Worker.work renames).
			// ourTypes: same-type method entities on other paths — anonymous class bodies
			// emit Type.method under the file that contains `new Type() { … }`, which is a
			// different path reference than the type's own declaration file.
			if !alsoTypes[t] && !alsoTypes[tLeaf] && !ourTypes[t] && !ourTypes[tLeaf] {
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

	// (2) Instance member access: m.oldLeaf method_invocation / field_access /
	// method_reference (and complex receivers).
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
				// Expanded hierarchy types are ours; don't treat their same-leaf
				// methods as foreign just because the entity ref isn't in sourceSet.
				recvLeaf := recv
				if i := strings.LastIndex(recv, "."); i >= 0 {
					recvLeaf = recv[i+1:]
				}
				if ourReceivers[recvLeaf] {
					continue
				}
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
			for _, e := range javaMemberAccessEdits(rel, content, oldLeaf, newLeaf, ourReceivers, foreignReceivers, implementsEdges) {
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
	if ident == "*" {
		return false
	}
	return ingest.IdentUsed(text, ident, ingest.IsIdentCharJava)
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
	ingest.MaskNonNewlinesInPlace(masked, int(decl.RemoveStart), int(decl.RemoveEnd))
	// Mask import lines themselves so we do not count import idents as body use.
	specs := parseJavaImportSpecs(content)
	for _, spec := range specs {
		ingest.MaskNonNewlinesInPlace(masked, int(spec.startByte), int(spec.endByte))
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

// javaMemberAccessEdits finds m.oldLeaf method_invocation name nodes,
// field_access field nodes, and method_reference method names to rewrite.
// implementsEdges is map[typeSimpleName]set[parentSimpleName] from extends/implements;
// used only for super.member decisions (may be nil).
func javaMemberAccessEdits(fileRel string, content []byte, oldLeaf, newLeaf string, ourReceivers, foreignReceivers map[string]bool, implementsEdges map[string]map[string]bool) []ingest.Edit {
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

	typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf := javaTypedLocals(pf.Root, content, ourSimple)
	// Same-file record/class members for new BoxA(...).a() peels (dual-class).
	// Also zero-arg methods with concrete return types (get/self) so ba.get().run()
	// / ba.self().a().run() peel under foreign same-leaf.
	typeMembers := javaMergeTypeMembers(
		javaRecordComponentIndex(pf.Root, content),
		javaClassFieldIndex(pf.Root, content),
		javaSameFileMethodReturns(pf.Root, content),
	)

	var edits []ingest.Edit
	var walk func(n *grammar.Node, enclosingClass string, switchMatchesOur bool)
	walk = func(n *grammar.Node, enclosingClass string, switchMatchesOur bool) {
		if n == nil || n.IsNull() {
			return
		}
		classHere := enclosingClass
		if n.Type() == "class_declaration" || n.Type() == "interface_declaration" || n.Type() == "enum_declaration" || n.Type() == "record_declaration" {
			if nameN := ingest.ChildByField(n, "name"); nameN != nil {
				classHere = ingest.NodeText(nameN, content)
			}
		}
		swHere := switchMatchesOur
		if n.Type() == "switch_expression" || n.Type() == "switch_statement" {
			swHere = javaSwitchExprMatchesOur(n, content, ourSimple, typedLocals)
		}
		switch n.Type() {
		case "method_invocation":
			obj := ingest.ChildByField(n, "object")
			name := ingest.ChildByField(n, "name")
			if name != nil && ingest.NodeText(name, content) == oldLeaf {
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges) {
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
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges) {
					edits = append(edits, ingest.Edit{
						File:      fileRel,
						StartByte: field.StartByte(),
						EndByte:   field.EndByte(),
						NewText:   newLeaf,
					})
				}
			}
		case "method_reference":
			// Children: receiver, "::", method name (optional type_arguments).
			// Type::method and this::method are often covered by usages; expr::method
			// and super::method need ExtraRename like instance invocations.
			var parts []*grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				child := n.Child(i)
				if child.Type() == "::" {
					continue
				}
				parts = append(parts, child)
			}
			if len(parts) >= 2 {
				obj, name := parts[0], parts[len(parts)-1]
				if name.Type() == "identifier" && ingest.NodeText(name, content) == oldLeaf {
					if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges) {
						edits = append(edits, ingest.Edit{
							File:      fileRel,
							StartByte: name.StartByte(),
							EndByte:   name.EndByte(),
							NewText:   newLeaf,
						})
					}
				}
			}
		case "switch_label":
			// case HELPER -> … / case HELPER: — bare enum constant labels.
			// Relations cover Color.HELPER field_access but not switch labels.
			if len(foreignSimple) == 0 || swHere {
				for i := uint32(0); i < n.ChildCount(); i++ {
					ch := n.Child(i)
					if (ch.Type() == "identifier" || ch.Type() == "type_identifier") &&
						ingest.NodeText(ch, content) == oldLeaf {
						edits = append(edits, ingest.Edit{
							File:      fileRel,
							StartByte: ch.StartByte(),
							EndByte:   ch.EndByte(),
							NewText:   newLeaf,
						})
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), classHere, swHere)
		}
	}
	walk(pf.Root, "", false)
	return edits
}

// javaSwitchExprMatchesOur reports whether the switch selector refers to our type
// (typed local or receiver name). Used to gate bare case LABEL renames when the
// constant leaf is not unique across types.
func javaSwitchExprMatchesOur(sw *grammar.Node, content []byte, ourReceivers, typedLocals map[string]bool) bool {
	if sw == nil {
		return false
	}
	cond := ingest.ChildByField(sw, "condition")
	if cond == nil {
		cond = ingest.ChildByField(sw, "value")
	}
	if cond == nil {
		for i := uint32(0); i < sw.ChildCount(); i++ {
			ch := sw.Child(i)
			if ch.Type() == "parenthesized_expression" || ch.Type() == "identifier" {
				cond = ch
				break
			}
		}
	}
	if cond == nil {
		return false
	}
	if cond.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(cond, "expression")
		if inner == nil {
			for i := uint32(0); i < cond.ChildCount(); i++ {
				ch := cond.Child(i)
				if ch.Type() != "(" && ch.Type() != ")" {
					inner = ch
					break
				}
			}
		}
		cond = inner
	}
	if cond == nil {
		return false
	}
	if cond.Type() == "identifier" {
		name := ingest.NodeText(cond, content)
		if ourReceivers[name] || typedLocals[name] {
			return true
		}
	}
	return false
}

// javaRenameByTypeMaps: our → rename; foreign → skip; typedLocals → rename; else unique-leaf only.
func javaRenameByTypeMaps(name string, ourReceivers, foreignReceivers, typedLocals map[string]bool) bool {
	if ourReceivers[name] {
		return true
	}
	if foreignReceivers[name] {
		return false
	}
	if typedLocals != nil && typedLocals[name] {
		return true
	}
	return len(foreignReceivers) == 0
}

// javaShouldRenameMemberAccess decides whether obj.oldLeaf targets our receiver type.
// obj may be nil for bare calls. implementsEdges maps type → parents (extends/implements).
// entryValOf maps Map.Entry locals → value type leaf (for e.getValue().m()).
// valOf maps Map locals → value type leaf (for am.firstEntry().getValue().m() /
// am.get(k).m()).
// elemOf maps collection/Optional/Supplier locals → element type leaf
// (for as.get(i).m() / oa.get().m() / sa.get().m() under foreign same-leaf methods).
// compOf maps "local.member" → type leaf for record component accessors and
// class/record field access (ba.a().m() / box.a.m() / var xa = ba.a() / var xa = box.a
// under foreign same-leaf methods).
func javaShouldRenameMemberAccess(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf map[string]string, typeMembers map[string]map[string]string, implementsEdges map[string]map[string]bool) bool {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < obj.ChildCount(); i++ {
			ch := obj.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		obj = inner
	}
	if obj == nil {
		return javaRenameByTypeMaps(enclosingClass, ourReceivers, foreignReceivers, nil)
	}
	// super.m: rewrite when a declared parent is among ourReceivers; leave alone when
	// only the enclosing type itself is ours (parent method keeps its name).
	if obj.Type() == "super" {
		if enclosingClass != "" {
			if parents := implementsEdges[enclosingClass]; parents != nil {
				for p := range parents {
					if ourReceivers[p] {
						return true
					}
				}
			}
			if ourReceivers[enclosingClass] {
				return false
			}
		}
		return javaRenameByTypeMaps(enclosingClass, ourReceivers, foreignReceivers, nil)
	}
	if obj.Type() == "this" {
		return javaRenameByTypeMaps(enclosingClass, ourReceivers, foreignReceivers, nil)
	}
	// Nested type / enum-constant: Outer.Nested.m(), Color.RED.m()
	// Also box.a.run() / ba.a.run() when box/ba is a typed local with field or
	// record component a of type T (compOf; under foreign same-leaf methods).
	// oa.h.box.run() — nested field peel via typeMembers under foreign same-leaf.
	if obj.Type() == "field_access" {
		text := ingest.NodeText(obj, content)
		if ourReceivers[text] || foreignReceivers[text] {
			return ourReceivers[text]
		}
		if ft := javaFieldAccessMemberType(obj, content, compOf, typeMembers); ft != "" {
			return javaRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		return javaRenameByTypeMaps(javaFieldAccessRoot(obj, content), ourReceivers, foreignReceivers, nil)
	}
	// new Point(...).sum() / ((Box) o).helper() — type field when present.
	if obj.Type() == "object_creation_expression" || obj.Type() == "cast_expression" {
		tn := ""
		if typeN := ingest.ChildByField(obj, "type"); typeN != nil {
			tn = javaTypeName(typeN, content)
		}
		return javaRenameByTypeMaps(tn, ourReceivers, foreignReceivers, nil)
	}
	// xs[0].helper() — recover element type when the array root is tracked in
	// elemOf (A[] / var arr = stream.toArray()) or is a toArray/array-creation
	// pipeline; otherwise peel into the array root (handles (xs)[0] / matrix[i][j]
	// and A[] params collapsed into typedLocals).
	if obj.Type() == "array_access" {
		if et := javaArrayAccessElemType(obj, content, elemOf, valOf); et != "" {
			return javaRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		arr := ingest.ChildByField(obj, "array")
		if arr == nil {
			return len(foreignReceivers) == 0
		}
		return javaShouldRenameMemberAccess(arr, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges)
	}
	if obj.Type() == "identifier" || obj.Type() == "type_identifier" {
		return javaRenameByTypeMaps(ingest.NodeText(obj, content), ourReceivers, foreignReceivers, typedLocals)
	}
	// Method-invocation receivers under foreign same-leaf methods: recover T when
	// the call is a typed element/value accessor (same leaf as var xa = <access>):
	//   e.getValue().m() / e.setValue(v).m() — Map.Entry value
	//   Map.entry(k, new A()).getValue().m() / am.firstEntry().getValue().m()
	//   am.get(k).m() / Map.of(k, new A()).get(k).m() — map value
	//   as.get(i).m() / List.of(new A()).get(i).m() — list element
	//   oa.get().m() / oa.orElse(d).m() / findFirst().get().m() — Optional element
	//   sa.get().m() — Supplier element (generic type arg)
	//   aa.getAndSet(v).m() / aa.updateAndGet(f).m() / aa.getPlain().m() —
	//     AtomicReference value (same V leaf as get)
	//   new AtomicReference<>(new A()).get().m() / WeakReference / SoftReference —
	//     holder construction peel (same V leaf as typed AtomicReference local)
	//   ca.call().m() — Callable element (generic type arg)
	//   fa.join().m() / fa.getNow(d).m() / fa.resultNow().m() — CompletableFuture
	//   fn.apply(x).m() — Function/UnaryOperator/BiFunction result (R / T)
	//   Objects.requireNonNull(x).m() / requireNonNullElse(x, d).m() /
	//     requireNonNullElseGet(x, s).m() — identity wrappers (type of x)
	//   qa.poll().m() / as.getFirst().m() / … — other collection accessors
	//   ba.a().m() — record component accessor (compOf from record header)
	//   s.findFirst().get().get(0).m() after var s = gather(window*) — window list
	//   oa.get().get(0).m() after var oa = s.findFirst() — Optional intermediate
	if obj.Type() == "method_invocation" {
		if et := javaCollectionAccessElemType(obj, content, elemOf, valOf, entryValOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers); et != "" {
			return javaRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// Objects.requireNonNull(a) / requireNonNullElse(a, d) /
		// requireNonNullElseGet(a, s) when a is a typed local (A a) — first-arg
		// type leaf under foreign same-leaf methods.
		if id := javaObjectsRequireNonNullArgIdent(obj, content); id != "" {
			return javaRenameByTypeMaps(id, ourReceivers, foreignReceivers, typedLocals)
		}
		// Function.identity().apply(a) when a is a typed local — apply arg type leaf.
		if id := javaFunctionIdentityApplyArgIdent(obj, content); id != "" {
			return javaRenameByTypeMaps(id, ourReceivers, foreignReceivers, typedLocals)
		}
		// Optional.of(a).get() / ofNullable(a).orElseThrow() when a is a typed local —
		// unwrap peels the ident's type under foreign same-leaf methods.
		if id := javaOptionalOfIdentUnwrap(obj, content); id != "" {
			return javaRenameByTypeMaps(id, ourReceivers, foreignReceivers, typedLocals)
		}
		// Unknown method receivers: unique-leaf only.
		return len(foreignReceivers) == 0
	}
	// (c ? a : x).run() / (c ? new A() : A.make()).run() — both arms agree on T.
	// (c ? ba.get() : ba.get()).run() — method-return arms (javaInferExprType
	// treats ident.method as type ident, so only positive our-type infers early;
	// otherwise both arms must rename as ours under foreign same-leaf).
	if obj.Type() == "ternary_expression" {
		if t := javaInferExprType(obj, content); t != "" &&
			javaRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil) {
			return true
		}
		cons := ingest.ChildByField(obj, "consequence")
		alt := ingest.ChildByField(obj, "alternative")
		// Typed-local / method-return arms: both must rename as ours
		// (disagree / foreign fail closed).
		if cons != nil && alt != nil &&
			javaShouldRenameMemberAccess(cons, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges) &&
			javaShouldRenameMemberAccess(alt, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges) {
			return true
		}
		return false
	}
	// switch (n) { default -> a; } / default -> new A() — construction arms via
	// javaInferSwitchExprType; typed-local arms agree like dual-class ternary.
	// Method-return arms: only positive our-type infers early (same as ternary).
	if obj.Type() == "switch_expression" {
		if t := javaInferSwitchExprType(obj, content); t != "" &&
			javaRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil) {
			return true
		}
		// Typed-local / method-return arms: every arm must rename as ours
		// (disagree / foreign fail closed). Mirrors ternary dual-class peels.
		if javaSwitchExprAllArmsRename(obj, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges) {
			return true
		}
		return false
	}
	// Unknown / complex receivers without recoverable static type: unique-leaf only.
	return len(foreignReceivers) == 0
}

// javaOptionalOfIdentUnwrap recovers the identifier name from
// Optional.of(a).get() / ofNullable(a).orElseThrow() / of(a).orElseGet(...) when
// the Optional factory wraps a bare identifier. Used with typedLocals so
// Optional.ofNullable(a).get().run() peels under foreign same-leaf methods.
// Non-Optional receivers / non-ident args / multi-arg fail closed ("").
func javaOptionalOfIdentUnwrap(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil {
		return ""
	}
	switch ingest.NodeText(nameN, content) {
	case "get", "orElseThrow", "orElse", "orElseGet":
		// ok — Optional unwrap
	default:
		return ""
	}
	// get/orElseThrow must be zero-arg for Optional unwrap (List.get(i) has args).
	method := ingest.NodeText(nameN, content)
	if method == "get" || method == "orElseThrow" {
		if !javaCallIsZeroArg(call) {
			return ""
		}
	}
	opt := ingest.ChildByField(call, "object")
	for opt != nil && !opt.IsNull() && opt.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(opt, "expression")
		if inner == nil {
			for i := uint32(0); i < opt.ChildCount(); i++ {
				ch := opt.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		opt = inner
	}
	if opt == nil || opt.Type() != "method_invocation" {
		return ""
	}
	optName := ingest.ChildByField(opt, "name")
	optRecv := ingest.ChildByField(opt, "object")
	if optName == nil || optRecv == nil {
		return ""
	}
	switch ingest.NodeText(optName, content) {
	case "of", "ofNullable":
		// ok
	default:
		return ""
	}
	if javaStaticFactoryReceiverName(optRecv, content) != "Optional" {
		return ""
	}
	// Single positional arg must be identifier.
	var args *grammar.Node
	for i := uint32(0); i < opt.ChildCount(); i++ {
		if opt.Child(i).Type() == "argument_list" {
			args = opt.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	var ident string
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if ch.Type() == "identifier" && ident == "" {
			ident = ingest.NodeText(ch, content)
		} else {
			return ""
		}
	}
	if count != 1 || ident == "" {
		return ""
	}
	return ident
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
// Also binds untyped stream/collection lambda params when the pipeline element
// type is ours (List<A> as → as.stream().map(a -> a.m()) / as.iterator().forEachRemaining(a -> a.m())
// / List.of(new A()).forEach(a -> a.m()) / Stream.of(new A()).map(a -> a.m())
// / Arrays.stream(as).forEach(a -> a.m()) / Arrays.stream(new A[]{...}).map(a -> a.m())
// / as.stream().findFirst().ifPresent(a -> a.m()) / Optional.of(new A()).ifPresent(a -> a.m())
// / Optional.flatMap(a -> Optional.of(a)).ifPresent(x -> x.m()) / flatMap(...).orElse(d) /
// / Optional.map(a -> a).ifPresent(x -> x.m()) / map(...).orElse(d) /
// / Optional<A>.ifPresent(a -> a.m()) / opt.ifPresentOrElse(a -> a.m(), () -> {}) /
// / CompletableFuture<A>.thenAccept(a -> a.m()) / thenApply(a -> a.m()) /
// / thenCompose(a -> …) / applyToEither(other, a -> a.m()) / acceptEither(other, a -> a.m()) —
// / CF result T under foreign same-leaf methods /
// / CompletableFuture.whenComplete((a,e) -> a.m()) / handle((a,e) -> a.m()) /
// / Map<K,A>.forEach((k,v) -> v.m()) /
// Map.computeIfPresent/compute/replaceAll((k,v) -> v.m()) / Map.merge((v1,v2) -> v1.m()) /
// map.values().forEach(v -> v.m()) types a/v as A), for (var a : as) loop variables
// from collection/array element types, and var locals from collection accessors
// (list.get(i) / map.get(k) / map.computeIfAbsent(k,f) / map.putIfAbsent(k,v) /
// map.compute(k,f) / map.computeIfPresent(k,f) /
// map.put(k,v) / map.replace(k,v) / map.merge(k,v,fn) /
// map.putFirst(k,v) / map.putLast(k,v) /
// it.next() / list.iterator().next() /
// listIterator.previous() / list.listIterator().next()/previous() /
// deque.descendingIterator().next() / navSet.descendingIterator().next() /
// enum.nextElement() / coll.elements().nextElement() /
// Collections.enumeration(coll).nextElement() /
// queue.poll()/peek() / queue.take() / deque.takeFirst()/takeLast() /
// list.remove(i) / list.getFirst()/getLast() /
// list.removeFirst()/removeLast() /
// vector.elementAt(i) / vector.firstElement()/lastElement() /
// opt.orElse(d) / opt.orElseGet(s) / opt.orElseThrow([s]) / findFirst().orElse(d) /
// Collections.min(as) / Collections.max(as) / stream.min/max().orElse(d) /
// stream.reduce(identity, op) / reduce(op).orElse(d) / reduce(op).ifPresent(...) /
// stream.toList() / collect(toList()/toSet()) chained forEach / for (var a : …) and
// var list = stream.toList()/toSet() element tracking for later forEach / enhanced-for).
// collect(groupingBy/partitioningBy) → Map of List<T> groups:
// var m = stream.collect(groupingBy/partitioningBy);
// m.values() / m.forEach / m.get → group lists with element T for nested forEach / for-var.
// collect(toMap|toConcurrentMap|toUnmodifiableMap(key, a -> a[, …])) → Map of T values:
// collect(collectingAndThen(toMap(...), finisher)) → same Map of T values:
// var m = stream.collect(toMap(...)); m.values() / m.forEach / m.get → T.
// Collections.unmodifiableMap/synchronizedMap/checkedMap(m) / singletonMap(k, new T()) /
// Map.of(k, new T()) / Map.ofEntries(Map.entry(k, new T())) / Map.copyOf(m) →
// same Map value typing for values/forEach/get.
// entryValOf maps Map.Entry locals → value type leaf for e.getValue().m() /
// e.setValue(v).m() (for (var e : m.entrySet()) / m.entrySet().forEach(e -> …) /
// Map.Entry<K,A> e / var ea = Map.entry(...) / var ea = am.firstEntry()).
// valOf maps Map locals → value type leaf (also returned for inline
// am.firstEntry().getValue().m() / am.firstEntry().setValue(v).m() /
// am.get(k).m() under foreign same-leaf methods).
// elemOf maps collection/Optional/Supplier locals → element type leaf (returned
// for inline as.get(i).m() / oa.get().m() / sa.get().m() under foreign same-leaf methods).
// compOf maps "local.member" → type leaf for record component accessors and
// class/record fields (ba.a() / box.a / var xa = ba.a() / var xa = box.a).
// windowStreamOf maps stream locals from gather(windowFixed|windowSliding) →
// upstream element T (Stream of List<T> windows; not scalar T — see
// javaWindowGatherStreamElemType / javaWindowListExprElemType).
// windowOptOf maps Optional locals from window-stream findFirst/findAny →
// upstream element T (Optional of List<T>; oa.get().get(0).m() peels).
func javaTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) (map[string]bool, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string) {
	out := map[string]bool{}
	entryValOf := map[string]string{}
	valOf := map[string]string{}
	// Collection/stream locals: name → element type leaf (List<A> as → "A").
	elemOf := map[string]string{}
	// Record/class locals: "ba.a" / "box.a" → "A" for ba.a() / box.a / var xa = ….
	compOf := map[string]string{}
	// Window-gather stream locals: name → upstream element T
	// (var s = stream.gather(windowFixed(n)) → s:"A" meaning Stream of List<A>).
	windowStreamOf := map[string]string{}
	// Window Optional locals: name → upstream element T
	// (var oa = s.findFirst() after window stream → oa:"A" meaning Optional<List<A>>).
	windowOptOf := map[string]string{}
	// groupingBy/partitioningBy maps: name → element type of each value list
	// (Map<K,List<T>> / Map<Boolean,List<T>> → "T").
	groupValOf := map[string]string{}
	// Entry locals from groupingBy/partitioningBy entrySet: name → element type T
	// of the value List (e.getValue() is List<T>, not scalar T — see entryValOf).
	entryGroupOf := map[string]string{}
	if root == nil || len(ourReceivers) == 0 {
		return out, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf
	}
	recordIndex := javaRecordComponentIndex(root, content)
	fieldIndex := javaClassFieldIndex(root, content)
	methodIndex := javaSameFileMethodReturns(root, content)
	typeMembers := javaMergeTypeMembers(recordIndex, fieldIndex, methodIndex)
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
				name := ingest.NodeText(nameN, content)
				javaRecordCollectionElem(typeN, name, content, elemOf, valOf)
				tn := javaTypeName(typeN, content)
				javaBindRecordLocalComps(name, tn, recordIndex, compOf)
				javaBindRecordLocalComps(name, tn, fieldIndex, compOf)
				javaBindRecordLocalComps(name, tn, methodIndex, compOf)
				if ourReceivers[tn] {
					out[name] = true
				} else if vt := javaMapEntryDeclaredValueType(typeN, content); vt != "" {
					// Map.Entry<K,A> e param — track value type for e.getValue().
					entryValOf[name] = vt
				}
			}
		case "local_variable_declaration", "field_declaration":
			typeN := ingest.ChildByField(n, "type")
			if typeN == nil {
				break
			}
			tn := javaTypeName(typeN, content)
			explicitOurs := ourReceivers[tn]
			// var a = new A() / A.make() / (A) x / as.get(0) / m.get(k) / it.next() —
			// recover type from initializer.
			inferFromInit := tn == "var"
			for i := uint32(0); i < n.ChildCount(); i++ {
				c := n.Child(i)
				if c.Type() != "variable_declarator" {
					continue
				}
				nameN := ingest.ChildByField(c, "name")
				if nameN == nil {
					nameN = ingest.ChildByType(c, "identifier")
				}
				if nameN == nil {
					continue
				}
				name := ingest.NodeText(nameN, content)
				// List<A> as / A[] xs / Map<K,A> m — track elem/value even when outer is not ours.
				javaRecordCollectionElem(typeN, name, content, elemOf, valOf)
				if !inferFromInit {
					javaBindRecordLocalComps(name, tn, recordIndex, compOf)
					javaBindRecordLocalComps(name, tn, fieldIndex, compOf)
					javaBindRecordLocalComps(name, tn, methodIndex, compOf)
				}
				if explicitOurs {
					out[name] = true
					continue
				}
				if vt := javaMapEntryDeclaredValueType(typeN, content); vt != "" {
					// Map.Entry<K,A> e = … — track value type for e.getValue().
					entryValOf[name] = vt
				}
				if !inferFromInit {
					continue
				}
				valN := ingest.ChildByField(c, "value")
				// javaInferExprType treats ident.method() as type ident (A.make()).
				// Only trust it when the leaf is ours or a known record/class type; else
				// fall through so ba.a() / box.a bind via member access.
				inferred := javaInferExprType(valN, content)
				if ourReceivers[inferred] {
					out[name] = true
				} else if valN != nil && valN.Type() == "switch_expression" && javaSwitchExprTypedLocalType(valN, content, out) {
					// var xa = switch (c) { case 0 -> a; default -> x; } when a/x are
					// typed locals of our receiver (dual-class under-rename).
					out[name] = true
				} else if valN != nil && valN.Type() == "ternary_expression" && javaTernaryTypedLocalType(valN, content, out) {
					// var xa = c ? a : x — both arms typed locals of our receiver.
					out[name] = true
				} else if recordIndex[inferred] != nil || fieldIndex[inferred] != nil || methodIndex[inferred] != nil {
					// var ba = new BoxA(...) / var box = new Box() — track members when
					// the outer type itself is not our receiver (incl. zero-arg methods).
					javaBindRecordLocalComps(name, inferred, recordIndex, compOf)
					javaBindRecordLocalComps(name, inferred, fieldIndex, compOf)
					javaBindRecordLocalComps(name, inferred, methodIndex, compOf)
				} else if et := javaFieldAccessMemberType(valN, content, compOf, typeMembers); et != "" {
					// var xa = box.a / var xa = ba.a — class/record field access when
					// box/ba is a typed local with member type A.
					// var ha = oa.h — outer field of non-our type HolderA so ha.get()
					// peels under foreign same-leaf (bind members, not only ourReceivers).
					// var ba = oa.h.box — nested field peel (typeMembers).
					if ourReceivers[et] {
						out[name] = true
					}
					if recordIndex[et] != nil || fieldIndex[et] != nil || methodIndex[et] != nil {
						javaBindRecordLocalComps(name, et, recordIndex, compOf)
						javaBindRecordLocalComps(name, et, fieldIndex, compOf)
						javaBindRecordLocalComps(name, et, methodIndex, compOf)
					}
				} else if et := javaWindowListExprElemType(valN, content, elemOf, valOf, windowStreamOf, windowOptOf); et != "" {
					// var w = stream.gather(Gatherers.windowFixed(n)).findFirst().get() /
					// var w = s.findFirst().get() after var s = gather(window*) /
					// var w = oa.get() after var oa = s.findFirst() —
					// List of stream element T (not scalar T); track for w.get(0).m().
					elemOf[name] = et
				} else if et := javaNestedCollectionGetElemType(valN, content, elemOf, valOf); et != "" {
					// var ga = oa.get() when oa: Optional<List<A>> /
					// var row = aa.get(0) when aa: List<List<A>> /
					// var ga = ma.get(k) when ma: Map<K, List<A>> —
					// List of T (not scalar T); track for ga.get(0).m().
					elemOf[name] = et
				} else if et := javaCollectionAccessElemType(valN, content, elemOf, valOf, entryValOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers); et != "" {
					// var xa = as.get(0) / am.get("k") / as.iterator().next() / ia.next()
					// / qa.poll() / qa.peek() / qa.take() / da.takeFirst()/takeLast()
					// / as.remove(0) / as.getFirst()
					// / as.removeFirst() / as.removeLast()
					// / List.of(new A()).removeFirst() / as.stream().toList().remove(0) /
					// / as.reversed().removeFirst() / List.copyOf(as).removeFirst()
					// / e.getValue() / e.setValue(v) when e is a Map.Entry local
					// / Map.entry(k, new A()).getValue() / am.firstEntry().getValue()
					// / am.firstEntry().setValue(v) / am.lastEntry().setValue(v)
					// / am.pollFirstEntry().getValue() / am.ceilingEntry(k).getValue()
					// / ba.a() when ba is a record local with component type A
					// / Objects.requireNonNull(new A()) / Objects.requireNonNull(as.get(0))
					// / Objects.requireNonNullElse(new A(), d) / requireNonNullElseGet(...)
					// / as[0] / (as)[0] when as is A[] (array element via elemOf)
					// / stream.gather(windowFixed).findFirst().get().get(0) — window list elem
					// / s.findFirst().get().get(0) after var s = gather(window*)
					// var xa = ba.self() — intermediate BoxA method return so xa.get().run()
					// peels under foreign same-leaf (bind members, not only ourReceivers).
					if ourReceivers[et] {
						out[name] = true
					}
					if recordIndex[et] != nil || fieldIndex[et] != nil || methodIndex[et] != nil {
						javaBindRecordLocalComps(name, et, recordIndex, compOf)
						javaBindRecordLocalComps(name, et, fieldIndex, compOf)
						javaBindRecordLocalComps(name, et, methodIndex, compOf)
					}
				} else if id := javaObjectsRequireNonNullArgIdent(valN, content); id != "" && out[id] {
					// var xa = Objects.requireNonNull(a) / requireNonNullElse(a, d) /
					// requireNonNullElseGet(a, s) when a is already a typed local
					// (A a formal / prior bind). First-arg type leaf preserved.
					out[name] = true
				} else if id := javaFunctionIdentityApplyArgIdent(valN, content); id != "" && out[id] {
					// var xa = Function.identity().apply(a) when a is a typed local.
					out[name] = true
				} else if vt := javaEntryExprValueType(valN, content, elemOf, valOf, entryValOf); vt != "" {
					// var ea = Map.entry(k, new A()) / am.firstEntry() /
					// am.pollLastEntry() / am.floorEntry(k) /
					// as.entrySet().iterator().next() /
					// as.entrySet().stream().findFirst().get() — Entry of V;
					// track value T for later ea.getValue().m() / ea.setValue(v).m()
					// (entry is not A itself).
					entryValOf[name] = vt
				} else if et := javaWindowGatherStreamElemType(valN, content, elemOf, valOf, windowStreamOf); et != "" {
					// var s = stream.gather(Gatherers.windowFixed(n)) /
					// var s = stream.gather(Gatherers.windowSliding(n)) —
					// Stream of List<T> windows (not scalar T); track for
					// s.findFirst().get().get(0).m() / s.forEach(w -> w.get(0).m()).
					windowStreamOf[name] = et
				} else if et := javaWindowOptExprElemType(valN, content, elemOf, valOf, windowStreamOf, windowOptOf); et != "" {
					// var oa = s.findFirst() / s.findAny() after window stream /
					// var oa = stream.gather(window*).findFirst() —
					// Optional of List<T> (not scalar T); track for
					// oa.get().get(0).m() / var w = oa.get(); w.get(0).m().
					windowOptOf[name] = et
				} else if et := javaStreamPipelineElemType(valN, content, elemOf, valOf); et != "" {
					// var list = as.stream().toList() / collect(Collectors.toList()/toSet()) /
					// var arr = as.stream().toArray() / toArray(new A[0]) /
					// var s = as.stream() / var opt = as.stream().findFirst() —
					// track collection/stream/Optional/array element type for later
					// list.forEach / for (var a : list) / arr[i] / opt.ifPresent (not a scalar A).
					// Also var ar = new AtomicReference<>(new A()) (Class holder peel).
					elemOf[name] = et
				} else if et := javaStaticCollectionOfObjectElemType(valN, content, compOf, typeMembers); et != "" {
					// var xs = Stream.of(ba.get()).collect(toList()) /
					// var xs = Collections.unmodifiableList(List.of(ba.get())) —
					// method-return factory/pipeline peels under foreign same-leaf.
					elemOf[name] = et
				} else if et := javaReferenceHolderCreationObjectElemType(valN, content, compOf, typeMembers); et != "" {
					// var ar = new AtomicReference<>(ba.get()) / WeakReference / SoftReference —
					// method-return holder peels under foreign same-leaf (Class peels via
					// javaStreamPipelineElemType above).
					elemOf[name] = et
					if ourReceivers[et] {
						out[name] = true
					}
				} else if et := javaMapPipelineValueType(valN, content, elemOf, valOf); et != "" {
					// var m = as.stream().collect(Collectors.toMap(k, a -> a[, …])) /
					// Collections.unmodifiableMap(as) / Collections.singletonMap(k, new A()) /
					// Map.of(k, new A()) / Map.ofEntries(Map.entry(k, new A())) / Map.copyOf(as) —
					// Map of T values; track value T for values/forEach/get.
					valOf[name] = et
				} else if et := javaGroupingByCollectElemType(valN, content, elemOf, valOf); et != "" {
					// var m = as.stream().collect(Collectors.groupingBy/partitioningBy(...)) —
					// Map of List<T>; track group element T for values/forEach/get.
					groupValOf[name] = et
				} else if et := javaGroupingByMapGetElemType(valN, content, elemOf, valOf, groupValOf); et != "" {
					// var g = m.get(k) when m is a groupingBy/partitioningBy map — g is List<T>.
					elemOf[name] = et
				} else if et := javaGroupingByEntryGetValueElemType(valN, content, elemOf, valOf, entryGroupOf, groupValOf); et != "" {
					// var g = e.getValue() when e is Entry from groupingBy/partitioningBy
					// entrySet — g is List<T>.
					elemOf[name] = et
				} else if et := javaGroupingByEntryExprGroupElemType(valN, content, elemOf, valOf, entryGroupOf, groupValOf); et != "" {
					// var ea = m.entrySet().iterator().next() /
					// m.entrySet().stream().findFirst().get() when m is groupingBy —
					// Entry of List<T>; track for ea.getValue().get(0).m().
					entryGroupOf[name] = et
				}
			}
		case "enhanced_for_statement":
			// for (A a : as) — explicit type. for (var a : as) — element of collection.
			// Without var→elem binding, a.run() is skipped when foreign same-leaf methods exist.
			// for (List<A> ga : ma.values()) — explicit List type; track elemOf for ga.get(0).
			// for (var e : m.entrySet()) / for (Map.Entry<K,A> e : m.entrySet()) —
			// entry is not A; bind entryValOf for e.getValue().m().
			// for (var e : m.entrySet()) when m is groupingBy/partitioningBy —
			// entryGroupOf for e.getValue() List peels.
			// for (var g : m.values()) when m is groupingBy/partitioningBy → g is List<T> (elemOf).
			// for (var a : e.getValue()) when e is groupingBy entry → a is T.
			typeN := ingest.ChildByField(n, "type")
			nameN := ingest.ChildByField(n, "name")
			if typeN != nil && nameN != nil {
				name := ingest.NodeText(nameN, content)
				tn := javaTypeName(typeN, content)
				// List<A> ga / Map.Entry<K,A> e — track elem/value even when outer is not ours.
				javaRecordCollectionElem(typeN, name, content, elemOf, valOf)
				if ourReceivers[tn] {
					out[name] = true
				} else if tn == "var" {
					valN := ingest.ChildByField(n, "value")
					if et := javaStreamPipelineElemType(valN, content, elemOf, valOf); ourReceivers[et] {
						out[name] = true
					}
					// for (var ga : sa) when sa: Set<List<A>> / List<List<A>> —
					// ga is List of A (elemOf), not scalar A.
					if et := javaNestedCollectionIdentElemType(valN, content, elemOf); et != "" {
						elemOf[name] = et
					}
					if vt := javaEntrySetPipelineValueType(valN, content, elemOf, valOf); vt != "" {
						entryValOf[name] = vt
					}
					if et := javaGroupingByEntrySetGroupElemType(valN, content, elemOf, valOf, groupValOf); et != "" {
						entryGroupOf[name] = et
					}
					if et := javaGroupingByValuesGroupElemType(valN, content, elemOf, valOf, groupValOf); et != "" {
						elemOf[name] = et
					}
					if et := javaGroupingByEntryGetValueElemType(valN, content, elemOf, valOf, entryGroupOf, groupValOf); ourReceivers[et] {
						out[name] = true
					}
				} else if vt := javaMapEntryDeclaredValueType(typeN, content); vt != "" {
					entryValOf[name] = vt
				}
			}
		case "instanceof_expression":
			// Pattern matching: `x instanceof A a` binds a with type A.
			// Without this, a.run() is skipped when foreign same-leaf methods exist.
			// Record patterns (`x instanceof Box(A a)`) bind via record_pattern_component.
			typeN := ingest.ChildByField(n, "right")
			if typeN == nil {
				typeN = ingest.ChildByField(n, "type")
			}
			nameN := ingest.ChildByField(n, "name")
			if typeN != nil && nameN != nil {
				if tn := javaTypeName(typeN, content); ourReceivers[tn] {
					out[ingest.NodeText(nameN, content)] = true
				}
			}
		case "type_pattern":
			// switch/case pattern: `case A b -> …` (and guards).
			javaBindTypePattern(n, content, ourReceivers, out)
		case "catch_formal_parameter":
			// catch (MyEx e) — type lives under catch_type, not a "type" field.
			javaBindCatchFormal(n, content, ourReceivers, out)
		case "resource":
			// try (A a = new A()) { a.m(); }
			typeN := ingest.ChildByField(n, "type")
			nameN := ingest.ChildByField(n, "name")
			if typeN != nil && nameN != nil {
				if tn := javaTypeName(typeN, content); ourReceivers[tn] {
					out[ingest.NodeText(nameN, content)] = true
				}
			}
		case "record_pattern_component":
			// case Box(A a) / instanceof Holder(A a) — component binding A a.
			javaBindRecordPatternComponent(n, content, ourReceivers, out)
		case "method_invocation":
			// as.stream().map(a -> a.m()) / as.forEach(a -> a.m()) /
			// m.forEach((k,v) -> v.m()) / m.computeIfPresent((k,v) -> v.m()) /
			// m.merge((v1,v2) -> v1.m()) / m.entrySet().forEach(e -> e.getValue().m()) /
			// collect(groupingBy|partitioningBy).values().forEach(g -> g.forEach(a -> a.m())) —
			// untyped lambda params (and entryValOf for entrySet / elemOf for groups).
			javaBindStreamLambdaParams(n, content, ourReceivers, elemOf, valOf, entryValOf, groupValOf, entryGroupOf, windowStreamOf, out)
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf
}

// javaMergeTypeMembers merges record component and class field indexes into a
// single type → member → leaf map for new T(...).m() peels under foreign same-leaf.
func javaMergeTypeMembers(parts ...map[string]map[string]string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range parts {
		if part == nil {
			continue
		}
		for tn, members := range part {
			if out[tn] == nil {
				out[tn] = map[string]string{}
			}
			for m, t := range members {
				if t != "" {
					out[tn][m] = t
				}
			}
		}
	}
	return out
}

// javaRecordComponentIndex maps record type name → component name → component type leaf
// from same-file record_declaration headers (BoxA(A a) → "BoxA" → {"a":"A"}).
func javaRecordComponentIndex(root *grammar.Node, content []byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "record_declaration" {
			nameN := ingest.ChildByField(n, "name")
			params := ingest.ChildByField(n, "parameters")
			if nameN != nil && params != nil {
				typeName := ingest.NodeText(nameN, content)
				comps := map[string]string{}
				for i := uint32(0); i < params.ChildCount(); i++ {
					child := params.Child(i)
					if child.Type() != "formal_parameter" && child.Type() != "spread_parameter" {
						continue
					}
					typeN := ingest.ChildByField(child, "type")
					cnameN := ingest.ChildByField(child, "name")
					if cnameN == nil {
						cnameN = ingest.ChildByType(child, "identifier")
					}
					if typeN == nil || cnameN == nil {
						continue
					}
					if tn := javaTypeName(typeN, content); tn != "" {
						comps[ingest.NodeText(cnameN, content)] = tn
					}
				}
				if len(comps) > 0 {
					out[typeName] = comps
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

// javaBeanGetterName returns the JavaBeans getter leaf for a field (a → getA).
// ASCII-first; product bean identifiers are simple Latin names.
func javaBeanGetterName(field string) string {
	if field == "" {
		return ""
	}
	return "get" + strings.ToUpper(field[:1]) + field[1:]
}

// javaClassFieldIndex maps class type name → field name → field type leaf from
// same-file class_declaration field_declaration (Box with A a → "Box" → {"a":"A"}).
// Also indexes the JavaBeans getter name (getA) so box.getA().run() / var xa = box.getA()
// recover the field leaf under foreign same-leaf methods (same path as ba.a() / box.a).
func javaClassFieldIndex(root *grammar.Node, content []byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "class_declaration" {
			nameN := ingest.ChildByField(n, "name")
			body := ingest.ChildByField(n, "body")
			if nameN != nil && body != nil {
				typeName := ingest.NodeText(nameN, content)
				fields := map[string]string{}
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					if ch.Type() != "field_declaration" {
						continue
					}
					typeN := ingest.ChildByField(ch, "type")
					if typeN == nil {
						continue
					}
					tn := javaTypeName(typeN, content)
					if tn == "" {
						continue
					}
					for j := uint32(0); j < ch.ChildCount(); j++ {
						decl := ch.Child(j)
						if decl.Type() != "variable_declarator" {
							continue
						}
						fnameN := ingest.ChildByField(decl, "name")
						if fnameN == nil {
							fnameN = ingest.ChildByType(decl, "identifier")
						}
						if fnameN == nil {
							continue
						}
						fname := ingest.NodeText(fnameN, content)
						fields[fname] = tn
						// Bean getter getA for field a — zero-arg accessor form.
						if getter := javaBeanGetterName(fname); getter != "" && getter != fname {
							fields[getter] = tn
						}
					}
				}
				if len(fields) > 0 {
					out[typeName] = fields
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

// javaBindRecordLocalComps records "local.member" → type for each component/field of
// a known same-file record or class type (enables ba.a() / box.a / var xa = … typing).
func javaBindRecordLocalComps(local, typeName string, index map[string]map[string]string, compOf map[string]string) {
	if local == "" || typeName == "" || index == nil || compOf == nil {
		return
	}
	comps := index[typeName]
	if comps == nil {
		return
	}
	for c, t := range comps {
		compOf[local+"."+c] = t
	}
}

// javaFieldAccessMemberType recovers T from box.a / ba.a when box/ba is a typed
// local with field or record component a of type T (identifier object).
// Nested oa.h.box peels when typeMembers is non-nil: resolve oa.h → HolderA then
// typeMembers[HolderA][box] → BoxA (dual-class under foreign same-leaf).
func javaFieldAccessMemberType(obj *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if obj == nil || obj.Type() != "field_access" || compOf == nil {
		return ""
	}
	base := ingest.ChildByField(obj, "object")
	field := ingest.ChildByField(obj, "field")
	if field == nil {
		field = ingest.ChildByType(obj, "identifier")
	}
	if base == nil || field == nil {
		return ""
	}
	fname := ingest.NodeText(field, content)
	if fname == "" {
		return ""
	}
	if base.Type() == "identifier" {
		return compOf[ingest.NodeText(base, content)+"."+fname]
	}
	// Nested field: oa.h.box — peel outer field type then type-level member.
	if base.Type() == "field_access" && typeMembers != nil {
		if bt := javaFieldAccessMemberType(base, content, compOf, typeMembers); bt != "" {
			if comps := typeMembers[bt]; comps != nil {
				return comps[fname]
			}
		}
	}
	return ""
}

// javaRecordCollectionElem records name → element/value types for arrays and generics
// (List<A> as → elem "A", A[] xs → elem "A", Stream<A> s → elem "A",
// Map<K,A> m → elem "K" (first arg) and val "A" (second arg)).
// List<List<A>> nested → elem "List" and elemOf["@nested."+name] = "A" so
// nested.stream().flatMap(Collection::stream) peels to A under foreign same-leaf.
func javaRecordCollectionElem(typeN *grammar.Node, name string, content []byte, elemOf, valOf map[string]string) {
	if typeN == nil || name == "" {
		return
	}
	switch typeN.Type() {
	case "array_type":
		if elemOf == nil {
			return
		}
		if elem := ingest.ChildByField(typeN, "element"); elem != nil {
			if et := javaTypeName(elem, content); et != "" {
				elemOf[name] = et
			}
		}
	case "generic_type":
		args := javaTypeArgNames(typeN, content)
		if len(args) == 0 {
			return
		}
		if elemOf != nil {
			elemOf[name] = args[0]
			// Collection-of-collection: List<List<A>> / Collection<List<A>> → nested A
			// for flatMap(Collection::stream) / flatMap(List::stream) peels
			// and aa.get(0).get(0).m() under foreign same-leaf.
			if nest := javaCollectionOfCollectionElemType(typeN, content); nest != "" {
				elemOf["@nested."+name] = nest
			}
			// Map of collection: Map<K, List<A>> → nested A for ma.get(k).get(0).m().
			if nest := javaMapOfCollectionElemType(typeN, content); nest != "" {
				elemOf["@nested."+name] = nest
			}
			// Optional of collection: Optional<List<A>> → nested A for
			// oa.get().get(0).m() / var ga = oa.get(); ga.get(0).m().
			if nest := javaOptionalOfCollectionElemType(typeN, content); nest != "" {
				elemOf["@nested."+name] = nest
			}
			// Collection of Optional: List<Optional<A>> → nested A for
			// as.stream().flatMap(Optional::stream) peels.
			if nest := javaCollectionOfOptionalElemType(typeN, content); nest != "" {
				elemOf["@nested."+name] = nest
			}
		}
		// Map<K,V> / HashMap<K,V> — second type arg is the value type.
		// Function<T,R> — second type arg is the apply result R.
		if valOf != nil && len(args) >= 2 {
			valOf[name] = args[1]
		}
		// BiFunction<T,U,R>.apply returns R (third type arg). Prefer R over U so
		// apply recovers the result leaf like Function's second arg (valOf).
		if valOf != nil && len(args) >= 3 && javaTypeName(typeN, content) == "BiFunction" {
			valOf[name] = args[2]
		}
	}
}

// javaCollectionOfCollectionElemType recovers T from List/Collection/Set of
// List/Collection/Set/Iterable of T (one nesting level only).
// List<List<A>> → "A"; List<A> / Map / multi-arg fail closed.
func javaCollectionOfCollectionElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil || typeN.Type() != "generic_type" {
		return ""
	}
	outer := javaTypeName(typeN, content)
	if !javaIsCollectionTypeName(outer) {
		return ""
	}
	// First type argument node (not just leaf name).
	inner := javaFirstTypeArgNode(typeN)
	if inner == nil || inner.Type() != "generic_type" {
		return ""
	}
	if !javaIsCollectionTypeName(javaTypeName(inner, content)) {
		return ""
	}
	args := javaTypeArgNames(inner, content)
	if len(args) != 1 || args[0] == "" {
		return ""
	}
	return args[0]
}

// javaMapOfCollectionElemType recovers T from Map/HashMap of List/Collection/Set of T
// (one nesting level only). Map<K, List<A>> → "A"; Map<K,A> / List fail closed.
func javaMapOfCollectionElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil || typeN.Type() != "generic_type" {
		return ""
	}
	outer := javaTypeName(typeN, content)
	switch outer {
	case "Map", "HashMap", "LinkedHashMap", "TreeMap", "ConcurrentHashMap",
		"NavigableMap", "SortedMap", "SequencedMap", "WeakHashMap", "IdentityHashMap",
		"EnumMap", "Hashtable":
		// ok
	default:
		return ""
	}
	// Second type argument node (value type).
	targs := ingest.ChildByField(typeN, "type_arguments")
	if targs == nil {
		for i := uint32(0); i < typeN.ChildCount(); i++ {
			if typeN.Child(i).Type() == "type_arguments" {
				targs = typeN.Child(i)
				break
			}
		}
	}
	if targs == nil {
		return ""
	}
	var valueType *grammar.Node
	argIdx := 0
	for i := uint32(0); i < targs.ChildCount(); i++ {
		ch := targs.Child(i)
		switch ch.Type() {
		case "type_identifier", "generic_type", "scoped_type_identifier", "array_type":
			argIdx++
			if argIdx == 2 {
				valueType = ch
			}
		}
	}
	if valueType == nil || valueType.Type() != "generic_type" {
		return ""
	}
	if !javaIsCollectionTypeName(javaTypeName(valueType, content)) {
		return ""
	}
	args := javaTypeArgNames(valueType, content)
	if len(args) != 1 || args[0] == "" {
		return ""
	}
	return args[0]
}

// javaOptionalOfCollectionElemType recovers T from Optional of List/Collection/Set of T
// (one nesting level only). Optional<List<A>> → "A"; Optional<A> / List fail closed.
func javaOptionalOfCollectionElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil || typeN.Type() != "generic_type" {
		return ""
	}
	if javaTypeName(typeN, content) != "Optional" {
		return ""
	}
	inner := javaFirstTypeArgNode(typeN)
	if inner == nil || inner.Type() != "generic_type" {
		return ""
	}
	if !javaIsCollectionTypeName(javaTypeName(inner, content)) {
		return ""
	}
	args := javaTypeArgNames(inner, content)
	if len(args) != 1 || args[0] == "" {
		return ""
	}
	return args[0]
}

// javaNestedCollectionGetElemType recovers T from aa.get(i) / ma.get(k) / oa.get() when the
// receiver is a collection/map/Optional of list-of-T (elemOf["@nested."+src]). Used as the
// outer get in aa.get(0).get(0).m() / ma.get(k).get(0).m() / oa.get().get(0).m() under
// foreign same-leaf. Unknown / non-nested sources fail closed.
func javaNestedCollectionGetElemType(val *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if elemOf == nil {
		return ""
	}
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		val = inner
	}
	if val == nil || val.IsNull() || val.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(val, "name")
	if nameN == nil {
		return ""
	}
	switch ingest.NodeText(nameN, content) {
	case "get", "getOrDefault", "getFirst", "getLast",
		"remove", "removeFirst", "removeLast", "set",
		"poll", "peek", "element", "take",
		"pollFirst", "pollLast", "peekFirst", "peekLast", "pop",
		"takeFirst", "takeLast",
		// Optional unwraps that yield the nested List value (same leaf as window opts).
		"orElse", "orElseGet", "orElseThrow":
		// ok — collection/map/Optional accessors that yield the nested list value
	default:
		return ""
	}
	obj := ingest.ChildByField(val, "object")
	if obj == nil {
		return ""
	}
	// Bare identifier: aa.get(0) / ma.get(k) / oa.get() with @nested.
	if obj.Type() == "identifier" {
		return elemOf["@nested."+ingest.NodeText(obj, content)]
	}
	// Optional.of(List.of(new A())).get() / ofNullable / orElseThrow —
	// factory Optional wrapping a collection factory of T. Nested list element
	// peels as T under foreign same-leaf (same leaf as Optional<List<A>> local).
	if nest := javaOptionalOfCollectionFactoryElemType(obj, content); nest != "" {
		return nest
	}
	return ""
}

// javaOptionalOfCollectionFactoryElemType recovers T when opt is
// Optional.of(List.of(new A())) / Optional.ofNullable(Arrays.asList(new A())) /
// Optional.of(Collections.singletonList(new A())) /
// Optional.of(Set.copyOf(List.of(new A()))) /
// Optional.of(List.copyOf(List.of(new A()))) — Optional wrapping a
// collection factory of T. Enables
// Optional.of(List.of(new A())).get().get(0).m() /
// Optional.of(List.of(new A())).orElseThrow().get(0).m() /
// Optional.of(Set.copyOf(List.of(new A()))).get().iterator().next().m() under
// foreign same-leaf. Scalar Optional.of(new A()) / unknown args fail closed
// (no nested list).
func javaOptionalOfCollectionFactoryElemType(opt *grammar.Node, content []byte) string {
	if opt == nil || opt.IsNull() || opt.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(opt, "name")
	if nameN == nil {
		return ""
	}
	name := ingest.NodeText(nameN, content)
	if name != "of" && name != "ofNullable" {
		return ""
	}
	recv := javaStaticFactoryReceiverName(ingest.ChildByField(opt, "object"), content)
	if recv != "Optional" {
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < opt.ChildCount(); i++ {
		if opt.Child(i).Type() == "argument_list" {
			args = opt.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	var first *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," || ch.Type() == "comment" {
			continue
		}
		count++
		if first == nil {
			first = ch
		}
	}
	// Optional.of takes one value arg (ofNullable same). Extra args fail closed.
	if count != 1 || first == nil || first.Type() != "method_invocation" {
		return ""
	}
	argNameN := ingest.ChildByField(first, "name")
	if argNameN == nil {
		return ""
	}
	argName := ingest.NodeText(argNameN, content)
	switch argName {
	case "of", "asList", "ofNullable", "singletonList", "singleton":
		// List.of(new A()) / Arrays.asList(new A()) / Set.of(new A()) /
		// Collections.singletonList(new A()) / Collections.singleton(new A()) —
		// element T is the nested list leaf (Optional holds Collection of T).
		return javaStaticCollectionOfElemType(first, content, argName)
	case "copyOf":
		// Set.copyOf(List.of(new A())) / List.copyOf(List.of(new A())) —
		// Collection of first-arg element type (pipeline peels List.of/…;
		// nil maps are fine for pure factory args).
		return javaListSetCopyOfElemType(first, content, nil, nil)
	case "nCopies":
		// Collections.nCopies(n, new A()) — List of T.
		return javaCollectionsNCopiesElemType(first, content)
	default:
		return ""
	}
}

// javaOptionalOrElseFallbackType recovers T from Optional.orElse(new T(...)) /
// Optional.orElseGet(() -> new T(...)) when the Optional receiver is untyped
// (Optional.empty() / unknown) but the default/supplier peels to concrete T.
// Enables Optional.<A>empty().orElseGet(() -> new A()).run() under foreign
// same-leaf. orElseThrow has no value default — fail closed here.
func javaOptionalOrElseFallbackType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil {
		return ""
	}
	name := ingest.NodeText(nameN, content)
	if name != "orElse" && name != "orElseGet" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) != 1 {
		return ""
	}
	arg := args[0]
	if name == "orElse" {
		// orElse(new A()) — default is object creation of T.
		if arg.Type() != "object_creation_expression" {
			return ""
		}
		typeN := ingest.ChildByField(arg, "type")
		if typeN == nil {
			return ""
		}
		return javaTypeName(typeN, content)
	}
	// orElseGet(() -> new A()) — zero-arg expression-bodied supplier.
	if arg.Type() != "lambda_expression" {
		return ""
	}
	if n := javaInferredLambdaParamNames(arg, content); len(n) != 0 {
		return ""
	}
	body := ingest.ChildByField(arg, "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(body, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaNestedCollectionIdentElemType recovers T from a bare identifier that is a
// collection/Optional of list-of-T (elemOf["@nested."+name]). Used for
// for (var ga : sa) when sa: Set<List<A>> — ga is List of A (elemOf), not A.
func javaNestedCollectionIdentElemType(val *grammar.Node, content []byte, elemOf map[string]string) string {
	if val == nil || val.IsNull() || elemOf == nil {
		return ""
	}
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		val = inner
	}
	if val == nil || val.IsNull() || val.Type() != "identifier" {
		return ""
	}
	return elemOf["@nested."+ingest.NodeText(val, content)]
}

// javaStreamNestedCollectionElemType recovers T from nestedA.stream() / … when
// nestedA is List<List<A>> (elemOf["@nested."+src]). Stream elements are List of T
// (not T); callers bind elemOf[row] = T for row.get(0).m() under foreign same-leaf.
func javaStreamNestedCollectionElemType(obj *grammar.Node, content []byte, elemOf map[string]string) string {
	if elemOf == nil {
		return ""
	}
	src := javaStreamSourceIdent(obj, content)
	if src == "" {
		return ""
	}
	return elemOf["@nested."+src]
}

// javaIsCollectionTypeName reports List/Collection/Set/Iterable and common impls
// used as outer/inner containers for flatMap(Collection::stream) peels.
func javaIsCollectionTypeName(name string) bool {
	switch name {
	case "List", "Collection", "Set", "Iterable", "Queue", "Deque",
		"ArrayList", "LinkedList", "HashSet", "TreeSet", "LinkedHashSet",
		"ArrayDeque", "Vector", "Stack", "NavigableSet", "SortedSet",
		"SequencedCollection", "SequencedSet":
		return true
	default:
		return false
	}
}

// javaFirstTypeArgNode returns the first type argument node of a generic_type.
func javaFirstTypeArgNode(typeN *grammar.Node) *grammar.Node {
	if typeN == nil || typeN.Type() != "generic_type" {
		return nil
	}
	targs := ingest.ChildByField(typeN, "type_arguments")
	if targs == nil {
		for i := uint32(0); i < typeN.ChildCount(); i++ {
			if typeN.Child(i).Type() == "type_arguments" {
				targs = typeN.Child(i)
				break
			}
		}
	}
	if targs == nil {
		return nil
	}
	for i := uint32(0); i < targs.ChildCount(); i++ {
		ch := targs.Child(i)
		switch ch.Type() {
		case "type_identifier", "generic_type", "scoped_type_identifier", "array_type":
			return ch
		}
	}
	return nil
}

// javaTypeArgNames returns simple type-arg leaves of a generic_type in order.
func javaTypeArgNames(typeN *grammar.Node, content []byte) []string {
	if typeN == nil || typeN.Type() != "generic_type" {
		return nil
	}
	targs := ingest.ChildByField(typeN, "type_arguments")
	if targs == nil {
		for i := uint32(0); i < typeN.ChildCount(); i++ {
			if typeN.Child(i).Type() == "type_arguments" {
				targs = typeN.Child(i)
				break
			}
		}
	}
	if targs == nil {
		return nil
	}
	var out []string
	for i := uint32(0); i < targs.ChildCount(); i++ {
		ch := targs.Child(i)
		switch ch.Type() {
		case "type_identifier", "generic_type", "scoped_type_identifier", "array_type":
			if tn := javaTypeName(ch, content); tn != "" {
				out = append(out, tn)
			}
		}
	}
	return out
}

// javaBindStreamLambdaParams types untyped (inferred) lambda parameters when the
// call is a stream/collection element consumer/mapper and the pipeline element is ours,
// or Map bi-lambdas (forEach/computeIfPresent/compute/replaceAll/merge) when the map
// value type is ours. entrySet pipelines bind entryValOf for e.getValue().m().
// groupingBy/partitioningBy maps bind elemOf for value-list params (List<T> groups).
// Typed (A a) -> params are already handled via formal_parameter.
func javaBindStreamLambdaParams(call *grammar.Node, content []byte, ourReceivers map[string]bool, elemOf, valOf, entryValOf, groupValOf, entryGroupOf, windowStreamOf map[string]string, out map[string]bool) {
	if call == nil || call.Type() != "method_invocation" || out == nil {
		return
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil {
		return
	}
	method := ingest.NodeText(nameN, content)
	// Stream.collect(Collectors.reducing/maxBy/minBy(...)) — BinaryOperator /
	// mapper lambdas sit on the collector, not on stream methods. Bind from the
	// collect receiver's stream element type (Collectors receiver has no elemOf).
	if method == "collect" {
		javaBindCollectorsReducingLambdaParams(call, content, ourReceivers, elemOf, valOf, out)
		return
	}
	if !javaStreamElementLambdaMethod(method) {
		return
	}
	obj := ingest.ChildByField(call, "object")
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return
	}
	// ConcurrentHashMap.reduce has two bi-lambdas (transformer then reducer);
	// only the transformer sees V as its second param.
	reduceBiSeen := 0
	// ConcurrentHashMap.forEachEntry 3-arg has two unaries (transformer then consumer).
	forEachEntryUnarySeen := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() != "lambda_expression" {
			continue
		}
		params := javaInferredLambdaParamNames(ch, content)
		switch len(params) {
		case 1:
			// ConcurrentHashMap.forEach(threshold, BiFunction<? super K,? super V,? extends U>,
			//                          Consumer<? super U>) — 3-arg form's sole unary is the
			// Consumer on U (transformer is the bi-lambda, handled in case 2).
			// Value-identity transformer ((k,v) -> v) yields U=V so the consumer param is V.
			// 1-arg Map.forEach / 2-arg CHM forEach are bi-lambdas only (case 2); stream
			// forEach is unary on the pipeline element (fall through when not 3-arg).
			if method == "forEach" {
				callArgs := javaCallArgs(call)
				if len(callArgs) == 3 {
					if javaIsValueIdentityBiLambda(callArgs[1], content) {
						if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" && ourReceivers[vt] {
							out[params[0]] = true
						}
					}
					continue
				}
			}
			// ConcurrentHashMap.forEachEntry(threshold, Function<? super Entry,? extends U>,
			//                               Consumer<? super U>) — 3-arg form has two unaries:
			// transformer on Entry then consumer on U. getValue transformer (e -> e.getValue())
			// yields U=V so the consumer param is V. 2-arg is a single Entry Consumer (below).
			if method == "forEachEntry" {
				callArgs := javaCallArgs(call)
				if len(callArgs) == 3 {
					forEachEntryUnarySeen++
					if forEachEntryUnarySeen == 1 {
						// Transformer: Entry — bind entryValOf for e.getValue() in body.
						if entryValOf != nil {
							if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" {
								entryValOf[params[0]] = vt
							}
						}
					} else if forEachEntryUnarySeen == 2 {
						// Consumer on U; getValue transformer → U=V.
						if javaIsEntryGetValueLambda(callArgs[1], content) {
							if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" && ourReceivers[vt] {
								out[params[0]] = true
							}
						}
					}
					continue
				}
			}
			// ConcurrentHashMap.searchValues(threshold, Function<? super V, ? extends U>) /
			// forEachValue(threshold, Consumer<? super V>) /
			// reduceValues(threshold, Function<? super V, ? extends U>, BiFunction) —
			// unary Function/Consumer applies to map values V (not keys / not stream elems).
			// reduceValues 3-arg transformer is the only unary on that form; 2-arg is bi-lambda.
			if javaMapValueUnaryLambdaMethod(method) {
				if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" && ourReceivers[vt] {
					out[params[0]] = true
				}
				continue
			}
			// ConcurrentHashMap.forEachEntry / searchEntries — unary Consumer/Function
			// applies to Map.Entry<K,V>; bind entryValOf for e.getValue().m()
			// (same Entry value path as entrySet().forEach, but receiver is the map).
			// forEachEntry 3-arg handled above (two unaries: Entry then U).
			if javaMapEntryUnaryLambdaMethod(method) {
				if entryValOf != nil {
					if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" {
						entryValOf[params[0]] = vt
					}
				}
				continue
			}
			// Unary: stream/collection element or map.values() element.
			et := javaStreamPipelineElemType(obj, content, elemOf, valOf)
			if et != "" && ourReceivers[et] {
				out[params[0]] = true
			} else if nest := javaStreamNestedCollectionElemType(obj, content, elemOf); nest != "" {
				// nestedA.stream().map(row -> row.get(0).m()) when nestedA: List<List<A>> —
				// param is List of nest (not nest); track for row.get(0).m().
				if elemOf != nil {
					elemOf[params[0]] = nest
				}
			} else if et := javaWindowGatherStreamElemType(obj, content, elemOf, valOf, windowStreamOf); et != "" {
				// stream.gather(windowFixed|windowSliding).forEach(w -> …) /
				// s.forEach(w -> …) after var s = gather(window*) —
				// param is List<T>, not T; track for w.get(0).m().
				if elemOf != nil {
					elemOf[params[0]] = et
				}
			} else if et := javaGroupingByValuesGroupElemType(obj, content, elemOf, valOf, groupValOf); et != "" {
				// m.values().forEach(g -> …) / collect(groupingBy|partitioningBy).values().forEach —
				// param is List<T>, not T; track for nested forEach / for-var.
				if elemOf != nil {
					elemOf[params[0]] = et
				}
			} else if et := javaGroupingByEntryGetValueElemType(obj, content, elemOf, valOf, entryGroupOf, groupValOf); et != "" {
				// e.getValue().forEach(a -> …) when e is Entry from groupingBy/partitioningBy
				// entrySet — param is T (element of value List).
				if ourReceivers[et] {
					out[params[0]] = true
				}
			}
			// m.entrySet().forEach(e -> e.getValue().m()) — param is Entry, not V.
			if entryValOf != nil {
				if vt := javaEntrySetPipelineValueType(obj, content, elemOf, valOf); vt != "" {
					entryValOf[params[0]] = vt
				}
			}
			// m.entrySet().forEach(e -> e.getValue().get(0).m()) when m is groupingBy —
			// param is Entry of List<T>; bind entryGroupOf (not entryValOf).
			if entryGroupOf != nil {
				if et := javaGroupingByEntrySetGroupElemType(obj, content, elemOf, valOf, groupValOf); et != "" {
					entryGroupOf[params[0]] = et
				}
			}
		case 2:
			// Stream.mapMulti(BiConsumer<? super T, ? super Consumer<R>>):
			// first param is stream element T; second is sink Consumer — leave unbound.
			// Identity accept pipelines (mapMulti((a,c)->c.accept(a)).forEach(...))
			// peel via javaStreamPipelineElemType / javaMapMultiResultElemType.
			if method == "mapMulti" {
				et := javaStreamPipelineElemType(obj, content, elemOf, valOf)
				if et != "" && ourReceivers[et] {
					out[params[0]] = true
				}
				continue
			}
			// CompletableFuture.whenComplete(BiConsumer<? super T,? super Throwable>) /
			// whenCompleteAsync(BiConsumer[, Executor]) /
			// handle(BiFunction<? super T,Throwable,? extends U>) /
			// handleAsync(BiFunction[, Executor]) —
			// first param is CF result T (same leaf as thenAccept/thenApply unary).
			// Second is Throwable — leave unbound. Optional Executor is non-lambda.
			// Identity handle / handleAsync return pipelines
			// (handle((a,e)->a).join()) peel via javaStreamPipelineElemType /
			// javaMapResultElemType (first-param identity only).
			if method == "whenComplete" || method == "whenCompleteAsync" ||
				method == "handle" || method == "handleAsync" {
				et := javaStreamPipelineElemType(obj, content, elemOf, valOf)
				if et != "" && ourReceivers[et] {
					out[params[0]] = true
				}
				continue
			}
			// CompletableFuture.thenCombine(other, BiFunction<? super T,? super U,? extends V>) /
			// thenCombineAsync(other, BiFunction[, Executor]) /
			// thenAcceptBoth(other, BiConsumer<? super T,? super U>) /
			// thenAcceptBothAsync(other, BiConsumer[, Executor]) —
			// first param is this CF's T; second is other stage's U when recoverable.
			// Optional Executor after the BiFunction is non-lambda.
			// runAfterBoth takes Runnable (zero-arg) — not a bi-lambda.
			if method == "thenCombine" || method == "thenCombineAsync" ||
				method == "thenAcceptBoth" || method == "thenAcceptBothAsync" {
				et := javaStreamPipelineElemType(obj, content, elemOf, valOf)
				if et != "" && ourReceivers[et] {
					out[params[0]] = true
				}
				callArgs := javaCallArgs(call)
				if len(callArgs) >= 1 {
					if ut := javaStreamPipelineElemType(callArgs[0], content, elemOf, valOf); ut != "" && ourReceivers[ut] {
						out[params[1]] = true
					}
				}
				continue
			}
			// ConcurrentHashMap.reduceEntries(threshold, BiFunction on Entry,Entry) —
			// 2-arg form: bind entryValOf for e.getValue().m() on both params.
			// 3-arg form's BiFunction is on U; getValue transformer (e -> e.getValue())
			// yields U=V so both reducer params are V. Non-getValue transformers fail closed.
			if method == "reduceEntries" {
				callArgs := javaCallArgs(call)
				switch len(callArgs) {
				case 2:
					if entryValOf != nil {
						if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" {
							entryValOf[params[0]] = vt
							entryValOf[params[1]] = vt
						}
					}
				case 3:
					// BiFunction reducer on U; getValue transformer → U=V.
					if javaIsEntryGetValueLambda(callArgs[1], content) {
						if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" && ourReceivers[vt] {
							out[params[0]] = true
							out[params[1]] = true
						}
					}
				}
				continue
			}
			// ConcurrentHashMap.reduce(threshold, BiFunction<K,V,U>, BiFunction<U,U,U>) —
			// transformer (first bi-lambda) has V as second param; reducer is on U.
			// Value-identity transformer ((k,v) -> v) yields U=V so both reducer params are V.
			// Stream.reduce is not a map receiver — javaMapPipelineValueType fails closed.
			if method == "reduce" {
				reduceBiSeen++
				if reduceBiSeen == 1 {
					if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" && ourReceivers[vt] {
						out[params[1]] = true
					}
				} else if reduceBiSeen == 2 {
					// BiFunction reducer on U; value-identity transformer → U=V.
					callArgs := javaCallArgs(call)
					if len(callArgs) >= 2 && javaIsValueIdentityBiLambda(callArgs[1], content) {
						if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" && ourReceivers[vt] {
							out[params[0]] = true
							out[params[1]] = true
						}
					}
				}
				continue
			}
			// Map bi-lambdas — value type from valOf[map] / collect(toMap(...)).
			// forEach/computeIfPresent/compute/replaceAll/ConcurrentHashMap.search:
			// (K,V) → second is V.
			// merge / ConcurrentHashMap.reduceValues: (V,V) → both params are V
			// (BiFunction remapping / reducer).
			// groupingBy/partitioningBy maps: value is List<T> — bind elemOf on the value param.
			if !javaMapValueBiLambdaMethod(method) {
				continue
			}
			vt := javaMapPipelineValueType(obj, content, elemOf, valOf)
			if vt != "" && ourReceivers[vt] {
				if method == "merge" {
					out[params[0]] = true
					out[params[1]] = true
				} else if method == "reduceValues" {
					// 2-arg form: BiFunction on V,V — both params are V.
					// 3-arg form: BiFunction on U; identity transformer (a -> a)
					// yields U=V so both reducer params are V. Non-identity fail closed.
					callArgs := javaCallArgs(call)
					switch len(callArgs) {
					case 2:
						out[params[0]] = true
						out[params[1]] = true
					case 3:
						if javaIsIdentityLambda(callArgs[1], content) {
							out[params[0]] = true
							out[params[1]] = true
						}
					}
				} else {
					out[params[1]] = true
				}
				continue
			}
			if et := javaGroupingByMapGroupElemType(obj, content, elemOf, valOf, groupValOf); et != "" && elemOf != nil {
				// m.forEach((k,g) -> g.forEach(...)) / collect(groupingBy|partitioningBy).forEach —
				// value param is List<T>.
				if method == "merge" || method == "reduceValues" {
					// merge/reduceValues values would be List — fail closed (not a product case).
					continue
				}
				elemOf[params[1]] = et
			}
		}
	}
}

// javaStreamElementLambdaMethod reports methods whose (first) functional arg is
// applied to the stream/collection element type, or Map methods with value bi-lambdas.
func javaStreamElementLambdaMethod(method string) bool {
	switch method {
	case "map", "mapToInt", "mapToLong", "mapToDouble",
		"flatMap", "flatMapToInt", "flatMapToLong", "flatMapToDouble",
		// Stream.mapMulti(BiConsumer<? super T, ? super Consumer<R>>) — bi-lambda
		// first param is T; second is sink. See case 2 / javaMapMultiResultElemType.
		"mapMulti",
		"filter", "peek", "forEach", "forEachOrdered", "forEachRemaining",
		// Spliterator.tryAdvance(Consumer) — same element Consumer as forEachRemaining.
		"tryAdvance",
		"takeWhile", "dropWhile",
		"anyMatch", "allMatch", "noneMatch",
		"removeIf", "ifPresent", "ifPresentOrElse",
		// CompletableFuture.thenAccept(Consumer<? super T>) /
		// thenAcceptAsync(Consumer[, Executor]) /
		// thenApply(Function<? super T,? extends U>) /
		// thenApplyAsync(Function[, Executor]) /
		// thenCompose(Function<? super T,? extends CompletionStage<U>>) /
		// thenComposeAsync(Function[, Executor]) /
		// applyToEither(other, Function<? super T,? extends U>) /
		// applyToEitherAsync(other, Function[, Executor]) /
		// acceptEither(other, Consumer<? super T>) /
		// acceptEitherAsync(other, Consumer[, Executor]) —
		// unary functional arg applied to CF result T (same leaf as join/getNow).
		// applyToEither/acceptEither take other stage first; the Function/Consumer
		// is the second arg and still binds as the sole unary lambda (other is not
		// a lambda; optional Executor is non-lambda too). Identity thenApply /
		// thenApplyAsync return pipelines (thenApplyAsync(a -> a).join()) peel via
		// javaStreamPipelineElemType / javaMapResultElemType; thenCompose /
		// thenComposeAsync rewrap pipelines peel via javaFlatMapResultElemType.
		// Type-changing mappers fail closed on the return path. Identity handle
		// ((a,e)->a).join() peels the same way (first-param bi-lambda identity).
		"thenAccept", "thenAcceptAsync",
		"thenApply", "thenApplyAsync",
		"thenCompose", "thenComposeAsync",
		"applyToEither", "applyToEitherAsync",
		"acceptEither", "acceptEitherAsync",
		// CompletableFuture.whenComplete(BiConsumer<? super T,? super Throwable>) /
		// whenCompleteAsync(BiConsumer[, Executor]) /
		// handle(BiFunction<? super T,Throwable,? extends U>) /
		// handleAsync(BiFunction[, Executor]) —
		// bi-lambda first param is CF result T (second is Throwable).
		// Identity handle / handleAsync return pipelines peel via javaMapResultElemType.
		"whenComplete", "whenCompleteAsync", "handle", "handleAsync",
		// CompletableFuture.thenCombine/thenAcceptBoth /
		// thenCombineAsync/thenAcceptBothAsync —
		// bi-lambda (T,U): first is this CF's T; second is other stage's U.
		// Optional Executor after the BiFunction is non-lambda.
		"thenCombine", "thenCombineAsync", "thenAcceptBoth", "thenAcceptBothAsync",
		// Map value bi-lambdas (see javaMapValueBiLambdaMethod).
		"computeIfPresent", "compute", "replaceAll", "merge",
		// ConcurrentHashMap.reduceValues — 2-arg BiFunction on V,V and 3-arg unary
		// transformer on V (see javaMapValueBiLambdaMethod / javaMapValueUnaryLambdaMethod).
		"reduceValues",
		// ConcurrentHashMap.search(threshold, BiFunction<? super K,? super V,? extends U>)
		// — (K,V) bi-lambda; value is second param (same as forEach).
		"search",
		// ConcurrentHashMap.reduce(threshold, BiFunction<K,V,U>, BiFunction<U,U,U>) —
		// transformer (K,V) bi-lambda; value is second param (reducer is on U).
		"reduce",
		// ConcurrentHashMap searchValues / forEachValue — unary Function/Consumer on V
		// (see javaMapValueUnaryLambdaMethod).
		"searchValues", "forEachValue",
		// ConcurrentHashMap forEachEntry / searchEntries / reduceEntries —
		// Entry Consumer/Function/BiFunction (see javaMapEntryUnaryLambdaMethod).
		"forEachEntry", "searchEntries", "reduceEntries":
		return true
	default:
		return false
	}
}

// javaMapEntryUnaryLambdaMethod reports ConcurrentHashMap-style methods whose
// unary functional arg is applied to Map.Entry<K,V> (not V / not stream elems):
// forEachEntry(threshold, Consumer<? super Map.Entry<K,V>>) — 2-arg form;
//
//	3-arg (Function+Consumer) is handled specially (consumer is on U, not Entry),
//
// searchEntries(threshold, Function<? super Map.Entry<K,V>, ? extends U>),
// reduceEntries(threshold, Function<? super Map.Entry<K,V>, ? extends U>, BiFunction)
// — 3-arg transformer only (2-arg form is an Entry bi-lambda; see case 2).
func javaMapEntryUnaryLambdaMethod(method string) bool {
	switch method {
	case "forEachEntry", "searchEntries", "reduceEntries":
		return true
	default:
		return false
	}
}

// javaMapValueUnaryLambdaMethod reports ConcurrentHashMap-style methods whose
// unary functional arg is applied to map values V (not keys / not stream elems):
// searchValues(threshold, Function<? super V, ? extends U>),
// forEachValue(threshold, Consumer<? super V>),
// reduceValues(threshold, Function<? super V, ? extends U>, BiFunction) — 3-arg
// transformer only (2-arg form is a V,V bi-lambda; see case 2).
func javaMapValueUnaryLambdaMethod(method string) bool {
	switch method {
	case "searchValues", "forEachValue", "reduceValues":
		return true
	default:
		return false
	}
}

// javaMapValueBiLambdaMethod reports Map methods whose bi-lambda args include the
// map value type: forEach/computeIfPresent/compute/replaceAll /
// ConcurrentHashMap.search → (K,V),
// merge / ConcurrentHashMap.reduceValues → (V,V).
func javaMapValueBiLambdaMethod(method string) bool {
	switch method {
	case "forEach", "computeIfPresent", "compute", "replaceAll", "merge", "reduceValues",
		"search":
		return true
	default:
		return false
	}
}

// javaStreamPipelineElemType recovers the element type of a stream pipeline object:
// as / as.stream() / as.iterator() / as.spliterator() / as.stream().filter(...) → elemOf[as],
// as.reversed() / as.reversed().stream() → elemOf[as] (SequencedCollection/List view),
// as.subList(i, j) / as.subList(i, j).get(0) → elemOf[as] (List view; bounds only),
// as.descendingSet() / as.headSet/tailSet/subSet(...) → elemOf[as]
// (NavigableSet/SortedSet views; order/bounds only),
// as.stream().findFirst() / findAny() / min() / max() / reduce(op) → same element
// (Optional wraps T; ifPresent / orElse use T),
// as.stream().toList() / collect(Collectors.toList()/toSet()/toUnmodifiableList()/
// toUnmodifiableSet()/toCollection(…)) / collect(toList()/toSet()/toUnmodifiable…/
// toCollection(…)) / collect(Collectors::toList / toSet / toUnmodifiableList /
// toUnmodifiableSet) / collect(collectingAndThen(toList()/toSet()/toUnmodifiable…/
// toCollection(…), …)) / collect(filtering(pred, toList()/…)) /
// collect(mapping(a -> a, toList()/…)) /
// collect(flatMapping(a -> Stream.of(a), toList()/…)) / collect(teeing(toList()/…, …, (list, …) -> list)) → same
// element (Collection<T> for forEach / enhanced-for),
// as.stream().toArray() / toArray(generator) / as.toArray([generator]) → same
// element (T[] for toArray()[i] / var arr = toArray(); arr[i]),
// as.clone() → same element (T[] for clone()[i] / var arr = as.clone(); arr[i]),
// (ArrayList<A>) as.clone() / (List<A>) x → "A" (single type-arg generic cast;
// multi-arg Map casts fail closed so .get uses the value path, not key-as-elem),
// (A[]) expr → "A" (array cast element), else peel cast value (as.clone() under cast),
// m.values() / m.sequencedValues() → valOf[m]
// (Collection/SequencedCollection of map values V; sequencedValues is order-only, Java 21),
// m.keySet() / navigableKeySet() / descendingKeySet() → elemOf[m]
// (Set of map keys K; Map stores K in elemOf — same key leaf as newSetFromMap),
// List.of(new A()) / Stream.of(new A()) / Arrays.asList(new A()) → "A",
// Arrays.asList(as) / Arrays.asList(new A[]{...}) → "A" (List of first-arg array
// elements; same first-arg peel as Arrays.stream when not homogeneous new T(...)),
// Collections.singletonList(new A()) / Collections.singleton(new A()) → "A",
// CompletableFuture.completedFuture(new A()) → "A" (CF of T; enables
// completedFuture(new A()).join() / var f = completedFuture(new A()); f.join()),
// Collections.nCopies(n, new A()) → "A",
// Collections.unmodifiableList/Set/Collection(as) / synchronizedList/Set/Collection(as) /
// checkedList/Set/Collection(as, …) → elemOf[as],
// Collections.unmodifiableSortedSet/NavigableSet(as) / synchronizedSortedSet/NavigableSet(as) /
// checkedSortedSet/NavigableSet(as, …) → elemOf[as] (SortedSet/NavigableSet wrappers;
// same element type; Class args ignored — mirrors unmodifiableSortedMap/NavigableMap),
// Collections.unmodifiableSequencedCollection/Set(as) / synchronizedSequencedCollection/Set(as)
// → elemOf[as] (SequencedCollection/Set wrappers; same E — Java 21; mirrors *SequencedMap),
// Collections.asLifoQueue(as) / checkedQueue(as, …) → elemOf[as] (Queue wrappers; same E;
// Class arg on checkedQueue ignored — mirrors checkedList/Set),
// Collections.newSetFromMap(m) → elemOf[m] (Set of map keys K; Map stores K in elemOf),
// Collections.list(Enumeration) / Collections.enumeration(coll) → enumeration/coll element type,
// List.copyOf(as) / Set.copyOf(as) → elemOf[as] (Collection of first-arg elements),
// Arrays.copyOf(as, n) / Arrays.copyOfRange(as, from, to) → "A" (T[] of first-arg elements;
// length/range bounds only; Class-typed overloads fail closed via first-arg peel only),
// Stream.concat(s1, s2) → element type when both stream args agree,
// Stream.generate(() -> new A()) → "A",
// CompletableFuture.supplyAsync(() -> new A()) / supplyAsync(() -> new A(), ex) → "A"
// (CF of T from supplier body; enables supplyAsync(() -> new A()).join() /
// var f = supplyAsync(() -> new A()); f.join()),
// new AtomicReference<>(new A()) / WeakReference / SoftReference → "A"
// (holder of V; enables new AtomicReference<>(new A()).get() /
// var ar = new AtomicReference<>(new A()); ar.get()),
// Stream.iterate(new A(), …) / iterate(new A(), pred, …) → "A" (seed creation type),
// Stream.ofNullable(new A()) → "A",
// Arrays.stream(as) / Arrays.stream(new A[]{...}) → "A",
// Optional.of(new A()) / Optional.ofNullable(new A()) → "A",
// Optional.flatMap(a -> Optional.of(a)) / ofNullable(a) / Optional::of → same element
// when the mapper clearly rewraps T (see javaFlatMapResultElemType),
// Optional.map(a -> a) / map(a -> new A()) → same or known element
// when the mapper clearly yields T (see javaMapResultElemType).
// Stream.gather(Gatherers.mapConcurrent(n, a -> a)) / mapConcurrent(n, a -> new A())
// → same or known element (identity/new only; see javaGatherResultElemType).
// CompletableFuture.thenApply(a -> a) / thenApply(a -> new A()) → same or known
// element (identity/new only; enables thenApply(a -> a).join() / var f2 = thenApply…).
// CompletableFuture.applyToEither(other, a -> a) / applyToEither(other, a -> new A()) →
// same or known element (identity/new only; Function is the second arg — other stage
// first; enables applyToEither(other, a -> a).join() / var f2 = applyToEither…).
// CompletableFuture.handle((a, e) -> a) / handle((a, e) -> new A()) → same or known
// element (first-param identity/new only; enables handle((a,e)->a).join() /
// var f2 = handle…).
// CompletableFuture.thenCombine(other, (a, b) -> a) / thenCombine(other, (a, b) -> new A()) →
// same or known element (first-param identity/new only; BiFunction after other stage —
// enables thenCombine(other, (a,b)->a).join() / var f2 = thenCombine…).
// CompletableFuture.thenCompose(a -> CompletableFuture.completedFuture(a)) → same element
// when the mapper clearly rewraps T as a CompletionStage (see javaFlatMapResultElemType;
// enables thenCompose(a -> completedFuture(a)).join() / var f2 = thenCompose…).
// CompletableFuture.whenComplete / copy / toCompletableFuture / orTimeout /
// completeOnTimeout / exceptionally / exceptionallyCompose / minimalCompletionStage →
// same element (always preserve T by API signature).
// Type-changing stages (unknown map / flatMap / thenApply / applyToEither / handle /
// thenCombine / thenCompose mappers) fail closed so later lambdas are not mis-typed.
func javaStreamPipelineElemType(obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		if elemOf == nil {
			return ""
		}
		return elemOf[ingest.NodeText(obj, content)]
	case "array_creation_expression":
		// new A[]{...} / new A[n] — element type for Arrays.stream first arg.
		if typeN := ingest.ChildByField(obj, "type"); typeN != nil {
			return javaTypeName(typeN, content)
		}
		return ""
	case "object_creation_expression":
		// new AtomicReference<>(new A()) / WeakReference / SoftReference → A
		// (holder of V; enables new AtomicReference<>(new A()).get().m() and
		// var ar = new AtomicReference<>(new A()); ar.get().m() under foreign
		// same-leaf methods). Other constructions fail closed here so
		// new A().m() stays on the direct object_creation rename path.
		if et := javaReferenceHolderCreationElemType(obj, content); et != "" {
			return et
		}
		// new FutureTask<>(() -> new A()) / FutureTask<>(runnable, new A()) → A
		// (Future/Callable holder; enables .get().m() and var ft = new FutureTask<>(…)).
		if et := javaFutureTaskCreationElemType(obj, content); et != "" {
			return et
		}
		// new CompletableFuture<A>() / new CompletableFuture<A>(…) → A
		// (empty/typed CF holder; enables completeAsync(...).join().m() /
		// var f = new CompletableFuture<A>(); f.join().m() under foreign same-leaf).
		// Diamond without a recoverable fill fails closed.
		if et := javaCompletableFutureCreationElemType(obj, content); et != "" {
			return et
		}
		// new ArrayList<>(List.of(new A())) / new LinkedList<>(as) / new HashSet<>(…)
		// — Collection copy/view ctors: element from declared type arg or first-arg
		// pipeline (List.of / typed local / stream.toList()). Enables .get(0).m() /
		// .forEach(a -> a.m()) / var al = new ArrayList<>(as) under foreign same-leaf.
		return javaCollectionCopyCreationElemType(obj, content, elemOf, valOf)
	case "cast_expression":
		// (ArrayList<A>) as.clone() / (List<A>) x / (A[]) arr.clone() —
		// recover E from a single type-arg generic cast or array cast type.
		// Multi-arg casts (Map<K,V>) fail closed here so .get(k) does not take
		// K as the element type (value recovery stays on javaMapPipelineValueType).
		// Otherwise peel the cast value so type-preserving pipelines under the
		// cast (as.clone()) still bind.
		if typeN := ingest.ChildByField(obj, "type"); typeN != nil {
			switch typeN.Type() {
			case "generic_type":
				args := javaTypeArgNames(typeN, content)
				if len(args) == 1 {
					return args[0]
				}
				// Map<K,V> (and other multi-arg) casts: do not peel to the value
				// (clone → map local would yield K via elemOf). Fail closed so
				// Map.get/forEach recover V via javaMapPipelineValueType.
				if len(args) >= 2 {
					return ""
				}
			case "array_type":
				if elem := ingest.ChildByField(typeN, "element"); elem != nil {
					if et := javaTypeName(elem, content); et != "" {
						return et
					}
				}
			}
		}
		if val := ingest.ChildByField(obj, "value"); val != nil {
			return javaStreamPipelineElemType(val, content, elemOf, valOf)
		}
		return ""
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		switch name := ingest.NodeText(nameN, content); name {
		case "stream", "parallelStream", "iterator",
			// listIterator() returns ListIterator<E> of the same element type
			// (List; previous/next yield E like Iterator.next).
			"listIterator",
			// descendingIterator() returns Iterator<E> of the same element type
			// (Deque / NavigableSet / NavigableMap key views; reverse order only).
			"descendingIterator",
			// spliterator() returns Spliterator<E> of the same element type
			// (Collection/Stream; tryAdvance/forEachRemaining yield E).
			"spliterator",
			// elements() returns Enumeration of the collection elements
			// (Vector; Hashtable/Dictionary use value type via special case below).
			"elements",
			"filter", "peek", "sorted", "distinct", "limit", "skip",
			"unordered", "sequential", "parallel", "onClose",
			"takeWhile", "dropWhile",
			// findFirst/findAny/min/max/reduce(BinaryOperator) return Optional<T>;
			// element T is preserved for ifPresent / orElse (Comparator/op args do
			// not change the element type). Identity reduce returns T directly —
			// handled in javaCollectionAccessElemType for bare var targets.
			// toList() returns List<T> — element preserved for forEach / for-var.
			// toArray() / toArray(generator) returns T[] (Object[] for zero-arg Stream)
			// — element preserved for toArray()[i] / var arr = toArray(); arr[i].
			// clone() returns T[] of the same element type for arrays (and shallow
			// Collection copies with the same E) — element preserved for clone()[i]
			// / var arr = as.clone(); arr[i].
			// reversed() returns List/SequencedCollection of the same element type
			// (Java 21 SequencedCollection; order only, element type unchanged).
			// Map.reversed() (SequencedMap) is not an element pipeline — key-type
			// elemOf would mis-type .get(k); value typing uses javaMapPipelineValueType.
			// subList(from, to) returns List of the same element type
			// (List view; bounds only, element type unchanged).
			// descendingSet() returns NavigableSet of the same element type
			// (NavigableSet reverse-order view; order only).
			// headSet/tailSet/subSet(...) return SortedSet/NavigableSet of the same
			// element type (range views; bounds/inclusivity args do not change E).
			// CompletableFuture.whenComplete / whenCompleteAsync / copy /
			// toCompletableFuture / orTimeout / completeOnTimeout /
			// exceptionally / exceptionallyAsync / exceptionallyCompose /
			// exceptionallyComposeAsync / minimalCompletionStage — always return
			// the same result T by API signature (side-effect / timeout / recovery /
			// Executor args do not change T). completeAsync is handled below
			// (typed CF peels receiver; diamond/raw falls back to supplier body).
			// Enables whenComplete(...).join() / whenCompleteAsync(...).join() /
			// exceptionallyAsync(...).join() / copy().join() under foreign
			// same-leaf methods. Type-changing stages (thenApply/handle/…) stay
			// on their own identity/rewrap peels.
			// Optional.or(Supplier) always returns Optional of the same T by API
			// signature (alternative Optional supplier does not change T).
			// Enables or(...).ifPresent / or(...).orElse / var o2 = or(...);
			// under foreign same-leaf methods. Stream has no or — Optional-only.
			"findFirst", "findAny", "min", "max", "reduce", "toList", "toArray", "clone", "reversed", "subList",
			"descendingSet", "headSet", "tailSet", "subSet",
			"whenComplete", "whenCompleteAsync", "copy", "toCompletableFuture",
			"orTimeout", "completeOnTimeout",
			"exceptionally", "exceptionallyAsync",
			"exceptionallyCompose", "exceptionallyComposeAsync",
			"minimalCompletionStage",
			"or":
			recv := ingest.ChildByField(obj, "object")
			// Arrays.stream(arr[, from, to]) — element type from first arg, not
			// from receiver Arrays (unlike coll.stream() which uses elemOf[coll]).
			if name == "stream" && javaIsArraysReceiver(recv, content) {
				return javaArraysStreamElemType(obj, content, elemOf, valOf)
			}
			// Hashtable/Dictionary.elements() yields Enumeration of values (V),
			// not keys — prefer valOf when the receiver is map-like (2 type args).
			// Vector.elements() has only elemOf and falls through to pipeline.
			if name == "elements" && recv != nil && recv.Type() == "identifier" && valOf != nil {
				if vt := valOf[ingest.NodeText(recv, content)]; vt != "" {
					return vt
				}
			}
			// SequencedMap.reversed(): fail closed here so .get / values / forEach
			// recover V via javaMapPipelineValueType (elemOf is the key type).
			if name == "reversed" && javaMapPipelineValueType(recv, content, elemOf, valOf) != "" {
				return ""
			}
			return javaStreamPipelineElemType(recv, content, elemOf, valOf)
		case "completeAsync":
			// CompletableFuture.completeAsync(() -> new A()) /
			// completeAsync(() -> new A(), executor) — returns this CF.
			// Typed new CompletableFuture<A>().completeAsync(...) peels T from
			// the receiver; diamond/raw new CompletableFuture<>().completeAsync(
			// () -> new A()) recovers T from the supplier body (same shapes as
			// supplyAsync). Enables .join().m() / var f = …completeAsync…;
			// f.join().m() under foreign same-leaf methods. Blocks / method refs
			// / non-creation bodies fail closed when the receiver has no T.
			recv := ingest.ChildByField(obj, "object")
			if et := javaStreamPipelineElemType(recv, content, elemOf, valOf); et != "" {
				return et
			}
			return javaCompleteAsyncSupplierElemType(obj, content)
		case "flatMap":
			// Optional.flatMap (and Stream.flatMap with the same rewrap shapes):
			// recover U from mapper when clearly Optional.of/ofNullable rewrap
			// or another tracked Optional/collection local. Unknown mappers fail closed.
			return javaFlatMapResultElemType(obj, content, elemOf, valOf)
		case "thenCompose", "thenComposeAsync":
			// CompletableFuture.thenCompose / thenComposeAsync(Function[, Executor]):
			// recover U when mapper clearly rewraps T as CompletionStage —
			// CompletableFuture.completedFuture(a) / completedFuture(new A()) /
			// tracked CF local — same rewrap shapes as Optional.flatMap. Executor
			// arg ignored; javaFlatMapResultElemType picks the first lambda among
			// args. Enables thenComposeAsync(a -> completedFuture(a)).join() /
			// var f2 = thenComposeAsync(...); f2.join() under foreign same-leaf.
			// Type-changing mappers fail closed.
			return javaFlatMapResultElemType(obj, content, elemOf, valOf)
		case "map":
			// Optional.map / Stream.map: recover U from mapper when clearly
			// identity (a -> a) or new T(...). Unknown/type-changing mappers fail closed.
			return javaMapResultElemType(obj, content, elemOf, valOf)
		case "mapMulti":
			// Stream.mapMulti(BiConsumer): recover R when clearly accept-of-identity
			// (a, c) -> c.accept(a) or accept(new T(...)). Unknown sinks fail closed.
			// Enables mapMulti((a,c)->c.accept(a)).forEach(x -> x.m()) /
			// var s = mapMulti(...); s.forEach(...) under foreign same-leaf methods.
			return javaMapMultiResultElemType(obj, content, elemOf, valOf)
		case "gather":
			// Stream.gather(Gatherer): recover R when the gatherer clearly preserves
			// or constructs a known element type:
			// Gatherers.mapConcurrent(n, a -> a) / mapConcurrent(n, a -> new T())
			// (identity / new), fold/scan with () -> new T() + identity integrator.
			// windowFixed/windowSliding yield Stream of List<T> — not a scalar T
			// pipeline (see javaWindowGatherStreamElemType / javaWindowListExprElemType
			// for List-window + get(0) peels). Custom gatherers fail closed.
			// Enables gather(Gatherers.mapConcurrent(1, a -> a)).findFirst()
			// .get().m() / var s = gather(...); s.findFirst().get().m() under foreign
			// same-leaf methods.
			return javaGatherResultElemType(obj, content, elemOf, valOf)
		case "thenApply", "thenApplyAsync":
			// CompletableFuture.thenApply / thenApplyAsync(Function[, Executor]):
			// recover U when mapper is identity (a -> a) or new T(...) — same shapes
			// as Optional.map. Executor arg ignored; javaMapResultElemType picks the
			// first lambda among args. Enables thenApplyAsync(a -> a).join() /
			// var f2 = thenApplyAsync(a -> a); f2.join() under foreign same-leaf.
			// Type-changing mappers fail closed.
			return javaMapResultElemType(obj, content, elemOf, valOf)
		case "applyToEither", "applyToEitherAsync":
			// CompletableFuture.applyToEither / applyToEitherAsync(other, Function[, Executor]):
			// recover U when the Function is identity (a -> a) or new T(...) — same
			// shapes as thenApply. Other stage is a non-lambda first arg; optional
			// Executor is non-lambda; javaMapResultElemType picks the first lambda
			// among args. Enables applyToEitherAsync(other, a -> a).join() /
			// var f2 = applyToEitherAsync(...); f2.join() under foreign same-leaf.
			// Type-changing mappers and acceptEither (Void) fail closed.
			return javaMapResultElemType(obj, content, elemOf, valOf)
		case "handle", "handleAsync":
			// CompletableFuture.handle / handleAsync(BiFunction[, Executor]): recover U
			// when bi-lambda is first-param identity ((a, e) -> a) or new T(...) —
			// same shapes as thenApply identity, bi form. Executor arg ignored.
			// Enables handleAsync((a,e)->a).join() / var f2 = handleAsync(...); f2.join()
			// under foreign same-leaf methods. Type-changing mappers fail closed.
			// whenComplete / whenCompleteAsync always return T by signature and peel
			// as type-preserving stages (see peel list).
			return javaMapResultElemType(obj, content, elemOf, valOf)
		case "thenCombine", "thenCombineAsync":
			// CompletableFuture.thenCombine / thenCombineAsync(other, BiFunction[, Executor]):
			// recover V when the BiFunction is first-param identity ((a, b) -> a) or
			// new T(...) — same shapes as handle, with other stage first like
			// applyToEither. Executor arg ignored; javaMapResultElemType picks the
			// first lambda among args. Enables thenCombineAsync(other, (a,b)->a).join()
			// / var f2 = thenCombineAsync(...); f2.join() under foreign same-leaf.
			// Type-changing mappers and thenAcceptBoth (Void) fail closed.
			return javaMapResultElemType(obj, content, elemOf, valOf)
		case "collect":
			// Stream.collect(Collectors.toList()/toSet()/toUnmodifiableList()/
			// toUnmodifiableSet()/toCollection(…)) / collect(toList()/toSet()/
			// toUnmodifiable…/toCollection(…)) / collect(Collectors::toList / toSet /
			// toUnmodifiableList / toUnmodifiableSet) /
			// collect(Collectors.collectingAndThen(toList()/toSet()/toUnmodifiable…/
			// toCollection(…), finisher)) /
			// collect(Collectors.filtering(pred, toList()/…)) /
			// collect(Collectors.mapping(a -> a, toList()/…)) (identity mapper only) /
			// collect(Collectors.flatMapping(a -> Stream.of(a), toList()/…))
			// (Stream.of / ofNullable rewrap only) /
			// collect(Collectors.teeing(toList()/…, …, (list, …) -> list)) —
			// Collection of the stream element type.
			// collect(Collectors.reducing(...)/maxBy(...)/minBy(...)) — Optional<T>
			// or T of the stream element (same leaf as stream.reduce/min/max for
			// orElse/ifPresent / var opt tracking). Other collectors
			// (groupingBy, type-changing mapping/flatMapping, toMap, …) fail closed here
			// (toMap values recovered via javaMapPipelineValueType / javaToMapCollectValueType).
			if javaIsToListOrSetCollector(obj, content) || javaIsReducingMaxMinCollector(obj, content) {
				return javaStreamPipelineElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
			}
			return ""
		case "values", "sequencedValues":
			// m.values() / collect(toMap(...)).values() — Collection of map values.
			// m.sequencedValues() — SequencedCollection of map values V (Java 21;
			// order only; same V leaf as values — enables getFirst/getLast/forEach).
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "keySet", "navigableKeySet", "descendingKeySet":
			// m.keySet() / navigableKeySet() / descendingKeySet() — Set of map keys K
			// (Map stores K in elemOf; same key leaf as Collections.newSetFromMap).
			// navigable/descending are order-only views; key type unchanged.
			// Map view receivers (descendingMap/headMap/…) recover K via the key
			// pipeline (dual of values → javaMapPipelineValueType).
			return javaMapPipelineKeyType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "get", "orElseThrow", "orElse", "orElseGet":
			// Optional.of(Set.of(new A())).get().iterator().next() /
			// Optional.of(Set.of(new A())).get().stream() / .forEach /
			// Optional.ofNullable(List.of(new A())).orElseThrow().get(0) /
			// Optional.of(Collections.singleton(new A())).get().iterator().next() —
			// Optional wrapping a collection factory of T: zero-arg get / orElse*
			// yield the Collection of T, so subsequent iterator/stream/forEach/get
			// peels T under foreign same-leaf (same leaf as javaNestedCollectionGet
			// for double-get List forms). Scalar Optional.of(new A()).get() stays
			// on the static-factory peel below via collection-access (not pipeline
			// of get). List.get(i) with args fails closed here (not Optional unwrap).
			recv := ingest.ChildByField(obj, "object")
			if name == "get" && !javaCallIsZeroArg(obj) {
				return ""
			}
			if nest := javaOptionalOfCollectionFactoryElemType(recv, content); nest != "" {
				return nest
			}
			return ""
		case "of", "asList", "ofNullable", "singletonList", "singleton", "completedFuture", "completedStage":
			// List.of(new A()) / Stream.of(new A(), new A()) / Arrays.asList(new A())
			// / Set.of(new A()) / Optional.of(new A()) / Optional.ofNullable(new A())
			// / Stream.ofNullable(new A())
			// / Collections.singletonList(new A()) / Collections.singleton(new A())
			// / CompletableFuture.completedFuture(new A())
			// / CompletableFuture.completedStage(new A())
			// — element type from homogeneous new T(...) args.
			// Enables completedFuture(new A()).join().m() / get() / getNow(d) /
			// resultNow() and var f = completedFuture(new A()); f.join().m() under
			// foreign same-leaf methods (javaStaticCollectionOfElemType already peels
			// completedFuture/completedStage; wire it into the pipeline switch).
			// completedStage returns CompletionStage; toCompletableFuture().join()
			// peels as type-preserving stages on the same T leaf.
			// Arrays.asList(as) / Arrays.asList(new A[]{...}) — when args are not
			// homogeneous creations, peel first-arg array element type (same path as
			// Arrays.stream / copyOf). List/Set.of stay creation-only.
			if et := javaStaticCollectionOfElemType(obj, content, name); et != "" {
				return et
			}
			if name == "asList" {
				recv := ingest.ChildByField(obj, "object")
				if javaIsArraysReceiver(recv, content) {
					return javaArraysStreamElemType(obj, content, elemOf, valOf)
				}
			}
			return ""
		case "nCopies":
			// Collections.nCopies(n, new A()) — List of T from the second arg.
			return javaCollectionsNCopiesElemType(obj, content)
		case "unmodifiableList", "synchronizedList", "checkedList",
			"unmodifiableSet", "synchronizedSet", "checkedSet",
			"unmodifiableSortedSet", "synchronizedSortedSet", "checkedSortedSet",
			"unmodifiableNavigableSet", "synchronizedNavigableSet", "checkedNavigableSet",
			"unmodifiableSequencedCollection", "synchronizedSequencedCollection",
			"unmodifiableSequencedSet", "synchronizedSequencedSet",
			"unmodifiableCollection", "synchronizedCollection", "checkedCollection",
			"asLifoQueue", "checkedQueue",
			"newSetFromMap",
			"list", "enumeration":
			// Collections.unmodifiableList/Set/Collection(as) /
			// synchronizedList/Set/Collection(as) /
			// checkedList/Set/Collection(as, A.class) — Collection of first-arg
			// element type (Class arg on checked* ignored).
			// Collections.unmodifiableSortedSet/NavigableSet(as) /
			// synchronizedSortedSet/NavigableSet(as) /
			// checkedSortedSet/NavigableSet(as, A.class) — same E (SortedSet/
			// NavigableSet wrappers; Class args ignored; mirrors *SortedMap/
			// *NavigableMap on the value path).
			// Collections.unmodifiableSequencedCollection/Set(as) /
			// synchronizedSequencedCollection/Set(as) — same E (Sequenced
			// wrappers; Java 21; mirrors unmodifiableSequencedMap).
			// Collections.asLifoQueue(deque) / checkedQueue(queue, A.class) —
			// Queue of first-arg element type (Class arg ignored).
			// Collections.newSetFromMap(map) — Set of first-arg map key type
			// (Map stores K in elemOf; same key leaf as Map.keySet).
			// Collections.list(Enumeration) — ArrayList of enumeration elements.
			// Collections.enumeration(coll) — Enumeration of coll elements.
			return javaCollectionsListWrapperElemType(obj, content, elemOf, valOf)
		case "copyOf", "copyOfRange":
			// Arrays.copyOf(arr, n) / Arrays.copyOfRange(arr, from, to) — T[] of first-arg
			// element type (length/range bounds only; same first-arg peel as Arrays.stream).
			// List.copyOf(coll) / Set.copyOf(coll) — Collection of first-arg element type
			// (copyOf only; unlike of/asList which take new T(...) args).
			recv := ingest.ChildByField(obj, "object")
			if javaIsArraysReceiver(recv, content) {
				return javaArraysStreamElemType(obj, content, elemOf, valOf)
			}
			if name == "copyOf" {
				return javaListSetCopyOfElemType(obj, content, elemOf, valOf)
			}
			return ""
		case "concat":
			// Stream.concat(s1, s2) — Stream of both args' element type when they agree.
			return javaStreamConcatElemType(obj, content, elemOf, valOf)
		case "generate":
			// Stream.generate(() -> new A()) — element type from supplier body.
			return javaStreamGenerateElemType(obj, content)
		case "supplyAsync":
			// CompletableFuture.supplyAsync(() -> new A()) /
			// supplyAsync(() -> new A(), executor) — result T from supplier body.
			// Enables supplyAsync(() -> new A()).join().m() / get() / getNow(d) /
			// resultNow() and var f = supplyAsync(() -> new A()); f.join().m() under
			// foreign same-leaf methods (same supplier peel as Stream.generate).
			// Executor arg ignored; blocks / method refs / non-creation bodies fail closed.
			// runAsync (Void) is not wired.
			return javaCompletableFutureSupplyAsyncElemType(obj, content)
		case "withInitial":
			// ThreadLocal.withInitial(() -> new A()) — T from supplier body.
			// Enables withInitial(() -> new A()).get().m() and
			// var tl = withInitial(() -> new A()); tl.get().m() under foreign
			// same-leaf methods (same supplier peel as Stream.generate).
			return javaThreadLocalWithInitialElemType(obj, content)
		case "submit":
			// ExecutorService.submit(() -> new A()) / submit(Callable) —
			// Future result T from zero-arg lambda body new T(...).
			// Enables submit(() -> new A()).get().m() and var f = submit(...); f.get()
			// under foreign same-leaf methods. Runnable submit (no result) and
			// non-creation bodies fail closed.
			return javaExecutorSubmitCallableElemType(obj, content)
		case "iterate":
			// Stream.iterate(new A(), a -> a) / iterate(new A(), pred, a -> a) —
			// element type from seed creation.
			return javaStreamIterateElemType(obj, content)
		default:
			// flatMapTo*/mapTo*/… change the element type — fail closed.
			return ""
		}
	default:
		return ""
	}
}

// javaMapResultElemType recovers U after Optional.map / Stream.map / CF thenApply
// / CF applyToEither / CF handle / CF thenCombine when the mapper clearly yields
// a known element type:
//
//	oa.map(a -> a) → elem(oa)                         // identity
//	oa.map(a -> new A()) → A                          // object creation
//	as.stream().map(a -> a).forEach(...) → elem(as)   // same for Stream
//	fa.thenApply(a -> a) → elem(fa)                   // CF Function identity
//	fa.applyToEither(other, a -> a) → elem(fa)        // CF Function identity (2nd arg)
//	fa.handle((a, e) -> a) → elem(fa)                 // CF BiFunction first-param identity
//	fa.thenCombine(other, (a, b) -> a) → elem(fa)     // CF BiFunction first-param identity (2nd arg)
//
// Expression-bodied lambdas only; blocks, method refs, and other mappers fail
// closed so type-changing maps stay unbound. Bi-lambda form binds only when the
// body is the first parameter ((a, e) -> a / (a, b) -> a); second-param bodies
// fail closed. The first lambda among call args is used (thenApply/map/handle:
// sole arg; applyToEither/thenCombine: Function/BiFunction after the other stage).
func javaMapResultElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	// First lambda among args: thenApply/map/handle take a sole functional arg;
	// applyToEither(other, Function) / thenCombine(other, BiFunction) place the
	// functional arg after a non-lambda stage.
	var first *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() == "lambda_expression" {
			first = ch
			break
		}
	}
	if first == nil {
		return ""
	}
	body := ingest.ChildByField(first, "body")
	if body == nil {
		return ""
	}
	params := javaInferredLambdaParamNames(first, content)
	param := ""
	switch len(params) {
	case 1:
		// Function: a -> a / a -> new T()
		param = params[0]
	case 2:
		// BiFunction (CF handle / thenCombine): (a, e) -> a / (a, b) -> a /
		// (a, e) -> new T() / (a, b) -> new T()
		// First-param identity only; body matching params[1] fails closed below.
		param = params[0]
	default:
		return ""
	}
	recv := ingest.ChildByField(call, "object")
	return javaMapMapperBodyElemType(body, param, recv, content, elemOf, valOf)
}

// javaGatherResultElemType recovers R after Stream.gather(gatherer) when the
// gatherer clearly preserves or constructs a known element type:
//
//	s.gather(Gatherers.mapConcurrent(n, a -> a)) → elem(s)
//	s.gather(Gatherers.mapConcurrent(n, a -> new A())) → A
//	s.gather(java.util.stream.Gatherers.mapConcurrent(n, a -> a)) → elem(s)
//	s.gather(Gatherers.fold(() -> new A(), (a, t) -> a)) → A
//	s.gather(Gatherers.scan(() -> new A(), (a, t) -> a)) → A
//
// mapConcurrent with expression-bodied identity/new mapper, and fold/scan with
// expression-bodied `() -> new T()` initializer plus identity integrator
// `(r, t) -> r`, are recognized. windowFixed/windowSliding yield List windows
// (not scalar R) — see javaWindowGatherStreamElemType. Custom fail closed.
func javaGatherResultElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	first, gName, gArgs := javaGathererCallParts(call, content)
	if first == nil || gArgs == nil {
		return ""
	}
	switch gName {
	case "mapConcurrent":
		return javaGatherMapConcurrentElemType(call, gArgs, content, elemOf, valOf)
	case "fold", "scan":
		return javaGatherFoldScanElemType(gArgs, content)
	default:
		return ""
	}
}

// javaGathererCallParts extracts the Gatherers.* invocation from stream.gather(…).
// Returns (gathererCall, gathererName, gathererArgs) or nils when not a Gatherers
// static call (windowFixed/mapConcurrent/fold/…).
func javaGathererCallParts(gatherCall *grammar.Node, content []byte) (*grammar.Node, string, *grammar.Node) {
	if gatherCall == nil || gatherCall.Type() != "method_invocation" {
		return nil, "", nil
	}
	var args *grammar.Node
	for i := uint32(0); i < gatherCall.ChildCount(); i++ {
		if gatherCall.Child(i).Type() == "argument_list" {
			args = gatherCall.Child(i)
			break
		}
	}
	if args == nil {
		return nil, "", nil
	}
	var first *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		first = ch
		break
	}
	if first == nil || first.Type() != "method_invocation" {
		return nil, "", nil
	}
	nameN := ingest.ChildByField(first, "name")
	if nameN == nil {
		return nil, "", nil
	}
	gName := ingest.NodeText(nameN, content)
	recv := ingest.ChildByField(first, "object")
	if !javaIsGatherersReceiver(recv, content) {
		return nil, "", nil
	}
	var gArgs *grammar.Node
	for i := uint32(0); i < first.ChildCount(); i++ {
		if first.Child(i).Type() == "argument_list" {
			gArgs = first.Child(i)
			break
		}
	}
	return first, gName, gArgs
}

// javaWindowGatherStreamElemType recovers T from stream.gather(Gatherers.windowFixed(n))
// / windowSliding(n) (and FQ Gatherers), or from a window-stream local
// (var s = gather(window*)). The gather stream yields List<T> windows; T is the
// upstream stream element type. Custom / non-window gatherers fail closed.
// Enables gather(windowFixed(1)).forEach(w -> w.get(0).m()) /
// s.forEach(w -> w.get(0).m()) under foreign same-leaf (param tracked as List of T
// via elemOf).
func javaWindowGatherStreamElemType(obj *grammar.Node, content []byte, elemOf, valOf, windowStreamOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	// var s = stream.gather(window*); s.findFirst()… / s.forEach(…)
	if obj.Type() == "identifier" {
		if windowStreamOf == nil {
			return ""
		}
		return windowStreamOf[ingest.NodeText(obj, content)]
	}
	if obj.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(obj, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "gather" {
		return ""
	}
	_, gName, gArgs := javaGathererCallParts(obj, content)
	if gArgs == nil {
		return ""
	}
	switch gName {
	case "windowFixed", "windowSliding":
		return javaStreamPipelineElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
	default:
		return ""
	}
}

// javaWindowListExprElemType recovers T from an Optional unwrap of a window-gather
// stream: gather(window*).findFirst().get() / findAny().get() /
// findFirst().orElseThrow([…]) / s.findFirst().get() after var s = gather(window*) /
// oa.get() after var oa = s.findFirst() (window Optional intermediate)
// → List of T's element type T for subsequent .get(0).m() / var w = …; w.get(0).m().
// Other unwraps fail closed.
func javaWindowListExprElemType(obj *grammar.Node, content []byte, elemOf, valOf, windowStreamOf, windowOptOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() || obj.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(obj, "name")
	if nameN == nil {
		return ""
	}
	switch ingest.NodeText(nameN, content) {
	case "get", "orElseThrow":
	default:
		return ""
	}
	// get must be zero-arg (Optional.get); orElseThrow may have a supplier arg.
	if ingest.NodeText(nameN, content) == "get" && !javaCallIsZeroArg(obj) {
		return ""
	}
	recv := ingest.ChildByField(obj, "object")
	for recv != nil && !recv.IsNull() && recv.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(recv, "expression")
		if inner == nil {
			for i := uint32(0); i < recv.ChildCount(); i++ {
				ch := recv.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		recv = inner
	}
	if recv == nil || recv.IsNull() {
		return ""
	}
	// var oa = s.findFirst(); oa.get() / oa.orElseThrow() — Optional intermediate.
	if recv.Type() == "identifier" {
		if windowOptOf == nil {
			return ""
		}
		return windowOptOf[ingest.NodeText(recv, content)]
	}
	if recv.Type() != "method_invocation" {
		return ""
	}
	rName := ingest.ChildByField(recv, "name")
	if rName == nil {
		return ""
	}
	switch ingest.NodeText(rName, content) {
	case "findFirst", "findAny":
	default:
		return ""
	}
	return javaWindowGatherStreamElemType(ingest.ChildByField(recv, "object"), content, elemOf, valOf, windowStreamOf)
}

// javaWindowOptExprElemType recovers T from findFirst/findAny on a window-gather
// stream: gather(window*).findFirst() / s.findFirst() after var s = gather(window*)
// → Optional of List<T> (T is upstream element). Used to track
// var oa = s.findFirst() for later oa.get().get(0).m() peels.
func javaWindowOptExprElemType(obj *grammar.Node, content []byte, elemOf, valOf, windowStreamOf, windowOptOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	// Rebind: var ob = oa when oa is already a window Optional local.
	if obj.Type() == "identifier" {
		if windowOptOf == nil {
			return ""
		}
		return windowOptOf[ingest.NodeText(obj, content)]
	}
	if obj.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(obj, "name")
	if nameN == nil {
		return ""
	}
	switch ingest.NodeText(nameN, content) {
	case "findFirst", "findAny":
	default:
		return ""
	}
	return javaWindowGatherStreamElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf, windowStreamOf)
}

// javaCallIsZeroArg reports whether a method_invocation has no positional args.
func javaCallIsZeroArg(n *grammar.Node) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil || ch.Type() != "argument_list" {
			continue
		}
		for j := uint32(0); j < ch.ChildCount(); j++ {
			a := ch.Child(j)
			if a == nil || a.Type() == "(" || a.Type() == ")" || a.Type() == "," {
				continue
			}
			return false
		}
		return true
	}
	return true
}

// javaGatherMapConcurrentElemType recovers R from Gatherers.mapConcurrent(n, mapper)
// when mapper is expression-bodied a -> a (stream elem) or a -> new T().
func javaGatherMapConcurrentElemType(gatherCall, gArgs *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	// Mapper is the first lambda among mapConcurrent args (after concurrency int).
	var mapper *grammar.Node
	for i := uint32(0); i < gArgs.ChildCount(); i++ {
		ch := gArgs.Child(i)
		if ch.Type() == "lambda_expression" {
			mapper = ch
			break
		}
	}
	if mapper == nil {
		return ""
	}
	params := javaInferredLambdaParamNames(mapper, content)
	if len(params) != 1 {
		return ""
	}
	body := javaLambdaExprBody(mapper)
	if body == nil || body.IsNull() {
		return ""
	}
	streamRecv := ingest.ChildByField(gatherCall, "object")
	switch body.Type() {
	case "identifier":
		// a -> a — identity preserves stream element type.
		if ingest.NodeText(body, content) == params[0] {
			return javaStreamPipelineElemType(streamRecv, content, elemOf, valOf)
		}
		return ""
	case "object_creation_expression":
		// a -> new A() — element type from construction.
		if typeN := ingest.ChildByField(body, "type"); typeN != nil {
			return javaTypeName(typeN, content)
		}
		return ""
	default:
		return ""
	}
}

// javaGatherFoldScanElemType recovers R from Gatherers.fold/scan when the
// initializer is expression-bodied `() -> new T()` and the integrator is
// expression-bodied identity on the accumulator `(r, t) -> r`. Other shapes
// (blocks, type-changing integrators, non-new suppliers) fail closed.
func javaGatherFoldScanElemType(gArgs *grammar.Node, content []byte) string {
	if gArgs == nil {
		return ""
	}
	var lambdas []*grammar.Node
	for i := uint32(0); i < gArgs.ChildCount(); i++ {
		ch := gArgs.Child(i)
		if ch != nil && ch.Type() == "lambda_expression" {
			lambdas = append(lambdas, ch)
		}
	}
	if len(lambdas) != 2 {
		return ""
	}
	init, integ := lambdas[0], lambdas[1]
	// Initializer: () -> new T()
	if len(javaInferredLambdaParamNames(init, content)) != 0 {
		return ""
	}
	initBody := javaLambdaExprBody(init)
	if initBody == nil || initBody.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(initBody, "type")
	if typeN == nil {
		return ""
	}
	tn := javaTypeName(typeN, content)
	if tn == "" {
		return ""
	}
	// Integrator: (r, t) -> r — identity on first param.
	params := javaInferredLambdaParamNames(integ, content)
	if len(params) != 2 {
		return ""
	}
	integBody := javaLambdaExprBody(integ)
	if integBody == nil || integBody.Type() != "identifier" {
		return ""
	}
	if ingest.NodeText(integBody, content) != params[0] {
		return ""
	}
	return tn
}

// javaLambdaExprBody returns the expression body of a lambda after unwrapping
// parenthesized_expression. Block bodies fail closed (nil).
func javaLambdaExprBody(lambda *grammar.Node) *grammar.Node {
	if lambda == nil {
		return nil
	}
	body := ingest.ChildByField(lambda, "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.IsNull() || body.Type() == "block" {
		return nil
	}
	return body
}

// javaIsGatherersReceiver reports whether n is Gatherers or a qualified
// java.util.stream.Gatherers (field_access / scoped identifier chain).
func javaIsGatherersReceiver(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "identifier":
		return ingest.NodeText(n, content) == "Gatherers"
	case "field_access":
		// java.util.stream.Gatherers — outermost field is Gatherers.
		if nameN := ingest.ChildByField(n, "field"); nameN != nil {
			return ingest.NodeText(nameN, content) == "Gatherers"
		}
		// Some grammars use "name" for the leaf.
		if nameN := ingest.ChildByField(n, "name"); nameN != nil {
			return ingest.NodeText(nameN, content) == "Gatherers"
		}
		return false
	case "scoped_type_identifier", "scoped_identifier":
		if nameN := ingest.ChildByField(n, "name"); nameN != nil {
			return ingest.NodeText(nameN, content) == "Gatherers"
		}
		return false
	default:
		return false
	}
}

// javaMapMultiResultElemType recovers R after Stream.mapMulti(mapper) when the
// BiConsumer clearly sinks the element (or a known construction) via accept:
//
//	s.mapMulti((a, c) -> c.accept(a)) → elem(s)
//	s.mapMulti((a, c) -> c.accept(new A())) → A
//
// Expression-bodied lambdas only; blocks and other bodies fail closed so
// type-changing sinks stay unbound.
func javaMapMultiResultElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	var first *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() == "lambda_expression" {
			first = ch
			break
		}
	}
	if first == nil {
		return ""
	}
	params := javaInferredLambdaParamNames(first, content)
	if len(params) != 2 {
		return ""
	}
	body := ingest.ChildByField(first, "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.IsNull() || body.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(body, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "accept" {
		return ""
	}
	// Sink must be the Consumer param (second).
	sink := ingest.ChildByField(body, "object")
	if sink == nil || sink.Type() != "identifier" || ingest.NodeText(sink, content) != params[1] {
		return ""
	}
	callArgs := javaCallArgs(body)
	if len(callArgs) != 1 {
		return ""
	}
	arg := callArgs[0]
	recv := ingest.ChildByField(call, "object")
	switch arg.Type() {
	case "identifier":
		// c.accept(a) — identity preserves receiver element type.
		if ingest.NodeText(arg, content) == params[0] {
			return javaStreamPipelineElemType(recv, content, elemOf, valOf)
		}
		return ""
	case "object_creation_expression":
		// c.accept(new A()) — element type from construction.
		if typeN := ingest.ChildByField(arg, "type"); typeN != nil {
			return javaTypeName(typeN, content)
		}
		return ""
	default:
		return ""
	}
}

// javaMapMapperBodyElemType recovers U from an expression-bodied map mapper.
func javaMapMapperBodyElemType(body *grammar.Node, param string, recv *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.IsNull() {
		return ""
	}
	switch body.Type() {
	case "identifier":
		// a -> a — identity preserves receiver element type.
		if param != "" && ingest.NodeText(body, content) == param {
			return javaStreamPipelineElemType(recv, content, elemOf, valOf)
		}
		return ""
	case "object_creation_expression":
		// a -> new A() — element type from construction.
		if typeN := ingest.ChildByField(body, "type"); typeN != nil {
			return javaTypeName(typeN, content)
		}
		return ""
	default:
		return ""
	}
}

// javaFlatMapResultElemType recovers T after Optional.flatMap(mapper) or
// CompletableFuture.thenCompose(mapper) when the mapper clearly yields
// Optional/CompletionStage of the same (or known) element:
//
//	oa.flatMap(a -> Optional.of(a)) / Optional.ofNullable(a) → elem(oa)
//	oa.flatMap(Optional::of) / Optional::ofNullable → elem(oa)
//	oa.flatMap(a -> other) when other is tracked in elemOf → elemOf[other]
//	oa.flatMap(a -> Optional.of(new A())) → A
//	fa.thenCompose(a -> CompletableFuture.completedFuture(a)) → elem(fa)
//	fa.thenCompose(CompletableFuture::completedFuture) → elem(fa)
//	fa.thenCompose(a -> other) when other is tracked in elemOf → elemOf[other]
//	fa.thenCompose(a -> CompletableFuture.completedFuture(new A())) → A
//
// Expression-bodied lambdas and Optional::of / ofNullable / Stream.of /
// Stream.ofNullable / Collection::stream / List::stream /
// CompletableFuture::completedFuture only; blocks and other mappers fail closed
// so type-changing flatMap / thenCompose stay unbound.
func javaFlatMapResultElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	var first *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			first = ch
		}
		if first != nil {
			break
		}
	}
	if first == nil {
		return ""
	}
	recv := ingest.ChildByField(call, "object")
	switch first.Type() {
	case "method_reference":
		if javaIsOptionalOfMethodRef(first, content) || javaIsStreamOfMethodRef(first, content) || javaIsCompletedFutureMethodRef(first, content) {
			return javaStreamPipelineElemType(recv, content, elemOf, valOf)
		}
		// nestedA.stream().flatMap(Collection::stream) / List::stream —
		// peel one collection nesting level (List<List<A>> → A).
		if javaIsCollectionStreamMethodRef(first, content) {
			return javaFlatMapCollectionStreamElemType(recv, content, elemOf)
		}
		// Stream.of(oa).flatMap(Optional::stream) / List<Optional<A>>.stream()
		// .flatMap(Optional::stream) — peel Optional to element T.
		if javaIsOptionalStreamMethodRef(first, content) {
			return javaFlatMapOptionalStreamElemType(recv, content, elemOf, valOf)
		}
		return ""
	case "lambda_expression":
		body := ingest.ChildByField(first, "body")
		if body == nil {
			return ""
		}
		params := javaInferredLambdaParamNames(first, content)
		param := ""
		if len(params) == 1 {
			param = params[0]
		}
		return javaFlatMapMapperBodyElemType(body, param, recv, content, elemOf, valOf)
	default:
		return ""
	}
}

// javaIsCollectionStreamMethodRef reports Collection::stream / List::stream /
// Iterable::stream / Set::stream (and common impl type names).
func javaIsCollectionStreamMethodRef(ref *grammar.Node, content []byte) bool {
	if ref == nil || ref.Type() != "method_reference" {
		return false
	}
	var parts []*grammar.Node
	for i := uint32(0); i < ref.ChildCount(); i++ {
		ch := ref.Child(i)
		if ch.Type() == "::" {
			continue
		}
		parts = append(parts, ch)
	}
	if len(parts) < 2 {
		return false
	}
	obj, name := parts[0], parts[len(parts)-1]
	if (obj.Type() != "identifier" && obj.Type() != "type_identifier") || name.Type() != "identifier" {
		return false
	}
	if ingest.NodeText(name, content) != "stream" {
		return false
	}
	return javaIsCollectionTypeName(ingest.NodeText(obj, content))
}

// javaFlatMapCollectionStreamElemType recovers T after stream.flatMap(Collection::stream)
// when the stream source is a collection-of-collection of T
// (List<List<A>> nestedA → nestedA.stream().flatMap(Collection::stream) → A).
// Stored as elemOf["@nested."+src]. Unknown / non-nested sources fail closed.
func javaFlatMapCollectionStreamElemType(recv *grammar.Node, content []byte, elemOf map[string]string) string {
	if elemOf == nil {
		return ""
	}
	src := javaStreamSourceIdent(recv, content)
	if src == "" {
		return ""
	}
	return elemOf["@nested."+src]
}

// javaIsOptionalStreamMethodRef reports Optional::stream (Java 9+ Optional.stream()).
func javaIsOptionalStreamMethodRef(ref *grammar.Node, content []byte) bool {
	if ref == nil || ref.Type() != "method_reference" {
		return false
	}
	var parts []*grammar.Node
	for i := uint32(0); i < ref.ChildCount(); i++ {
		ch := ref.Child(i)
		if ch.Type() == "::" {
			continue
		}
		parts = append(parts, ch)
	}
	if len(parts) < 2 {
		return false
	}
	obj, name := parts[0], parts[len(parts)-1]
	if (obj.Type() != "identifier" && obj.Type() != "type_identifier") || name.Type() != "identifier" {
		return false
	}
	return ingest.NodeText(obj, content) == "Optional" && ingest.NodeText(name, content) == "stream"
}

// javaFlatMapOptionalStreamElemType recovers T after stream.flatMap(Optional::stream)
// / flatMap(o -> o.stream()) when the stream carries Optional of T:
//
//	Stream.of(oa).flatMap(Optional::stream) with Optional<A> oa → A
//	Stream.of(Optional.of(new A())).flatMap(Optional::stream) → A
//	as.stream().flatMap(Optional::stream) with List<Optional<A>> as → A
//
// Unknown / non-Optional stream sources fail closed.
func javaFlatMapOptionalStreamElemType(recv *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if recv == nil {
		return ""
	}
	// Peel type-preserving stream stages (filter/sorted/…) to the root source.
	for recv != nil && !recv.IsNull() && recv.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(recv, "expression")
		if inner == nil {
			for i := uint32(0); i < recv.ChildCount(); i++ {
				ch := recv.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		recv = inner
	}
	if recv == nil || recv.IsNull() {
		return ""
	}
	if recv.Type() == "method_invocation" {
		nameN := ingest.ChildByField(recv, "name")
		if nameN == nil {
			return ""
		}
		name := ingest.NodeText(nameN, content)
		switch name {
		case "filter", "peek", "sorted", "distinct", "limit", "skip",
			"unordered", "sequential", "parallel", "onClose", "takeWhile", "dropWhile":
			return javaFlatMapOptionalStreamElemType(ingest.ChildByField(recv, "object"), content, elemOf, valOf)
		case "stream", "parallelStream":
			// as.stream() with List<Optional<A>> as — nested A via @nested.
			if elemOf != nil {
				src := javaStreamSourceIdent(recv, content)
				if src != "" {
					if nest := elemOf["@nested."+src]; nest != "" {
						return nest
					}
				}
			}
			return ""
		case "of", "ofNullable":
			// Stream.of(oa) / Stream.ofNullable(oa) / Stream.of(Optional.of(...)).
			obj := ingest.ChildByField(recv, "object")
			if javaStaticFactoryReceiverName(obj, content) != "Stream" {
				return ""
			}
			arg := javaFirstCallArg(recv)
			if arg == nil {
				return ""
			}
			return javaOptionalStreamArgElemType(arg, content, elemOf, valOf)
		default:
			return ""
		}
	}
	// Bare stream local: only when nested Optional elem was recorded (rare).
	if recv.Type() == "identifier" && elemOf != nil {
		if nest := elemOf["@nested."+ingest.NodeText(recv, content)]; nest != "" {
			return nest
		}
	}
	return ""
}

// javaOptionalStreamArgElemType recovers T from an Optional-of-T expression used
// as a Stream.of / ofNullable argument (or similar Optional carrier).
func javaOptionalStreamArgElemType(arg *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if arg == nil {
		return ""
	}
	for arg != nil && !arg.IsNull() && arg.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(arg, "expression")
		if inner == nil {
			for i := uint32(0); i < arg.ChildCount(); i++ {
				ch := arg.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		arg = inner
	}
	if arg == nil || arg.IsNull() {
		return ""
	}
	switch arg.Type() {
	case "identifier":
		// Optional<A> oa tracked as elemOf[oa] = "A".
		if elemOf == nil {
			return ""
		}
		return elemOf[ingest.NodeText(arg, content)]
	case "method_invocation":
		nameN := ingest.ChildByField(arg, "name")
		if nameN == nil {
			return ""
		}
		name := ingest.NodeText(nameN, content)
		switch name {
		case "of", "ofNullable":
			obj := ingest.ChildByField(arg, "object")
			if javaStaticFactoryReceiverName(obj, content) != "Optional" {
				return ""
			}
			// Optional.of(new A()) / ofNullable(new A()).
			if et := javaStaticCollectionOfElemType(arg, content, name); et != "" {
				return et
			}
			// Optional.of(a) / ofNullable(a) when a is a typed local.
			if inner := javaFirstCallArg(arg); inner != nil && inner.Type() == "identifier" && elemOf != nil {
				return elemOf[ingest.NodeText(inner, content)]
			}
			return ""
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaCollectionOfOptionalElemType recovers T from List/Collection/Set of Optional of T
// (one nesting level). List<Optional<A>> → "A"; List<A> / Optional / multi-arg fail closed.
func javaCollectionOfOptionalElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil || typeN.Type() != "generic_type" {
		return ""
	}
	outer := javaTypeName(typeN, content)
	if !javaIsCollectionTypeName(outer) {
		return ""
	}
	inner := javaFirstTypeArgNode(typeN)
	if inner == nil || inner.Type() != "generic_type" {
		return ""
	}
	if javaTypeName(inner, content) != "Optional" {
		return ""
	}
	args := javaTypeArgNames(inner, content)
	if len(args) != 1 || args[0] == "" {
		return ""
	}
	return args[0]
}

// javaStreamSourceIdent peels a type-preserving stream pipeline down to the
// root collection/stream identifier (nestedA.stream().filter(...).sorted() → nestedA).
// Non-identifier sources fail closed.
func javaStreamSourceIdent(obj *grammar.Node, content []byte) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		return ingest.NodeText(obj, content)
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		// Type-preserving stages only — same set as pipeline peels that keep elem type.
		switch ingest.NodeText(nameN, content) {
		case "stream", "parallelStream", "filter", "peek", "sorted", "distinct",
			"limit", "skip", "unordered", "sequential", "parallel", "onClose",
			"takeWhile", "dropWhile":
			return javaStreamSourceIdent(ingest.ChildByField(obj, "object"), content)
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaIsOptionalOfMethodRef reports Optional::of / Optional::ofNullable.
func javaIsOptionalOfMethodRef(ref *grammar.Node, content []byte) bool {
	if ref == nil || ref.Type() != "method_reference" {
		return false
	}
	var parts []*grammar.Node
	for i := uint32(0); i < ref.ChildCount(); i++ {
		ch := ref.Child(i)
		if ch.Type() == "::" {
			continue
		}
		parts = append(parts, ch)
	}
	if len(parts) < 2 {
		return false
	}
	obj, name := parts[0], parts[len(parts)-1]
	if (obj.Type() != "identifier" && obj.Type() != "type_identifier") || name.Type() != "identifier" {
		return false
	}
	if ingest.NodeText(obj, content) != "Optional" {
		return false
	}
	switch ingest.NodeText(name, content) {
	case "of", "ofNullable":
		return true
	default:
		return false
	}
}

// javaIsCompletedFutureMethodRef reports CompletableFuture::completedFuture.
func javaIsCompletedFutureMethodRef(ref *grammar.Node, content []byte) bool {
	if ref == nil || ref.Type() != "method_reference" {
		return false
	}
	var parts []*grammar.Node
	for i := uint32(0); i < ref.ChildCount(); i++ {
		ch := ref.Child(i)
		if ch.Type() == "::" {
			continue
		}
		parts = append(parts, ch)
	}
	if len(parts) < 2 {
		return false
	}
	obj, name := parts[0], parts[len(parts)-1]
	if (obj.Type() != "identifier" && obj.Type() != "type_identifier") || name.Type() != "identifier" {
		return false
	}
	return ingest.NodeText(obj, content) == "CompletableFuture" &&
		ingest.NodeText(name, content) == "completedFuture"
}

// javaFlatMapMapperBodyElemType recovers U from an expression-bodied flatMap mapper.
func javaFlatMapMapperBodyElemType(body *grammar.Node, param string, recv *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.IsNull() {
		return ""
	}
	switch body.Type() {
	case "identifier":
		// a -> otherOpt (Optional/collection local tracked in elemOf).
		if elemOf == nil {
			return ""
		}
		return elemOf[ingest.NodeText(body, content)]
	case "method_invocation":
		nameN := ingest.ChildByField(body, "name")
		if nameN == nil {
			return ""
		}
		name := ingest.NodeText(nameN, content)
		switch name {
		case "of", "ofNullable":
			obj := ingest.ChildByField(body, "object")
			if obj == nil || (obj.Type() != "identifier" && obj.Type() != "type_identifier") {
				return ""
			}
			recvName := ingest.NodeText(obj, content)
			// Optional.flatMap rewrap: a -> Optional.of(a) / ofNullable(a)
			// Stream.flatMap rewrap: a -> Stream.of(a) / ofNullable(a)
			if recvName != "Optional" && recvName != "Stream" {
				return ""
			}
			// a -> Optional/Stream.of(a) / ofNullable(a) — same element as receiver.
			if arg := javaFirstCallArg(body); arg != nil && arg.Type() == "identifier" && param != "" && ingest.NodeText(arg, content) == param {
				return javaStreamPipelineElemType(recv, content, elemOf, valOf)
			}
			// a -> Optional/Stream.of(new A()) — element from creation args.
			return javaStaticCollectionOfElemType(body, content, name)
		case "completedFuture":
			// CompletableFuture.thenCompose rewrap:
			// a -> CompletableFuture.completedFuture(a) / completedFuture(new A()).
			obj := ingest.ChildByField(body, "object")
			if obj == nil || (obj.Type() != "identifier" && obj.Type() != "type_identifier") {
				return ""
			}
			if ingest.NodeText(obj, content) != "CompletableFuture" {
				return ""
			}
			// a -> CompletableFuture.completedFuture(a) — same element as receiver.
			if arg := javaFirstCallArg(body); arg != nil && arg.Type() == "identifier" && param != "" && ingest.NodeText(arg, content) == param {
				return javaStreamPipelineElemType(recv, content, elemOf, valOf)
			}
			// a -> CompletableFuture.completedFuture(new A()) — element from creation.
			return javaStaticCollectionOfElemType(body, content, name)
		case "stream":
			// xs -> xs.stream() — collection-of-collection flatMap peel
			// (nestedA.stream().flatMap(xs -> xs.stream()) → nested A).
			// Also Optional stream peel: Stream.of(oa).flatMap(o -> o.stream()) /
			// List<Optional<A>>.stream().flatMap(o -> o.stream()) → A.
			// Zero-arg only; receiver must be the lambda param.
			obj := ingest.ChildByField(body, "object")
			if obj == nil || obj.Type() != "identifier" || param == "" {
				return ""
			}
			if ingest.NodeText(obj, content) != param {
				return ""
			}
			if !javaCallIsZeroArg(body) {
				return ""
			}
			if et := javaFlatMapCollectionStreamElemType(recv, content, elemOf); et != "" {
				return et
			}
			return javaFlatMapOptionalStreamElemType(recv, content, elemOf, valOf)
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaFirstCallArg returns the first non-punctuation argument of a method_invocation.
func javaFirstCallArg(call *grammar.Node) *grammar.Node {
	if call == nil {
		return nil
	}
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() != "argument_list" {
			continue
		}
		al := call.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			ch := al.Child(j)
			switch ch.Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				return ch
			}
		}
	}
	return nil
}

// javaListSetCopyOfElemType recovers the element type of List.copyOf(coll) /
// Set.copyOf(coll): a Collection of the first argument's element type.
// Receiver must be List or Set; other receivers (and missing/empty args) fail closed.
func javaListSetCopyOfElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	switch javaStaticFactoryReceiverName(recvN, content) {
	case "List", "Set":
	default:
		return ""
	}
	first := javaFirstCallArg(call)
	if first == nil {
		return ""
	}
	return javaStreamPipelineElemType(first, content, elemOf, valOf)
}

// javaCollectionsNCopiesElemType recovers T from Collections.nCopies(n, new T(...)).
// Count is ignored; only the second arg's creation type matters. Non-Collections
// receivers, wrong arity, or non-creation second args fail closed.
func javaCollectionsNCopiesElemType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Collections" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) != 2 {
		return ""
	}
	arg := args[1]
	if arg.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(arg, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaCollectionsListWrapperElemType recovers the element type of
// Collections.unmodifiableList/Set/Collection(coll) /
// synchronizedList/Set/Collection(coll) / checkedList/Set/Collection(coll, type) /
// Collections.unmodifiableSortedSet/NavigableSet(coll) /
// synchronizedSortedSet/NavigableSet(coll) / checkedSortedSet/NavigableSet(coll, type) /
// Collections.unmodifiableSequencedCollection/Set(coll) /
// synchronizedSequencedCollection/Set(coll) /
// Collections.asLifoQueue(coll) / checkedQueue(coll, type) /
// Collections.newSetFromMap(map) /
// Collections.list(enumeration) / Collections.enumeration(coll).
// First arg's element type (map key type for newSetFromMap — Map stores K in
// elemOf); the Class arg on checked* is ignored.
// Non-Collections receivers fail closed.
func javaCollectionsListWrapperElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Collections" {
		return ""
	}
	first := javaFirstCallArg(call)
	if first == nil {
		return ""
	}
	return javaStreamPipelineElemType(first, content, elemOf, valOf)
}

// javaStreamConcatElemType recovers T from Stream.concat(s1, s2) when both stream
// arguments share the same recoverable element type (as.stream() / Stream.of(new A())
// / tracked stream locals). Receiver must be Stream; wrong arity, unknown args, or
// mismatched element types fail closed so A vs B same-leaf renames stay isolated.
func javaStreamConcatElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Stream" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) != 2 {
		return ""
	}
	et1 := javaStreamPipelineElemType(args[0], content, elemOf, valOf)
	if et1 == "" {
		return ""
	}
	et2 := javaStreamPipelineElemType(args[1], content, elemOf, valOf)
	if et2 == "" || et1 != et2 {
		return ""
	}
	return et1
}

// javaStreamGenerateElemType recovers T from Stream.generate(() -> new T(...)).
// Supplier must be an expression-bodied lambda whose body is object creation.
// Blocks, method refs, and non-creation bodies fail closed.
func javaStreamGenerateElemType(call *grammar.Node, content []byte) string {
	return javaStaticSupplierCreationElemType(call, content, "Stream", 1)
}

// javaCompletableFutureSupplyAsyncElemType recovers T from
// CompletableFuture.supplyAsync(() -> new T(...)) /
// supplyAsync(() -> new T(...), executor).
// First arg must be an expression-bodied supplier lambda whose body is object
// creation; optional Executor is ignored. Blocks, method refs, and non-creation
// bodies fail closed (same shapes as Stream.generate).
func javaCompletableFutureSupplyAsyncElemType(call *grammar.Node, content []byte) string {
	return javaStaticSupplierCreationElemType(call, content, "CompletableFuture", 2)
}

// javaCompleteAsyncSupplierElemType recovers T from instance
// completeAsync(() -> new T(...)) / completeAsync(() -> new T(...), executor)
// when the CF receiver has no recoverable type arg (diamond/raw
// new CompletableFuture<>()). First arg must be an expression-bodied zero-arg
// supplier lambda whose body is object creation; optional Executor is ignored.
// Blocks, method refs, and non-creation bodies fail closed (same shapes as
// supplyAsync / Stream.generate).
func javaCompleteAsyncSupplierElemType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) < 1 || len(args) > 2 || args[0].Type() != "lambda_expression" {
		return ""
	}
	// Zero-arg Supplier only.
	if n := javaInferredLambdaParamNames(args[0], content); len(n) != 0 {
		return ""
	}
	body := ingest.ChildByField(args[0], "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(body, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaThreadLocalWithInitialElemType recovers T from
// ThreadLocal.withInitial(() -> new T(...)).
// Same supplier-body shapes as Stream.generate (expression-bodied zero-arg
// lambda with object-creation body only).
func javaThreadLocalWithInitialElemType(call *grammar.Node, content []byte) string {
	return javaStaticSupplierCreationElemType(call, content, "ThreadLocal", 1)
}

// javaExecutorSubmitCallableElemType recovers V from
// executor.submit(() -> new T(...)) when the first arg is an expression-bodied
// zero-arg lambda whose body is object creation (Callable shape).
// Runnable submit / multi-arg forms / blocks / method refs fail closed.
// Enables submit(() -> new A()).get().m() under foreign same-leaf methods.
func javaExecutorSubmitCallableElemType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) != 1 || args[0].Type() != "lambda_expression" {
		return ""
	}
	// Zero-arg Callable only (inferred/formal params empty).
	if n := javaInferredLambdaParamNames(args[0], content); len(n) != 0 {
		return ""
	}
	body := ingest.ChildByField(args[0], "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(body, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaCollectionCopyCreationElemType recovers E from Collection copy constructors:
//
//	new ArrayList<>(List.of(new A())) / new LinkedList<>(as) / new HashSet<>(coll)
//	new ArrayList<A>(…) / new LinkedList<A>(…)
//
// Prefers declared single type arg; diamond peels the first constructor arg as a
// stream/collection pipeline element (javaStreamPipelineElemType). Map/multi-arg
// ctors and unknown types fail closed so new A() stays on the direct rename path.
func javaCollectionCopyCreationElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(call, "type")
	if typeN == nil {
		return ""
	}
	leaf := javaTypeName(typeN, content)
	if !javaIsCollectionCopyCtorType(leaf) {
		return ""
	}
	if args := javaTypeArgNames(typeN, content); len(args) == 1 && args[0] != "" {
		return args[0]
	}
	ctorArgs := javaCallArgs(call)
	if len(ctorArgs) < 1 {
		return ""
	}
	// First arg is the source collection (capacity-int overloads fail closed when
	// the arg is not a typed pipeline).
	return javaStreamPipelineElemType(ctorArgs[0], content, elemOf, valOf)
}

// javaIsCollectionCopyCtorType reports concrete Collection types whose single-arg
// copy constructor preserves element type E (not Map; not raw Object).
func javaIsCollectionCopyCtorType(leaf string) bool {
	switch leaf {
	case "ArrayList", "LinkedList", "Vector", "Stack",
		"HashSet", "LinkedHashSet", "TreeSet", "ConcurrentSkipListSet",
		"ArrayDeque", "PriorityQueue", "ConcurrentLinkedQueue", "ConcurrentLinkedDeque",
		"CopyOnWriteArrayList", "CopyOnWriteArraySet",
		"LinkedBlockingQueue", "LinkedBlockingDeque", "ArrayBlockingQueue",
		"PriorityBlockingQueue", "DelayQueue", "SynchronousQueue":
		return true
	default:
		return false
	}
}

// javaMapCopyCreationValueType recovers V from Map copy constructors:
//
//	new HashMap<>(Map.of("k", new A())) / new LinkedHashMap<>(m) / new TreeMap<>(m)
//	new HashMap<K,V>(…) — V from declared type args
//
// Prefers declared second type arg; diamond peels the first constructor arg as a
// map pipeline value (javaMapPipelineValueType). Non-map types / empty ctors fail
// closed so collection copy ctors stay on the element path.
func javaMapCopyCreationValueType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(call, "type")
	if typeN == nil {
		return ""
	}
	leaf := javaTypeName(typeN, content)
	if !javaIsMapCopyCtorType(leaf) {
		return ""
	}
	if args := javaTypeArgNames(typeN, content); len(args) >= 2 && args[1] != "" {
		return args[1]
	}
	ctorArgs := javaCallArgs(call)
	if len(ctorArgs) < 1 {
		return ""
	}
	// First arg is the source map (capacity/load-factor overloads fail closed when
	// the arg is not a typed map pipeline).
	return javaMapPipelineValueType(ctorArgs[0], content, elemOf, valOf)
}

// javaIsMapCopyCtorType reports concrete Map types whose Map-copy constructor
// preserves value type V.
func javaIsMapCopyCtorType(leaf string) bool {
	switch leaf {
	case "HashMap", "LinkedHashMap", "TreeMap", "WeakHashMap", "IdentityHashMap",
		"Hashtable", "ConcurrentHashMap", "ConcurrentSkipListMap",
		"EnumMap":
		return true
	default:
		return false
	}
}

// javaStaticSupplierCreationElemType recovers T from a static factory whose first
// argument is () -> new T(...) (expression-bodied). recvWant is the simple
// receiver leaf (Stream / CompletableFuture); maxArgs is 1 for generate and 2 for
// supplyAsync (optional Executor). Other arities / receivers / bodies fail closed.
func javaStaticSupplierCreationElemType(call *grammar.Node, content []byte, recvWant string, maxArgs int) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != recvWant {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) < 1 || len(args) > maxArgs || args[0].Type() != "lambda_expression" {
		return ""
	}
	body := ingest.ChildByField(args[0], "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(body, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaStreamIterateElemType recovers T from
// Stream.iterate(new T(...), UnaryOperator) /
// Stream.iterate(new T(...), Predicate, UnaryOperator).
// Seed must be object creation; operator/predicate args are not inspected
// (common identity / same-type product case). Non-Stream receivers, wrong arity,
// or non-creation seeds fail closed.
func javaStreamIterateElemType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Stream" {
		return ""
	}
	args := javaCallArgs(call)
	// iterate(seed, f) or iterate(seed, hasNext, next) — 2 or 3 args.
	if len(args) < 2 || len(args) > 3 {
		return ""
	}
	seed := args[0]
	if seed.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(seed, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaIsToListOrSetCollector reports Stream.collect args that produce a
// Collection of the stream element type: Collectors.toList()/toSet()/
// toUnmodifiableList()/toUnmodifiableSet()/toCollection(supplier),
// toList()/toSet()/toUnmodifiableList()/toUnmodifiableSet()/toCollection(supplier)
// (static import), Collectors::toList / toSet / toUnmodifiableList /
// toUnmodifiableSet, Collectors.collectingAndThen(toList()/toSet()/
// toUnmodifiable…/toCollection(…), finisher) (finisher is treated as preserving
// Collection of the same element type — identity, Collections::unmodifiableList,
// …), Collectors.filtering(pred, toList()/…) (predicate does not change element
// type), Collectors.mapping(a -> a, toList()/…) when the mapper is identity,
// Collectors.flatMapping(a -> Stream.of(a), toList()/…) when the mapper is a
// Stream.of / Stream.ofNullable rewrap (or Stream::of / ofNullable), and
// Collectors.teeing(d1, d2, merger) when the merger clearly returns a
// toList/toSet/toUnmodifiable/toCollection downstream result. Other collectors
// fail closed.
func javaIsToListOrSetCollector(collectCall *grammar.Node, content []byte) bool {
	if collectCall == nil {
		return false
	}
	first := javaCollectFirstArg(collectCall)
	if first == nil {
		return false
	}
	if javaIsToListOrSetCollectorExpr(first, content) {
		return true
	}
	// collectingAndThen(downstream, finisher) when downstream is toList/toSet/
	// toUnmodifiableList/toUnmodifiableSet.
	if javaIsCollectingAndThenToListOrSet(first, content) {
		return true
	}
	// filtering(pred, downstream) when downstream is toList/toSet/…
	if javaIsFilteringToListOrSet(first, content) {
		return true
	}
	// mapping(a -> a, downstream) when mapper is identity and downstream is toList/toSet/…
	if javaIsMappingIdentityToListOrSet(first, content) {
		return true
	}
	// flatMapping(a -> Stream.of(a), downstream) when mapper rewraps T as Stream<T>
	// and downstream is toList/toSet/…
	if javaIsFlatMappingStreamOfToListOrSet(first, content) {
		return true
	}
	// teeing(d1, d2, merger) when merger returns a toList/toSet/toUnmodifiable downstream.
	return javaIsTeeingToListOrSet(first, content)
}

// javaIsToListOrSetCollectorExpr reports a collector expression that yields a
// Collection of the stream element type: toList()/toSet()/toUnmodifiableList()/
// toUnmodifiableSet(), Collectors.toList()/toSet()/toUnmodifiableList()/
// toUnmodifiableSet(), Collectors::toList / toSet / toUnmodifiableList /
// toUnmodifiableSet, and toCollection(supplier) / Collectors.toCollection(supplier)
// (exactly one supplier arg; supplier form is not inspected).
func javaIsToListOrSetCollectorExpr(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "method_invocation":
		// toList()/toSet()/toUnmodifiableList()/toUnmodifiableSet() /
		// Collectors.toList()/toSet()/toUnmodifiableList()/toUnmodifiableSet()
		// — zero-arg only.
		// toCollection(supplier) / Collectors.toCollection(supplier) — one arg.
		nameN := ingest.ChildByField(n, "name")
		if nameN == nil {
			return false
		}
		name := ingest.NodeText(nameN, content)
		var wantArgs int
		switch name {
		case "toList", "toSet", "toUnmodifiableList", "toUnmodifiableSet":
			wantArgs = 0
		case "toCollection":
			// Collection of stream element type; supplier only picks the concrete
			// Collection implementation (ArrayList::new, HashSet::new, …).
			wantArgs = 1
		default:
			return false
		}
		if obj := ingest.ChildByField(n, "object"); obj != nil {
			if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
				return false
			}
			if ingest.NodeText(obj, content) != "Collectors" {
				return false
			}
		}
		args := 0
		for i := uint32(0); i < n.ChildCount(); i++ {
			if n.Child(i).Type() != "argument_list" {
				continue
			}
			al := n.Child(i)
			for j := uint32(0); j < al.ChildCount(); j++ {
				switch al.Child(j).Type() {
				case "(", ")", "comment":
					continue
				default:
					args++
				}
			}
		}
		return args == wantArgs
	case "method_reference":
		// Collectors::toList / toSet / toUnmodifiableList / toUnmodifiableSet —
		// children are receiver, "::", name. Collectors::toCollection alone is
		// not a Collector (needs a supplier) so it fails closed here.
		var parts []*grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			child := n.Child(i)
			if child.Type() == "::" {
				continue
			}
			parts = append(parts, child)
		}
		if len(parts) < 2 {
			return false
		}
		obj, name := parts[0], parts[len(parts)-1]
		if name.Type() != "identifier" {
			return false
		}
		switch ingest.NodeText(name, content) {
		case "toList", "toSet", "toUnmodifiableList", "toUnmodifiableSet":
		default:
			return false
		}
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		return ingest.NodeText(obj, content) == "Collectors"
	default:
		return false
	}
}

// javaIsCollectingAndThenToListOrSet reports
// Collectors.collectingAndThen(toList()/toSet()/toUnmodifiableList()/
// toUnmodifiableSet()/toCollection(…)/Collectors::toList/…, finisher) /
// collectingAndThen(...) (static import). Downstream must itself be a
// toList/toSet/toUnmodifiable/toCollection collector; finisher is not inspected
// (fail-open for element preservation, matching common unmodifiableList /
// identity finishers).
func javaIsCollectingAndThenToListOrSet(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "collectingAndThen" {
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	return javaIsToListOrSetCollectorExpr(javaFirstCallArg(n), content)
}

// javaIsFilteringToListOrSet reports Collectors.filtering(pred, downstream) /
// filtering(...) (static import) when downstream is a toList/toSet/
// toUnmodifiableList/toUnmodifiableSet/toCollection collector. The predicate
// does not change the stream element type. Other downstreams fail closed.
func javaIsFilteringToListOrSet(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "filtering" {
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	args := javaCallArgs(n)
	if len(args) != 2 {
		return false
	}
	// args[0] is the predicate — not inspected (does not change element type).
	return javaIsToListOrSetCollectorExpr(args[1], content)
}

// javaIsMappingIdentityToListOrSet reports Collectors.mapping(mapper, downstream) /
// mapping(...) (static import) when mapper is an identity lambda (a -> a) and
// downstream is a toList/toSet/toUnmodifiableList/toUnmodifiableSet/toCollection
// collector. Type-changing mappers and method-ref mappers fail closed.
func javaIsMappingIdentityToListOrSet(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "mapping" {
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	args := javaCallArgs(n)
	if len(args) != 2 {
		return false
	}
	if !javaIsIdentityLambda(args[0], content) {
		return false
	}
	return javaIsToListOrSetCollectorExpr(args[1], content)
}

// javaIsFlatMappingStreamOfToListOrSet reports Collectors.flatMapping(mapper, downstream) /
// flatMapping(...) (static import) when mapper rewraps the stream element as
// Stream.of(a) / Stream.ofNullable(a) / Stream::of / Stream::ofNullable and
// downstream is a toList/toSet/toUnmodifiableList/toUnmodifiableSet/toCollection
// collector. Type-changing flat mappers and blocks fail closed.
func javaIsFlatMappingStreamOfToListOrSet(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "flatMapping" {
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	args := javaCallArgs(n)
	if len(args) != 2 {
		return false
	}
	if !javaIsStreamOfRewrapMapper(args[0], content) {
		return false
	}
	return javaIsToListOrSetCollectorExpr(args[1], content)
}

// javaIsStreamOfRewrapMapper reports flatMapping mappers that rewrap T as Stream<T>:
//
//	a -> Stream.of(a) / Stream.ofNullable(a)
//	Stream::of / Stream::ofNullable
//
// Blocks, multi-param forms, and other mappers fail closed.
func javaIsStreamOfRewrapMapper(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "method_reference":
		return javaIsStreamOfMethodRef(n, content)
	case "lambda_expression":
		params := javaInferredLambdaParamNames(n, content)
		if len(params) != 1 {
			return false
		}
		body := ingest.ChildByField(n, "body")
		for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
			inner := ingest.ChildByField(body, "expression")
			if inner == nil {
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					if ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					inner = ch
					break
				}
			}
			body = inner
		}
		if body == nil || body.IsNull() || body.Type() != "method_invocation" {
			return false
		}
		nameN := ingest.ChildByField(body, "name")
		if nameN == nil {
			return false
		}
		switch ingest.NodeText(nameN, content) {
		case "of", "ofNullable":
		default:
			return false
		}
		obj := ingest.ChildByField(body, "object")
		if obj == nil || (obj.Type() != "identifier" && obj.Type() != "type_identifier") {
			return false
		}
		if ingest.NodeText(obj, content) != "Stream" {
			return false
		}
		arg := javaFirstCallArg(body)
		return arg != nil && arg.Type() == "identifier" && ingest.NodeText(arg, content) == params[0]
	default:
		return false
	}
}

// javaIsStreamOfMethodRef reports Stream::of / Stream::ofNullable.
func javaIsStreamOfMethodRef(ref *grammar.Node, content []byte) bool {
	if ref == nil || ref.Type() != "method_reference" {
		return false
	}
	var parts []*grammar.Node
	for i := uint32(0); i < ref.ChildCount(); i++ {
		ch := ref.Child(i)
		if ch.Type() == "::" {
			continue
		}
		parts = append(parts, ch)
	}
	if len(parts) < 2 {
		return false
	}
	obj, name := parts[0], parts[len(parts)-1]
	if (obj.Type() != "identifier" && obj.Type() != "type_identifier") || name.Type() != "identifier" {
		return false
	}
	if ingest.NodeText(obj, content) != "Stream" {
		return false
	}
	switch ingest.NodeText(name, content) {
	case "of", "ofNullable":
		return true
	default:
		return false
	}
}

// javaIsIdentityLambda reports expression-bodied unary lambdas of the form a -> a
// (optionally parenthesized body). Blocks and multi-param forms fail closed.
func javaIsIdentityLambda(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "lambda_expression" {
		return false
	}
	params := javaInferredLambdaParamNames(n, content)
	if len(params) != 1 {
		return false
	}
	return javaLambdaBodyIsIdent(n, content, params[0])
}

// javaIsIdentityMapCall reports stream.map(a -> a) / optional.map(a -> a) with a
// sole unary identity lambda. Used by method-return object peels so
// Stream.of(ba.get()).map(x -> x).findFirst().get() preserves T under foreign
// same-leaf (non-identity mappers fail closed).
func javaIsIdentityMapCall(call *grammar.Node, content []byte) bool {
	if call == nil || call.Type() != "method_invocation" {
		return false
	}
	if name := javaMethodInvocationName(call, content); name != "map" {
		return false
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return false
	}
	var first *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," || ch.Type() == "comment" {
			continue
		}
		count++
		if first == nil {
			first = ch
		}
	}
	return count == 1 && javaIsIdentityLambda(first, content)
}

// javaIsIdentityFlatMapRewrapCall reports flatMap mappers that rewrap T as
// Stream/Optional of the same T (type-preserving under foreign same-leaf):
//
//	x -> Stream.of(x) / Stream.ofNullable(x)
//	x -> Optional.of(x) / Optional.ofNullable(x)
//	Stream::of / Stream::ofNullable
//	Optional::of / Optional::ofNullable
//
// Used by method-return object peels so Stream.of(ba.get()).flatMap(x -> Stream.of(x))
// and Optional.of(ba.get()).flatMap(x -> Optional.of(x)) preserve T (Class peels
// already work via javaStreamPipelineElemType). Blocks / other mappers fail closed.
func javaIsIdentityFlatMapRewrapCall(call *grammar.Node, content []byte) bool {
	if call == nil || call.Type() != "method_invocation" {
		return false
	}
	if name := javaMethodInvocationName(call, content); name != "flatMap" {
		return false
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return false
	}
	var first *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," || ch.Type() == "comment" {
			continue
		}
		count++
		if first == nil {
			first = ch
		}
	}
	if count != 1 || first == nil {
		return false
	}
	switch first.Type() {
	case "method_reference":
		return javaIsOptionalOfMethodRef(first, content) || javaIsStreamOfMethodRef(first, content)
	case "lambda_expression":
		params := javaInferredLambdaParamNames(first, content)
		if len(params) != 1 {
			return false
		}
		param := params[0]
		body := ingest.ChildByField(first, "body")
		for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
			inner := ingest.ChildByField(body, "expression")
			if inner == nil {
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					if ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					inner = ch
					break
				}
			}
			body = inner
		}
		if body == nil || body.IsNull() || body.Type() != "method_invocation" {
			return false
		}
		nameN := ingest.ChildByField(body, "name")
		if nameN == nil {
			return false
		}
		name := ingest.NodeText(nameN, content)
		if name != "of" && name != "ofNullable" {
			return false
		}
		obj := ingest.ChildByField(body, "object")
		if obj == nil || (obj.Type() != "identifier" && obj.Type() != "type_identifier") {
			return false
		}
		recvName := ingest.NodeText(obj, content)
		if recvName != "Optional" && recvName != "Stream" {
			return false
		}
		arg := javaFirstCallArg(body)
		return arg != nil && arg.Type() == "identifier" && ingest.NodeText(arg, content) == param
	default:
		return false
	}
}

// javaIsValueIdentityBiLambda reports expression-bodied bi-lambdas that return
// their second (value) parameter: (k, v) -> v. Used to recover U=V from
// ConcurrentHashMap.search identity BiFunctions. Blocks / other bodies fail closed.
func javaIsValueIdentityBiLambda(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "lambda_expression" {
		return false
	}
	params := javaInferredLambdaParamNames(n, content)
	if len(params) != 2 {
		return false
	}
	return javaLambdaBodyIsIdent(n, content, params[1])
}

// javaLambdaBodyIsIdent reports expression-bodied lambdas whose body is the
// given identifier (after peeling parentheses).
func javaLambdaBodyIsIdent(n *grammar.Node, content []byte, want string) bool {
	body := ingest.ChildByField(n, "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.IsNull() || body.Type() != "identifier" {
		return false
	}
	return ingest.NodeText(body, content) == want
}

// javaIsTeeingToListOrSet reports Collectors.teeing(d1, d2, merger) /
// teeing(...) (static import) when merger is an expression-bodied bi-lambda
// that returns one of its parameters and that parameter's downstream is a
// toList/toSet collector:
//
//	teeing(toList(), counting(), (list, n) -> list) → List<T>
//	teeing(toList(), toList(), (a, b) -> a) → List<T>
//	teeing(counting(), toList(), (n, list) -> list) → List<T>
//
// Method-ref mergers and non-identity bodies fail closed (result type is the
// merger's R, not automatically a Collection of the stream element).
func javaIsTeeingToListOrSet(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "teeing" {
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	args := javaCallArgs(n)
	if len(args) != 3 {
		return false
	}
	d1, d2, merger := args[0], args[1], args[2]
	if merger.Type() != "lambda_expression" {
		return false
	}
	params := javaInferredLambdaParamNames(merger, content)
	if len(params) != 2 {
		return false
	}
	body := ingest.ChildByField(merger, "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.IsNull() || body.Type() != "identifier" {
		return false
	}
	ret := ingest.NodeText(body, content)
	switch ret {
	case params[0]:
		return javaIsToListOrSetCollectorExpr(d1, content)
	case params[1]:
		return javaIsToListOrSetCollectorExpr(d2, content)
	default:
		return false
	}
}

// javaCallArgs returns non-punctuation arguments of a method_invocation.
func javaCallArgs(call *grammar.Node) []*grammar.Node {
	if call == nil {
		return nil
	}
	var out []*grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() != "argument_list" {
			continue
		}
		al := call.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			ch := al.Child(j)
			switch ch.Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				out = append(out, ch)
			}
		}
	}
	return out
}

// javaCollectFirstArg returns the first non-punctuation argument of a collect(...) call.
func javaCollectFirstArg(collectCall *grammar.Node) *grammar.Node {
	if collectCall == nil {
		return nil
	}
	var args *grammar.Node
	for i := uint32(0); i < collectCall.ChildCount(); i++ {
		if collectCall.Child(i).Type() == "argument_list" {
			args = collectCall.Child(i)
			break
		}
	}
	if args == nil {
		return nil
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			return ch
		}
	}
	return nil
}

// javaIsGroupingByCollector reports Stream.collect args that yield Map of
// List/Collection groups of the stream element type:
//
//	Collectors.groupingBy(classifier) / groupingBy(classifier)
//	groupingBy(classifier, toList()/toSet()/toUnmodifiable…/toCollection(…))
//	groupingBy(classifier, mapFactory, toList()/…)
//	partitioningBy(predicate) / partitioningBy(pred, toList()/…) /
//	partitioningBy(pred, mapFactory, toList()/…)
//	collectingAndThen(groupingBy(…)/partitioningBy(…), finisher)
//
// Downstream must preserve Collection of T (toList/toSet/toUnmodifiable/
// toCollection); counting/mapping/reducing/… fail closed. Finisher of
// collectingAndThen is not inspected (same fail-open as toList collectingAndThen).
func javaIsGroupingByCollector(collectCall *grammar.Node, content []byte) bool {
	first := javaCollectFirstArg(collectCall)
	if first == nil {
		return false
	}
	if javaIsGroupingByCollectorExpr(first, content) {
		return true
	}
	// collectingAndThen(groupingBy|partitioningBy(…), finisher) — finisher
	// treated as map-preserving (unmodifiableMap / identity / …).
	return javaIsCollectingAndThenGroupingBy(first, content)
}

// javaIsCollectingAndThenGroupingBy reports
// Collectors.collectingAndThen(groupingBy|partitioningBy(…), finisher) /
// collectingAndThen(...) (static import) when the downstream is a groupingBy/
// partitioningBy collector of Collection groups of T.
func javaIsCollectingAndThenGroupingBy(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "collectingAndThen" {
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	// collectingAndThen(downstream, finisher) — require exactly two real args.
	var args []*grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.Child(i).Type() != "argument_list" {
			continue
		}
		al := n.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			ch := al.Child(j)
			switch ch.Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				args = append(args, ch)
			}
		}
	}
	if len(args) != 2 {
		return false
	}
	return javaIsGroupingByCollectorExpr(args[0], content)
}

// javaIsGroupingByCollectorExpr reports a groupingBy/partitioningBy collector
// expression (not the outer collect call) whose value groups are Collection of
// the stream element type. See javaIsGroupingByCollector.
func javaIsGroupingByCollectorExpr(first *grammar.Node, content []byte) bool {
	if first == nil || first.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(first, "name")
	if nameN == nil {
		return false
	}
	switch ingest.NodeText(nameN, content) {
	case "groupingBy", "partitioningBy":
	default:
		return false
	}
	if obj := ingest.ChildByField(first, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	// Real args: classifier[/predicate] [, mapFactory] [, downstream].
	var args []*grammar.Node
	for i := uint32(0); i < first.ChildCount(); i++ {
		if first.Child(i).Type() != "argument_list" {
			continue
		}
		al := first.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			ch := al.Child(j)
			switch ch.Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				args = append(args, ch)
			}
		}
	}
	switch len(args) {
	case 1:
		// Default downstream is toList() — Map of List<T>.
		return true
	case 2:
		// classifier/predicate + downstream (or 2-arg with non-list fails closed).
		return javaIsListGroupDownstream(args[1], content)
	case 3:
		// classifier/predicate + mapFactory + downstream.
		return javaIsListGroupDownstream(args[2], content)
	default:
		return false
	}
}

// javaIsListGroupDownstream reports a groupingBy/partitioningBy downstream
// collector that yields Collection of the stream element type: toList/toSet/
// toUnmodifiable/toCollection, plus identity-preserving wrappers
// (collectingAndThen / filtering / mapping / flatMapping Stream.of) that the
// toList collect path already accepts. counting / reducing / summing fail closed.
func javaIsListGroupDownstream(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	if javaIsToListOrSetCollectorExpr(n, content) {
		return true
	}
	if javaIsCollectingAndThenToListOrSet(n, content) {
		return true
	}
	if javaIsFilteringToListOrSet(n, content) {
		return true
	}
	if javaIsMappingIdentityToListOrSet(n, content) {
		return true
	}
	return javaIsFlatMappingStreamOfToListOrSet(n, content)
}

// javaGroupingByCollectElemType recovers T from stream.collect(groupingBy(classifier))
// / collect(partitioningBy(predicate)). Result is Map of List<T>; T is the stream
// element type.
func javaGroupingByCollectElemType(val *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		val = inner
	}
	if val == nil || val.IsNull() || val.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(val, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "collect" {
		return ""
	}
	if !javaIsGroupingByCollector(val, content) {
		return ""
	}
	return javaStreamPipelineElemType(ingest.ChildByField(val, "object"), content, elemOf, valOf)
}

// javaGroupingByMapGroupElemType recovers T when obj is a groupingBy/partitioningBy
// map expression: m (tracked in groupValOf) or stream.collect(groupingBy/partitioningBy(...)).
func javaGroupingByMapGroupElemType(obj *grammar.Node, content []byte, elemOf, valOf, groupValOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		if groupValOf == nil {
			return ""
		}
		return groupValOf[ingest.NodeText(obj, content)]
	case "method_invocation":
		return javaGroupingByCollectElemType(obj, content, elemOf, valOf)
	default:
		return ""
	}
}

// javaGroupingByValuesGroupElemType recovers T from groupingBy/partitioningBy map
// .values(): m.values() / collect(groupingBy|partitioningBy).values() → T
// (element of each value List).
func javaGroupingByValuesGroupElemType(obj *grammar.Node, content []byte, elemOf, valOf, groupValOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() || obj.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(obj, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "values" {
		return ""
	}
	return javaGroupingByMapGroupElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf, groupValOf)
}

// javaGroupingByMapGetElemType recovers T from m.get(k) / m.getOrDefault when m is
// a groupingBy/partitioningBy map (value is List<T>), including inline
// stream.collect(groupingBy/partitioningBy(...)).get(k). Used for:
// var g = m.get(k) (g is List of T via elemOf) and chained m.get(k).get(0).m().
func javaGroupingByMapGetElemType(val *grammar.Node, content []byte, elemOf, valOf, groupValOf map[string]string) string {
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		val = inner
	}
	if val == nil || val.IsNull() || val.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(val, "name")
	if nameN == nil {
		return ""
	}
	switch ingest.NodeText(nameN, content) {
	case "get", "getOrDefault":
	default:
		return ""
	}
	return javaGroupingByMapGroupElemType(ingest.ChildByField(val, "object"), content, elemOf, valOf, groupValOf)
}

// javaGroupingByEntrySetGroupElemType recovers T from a groupingBy/partitioningBy
// map entrySet pipeline: m.entrySet() / m.entrySet().stream() / filter… /
// collect(groupingBy|partitioningBy).entrySet() → T (element of each value List).
// Type-changing stages (map/flatMap) fail closed.
func javaGroupingByEntrySetGroupElemType(obj *grammar.Node, content []byte, elemOf, valOf, groupValOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() || obj.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(obj, "name")
	if nameN == nil {
		return ""
	}
	switch name := ingest.NodeText(nameN, content); name {
	case "entrySet":
		return javaGroupingByMapGroupElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf, groupValOf)
	case "stream", "parallelStream",
		"iterator", "listIterator", "descendingIterator", "spliterator",
		"filter", "peek", "sorted", "distinct", "limit", "skip",
		"unordered", "sequential", "parallel", "onClose",
		"takeWhile", "dropWhile":
		return javaGroupingByEntrySetGroupElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf, groupValOf)
	default:
		return ""
	}
}

// javaGroupingByEntryExprGroupElemType recovers T when obj is a Map.Entry whose
// value is List<T> from a groupingBy/partitioningBy map:
//
//	e                         → entryGroupOf[e]
//	m.entrySet().iterator().next() / listIterator().previous() → group T
//	m.entrySet().stream().findFirst().get() / findAny().orElseThrow() → group T
//
// Used so e.getValue().get(0).m() / findFirst().get().getValue().get(0).m() peel
// under foreign same-leaf methods. Unknown shapes fail closed.
func javaGroupingByEntryExprGroupElemType(obj *grammar.Node, content []byte, elemOf, valOf, entryGroupOf, groupValOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	switch obj.Type() {
	case "identifier", "type_identifier":
		if entryGroupOf == nil {
			return ""
		}
		return entryGroupOf[ingest.NodeText(obj, content)]
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		switch name := ingest.NodeText(nameN, content); name {
		case "next", "previous":
			// m.entrySet().iterator().next() — Entry of List<T>.
			return javaGroupingByEntrySetGroupElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf, groupValOf)
		case "get", "orElse", "orElseGet", "orElseThrow":
			// m.entrySet().stream().findFirst().get() — Optional of Entry of List<T>.
			// Zero-arg get only (Optional.get); List.get(i)/Map.get(k) fail closed.
			if name == "get" && len(javaCallArgs(obj)) != 0 {
				return ""
			}
			return javaGroupingByOptionalEntryGroupElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf, entryGroupOf, groupValOf)
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaGroupingByOptionalEntryGroupElemType recovers T when opt is an Optional of
// Map.Entry from a groupingBy/partitioningBy entrySet stream:
// m.entrySet().stream().findFirst() / findAny() / min() / max() → group T.
func javaGroupingByOptionalEntryGroupElemType(opt *grammar.Node, content []byte, elemOf, valOf, entryGroupOf, groupValOf map[string]string) string {
	for opt != nil && !opt.IsNull() && opt.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(opt, "expression")
		if inner == nil {
			for i := uint32(0); i < opt.ChildCount(); i++ {
				ch := opt.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		opt = inner
	}
	if opt == nil || opt.IsNull() || opt.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(opt, "name")
	if nameN == nil {
		return ""
	}
	switch name := ingest.NodeText(nameN, content); name {
	case "findFirst", "findAny", "min", "max":
		return javaGroupingByEntrySetGroupElemType(ingest.ChildByField(opt, "object"), content, elemOf, valOf, groupValOf)
	case "of", "ofNullable":
		// Optional.of(entry) — Entry from first arg when entry is groupingBy entry.
		recv := ingest.ChildByField(opt, "object")
		if recv == nil || javaStaticFactoryReceiverName(recv, content) != "Optional" {
			return ""
		}
		first := javaFirstCallArg(opt)
		if first == nil {
			return ""
		}
		return javaGroupingByEntryExprGroupElemType(first, content, elemOf, valOf, entryGroupOf, groupValOf)
	default:
		return ""
	}
}

// javaGroupingByEntryGetValueElemType recovers T from e.getValue() when e is a
// Map.Entry from a groupingBy/partitioningBy map (value is List<T>), including
// chained entry producers (findFirst().get().getValue()). Used for:
// var g = e.getValue() (g is List of T via elemOf), e.getValue().get(0).m(),
// e.getValue().forEach(a -> a.m()), and for (var a : e.getValue()).
func javaGroupingByEntryGetValueElemType(val *grammar.Node, content []byte, elemOf, valOf, entryGroupOf, groupValOf map[string]string) string {
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		val = inner
	}
	if val == nil || val.IsNull() || val.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(val, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "getValue" {
		return ""
	}
	return javaGroupingByEntryExprGroupElemType(ingest.ChildByField(val, "object"), content, elemOf, valOf, entryGroupOf, groupValOf)
}

// javaIsArraysReceiver reports whether obj is the Arrays type name (static call site),
// including fully-qualified java.util.Arrays.
func javaIsArraysReceiver(obj *grammar.Node, content []byte) bool {
	return javaStaticFactoryReceiverName(obj, content) == "Arrays"
}

// javaArraysStreamElemType recovers T from Arrays.stream(T[] arr[, from, to]),
// Arrays.copyOf(T[] arr, n), Arrays.copyOfRange(T[] arr, from, to), and
// Arrays.asList(T[] arr) (List of array elements when not homogeneous new T(...)).
// First argument is the array (identifier with elemOf, or new T[]{...}).
// Length / range bounds do not change the element type.
func javaArraysStreamElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil {
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			// Arrays.stream(as) / Arrays.stream(new A[]{...}) / Arrays.stream(as, 0, 1)
			// / Arrays.copyOf(as, n) / Arrays.copyOfRange(as, from, to)
			return javaStreamPipelineElemType(ch, content, elemOf, valOf)
		}
	}
	return ""
}

// javaMapPipelineKeyType recovers the key type of a Map-typed expression:
// m → elemOf[m] when m is map-like (also recorded in valOf),
// Collections.unmodifiableMap/synchronizedMap/checkedMap(m[, …]) → elemOf[m],
// Collections.unmodifiableSequencedMap/synchronizedSequencedMap(m) → elemOf[m]
// (and Sorted/Navigable wrappers; same K — dual of value path),
// Map.copyOf(m) → elemOf[m],
// m.reversed() → elemOf[m] (SequencedMap view; order only),
// m.descendingMap() → elemOf[m] (NavigableMap reverse-order view),
// m.headMap/tailMap/subMap(...) → elemOf[m] (SortedMap/NavigableMap range views;
// bounds/inclusivity args do not change the key type leaf).
// Used by keySet / navigableKeySet / descendingKeySet. Fail closed on factories
// whose keys are not statically recoverable (Map.of key slots, toMap key mappers, …).
func javaMapPipelineKeyType(obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		if elemOf == nil {
			return ""
		}
		id := ingest.NodeText(obj, content)
		// Map-like only: value type is also recorded (List has elemOf without valOf).
		if valOf == nil || valOf[id] == "" {
			return ""
		}
		return elemOf[id]
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		switch name := ingest.NodeText(nameN, content); name {
		case "unmodifiableMap", "synchronizedMap", "checkedMap",
			"unmodifiableSortedMap", "synchronizedSortedMap", "checkedSortedMap",
			"unmodifiableNavigableMap", "synchronizedNavigableMap", "checkedNavigableMap",
			"unmodifiableSequencedMap", "synchronizedSequencedMap":
			// Collections.*Map wrappers — key type of first-arg map (Class args ignored).
			return javaCollectionsMapWrapperKeyType(obj, content, elemOf, valOf)
		case "copyOf":
			// Map.copyOf(m) — key type of first-arg map (List/Set.copyOf stay in stream path).
			return javaMapCopyOfKeyType(obj, content, elemOf, valOf)
		case "reversed":
			// SequencedMap.reversed() — same key type (order only; Java 21).
			return javaMapPipelineKeyType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "descendingMap", "headMap", "tailMap", "subMap":
			// NavigableMap.descendingMap / SortedMap|NavigableMap headMap/tailMap/subMap
			// — same key type (order/bounds only; inclusivity args ignored).
			return javaMapPipelineKeyType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaCollectionsMapWrapperKeyType recovers K from
// Collections.unmodifiableMap/synchronizedMap/checkedMap(m[, keyClass, valueClass])
// (and Sorted/Navigable/Sequenced variants). First arg's map key type; Class args ignored.
// Non-Collections receivers fail closed.
func javaCollectionsMapWrapperKeyType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Collections" {
		return ""
	}
	first := javaFirstCallArg(call)
	if first == nil {
		return ""
	}
	return javaMapPipelineKeyType(first, content, elemOf, valOf)
}

// javaMapCopyOfKeyType recovers K from Map.copyOf(m): key type of the first
// argument map. Non-Map receivers fail closed (List/Set.copyOf stay on the
// element pipeline via javaListSetCopyOfElemType).
func javaMapCopyOfKeyType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Map" {
		return ""
	}
	first := javaFirstCallArg(call)
	if first == nil {
		return ""
	}
	return javaMapPipelineKeyType(first, content, elemOf, valOf)
}

// javaMapPipelineValueType recovers the value type of a Map-typed expression:
// m → valOf[m],
// stream.collect(toMap(key, a -> a[, …])) → stream element type,
// stream.collect(collectingAndThen(toMap(key, a -> a[, …]), finisher)) → same,
// Collections.unmodifiableMap/synchronizedMap/checkedMap(m[, …]) → valOf[m],
// Collections.unmodifiableSequencedMap/synchronizedSequencedMap(m) → valOf[m]
// (SequencedMap wrappers; same V — Java 21; mirrors unmodifiableSequencedSet),
// Collections.singletonMap(k, new T(...)) → T,
// Map.of(k, new T(...), …) → T (homogeneous value creations),
// Map.ofEntries(Map.entry(k, new T(...)), …) → T (homogeneous entry values),
// Map.copyOf(m) → valOf[m],
// m.clone() → valOf[m] (shallow map copy; same V — typically under a Map cast),
// (HashMap<K,V>) m.clone() / (Map<K,V>) x → V (second type-arg of generic cast;
// single-arg List casts stay on the element pipeline),
// m.reversed() → valOf[m] (SequencedMap view; order only),
// m.descendingMap() → valOf[m] (NavigableMap reverse-order view),
// m.headMap/tailMap/subMap(...) → valOf[m] (SortedMap/NavigableMap range views;
// bounds/inclusivity args do not change the value type leaf).
// Fail closed on other shapes (type-changing value mappers, groupingBy lists, …).
func javaMapPipelineValueType(obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		if valOf == nil {
			return ""
		}
		return valOf[ingest.NodeText(obj, content)]
	case "object_creation_expression":
		// new HashMap<>(Map.of("k", new A())) / new LinkedHashMap<>(m) /
		// new TreeMap<>(m) — Map copy ctor value type V (declared type args or
		// first-arg map pipeline). Enables .get(k).m() / .values().forEach /
		// var m2 = new HashMap<>(m) under foreign same-leaf methods.
		return javaMapCopyCreationValueType(obj, content, elemOf, valOf)
	case "cast_expression":
		// (HashMap<K,V>) m.clone() / (Map<K,V>) x — recover V from a multi-arg
		// generic cast. Single-arg List casts fail closed here (element path).
		// Otherwise peel the cast value so type-preserving map pipelines under
		// the cast (m.clone() when raw/unknown) still bind.
		if typeN := ingest.ChildByField(obj, "type"); typeN != nil && typeN.Type() == "generic_type" {
			if args := javaTypeArgNames(typeN, content); len(args) >= 2 {
				return args[1]
			}
		}
		if val := ingest.ChildByField(obj, "value"); val != nil {
			return javaMapPipelineValueType(val, content, elemOf, valOf)
		}
		return ""
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		switch name := ingest.NodeText(nameN, content); name {
		case "collect":
			return javaToMapCollectValueType(obj, content, elemOf, valOf)
		case "unmodifiableMap", "synchronizedMap", "checkedMap",
			"unmodifiableSortedMap", "synchronizedSortedMap", "checkedSortedMap",
			"unmodifiableNavigableMap", "synchronizedNavigableMap", "checkedNavigableMap",
			"unmodifiableSequencedMap", "synchronizedSequencedMap":
			// Collections.*Map wrappers — value type of first-arg map (Class args ignored).
			// SequencedMap wrappers (Java 21) same V; mirrors *SequencedSet element path.
			return javaCollectionsMapWrapperValueType(obj, content, elemOf, valOf)
		case "singletonMap":
			// Collections.singletonMap(k, new T(...)) — value type from second arg.
			return javaCollectionsSingletonMapValueType(obj, content)
		case "of":
			// Map.of(k, new T(...), …) — homogeneous value creations at odd slots.
			return javaMapOfValueType(obj, content)
		case "ofEntries":
			// Map.ofEntries(Map.entry(k, new T(...)), …) — homogeneous entry value types.
			return javaMapOfEntriesValueType(obj, content)
		case "copyOf":
			// Map.copyOf(m) — value type of first-arg map (List/Set.copyOf stay in stream path).
			return javaMapCopyOfValueType(obj, content, elemOf, valOf)
		case "clone":
			// m.clone() — same value type (shallow map copy; usually under a Map cast
			// because Object.clone() returns Object). List/array clone stay on the
			// element pipeline (no valOf on those receivers).
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "reversed":
			// SequencedMap.reversed() — same value type (order only; Java 21).
			// List/SequencedCollection.reversed stay on the element pipeline.
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "descendingMap", "headMap", "tailMap", "subMap":
			// NavigableMap.descendingMap / SortedMap|NavigableMap headMap/tailMap/subMap
			// — same value type (order/bounds only; inclusivity args ignored).
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaCollectionsMapWrapperValueType recovers V from
// Collections.unmodifiableMap/synchronizedMap/checkedMap(m[, keyClass, valueClass])
// (and Sorted/Navigable/Sequenced variants). First arg's map value type; Class args ignored.
// Non-Collections receivers fail closed.
func javaCollectionsMapWrapperValueType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Collections" {
		return ""
	}
	first := javaFirstCallArg(call)
	if first == nil {
		return ""
	}
	return javaMapPipelineValueType(first, content, elemOf, valOf)
}

// javaCollectionsSingletonMapValueType recovers T from
// Collections.singletonMap(k, new T(...)). Key is ignored; only the second arg's
// creation type matters. Non-Collections receivers, wrong arity, or non-creation
// value args fail closed.
func javaCollectionsSingletonMapValueType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Collections" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) != 2 {
		return ""
	}
	arg := args[1]
	if arg.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(arg, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaMapOfValueType recovers T from Map.of(k1, new T(...), k2, new T(...), …)
// when every value slot (odd index) is `new T(...)` with the same T. Empty of(),
// odd arity, non-Map receivers, and non-creation values fail closed.
func javaMapOfValueType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Map" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) < 2 || len(args)%2 != 0 {
		return ""
	}
	var elem string
	saw := false
	for i := 1; i < len(args); i += 2 {
		arg := args[i]
		if arg.Type() != "object_creation_expression" {
			return ""
		}
		typeN := ingest.ChildByField(arg, "type")
		if typeN == nil {
			return ""
		}
		tn := javaTypeName(typeN, content)
		if tn == "" {
			return ""
		}
		if !saw {
			elem = tn
			saw = true
			continue
		}
		if tn != elem {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// javaMapOfEntriesValueType recovers T from
// Map.ofEntries(Map.entry(k1, new T(...)), Map.entry(k2, new T(...)), …)
// when every entry arg is Map.entry(k, new T(...)) with the same T. Empty
// ofEntries(), non-Map receivers, non-entry args, and non-creation values fail closed.
func javaMapOfEntriesValueType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Map" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) == 0 {
		return ""
	}
	var elem string
	saw := false
	for _, arg := range args {
		tn := javaMapEntryCreationValueType(arg, content)
		if tn == "" {
			return ""
		}
		if !saw {
			elem = tn
			saw = true
			continue
		}
		if tn != elem {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// javaEntryExprValueType recovers V from a Map.Entry-producing expression:
//
//	e                         → entryValOf[e] (Entry local from entrySet / var)
//	(Map.Entry<K,V>) e        → V (generic Entry cast; single-arg casts peel value)
//	Map.entry(k, new T(...))  → T (creation value; key ignored)
//	new AbstractMap.SimpleEntry<>(k, new T(...)) → T (same V leaf as Map.entry)
//	new AbstractMap.SimpleImmutableEntry<>(k, new T(...)) → T
//	am.firstEntry()           → valOf[am] (NavigableMap entry endpoints)
//	am.lastEntry()            → valOf[am]
//	am.pollFirstEntry() / am.pollLastEntry() → valOf[am]
//	am.ceilingEntry(k) / am.floorEntry(k) / am.higherEntry(k) / am.lowerEntry(k) → valOf[am]
//	m.entrySet().iterator().next() / m.entrySet().listIterator().previous() → valOf[m]
//	m.entrySet().stream().findFirst().get() / findAny().orElseThrow() → valOf[m]
//	Optional.of(entry).get() / ofNullable(entry).orElseThrow() → V of entry
//
// Used for e.getValue() / ((Map.Entry<K,A>) e).getValue() / var ea = Map.entry(...) /
// var ea = new AbstractMap.SimpleEntry<>(...) / var ea = (Map.Entry<K,A>) e /
// var ea = am.firstEntry() / var ea = m.entrySet().iterator().next() /
// var ea = m.entrySet().stream().findFirst().get() so method rename hits value
// call sites under foreign same-leaf methods.
// Unknown shapes fail closed.
func javaEntryExprValueType(obj *grammar.Node, content []byte, elemOf, valOf, entryValOf map[string]string) string {
	return javaEntryExprValueTypeEx(obj, content, elemOf, valOf, entryValOf, nil, nil)
}

func javaEntryExprValueTypeEx(obj *grammar.Node, content []byte, elemOf, valOf, entryValOf, compOf map[string]string, typeMembers map[string]map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	switch obj.Type() {
	case "identifier", "type_identifier":
		if entryValOf == nil {
			return ""
		}
		return entryValOf[ingest.NodeText(obj, content)]
	case "cast_expression":
		// (Map.Entry<K,V>) e / (Entry<K,V>) e — recover V from the cast type.
		// Non-Entry casts peel the value so entry pipelines under a cast still bind.
		if typeN := ingest.ChildByField(obj, "type"); typeN != nil {
			if vt := javaMapEntryDeclaredValueType(typeN, content); vt != "" {
				return vt
			}
		}
		if val := ingest.ChildByField(obj, "value"); val != nil {
			return javaEntryExprValueTypeEx(val, content, elemOf, valOf, entryValOf, compOf, typeMembers)
		}
		return ""
	case "object_creation_expression":
		// new AbstractMap.SimpleEntry<>(k, new T(...)) /
		// new AbstractMap.SimpleImmutableEntry<>(k, new T(...)) —
		// same V leaf as Map.entry (constructor value arg / declared type args).
		return javaSimpleEntryCreationValueType(obj, content)
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		switch name := ingest.NodeText(nameN, content); name {
		case "entry":
			// Map.entry(k, new T(...)) / Map.entry(k, ba.get()) — value type.
			return javaMapEntryObjectValueType(obj, content, compOf, typeMembers)
		case "firstEntry", "lastEntry",
			// NavigableMap poll/search entry accessors also return Map.Entry<K,V>;
			// V from map value type (same path as firstEntry/lastEntry).
			"pollFirstEntry", "pollLastEntry",
			"ceilingEntry", "floorEntry", "higherEntry", "lowerEntry":
			// NavigableMap.*Entry → Map.Entry<K,V>; V from map value type.
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "reduceEntries":
			// ConcurrentHashMap.reduceEntries(threshold, BiFunction) returns
			// Map.Entry<K,V> (2-arg form). 3-arg returns U — fail closed here.
			if len(javaCallArgs(obj)) == 2 {
				return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
			}
			return ""
		case "next", "previous":
			// m.entrySet().iterator().next() / m.entrySet().listIterator().previous()
			// — Entry of V; recover V via the entrySet pipeline under the iterator.
			return javaEntrySetPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "get", "orElse", "orElseGet", "orElseThrow":
			// Optional unwrap of Map.Entry:
			//   m.entrySet().stream().findFirst().get()
			//   m.entrySet().stream().findAny().orElseThrow()
			//   Optional.of(entry).get() / ofNullable(entry).orElseThrow()
			// Zero-arg get only (Optional.get); List.get(i)/Map.get(k) fail closed.
			if name == "get" && len(javaCallArgs(obj)) != 0 {
				return ""
			}
			return javaOptionalEntryValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf, entryValOf)
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaOptionalEntryValueType recovers V when opt is an Optional of Map.Entry:
//
//	m.entrySet().stream().findFirst() / findAny() / min() / max() → valOf[m]
//	Optional.of(entry) / ofNullable(entry) → V of entry (creation / local / pipeline)
//
// Used under Optional.get/orElse/orElseGet/orElseThrow so
// findFirst().get().getValue() / Optional.of(e).get().getValue() bind under
// foreign same-leaf methods. Unknown Optional sources fail closed.
func javaOptionalEntryValueType(opt *grammar.Node, content []byte, elemOf, valOf, entryValOf map[string]string) string {
	for opt != nil && !opt.IsNull() && opt.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(opt, "expression")
		if inner == nil {
			for i := uint32(0); i < opt.ChildCount(); i++ {
				ch := opt.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		opt = inner
	}
	if opt == nil || opt.IsNull() || opt.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(opt, "name")
	if nameN == nil {
		return ""
	}
	switch name := ingest.NodeText(nameN, content); name {
	case "findFirst", "findAny", "min", "max":
		// Stream of Map.Entry → Optional<Entry>; V from the entrySet pipeline.
		// One-arg reduce returns Optional too but is type-changing more often — fail closed.
		return javaEntrySetPipelineValueType(ingest.ChildByField(opt, "object"), content, elemOf, valOf)
	case "of", "ofNullable":
		// Optional.of(entry) / ofNullable(entry) — Entry from the first arg.
		recv := ingest.ChildByField(opt, "object")
		if recv == nil {
			return ""
		}
		if javaStaticFactoryReceiverName(recv, content) != "Optional" {
			return ""
		}
		first := javaFirstCallArg(opt)
		if first == nil {
			return ""
		}
		return javaEntryExprValueType(first, content, elemOf, valOf, entryValOf)
	default:
		return ""
	}
}

// javaMapEntryCreationValueType recovers T from Map.entry(k, new T(...)).
// Key is ignored; only the second arg's creation type matters. Non-Map receivers,
// wrong arity, or non-creation values fail closed.
func javaMapEntryCreationValueType(call *grammar.Node, content []byte) string {
	return javaMapEntryObjectValueType(call, content, nil, nil)
}

// javaMapEntryObjectValueType peels Map.entry(k, new T(...)) / Map.entry(k, ba.get())
// value arg under foreign same-leaf. Class()-only peels live in
// javaMapEntryCreationValueType.
func javaMapEntryObjectValueType(call *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Map" {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "entry" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) != 2 {
		return ""
	}
	// Prefer method-return / Class() object peels when typeMembers available.
	if typeMembers != nil {
		if t := javaObjectArgType(args[1], content, compOf, typeMembers); t != "" {
			return t
		}
	}
	arg := args[1]
	if arg.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(arg, "type")
	if typeN == nil {
		return ""
	}
	return javaTypeName(typeN, content)
}

// javaFutureTaskCreationElemType recovers V from
// new FutureTask<>(() -> new T(...)) / new FutureTask<T>(…) /
// new FutureTask<>(runnable, new T(...)).
// Prefers declared single type arg when present; diamond recovers V from:
//   - Callable ctor: expression-bodied zero-arg lambda whose body is new T(...)
//   - Runnable+result ctor: second arg new T(...) (first is Runnable)
//
// Other types / arities / bodies fail closed.
// Enables new FutureTask<>(() -> new A()).get().m() and
// var ft = new FutureTask<>(() -> new A()); ft.get().m() under foreign
// same-leaf methods (pipeline peel + elemOf bind).
func javaFutureTaskCreationElemType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(call, "type")
	if typeN == nil {
		return ""
	}
	if args := javaTypeArgNames(typeN, content); len(args) == 1 && args[0] != "" {
		if javaTypeName(typeN, content) == "FutureTask" {
			return args[0]
		}
	}
	if javaTypeName(typeN, content) != "FutureTask" {
		return ""
	}
	ctorArgs := javaCallArgs(call)
	switch len(ctorArgs) {
	case 1:
		// FutureTask(Callable<V>): () -> new T(...)
		if ctorArgs[0].Type() != "lambda_expression" {
			return ""
		}
		// Zero-arg lambda only (Callable.call has no params).
		if len(javaInferredLambdaParamNames(ctorArgs[0], content)) != 0 {
			// Typed/empty formal params: still accept zero-param forms via child scan.
			// Inferred non-empty fails closed (not a Callable shape).
			return ""
		}
		body := ingest.ChildByField(ctorArgs[0], "body")
		for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
			inner := ingest.ChildByField(body, "expression")
			if inner == nil {
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					if ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					inner = ch
					break
				}
			}
			body = inner
		}
		if body == nil || body.Type() != "object_creation_expression" {
			return ""
		}
		argType := ingest.ChildByField(body, "type")
		if argType == nil {
			return ""
		}
		return javaTypeName(argType, content)
	case 2:
		// FutureTask(Runnable, V): second arg new T(...)
		arg := ctorArgs[1]
		if arg.Type() != "object_creation_expression" {
			return ""
		}
		argType := ingest.ChildByField(arg, "type")
		if argType == nil {
			return ""
		}
		return javaTypeName(argType, content)
	default:
		return ""
	}
}

// javaCompletableFutureCreationElemType recovers T from
// new CompletableFuture<T>() / new CompletableFuture<T>(…). Enables
// new CompletableFuture<A>().completeAsync(...).join().m() and
// var f = new CompletableFuture<A>(); f.join().m() under foreign same-leaf.
// Diamond / raw / non-CF types fail closed (no T without a fill stage).
func javaCompletableFutureCreationElemType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(call, "type")
	if typeN == nil {
		return ""
	}
	if javaTypeName(typeN, content) != "CompletableFuture" {
		return ""
	}
	args := javaTypeArgNames(typeN, content)
	if len(args) >= 1 && args[0] != "" {
		return args[0]
	}
	return ""
}

// javaReferenceHolderCreationElemType recovers V from
// new AtomicReference<>(new T(...)) / new AtomicReference<T>(…) /
// new WeakReference<>(new T(...)) / new SoftReference<>(new T(...))
// (and queue overloads: first ctor arg is the referent).
// Prefers declared single type arg when present; diamond/raw recover V from the
// first constructor arg when it is new T(...). Other types / empty ctor /
// non-creation referents fail closed.
// Enables new AtomicReference<>(new A()).get().m() and
// var ar = new AtomicReference<>(new A()); ar.get().m() under foreign
// same-leaf methods (pipeline peel + elemOf bind).
// Method-return referents (ba.get()) use javaReferenceHolderCreationObjectElemType.
func javaReferenceHolderCreationElemType(call *grammar.Node, content []byte) string {
	return javaReferenceHolderCreationObjectElemType(call, content, nil, nil)
}

// javaReferenceHolderCreationObjectElemType recovers V from
// new AtomicReference<>(new T(...)) / new AtomicReference<>(ba.get()) /
// WeakReference / SoftReference under foreign same-leaf when typeMembers is
// provided (zero-arg method return). Class()-only peels work with nil maps.
func javaReferenceHolderCreationObjectElemType(call *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if call == nil || call.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(call, "type")
	if typeN == nil {
		return ""
	}
	// new AtomicReference<A>(…) / WeakReference<A> / SoftReference<A> —
	// V from the single declared type arg.
	if args := javaTypeArgNames(typeN, content); len(args) == 1 && args[0] != "" {
		switch javaTypeName(typeN, content) {
		case "AtomicReference", "WeakReference", "SoftReference":
			return args[0]
		}
	}
	// Diamond / raw: only known V holders.
	switch javaTypeName(typeN, content) {
	case "AtomicReference", "WeakReference", "SoftReference":
		// ok
	default:
		return ""
	}
	// First constructor arg is the referent (AtomicReference(V) /
	// WeakReference(T) / SoftReference(T) / *Reference(T, ReferenceQueue)).
	args := javaCallArgs(call)
	if len(args) < 1 || len(args) > 2 {
		return ""
	}
	// new AtomicReference<>(new A()) / new AtomicReference<>(ba.get()) —
	// Class creation or zero-arg method return under foreign same-leaf.
	if tn := javaObjectArgType(args[0], content, compOf, typeMembers); tn != "" {
		return tn
	}
	// Class-only fallback when typeMembers is nil (stream-pipeline path).
	arg := args[0]
	if arg.Type() != "object_creation_expression" {
		return ""
	}
	argType := ingest.ChildByField(arg, "type")
	if argType == nil {
		return ""
	}
	return javaTypeName(argType, content)
}

// javaSimpleEntryCreationValueType recovers V from
// new AbstractMap.SimpleEntry<>(k, new T(...)) /
// new AbstractMap.SimpleImmutableEntry<>(k, new T(...)) /
// new SimpleEntry<>(k, new T(...)) (same-leaf Map.Entry implementations).
// Prefers declared type args when present; diamond/raw recover V from the second
// constructor arg when it is new T(...). Other types / arities / non-creation
// values fail closed.
func javaSimpleEntryCreationValueType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "object_creation_expression" {
		return ""
	}
	typeN := ingest.ChildByField(call, "type")
	if typeN == nil {
		return ""
	}
	// new AbstractMap.SimpleEntry<K,V>(...) — V from declared type args.
	if vt := javaMapEntryDeclaredValueType(typeN, content); vt != "" {
		return vt
	}
	// Diamond / raw: only SimpleEntry / SimpleImmutableEntry.
	switch javaTypeName(typeN, content) {
	case "SimpleEntry", "SimpleImmutableEntry":
		// ok
	default:
		return ""
	}
	// Second constructor arg is the value (key first); same leaf as Map.entry.
	args := javaCallArgs(call)
	if len(args) != 2 {
		return ""
	}
	arg := args[1]
	if arg.Type() != "object_creation_expression" {
		return ""
	}
	argType := ingest.ChildByField(arg, "type")
	if argType == nil {
		return ""
	}
	return javaTypeName(argType, content)
}

// javaMapCopyOfValueType recovers V from Map.copyOf(m): value type of the first
// arg map. Receiver must be Map; other receivers (and missing args) fail closed.
func javaMapCopyOfValueType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if javaStaticFactoryReceiverName(recvN, content) != "Map" {
		return ""
	}
	first := javaFirstCallArg(call)
	if first == nil {
		return ""
	}
	return javaMapPipelineValueType(first, content, elemOf, valOf)
}

// javaIsToMapIdentityValueCollector reports
// Stream.collect(Collectors.toMap/toConcurrentMap/toUnmodifiableMap(
// keyMapper, valueMapper[, merge[, mapFactory]])) /
// collect(toMap|toConcurrentMap|toUnmodifiableMap(...)) (static import) when the
// value mapper is an identity lambda (a -> a), and
// collect(Collectors.collectingAndThen(toMap|toConcurrentMap|toUnmodifiableMap(...), finisher))
// when the downstream is such a toMap (finisher not inspected — identity /
// Collections::unmodifiableMap common). Result is Map<?, T> with T the stream
// element type. Type-changing value mappers and method-ref value mappers fail closed.
// toUnmodifiableMap accepts at most merge (no mapFactory); arity 2–4 still fails closed
// on non-identity value mappers only.
func javaIsToMapIdentityValueCollector(collectCall *grammar.Node, content []byte) bool {
	first := javaCollectFirstArg(collectCall)
	if first == nil {
		return false
	}
	// collectingAndThen(toMap(...), finisher) — unwrap to the downstream toMap.
	if javaIsCollectingAndThenCall(first, content) {
		first = javaFirstCallArg(first)
	}
	return javaIsToMapIdentityValueCollectorExpr(first, content)
}

// javaIsCollectingAndThenCall reports Collectors.collectingAndThen(...) /
// collectingAndThen(...) (static import). Downstream/finisher are not inspected.
func javaIsCollectingAndThenCall(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "collectingAndThen" {
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	return true
}

// javaIsToMapIdentityValueCollectorExpr reports toMap/toConcurrentMap/
// toUnmodifiableMap(key, a -> a[, …]) / Collectors.toMap(...) (static import or
// Collectors. receiver) with an identity value mapper. Used for bare collect(toMap)
// and as the downstream of collectingAndThen(toMap, finisher).
func javaIsToMapIdentityValueCollectorExpr(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil {
		return false
	}
	switch ingest.NodeText(nameN, content) {
	case "toMap", "toConcurrentMap", "toUnmodifiableMap":
	default:
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	args := javaCallArgs(n)
	// keyMapper + valueMapper required; mergeFunction and mapFactory optional
	// (toMap/toConcurrentMap up to 4; toUnmodifiableMap up to 3 in the JDK).
	if len(args) < 2 || len(args) > 4 {
		return false
	}
	return javaIsIdentityLambda(args[1], content)
}

// javaToMapCollectValueType recovers T from stream.collect(toMap|toConcurrentMap|
// toUnmodifiableMap(key, a -> a[, …])) and
// collect(collectingAndThen(toMap|…(key, a -> a[, …]), finisher)).
// Result is Map of T values (not List groups like groupingBy).
func javaToMapCollectValueType(val *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		val = inner
	}
	if val == nil || val.IsNull() || val.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(val, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "collect" {
		return ""
	}
	if !javaIsToMapIdentityValueCollector(val, content) {
		return ""
	}
	return javaStreamPipelineElemType(ingest.ChildByField(val, "object"), content, elemOf, valOf)
}

// javaEntrySetPipelineValueType recovers the map value type from an entrySet pipeline:
// m.entrySet() / m.entrySet().stream() / m.entrySet().stream().filter(...) → valOf[m],
// m.entrySet().iterator() / m.entrySet().listIterator() / m.entrySet().spliterator() → valOf[m],
// collect(toMap(...)).entrySet() → stream element type.
// Type-changing stages (map/flatMap) fail closed.
func javaEntrySetPipelineValueType(obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() || obj.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(obj, "name")
	if nameN == nil {
		return ""
	}
	switch name := ingest.NodeText(nameN, content); name {
	case "entrySet":
		return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
	case "stream", "parallelStream",
		// iterator/listIterator/spliterator yield Entry views of the same V
		// (for next/previous/forEachRemaining under foreign same-leaf methods).
		"iterator", "listIterator", "descendingIterator", "spliterator",
		"filter", "peek", "sorted", "distinct", "limit", "skip",
		"unordered", "sequential", "parallel", "onClose",
		"takeWhile", "dropWhile":
		return javaEntrySetPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
	default:
		return ""
	}
}

// javaMapEntryDeclaredValueType recovers V from Map.Entry<K,V> / Entry<K,V> /
// AbstractMap.SimpleEntry<K,V> / SimpleImmutableEntry<K,V> type nodes.
// Used for for (Map.Entry<K,A> e : …), Map.Entry locals, and SimpleEntry locals —
// value type for getValue().
func javaMapEntryDeclaredValueType(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	switch javaTypeName(typeN, content) {
	case "Entry", "SimpleEntry", "SimpleImmutableEntry":
		// ok — Map.Entry and its AbstractMap concrete same-leaf implementations.
	default:
		return ""
	}
	args := javaTypeArgNames(typeN, content)
	if len(args) < 2 {
		return ""
	}
	return args[1]
}

// javaStaticCollectionOfElemType recovers the element type of List/Stream/Set.of(...)
// Arrays.asList(...), Optional.of/ofNullable(...), Stream.ofNullable(...),
// CompletableFuture.completedFuture(...)/completedStage(...),
// Collections.singletonList(...), and Collections.singleton(...) when every
// argument is `new T(...)` with the same T.
// Non-creation args and mixed types fail closed.
func javaStaticCollectionOfElemType(call *grammar.Node, content []byte, method string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	// Stream / java.util.stream.Stream / List / java.util.List — leaf simple name.
	recv := javaStaticFactoryReceiverName(recvN, content)
	if recv == "" {
		return ""
	}
	switch method {
	case "of":
		switch recv {
		case "List", "Stream", "Set", "Optional":
			// ok
		default:
			return ""
		}
	case "ofNullable":
		// Optional.ofNullable(new T(...)) / Stream.ofNullable(new T(...)).
		// Also java.util.Optional / java.util.stream.Stream FQ forms.
		if recv != "Optional" && recv != "Stream" {
			return ""
		}
	case "completedFuture", "completedStage":
		// CompletableFuture.completedFuture(new T(...)) /
		// CompletableFuture.completedStage(new T(...)).
		if recv != "CompletableFuture" {
			return ""
		}
	case "asList":
		if recv != "Arrays" {
			return ""
		}
	case "singletonList", "singleton":
		if recv != "Collections" {
			return ""
		}
	default:
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	return javaHomogeneousCreationElem(args, content)
}

// javaStaticFactoryReceiverName returns the simple type/class leaf of a static
// factory receiver: Stream, List, Optional, Map, Collections, Arrays, or the
// outermost field of a qualified form (java.util.stream.Stream → Stream,
// java.util.Collections → Collections). Unknown shapes fail closed.
func javaStaticFactoryReceiverName(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "identifier", "type_identifier":
		return ingest.NodeText(n, content)
	case "field_access":
		// java.util.stream.Stream — outermost field is the simple class name.
		if nameN := ingest.ChildByField(n, "field"); nameN != nil {
			return ingest.NodeText(nameN, content)
		}
		if nameN := ingest.ChildByField(n, "name"); nameN != nil {
			return ingest.NodeText(nameN, content)
		}
		return ""
	case "scoped_type_identifier", "scoped_identifier":
		if nameN := ingest.ChildByField(n, "name"); nameN != nil {
			return ingest.NodeText(nameN, content)
		}
		return ""
	default:
		return ""
	}
}

// javaHomogeneousCreationElem returns T when every argument is `new T(...)` (same T).
func javaHomogeneousCreationElem(args *grammar.Node, content []byte) string {
	if args == nil || args.Type() != "argument_list" {
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "object_creation_expression":
			typeN := ingest.ChildByField(ch, "type")
			if typeN == nil {
				return ""
			}
			tn := javaTypeName(typeN, content)
			if tn == "" {
				return ""
			}
			if !saw {
				elem = tn
				saw = true
				continue
			}
			if tn != elem {
				return ""
			}
		default:
			// Identifiers / method calls / mixed shapes — fail closed.
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// javaObjectArgType recovers T from a factory argument that is either `new T(...)`
// or a same-file zero-arg method return (ba.get() → A) under foreign same-leaf.
// Other shapes fail closed.
func javaObjectArgType(arg *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if arg == nil {
		return ""
	}
	for arg != nil && !arg.IsNull() && arg.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(arg, "expression")
		if inner == nil {
			for i := uint32(0); i < arg.ChildCount(); i++ {
				ch := arg.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		arg = inner
	}
	if arg == nil || arg.IsNull() {
		return ""
	}
	if arg.Type() == "object_creation_expression" {
		typeN := ingest.ChildByField(arg, "type")
		if typeN == nil {
			return ""
		}
		return javaTypeName(typeN, content)
	}
	if arg.Type() == "method_invocation" {
		return javaRecordComponentAccessType(arg, ingest.ChildByField(arg, "object"),
			javaMethodInvocationName(arg, content), content, compOf, typeMembers)
	}
	return ""
}

// javaHomogeneousObjectElem returns T when every argument peels to the same Class
// leaf via new T(...) or zero-arg method return (ba.get()). Enables
// List.of(ba.get()) / Stream.of(ba.get()) / Arrays.asList(ba.get()) under foreign
// same-leaf (Class()-only peels live in javaHomogeneousCreationElem).
func javaHomogeneousObjectElem(args *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if args == nil || args.Type() != "argument_list" {
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			tn := javaObjectArgType(ch, content, compOf, typeMembers)
			if tn == "" {
				return ""
			}
			if !saw {
				elem = tn
				saw = true
				continue
			}
			if tn != elem {
				return ""
			}
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// javaStaticCollectionOfObjectElemType recovers T from static collection/stream
// factories whose args peel via javaHomogeneousObjectElem, including type-
// preserving pipeline stages under foreign same-leaf:
//
//	List.of(ba.get()) / Arrays.asList(ba.get()) / Set.of(ba.get())
//	Collections.singletonList(ba.get()) / Collections.singleton(ba.get())
//	Stream.of(ba.get()) / Stream.of(ba.get()).findFirst() / .toList()
//	Optional.of(ba.get()) / ofNullable(ba.get())
//	CompletableFuture.completedFuture(ba.get())
//
// Class()-only peels live in javaStaticCollectionOfElemType. Mixed / unknown
// methods / non-homogeneous args fail closed.
func javaStaticCollectionOfObjectElemType(obj *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	for obj != nil && !obj.IsNull() {
		for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
			inner := ingest.ChildByField(obj, "expression")
			if inner == nil {
				for i := uint32(0); i < obj.ChildCount(); i++ {
					ch := obj.Child(i)
					if ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					inner = ch
					break
				}
			}
			obj = inner
		}
		if obj == nil || obj.IsNull() || obj.Type() != "method_invocation" {
			return ""
		}
		name := javaMethodInvocationName(obj, content)
		recv := ingest.ChildByField(obj, "object")
		switch name {
		case "of", "ofNullable", "asList", "singletonList", "singleton", "completedFuture", "completedStage":
			// Factory leaf — peel homogeneous object args.
			var args *grammar.Node
			for i := uint32(0); i < obj.ChildCount(); i++ {
				if obj.Child(i).Type() == "argument_list" {
					args = obj.Child(i)
					break
				}
			}
			// Restrict receivers like javaStaticCollectionOfElemType.
			recvName := javaStaticFactoryReceiverName(recv, content)
			switch name {
			case "of":
				switch recvName {
				case "List", "Stream", "Set", "Optional":
					// ok
				default:
					return ""
				}
			case "ofNullable":
				if recvName != "Optional" && recvName != "Stream" {
					return ""
				}
			case "completedFuture", "completedStage":
				if recvName != "CompletableFuture" {
					return ""
				}
			case "asList":
				if recvName != "Arrays" {
					return ""
				}
			case "singletonList", "singleton":
				if recvName != "Collections" {
					return ""
				}
			}
			return javaHomogeneousObjectElem(args, content, compOf, typeMembers)
		case "findFirst", "findAny", "toList", "toSet",
			"stream", "parallelStream", "iterator", "listIterator",
			"descendingIterator", "spliterator",
			"reversed", "sequencedCollection",
			"filter", // Optional.filter — type-preserving when present
			"or",     // Optional.or(Supplier) — always same T
			"orElseThrow",
			"join", "getNow", "resultNow",
			"get": // zero-arg Optional/CF get on pipeline
			if name == "get" && !javaCallIsZeroArg(obj) {
				return ""
			}
			// Type-preserving stage — peel receiver.
			obj = recv
			continue
		case "limit", "skip", "sorted", "distinct", "unordered", "sequential", "parallel",
			"peek", "onClose", "takeWhile", "dropWhile":
			// Stream intermediate type-preserving stages.
			obj = recv
			continue
		case "map":
			// Stream.of(ba.get()).map(x -> x).findFirst().get() — identity mapper
			// preserves method-return element under foreign same-leaf (filter/peek
			// already type-preserve; non-identity mappers fail closed here so
			// type-changing map stays unbound on the Class pipeline path).
			if javaIsIdentityMapCall(obj, content) {
				obj = recv
				continue
			}
			return ""
		case "flatMap":
			// Stream.of(ba.get()).flatMap(x -> Stream.of(x)).findFirst().get() /
			// Optional.of(ba.get()).flatMap(x -> Optional.of(x)).get() — identity
			// rewrap preserves method-return element under foreign same-leaf
			// (Class peels via javaStreamPipelineElemType; non-identity fail closed).
			if javaIsIdentityFlatMapRewrapCall(obj, content) {
				obj = recv
				continue
			}
			return ""
		case "collect":
			// Stream.of(ba.get()).collect(Collectors.toList()/toSet()/…) —
			// type-preserving collector under foreign same-leaf (Class peels via
			// javaStreamPipelineElemType). Other collectors fail closed.
			if javaIsToListOrSetCollector(obj, content) {
				obj = recv
				continue
			}
			return ""
		case "unmodifiableList", "synchronizedList", "checkedList",
			"unmodifiableSet", "synchronizedSet", "checkedSet",
			"unmodifiableSortedSet", "synchronizedSortedSet", "checkedSortedSet",
			"unmodifiableNavigableSet", "synchronizedNavigableSet", "checkedNavigableSet",
			"unmodifiableSequencedCollection", "synchronizedSequencedCollection",
			"unmodifiableSequencedSet", "synchronizedSequencedSet",
			"unmodifiableCollection", "synchronizedCollection", "checkedCollection",
			"asLifoQueue", "checkedQueue",
			"list", "enumeration",
			"copyOf":
			// Collections.unmodifiableList(List.of(ba.get())) / List.copyOf(…) —
			// first-arg element type under foreign same-leaf.
			if name == "copyOf" {
				// List.copyOf / Set.copyOf only (Arrays.copyOf is array, not object peel).
				rcvName := javaStaticFactoryReceiverName(recv, content)
				if rcvName != "List" && rcvName != "Set" {
					return ""
				}
			} else if javaStaticFactoryReceiverName(recv, content) != "Collections" {
				return ""
			}
			first := javaFirstCallArg(obj)
			if first == nil {
				return ""
			}
			obj = first
			continue
		default:
			return ""
		}
	}
	return ""
}

// javaCollectionsNCopiesObjectElemType recovers T from Collections.nCopies(n, ba.get())
// when the second arg peels via javaObjectArgType. Class()-only peels live in
// javaCollectionsNCopiesElemType.
func javaCollectionsNCopiesObjectElemType(obj *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if obj == nil || obj.Type() != "method_invocation" {
		return ""
	}
	if javaMethodInvocationName(obj, content) != "nCopies" {
		return ""
	}
	recvN := ingest.ChildByField(obj, "object")
	if javaStaticFactoryReceiverName(recvN, content) != "Collections" {
		return ""
	}
	args := javaCallArgs(obj)
	if len(args) != 2 {
		return ""
	}
	return javaObjectArgType(args[1], content, compOf, typeMembers)
}

// javaMapOfObjectValueType recovers T from Map.of(k, ba.get()) / Map.of(k, new A())
// when every value arg peels to the same Class leaf via javaObjectArgType.
// Class()-only peels live in javaMapOfValueType.
func javaMapOfObjectValueType(obj *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if obj == nil || obj.Type() != "method_invocation" {
		return ""
	}
	if javaMethodInvocationName(obj, content) != "of" {
		return ""
	}
	recvN := ingest.ChildByField(obj, "object")
	if javaStaticFactoryReceiverName(recvN, content) != "Map" {
		return ""
	}
	args := javaCallArgs(obj)
	if len(args) < 2 || len(args)%2 != 0 {
		return ""
	}
	var elem string
	saw := false
	for i := 1; i < len(args); i += 2 {
		tn := javaObjectArgType(args[i], content, compOf, typeMembers)
		if tn == "" {
			return ""
		}
		if !saw {
			elem = tn
			saw = true
			continue
		}
		if tn != elem {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// javaInferredLambdaParamNames returns parameter names for untyped lambdas:
// a -> … and (a, b) -> …. Typed (A a) -> uses formal_parameters and returns nil.
func javaInferredLambdaParamNames(lambda *grammar.Node, content []byte) []string {
	if lambda == nil || lambda.Type() != "lambda_expression" {
		return nil
	}
	for i := uint32(0); i < lambda.ChildCount(); i++ {
		ch := lambda.Child(i)
		switch ch.Type() {
		case "->":
			return nil
		case "identifier":
			return []string{ingest.NodeText(ch, content)}
		case "inferred_parameters":
			var names []string
			for j := uint32(0); j < ch.ChildCount(); j++ {
				p := ch.Child(j)
				if p.Type() == "identifier" {
					names = append(names, ingest.NodeText(p, content))
				}
			}
			return names
		case "formal_parameters":
			return nil
		}
	}
	return nil
}

// javaCollectionAccessElemType recovers the element/value type of collection
// accessors used as var initializers:
//
//	as.get(i) / as.get(0)     → elemOf[as]   (List/Collection)
//	am.get(k) / am.getOrDefault(k, d) → valOf[am] (Map; prefer value over key)
//	am.computeIfAbsent(k, f) / am.putIfAbsent(k, v) → valOf[am] (Map returns V)
//	am.compute(k, f) / am.computeIfPresent(k, f) → valOf[am] (Map returns V)
//	am.put(k, v) / am.replace(k, v) / am.merge(k, v, remapping) → valOf[am] (Map returns V)
//	am.putFirst(k, v) / am.putLast(k, v) → valOf[am] (SequencedMap returns V)
//	as.remove(i) / am.remove(k) → same element/value type (index/key remove returns E/V)
//	as.set(i, e) → elemOf[as] (List returns previous E)
//	qa.poll() / qa.peek() / qa.element() → elemOf[qa] (Queue)
//	qa.take() → elemOf[qa] (BlockingQueue)
//	da.pollFirst()/pollLast()/peekFirst()/peekLast()/pop() → elemOf[da] (Deque)
//	sa.push(e) → elemOf[sa] (Stack returns E; Deque.push is void)
//	da.takeFirst() / da.takeLast() → elemOf[da] (BlockingDeque)
//	as.getFirst() / as.getLast() → elemOf[as] (SequencedCollection / List / Deque)
//	as.removeFirst() / as.removeLast() → elemOf[as] (SequencedCollection / List / Deque)
//	vs.elementAt(i) / vs.firstElement() / vs.lastElement() → elemOf[vs] (Vector)
//	ss.first() / ss.last() → elemOf[ss] (SortedSet / NavigableSet)
//	ns.ceiling(e) / ns.floor(e) / ns.higher(e) / ns.lower(e) → elemOf[ns] (NavigableSet)
//	ia.next()                 → elemOf[ia]   (Iterator<A>)
//	as.iterator().next()      → elemOf[as]   (via type-preserving iterator())
//	lia.previous()            → elemOf[lia]  (ListIterator<A>)
//	as.listIterator().next()/previous() → elemOf[as] (type-preserving listIterator())
//	ea.nextElement()          → elemOf[ea]   (Enumeration<A>)
//	vs.elements().nextElement() → elemOf[vs] (Vector; Hashtable uses valOf)
//	Collections.enumeration(as).nextElement() → elemOf[as]
//	oa.orElse(d) / oa.orElseGet(s) / oa.orElseThrow([s]) → elemOf[oa]
//	  (Optional<A>; also findFirst().orElse / findFirst().orElseThrow)
//	oa.get() / as.stream().findFirst().get() / Optional.of(new A()).get() → T
//	  (zero-arg Optional.get; also List.of(new A()).get(i) / toList().get(i) via pipeline)
//	as[0] / (as)[0] / matrix[i][j] → elemOf[as] (array element; index does not change T)
//	  (A[] as / A[][] matrix; new A[]{...}[i] via array creation type)
//	as.stream().toArray()[i] / as.stream().toArray(new A[0])[i] / as.toArray(new A[0])[i]
//	  → stream/collection element type (toArray preserves T for index access)
//	Arrays.copyOf(as, n)[i] / Arrays.copyOfRange(as, from, to)[i]
//	  → first-arg array element type (length/range only)
//	aa.getAndSet(v) / aa.getAndUpdate(f) / aa.updateAndGet(f) /
//	  aa.accumulateAndGet(x, f) / aa.compareAndExchange(e, u) /
//	  aa.getPlain() / aa.getAcquire() / aa.getOpaque() → elemOf[aa]
//	  (AtomicReference<V> and friends; return V like get; args do not change T)
//	new AtomicReference<>(new A()).get() → A
//	new WeakReference<>(new A()).get() / SoftReference → A
//	  (holder construction peel; same leaf as var ar = new AtomicReference<>(new A()))
//	ca.call() → elemOf[ca] (Callable<A>; zero-arg call returns V)
//	fa.join() / fa.resultNow() → elemOf[fa] (CompletableFuture<A>; zero-arg)
//	fa.getNow(d) → elemOf[fa] (CompletableFuture<A>; default does not change T)
//	CompletableFuture.completedFuture(new A()).join() / get() / getNow(d) / resultNow() → A
//	  (factory pipeline peel; same leaf as var f = completedFuture(new A()); f.join())
//	CompletableFuture.supplyAsync(() -> new A()).join() / get() / getNow(d) / resultNow() → A
//	  (supplier-body peel; same leaf as var f = supplyAsync(() -> new A()); f.join())
//	fa.thenApply(a -> a).join() / getNow(d) / resultNow() → T (identity mapper only;
//	  type-changing thenApply mappers fail closed — same shapes as Optional.map)
//	fa.applyToEither(other, a -> a).join() / getNow(d) / resultNow() → T (identity
//	  Function only; other stage first arg; type-changing mappers fail closed)
//	fa.handle((a, e) -> a).join() / getNow(d) / resultNow() → T (first-param identity
//	  bi-lambda only; type-changing handle mappers fail closed)
//	fa.thenCombine(other, (a, b) -> a).join() / getNow(d) / resultNow() → T
//	  (first-param identity BiFunction only; other stage first arg; type-changing fail closed)
//	fa.thenCompose(a -> CompletableFuture.completedFuture(a)).join() / getNow(d) / resultNow() → T
//	  (completedFuture rewrap only; type-changing thenCompose mappers fail closed —
//	  same shapes as Optional.flatMap)
//	fa.whenComplete((a,e)->…).join() / fa.copy().join() / fa.toCompletableFuture().join() /
//	  fa.orTimeout(…).join() / fa.completeOnTimeout(…).join() /
//	  fa.exceptionally(…).join() / fa.exceptionallyCompose(…).join() → T
//	  (always preserve CF result T by API signature)
//	fn.apply(x) → valOf[fn] / elemOf[fn] (Function<T,R> R / UnaryOperator<T> T /
//	  BiFunction<T,U,R> R; apply args do not change the result type leaf)
//	Objects.requireNonNull(x[, msg]) / requireNonNullElse(x, d) /
//	  requireNonNullElseGet(x, s) → type of x (first-arg leaf; msg/default/supplier
//	  do not change T) (new A() / A.make() / as.get(i) / oa.get() / …; bare typed
//	  locals via javaObjectsRequireNonNullArgIdent + typedLocals)
//	Map.of(k, new A()).get(k) / Map.ofEntries(...).get(k) /
//	  Collections.singletonMap(k, new A()).get(k) / Map.copyOf(m).get(k) → A
//	  (map factory/pipeline value type; getOrDefault/remove/put/compute same V)
//	Collections.min(as) / Collections.max(as[, cmp]) → elemOf[as]
//	stream.reduce(identity, op[, combiner]) → stream element type (returns T/U)
//	e.getValue() / e.setValue(v) → entryValOf[e] (Map.Entry; setValue returns previous V)
//	Map.entry(k, new T()).getValue() → T
//	new AbstractMap.SimpleEntry<>(k, new T()).getValue() → T
//	new AbstractMap.SimpleImmutableEntry<>(k, new T()).getValue() → T
//	am.firstEntry().getValue() / am.lastEntry().getValue() → valOf[am]
//	am.firstEntry().setValue(v) / am.lastEntry().setValue(v) → valOf[am]
//	am.pollFirstEntry()/pollLastEntry()/ceilingEntry(k)/… .getValue() → valOf[am]
//	m.entrySet().iterator().next().getValue() / .setValue(v) → valOf[m]
//	m.entrySet().stream().findFirst().get().getValue() /
//	  findAny().orElseThrow().getValue() → valOf[m]
//	Optional.of(entry).get().getValue() → V of entry
//
// Optional.get() works for Optional<A> locals (elemOf) and for Optional-yielding
// pipelines (findFirst/findAny/min/max/Optional.of) via zero-arg get + pipeline.
// Callable.call / CompletableFuture.join|getNow|resultNow use the same elemOf path
// as Supplier.get / Future.get (generic type arg → T).
// AtomicReference getAndSet/getAndUpdate/updateAndGet/accumulateAndGet/
// compareAndExchange/getPlain/getAcquire/getOpaque share get's elemOf path
// (return V; updater/expected args do not change the type leaf).
// new AtomicReference<>(new A()).get() / WeakReference / SoftReference peel V
// from the holder construction (declared type arg or diamond first-arg creation)
// so inline get and var ar = new AtomicReference<>(new A()) bind under foreign
// same-leaf methods.
// Array index as[0] / (as)[0] recovers elemOf[as] (same leaf as List.get; index
// does not change T). Nested matrix[i][j] peels to the root array local.
// Stream/Collection.toArray()[i] recovers the pipeline element type (toArray is
// type-preserving in javaStreamPipelineElemType).
// Function.apply prefers valOf (R) then elemOf (UnaryOperator/BinaryOperator T);
// BiFunction stores R in valOf at bind time (see javaRecordCollectionElem).
// ConcurrentHashMap.searchValues(threshold, Function) returns U; identity
// Function (a -> a) recovers V of the map (var/chain under foreign same-leaf).
// ConcurrentHashMap.search(threshold, BiFunction) returns U; identity
// BiFunction ((k,v) -> v) recovers V of the map (var/chain under foreign same-leaf).
// Fail closed on other methods / unknown receivers.
// One-arg stream.reduce(BinaryOperator) returns Optional — use orElse/ifPresent
// (pipeline typing), not bare var of the element type.
func javaCollectionAccessElemType(val *grammar.Node, content []byte, elemOf, valOf, entryValOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf map[string]string, typeMembers map[string]map[string]string) string {
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		val = inner
	}
	if val == nil || val.IsNull() {
		return ""
	}
	// as[0] / (as)[0] / matrix[i][j] — element of a typed array local.
	// Direct as[0].m() already renames via typedLocals (javaTypeName collapses
	// A[] → A on the array param); var xa = as[0] needs this elemOf path.
	// as.stream().toArray()[i] / as.toArray(new A[0])[i] — pipeline element type.
	if val.Type() == "array_access" {
		return javaArrayAccessElemType(val, content, elemOf, valOf)
	}
	if val.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(val, "name")
	if nameN == nil {
		return ""
	}
	obj := ingest.ChildByField(val, "object")
	method := ingest.NodeText(nameN, content)
	// Record component accessors before collection method names: ba.a() when
	// ba is a known record local (zero-arg; component type from header).
	if t := javaRecordComponentAccessType(val, obj, method, content, compOf, typeMembers); t != "" {
		return t
	}
	// Optional.of(ba.get()).get() / ofNullable / orElseThrow — zero-arg method
	// return wrapped in Optional.of under foreign same-leaf (assigned form via
	// typedLocals; bare Optional.of(a).get() already peels via ident unwrap).
	if t := javaOptionalOfMethodReturnType(val, content, compOf, typeMembers); t != "" {
		return t
	}
	switch method {
	case "get", "getOrDefault",
		// Map computeIfAbsent/putIfAbsent return V (same as get for typed locals).
		// Mapping/default args do not change the value type leaf.
		"computeIfAbsent", "putIfAbsent",
		// Map compute/computeIfPresent also return V (current or remapped value).
		// Bi-lambda value params are already typed; this binds var from the call result.
		"compute", "computeIfPresent",
		// Map put/replace/merge also return V (previous or merged value).
		// replace(K,V,V) returns boolean at runtime — fail open like remove(Object);
		// product case is replace(K,V) / put / merge under foreign same-leaf methods.
		"put", "replace", "merge",
		// SequencedMap.putFirst/putLast return V (previous mapping for key; Java 21).
		// Same value type leaf as put for typed-local inference.
		"putFirst", "putLast",
		// Element/value-returning mutators and endpoints (same type as get).
		// Map.remove(k) → V via valOf; List.remove(i) → E via elemOf.
		// remove(Object) also returns boolean at runtime for List — we still bind
		// the element type when the receiver is a known collection (fail open on
		// the boolean overload; callers using var xa = as.remove(x) for element
		// extract are the product case under foreign same-leaf methods).
		"remove",
		// List.set(i, e) returns the previous element E (same type as get).
		// Replacement arg does not change the element type leaf.
		"set",
		"poll", "peek", "element",
		// BlockingQueue.take returns E (blocks until an element is available).
		"take",
		"pollFirst", "pollLast", "peekFirst", "peekLast", "pop",
		// Stack.push(E) returns E (the item argument). Deque.push is void and
		// cannot appear as a var initializer in valid Java.
		"push",
		// BlockingDeque.takeFirst/takeLast return E (same element type as pollFirst).
		"takeFirst", "takeLast",
		"getFirst", "getLast",
		// SequencedCollection / List / Deque removeFirst/removeLast return E
		// (Java 21; same element type as getFirst/getLast).
		"removeFirst", "removeLast",
		// Vector.elementAt / firstElement / lastElement return E
		// (same element type as List.get / getFirst / getLast).
		"elementAt", "firstElement", "lastElement",
		// SortedSet / NavigableSet first/last return E (same element type as get).
		"first", "last",
		// NavigableSet ceiling/floor/higher/lower return E (search endpoints;
		// probe arg does not change the element type leaf).
		"ceiling", "floor", "higher", "lower",
		// Callable.call returns V (zero-arg; same element type leaf as Supplier.get).
		"call",
		// CompletableFuture.join / resultNow return T (zero-arg).
		// getNow(defaultValue) returns T (default does not change the type leaf).
		// Future.get / CF.get already covered by "get" above.
		"join", "getNow", "resultNow",
		// AtomicReference (and similar V holders): accessors that return V like get.
		// getAndSet / getAndUpdate return the previous V; updateAndGet /
		// accumulateAndGet return the updated V; compareAndExchange returns the
		// witness V; getPlain/getAcquire/getOpaque are memory-ordering variants of get.
		// Updater/expected/update args do not change the value type leaf.
		"getAndSet", "getAndUpdate", "updateAndGet", "accumulateAndGet",
		"compareAndExchange", "getPlain", "getAcquire", "getOpaque",
		// Function.apply / UnaryOperator.apply / BiFunction.apply return R (or T when
		// UnaryOperator/BinaryOperator). Function<T,R> stores R in valOf; UnaryOperator
		// stores T in elemOf; BiFunction stores R in valOf (third type arg at bind).
		// Apply arguments do not change the result type leaf.
		"apply":
		// Map-like (2 type args recorded in valOf) → value type; else element type.
		// Identifier receiver: List.get / Map.get / Optional local get /
		// Callable.call / CompletableFuture.join|getNow|resultNow /
		// Function/UnaryOperator/BiFunction.apply / …
		if obj != nil && obj.Type() == "identifier" {
			id := ingest.NodeText(obj, content)
			if valOf != nil {
				if vt := valOf[id]; vt != "" {
					return vt
				}
			}
			if elemOf != nil {
				return elemOf[id]
			}
		}
		// Function.identity().apply(new A()) / Function.<A>identity().apply(a) /
		// Function.identity().apply(ba.get()) — identity Function peels apply arg
		// to T under foreign same-leaf (method-return via typeMembers).
		if method == "apply" {
			if t := javaFunctionIdentityApplyType(val, content, elemOf, valOf, compOf, typeMembers); t != "" {
				return t
			}
		}
		// Element accessors on factory/pipeline receivers (not just bare identifiers):
		//   as.stream().findFirst().get() / as.findAny().get() /
		//   Optional.of(new A()).get() / Optional.ofNullable(new A()).get()
		//   List.of(new A()).get(0) / Arrays.asList(new A()).get(0) /
		//   Collections.singletonList(new A()).get(0) / as.stream().toList().get(0)
		//   List.of(new A()).getFirst() / getLast() (Java 21 SequencedCollection)
		//   List.of(new A()).removeFirst() / removeLast() /
		//   as.stream().toList().remove(0) / as.reversed().removeFirst() /
		//   List.copyOf(as).removeFirst() / Arrays.asList(new A()).set(0, x)
		//   fa.thenApply(a -> a).join() / getNow(d) / resultNow() — identity
		//   thenApply preserves CF result T (same shapes as Optional.map).
		//   fa.applyToEither(other, a -> a).join() / getNow(d) / resultNow() —
		//   identity Function preserves CF result T (other stage first arg).
		//   fa.handle((a, e) -> a).join() / getNow(d) / resultNow() — first-param
		//   identity bi-lambda preserves CF result T.
		//   fa.thenCombine(other, (a, b) -> a).join() / getNow(d) / resultNow() —
		//   first-param identity BiFunction preserves CF result T (other stage first).
		//   fa.thenCompose(a -> completedFuture(a)).join() / getNow(d) / resultNow() —
		//   completedFuture rewrap preserves CF result T (flatMap-style).
		//   fa.whenComplete(...).join() / copy().join() / toCompletableFuture().join() /
		//   orTimeout(...).join() / completeOnTimeout(...).join() /
		//   exceptionally(...).join() / exceptionallyCompose(...).join() —
		//   always preserve CF result T by API signature.
		// Pipeline typing already treats findFirst/findAny/Optional.of/List.of/
		// toList/reversed/copyOf/thenApply(identity)/applyToEither(identity)/
		// handle(first-param identity)/thenCombine(first-param identity)/
		// thenCompose(completedFuture rewrap)/whenComplete/copy/… as T. Map
		// factories (Map.of / ofEntries / singletonMap / copyOf / unmodifiableMap /
		// collect(toMap)) are not element pipelines — fall through to
		// javaMapPipelineValueType for value-returning accessors
		// (Map.of(k, new A()).get(k)).
		switch method {
		case "get", "getFirst", "getLast",
			"remove", "removeFirst", "removeLast", "set",
			"poll", "peek", "element", "take",
			"pollFirst", "pollLast", "peekFirst", "peekLast", "pop",
			"push", "takeFirst", "takeLast",
			"elementAt", "firstElement", "lastElement",
			"first", "last", "ceiling", "floor", "higher", "lower",
			// CompletableFuture.join / getNow / resultNow on pipeline receivers
			// (thenApply(a -> a).join() / applyToEither(other, a -> a).join() /
			// handle((a,e)->a).join() / thenCombine(other, (a,b)->a).join() /
			// thenCompose(a -> completedFuture(a)).join() /
			// whenComplete(...).join() / copy().join() / …) —
			// peel CF type-preserving stages.
			"join", "getNow", "resultNow":
			if et := javaStreamPipelineElemType(obj, content, elemOf, valOf); et != "" {
				return et
			}
			// List.of(ba.get()).get(0) / Arrays.asList(ba.get()).get(0) /
			// Collections.singletonList(ba.get()).get(0) /
			// Stream.of(ba.get()).findFirst().get() / toList().get(0) /
			// Collections.nCopies(n, ba.get()).get(0) /
			// CompletableFuture.completedFuture(ba.get()).join() —
			// method-return static factory peels under foreign same-leaf
			// (Class() peels via javaStreamPipelineElemType / javaHomogeneousCreationElem).
			if et := javaStaticCollectionOfObjectElemType(obj, content, compOf, typeMembers); et != "" {
				return et
			}
			if et := javaCollectionsNCopiesObjectElemType(obj, content, compOf, typeMembers); et != "" {
				return et
			}
			// new AtomicReference<>(ba.get()).get() / WeakReference / SoftReference —
			// method-return holder peels under foreign same-leaf (Class peels via
			// javaStreamPipelineElemType / javaReferenceHolderCreationElemType).
			if et := javaReferenceHolderCreationObjectElemType(obj, content, compOf, typeMembers); et != "" {
				return et
			}
			// stream.gather(windowFixed|windowSliding).findFirst().get().get(0) /
			// s.findFirst().get().get(0) after var s = gather(window*) /
			// oa.get().get(0) after var oa = s.findFirst() /
			// .getFirst() / .getLast() — window list element T (List-window + get).
			if method == "get" || method == "getFirst" || method == "getLast" ||
				method == "remove" || method == "removeFirst" || method == "removeLast" ||
				method == "set" {
				if et := javaWindowListExprElemType(obj, content, elemOf, valOf, windowStreamOf, windowOptOf); et != "" {
					return et
				}
			}
			// m.get(k).get(0) / collect(groupingBy|partitioningBy).get(k).get(0) —
			// groupingBy map values are List<T>; outer get peels list element T.
			if method == "get" || method == "getFirst" || method == "getLast" ||
				method == "remove" || method == "removeFirst" || method == "removeLast" ||
				method == "set" {
				if et := javaGroupingByMapGetElemType(obj, content, elemOf, valOf, groupValOf); et != "" {
					return et
				}
			}
			// e.getValue().get(0) / findFirst().get().getValue().get(0) —
			// groupingBy entry value lists are List<T>; outer get peels list element T.
			if method == "get" || method == "getFirst" || method == "getLast" ||
				method == "remove" || method == "removeFirst" || method == "removeLast" ||
				method == "set" {
				if et := javaGroupingByEntryGetValueElemType(obj, content, elemOf, valOf, entryGroupOf, groupValOf); et != "" {
					return et
				}
			}
			// aa.get(0).get(0) / ma.get(k).get(0) when aa: List<List<A>> /
			// ma: Map<K, List<A>> — outer get peels nested element T.
			if method == "get" || method == "getFirst" || method == "getLast" ||
				method == "remove" || method == "removeFirst" || method == "removeLast" ||
				method == "set" {
				if et := javaNestedCollectionGetElemType(obj, content, elemOf, valOf); et != "" {
					return et
				}
			}
			// Map.of(k, new A()).get(k) / Map.ofEntries(...).get(k) /
			// Collections.singletonMap(k, new A()).get(k) /
			// Map.copyOf(m).get(k) / Collections.unmodifiableMap(m).get(k) /
			// stream.collect(toMap(...)).get(k) — and remove(k) → V.
			// Map.of(k, ba.get()).get(k) — method-return map value peels.
			// getFirst/getLast/set/queue endpoints / CF join stay list/deque/CF-only.
			if method == "get" || method == "remove" {
				if vt := javaMapPipelineValueType(obj, content, elemOf, valOf); vt != "" {
					return vt
				}
				return javaMapOfObjectValueType(obj, content, compOf, typeMembers)
			}
			return ""
		}
		// Map-only value mutators on non-id factories/pipelines (same V leaf).
		switch method {
		case "getOrDefault",
			"computeIfAbsent", "putIfAbsent", "compute", "computeIfPresent",
			"put", "replace", "merge", "putFirst", "putLast":
			return javaMapPipelineValueType(obj, content, elemOf, valOf)
		}
		return ""
	case "getValue", "setValue":
		// e.getValue() — Map.Entry local (entrySet for-var / forEach / var ea = …).
		// e.setValue(v) — returns previous V (same value type leaf as getValue).
		// Map.entry(k, new T(...)).getValue() / Map.entry(k, ba.get()).getValue() /
		// am.firstEntry().getValue() / am.firstEntry().setValue(v) — same V.
		return javaEntryExprValueTypeEx(obj, content, elemOf, valOf, entryValOf, compOf, typeMembers)
	case "next", "previous", "nextElement":
		// it.next() / as.iterator().next() — element of iterator or pipeline.
		// lia.previous() / as.listIterator().previous() — same E (ListIterator).
		// listIterator() is type-preserving in javaStreamPipelineElemType.
		// as.descendingIterator().next() — same E (Deque/NavigableSet reverse iter).
		// descendingIterator() is type-preserving in javaStreamPipelineElemType.
		// ea.nextElement() / vs.elements().nextElement() /
		// Collections.enumeration(as).nextElement() — same E (Enumeration).
		// elements()/enumeration() are type-preserving in javaStreamPipelineElemType.
		// List.of(ba.get()).iterator().next() — method-return factory peels.
		if et := javaStreamPipelineElemType(obj, content, elemOf, valOf); et != "" {
			return et
		}
		return javaStaticCollectionOfObjectElemType(obj, content, compOf, typeMembers)
	case "orElse", "orElseGet", "orElseThrow":
		// Optional.orElse / orElseGet / orElseThrow return T; receiver may be
		// Optional<A> local or a pipeline that yields Optional
		// (findFirst/findAny / Optional.of). Exception supplier on orElseThrow
		// does not change the value type leaf.
		// Prefer pipeline T; fall back to orElseGet/orElse supplier/default
		// creation peels when the Optional source is untyped (Optional.empty()).
		// Stream.of(ba.get()).findFirst().orElse(…) — method-return factory peels.
		if t := javaStreamPipelineElemType(obj, content, elemOf, valOf); t != "" {
			return t
		}
		if t := javaStaticCollectionOfObjectElemType(obj, content, compOf, typeMembers); t != "" {
			return t
		}
		return javaOptionalOrElseFallbackType(val, content)
	case "min", "max":
		// Collections.min(coll) / Collections.max(coll[, cmp]) return the element type.
		// Stream.min/max return Optional — bind via orElse/ifPresent on the pipeline,
		// not as a bare var of the element type.
		return javaCollectionsMinMaxElemType(val, obj, content, elemOf, valOf)
	case "requireNonNull", "requireNonNullElse", "requireNonNullElseGet":
		// Objects.requireNonNull(x[, msg]) / requireNonNullElse(x, d) /
		// requireNonNullElseGet(x, s) return T of x (first arg). Message, default,
		// and supplier do not change the type leaf. Bare typed-local identifiers
		// are handled via javaObjectsRequireNonNullArgIdent + typedLocals.
		return javaObjectsRequireNonNullElemType(val, obj, content, elemOf, valOf, entryValOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf)
	case "reduce":
		// ConcurrentHashMap.reduce(threshold, BiFunction<K,V,U>, BiFunction<U,U,U>)
		// returns U. Value-identity transformer ((k,v) -> v) yields V of the map
		// (var xa = m.reduce(1L, (k,v) -> v, (a,b) -> a); xa.m() / chained .m()).
		// Prefer CHM recovery when the receiver is a map; otherwise stream reduce.
		if t := javaCHMReduceReturnType(val, obj, content, elemOf, valOf); t != "" {
			return t
		}
		// Stream.reduce(identity, accumulator[, combiner]) returns the identity type
		// (T for BinaryOperator; U for the 3-arg form when identity is U — we recover
		// the stream element type, which matches the common T-identity product case).
		// One-arg reduce(BinaryOperator) returns Optional<T> — fail closed here.
		return javaStreamReduceIdentityElemType(val, obj, content, elemOf, valOf)
	case "collect":
		// Stream.collect(Collectors.reducing(identity, op[, mapper])) returns T when
		// the collector is identity reducing (2-arg or identity-mapper 3-arg).
		// Optional forms (1-arg reducing / maxBy / minBy) fail closed here so var
		// binds via javaStreamPipelineElemType → elemOf for orElse/ifPresent.
		// collectingAndThen(toList()/toSet(), l -> l.get(0)) returns scalar T when
		// the finisher extracts one element (dual-class under foreign same-leaf).
		// toList/toSet collect (identity finisher) is Collection — fails closed here
		// (elemOf path).
		if t := javaStreamCollectCollectingAndThenElemType(val, obj, content, elemOf, valOf); t != "" {
			return t
		}
		return javaStreamCollectReducingIdentityElemType(val, obj, content, elemOf, valOf)
	case "searchValues":
		// ConcurrentHashMap.searchValues(threshold, Function<? super V,? extends U>)
		// returns U. Identity Function (a -> a) yields V of the map receiver
		// (var xa = m.searchValues(1L, a -> a); xa.m() / m.searchValues(1L, a -> a).m()).
		// Non-identity / block Functions fail closed (U is not statically V).
		return javaSearchValuesReturnType(val, obj, content, elemOf, valOf)
	case "search":
		// ConcurrentHashMap.search(threshold, BiFunction<? super K,? super V,? extends U>)
		// returns U. Identity BiFunction ((k,v) -> v) yields V of the map receiver
		// (var xa = m.search(1L, (k,v) -> v); xa.m() / m.search(1L, (k,v) -> v).m()).
		// Non-identity / block BiFunctions fail closed (U is not statically V).
		return javaSearchReturnType(val, obj, content, elemOf, valOf)
	case "reduceValues":
		// ConcurrentHashMap.reduceValues(threshold, BiFunction<? super V,? super V,? extends V>)
		// returns V (2-arg reducer form). Threshold is ignored.
		// 3-arg form (transformer + reducer) returns U — only recover when the
		// unary transformer is identity so U = V; otherwise fail closed.
		return javaReduceValuesReturnType(val, obj, content, elemOf, valOf)
	case "searchEntries":
		// ConcurrentHashMap.searchEntries(threshold, Function<? super Entry,? extends U>)
		// returns U. Entry getValue mapper (e -> e.getValue()) yields V of the map
		// (var xa = m.searchEntries(1L, e -> e.getValue()); xa.m()).
		// Non-getValue / block Functions fail closed (U is not statically V).
		return javaSearchEntriesReturnType(val, obj, content, elemOf, valOf)
	case "reduceEntries":
		// ConcurrentHashMap.reduceEntries 3-arg (threshold, Function, BiFunction)
		// returns U. Entry getValue transformer (e -> e.getValue()) yields U=V of the map
		// (var xa = m.reduceEntries(1L, e -> e.getValue(), (a,b) -> a); xa.m() /
		//  m.reduceEntries(1L, e -> e.getValue(), (a,b) -> a).m()).
		// 2-arg form returns Entry — recovered via javaEntryExprValueType for .getValue().
		// Non-getValue transformers fail closed (U is not statically V).
		return javaReduceEntriesReturnType(val, obj, content, elemOf, valOf)
	default:
		return ""
	}
}

// javaSearchEntriesReturnType recovers U from ConcurrentHashMap.searchEntries when
// the Function is an expression-bodied Entry getValue mapper (e -> e.getValue()),
// so U = V of the map receiver. Threshold arg is ignored. Other Functions fail closed.
func javaSearchEntriesReturnType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	args := javaCallArgs(call)
	if len(args) < 2 {
		return ""
	}
	fn := args[len(args)-1]
	if !javaIsEntryGetValueLambda(fn, content) {
		return ""
	}
	return javaMapPipelineValueType(obj, content, elemOf, valOf)
}

// javaReduceEntriesReturnType recovers U from ConcurrentHashMap.reduceEntries 3-arg when
// the Function transformer is an expression-bodied Entry getValue mapper (e -> e.getValue()),
// so U = V of the map receiver. 2-arg form returns Entry (not V) — fail closed here
// (Entry V is recovered via javaEntryExprValueType under .getValue()). Other
// transformers fail closed.
func javaReduceEntriesReturnType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	args := javaCallArgs(call)
	if len(args) != 3 {
		return ""
	}
	// threshold + Function<? super Entry,? extends U> + BiFunction → U
	if !javaIsEntryGetValueLambda(args[1], content) {
		return ""
	}
	return javaMapPipelineValueType(obj, content, elemOf, valOf)
}

// javaIsEntryGetValueLambda reports expression-bodied unary lambdas of the form
// e -> e.getValue() (optionally parenthesized body). Used to recover U=V from
// ConcurrentHashMap.searchEntries getValue mappers. Blocks / other bodies fail closed.
func javaIsEntryGetValueLambda(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "lambda_expression" {
		return false
	}
	params := javaInferredLambdaParamNames(n, content)
	if len(params) != 1 {
		return false
	}
	body := ingest.ChildByField(n, "body")
	for body != nil && !body.IsNull() && body.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(body, "expression")
		if inner == nil {
			for i := uint32(0); i < body.ChildCount(); i++ {
				ch := body.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		body = inner
	}
	if body == nil || body.IsNull() || body.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(body, "name")
	objN := ingest.ChildByField(body, "object")
	if nameN == nil || objN == nil {
		return false
	}
	if ingest.NodeText(nameN, content) != "getValue" {
		return false
	}
	if len(javaCallArgs(body)) != 0 {
		return false
	}
	if objN.Type() != "identifier" && objN.Type() != "type_identifier" {
		return false
	}
	return ingest.NodeText(objN, content) == params[0]
}

// javaReduceValuesReturnType recovers V from ConcurrentHashMap.reduceValues:
// 2-arg (threshold, BiFunction) always returns V of the map;
// 3-arg (threshold, Function, BiFunction) returns U — identity Function only.
func javaReduceValuesReturnType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	args := javaCallArgs(call)
	switch len(args) {
	case 2:
		// threshold + BiFunction<? super V,? super V,? extends V> → V
		return javaMapPipelineValueType(obj, content, elemOf, valOf)
	case 3:
		// threshold + Function<? super V,? extends U> + BiFunction → U
		// Identity transformer: U = V.
		if javaIsIdentityLambda(args[1], content) {
			return javaMapPipelineValueType(obj, content, elemOf, valOf)
		}
		return ""
	default:
		return ""
	}
}

// javaSearchValuesReturnType recovers U from ConcurrentHashMap.searchValues when
// the Function is an identity lambda (a -> a), so U = V of the map receiver.
// Threshold arg is ignored. Non-identity Functions fail closed.
func javaSearchValuesReturnType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	args := javaCallArgs(call)
	if len(args) < 2 {
		return ""
	}
	// Function is the last arg (threshold is first).
	fn := args[len(args)-1]
	if !javaIsIdentityLambda(fn, content) {
		return ""
	}
	return javaMapPipelineValueType(obj, content, elemOf, valOf)
}

// javaSearchReturnType recovers U from ConcurrentHashMap.search when the
// BiFunction is a value-identity bi-lambda ((k,v) -> v), so U = V of the map.
// Threshold arg is ignored. Non-identity BiFunctions fail closed.
func javaSearchReturnType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	args := javaCallArgs(call)
	if len(args) < 2 {
		return ""
	}
	// BiFunction is the last arg (threshold is first).
	fn := args[len(args)-1]
	if !javaIsValueIdentityBiLambda(fn, content) {
		return ""
	}
	return javaMapPipelineValueType(obj, content, elemOf, valOf)
}

// javaCHMReduceReturnType recovers U from ConcurrentHashMap.reduce when the
// transformer BiFunction is a value-identity bi-lambda ((k,v) -> v), so U = V
// of the map receiver. Form: reduce(threshold, transformer, reducer) — 3 args,
// transformer is args[1]. Stream.reduce and non-identity transformers fail closed
// (no map V / non-identity body).
func javaCHMReduceReturnType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	args := javaCallArgs(call)
	if len(args) != 3 {
		return ""
	}
	// Transformer is the second arg (threshold first, reducer third).
	if !javaIsValueIdentityBiLambda(args[1], content) {
		return ""
	}
	return javaMapPipelineValueType(obj, content, elemOf, valOf)
}

// javaArrayAccessElemType recovers T from as[i] / (as)[i] / matrix[i][j] when the
// root array is a local tracked in elemOf (A[] as → "A"), from new A[]{...}[i]
// via the creation type, or from Stream/Collection.toArray()[i] via the pipeline
// element type. Index expressions do not change the element type leaf.
// Nested array_access peels to the root; unknown roots fail closed.
func javaArrayAccessElemType(val *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	for val != nil && !val.IsNull() {
		for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
			inner := ingest.ChildByField(val, "expression")
			if inner == nil {
				for i := uint32(0); i < val.ChildCount(); i++ {
					ch := val.Child(i)
					if ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					inner = ch
					break
				}
			}
			val = inner
		}
		if val == nil || val.IsNull() {
			return ""
		}
		if val.Type() != "array_access" {
			break
		}
		arr := ingest.ChildByField(val, "array")
		if arr == nil {
			return ""
		}
		val = arr
	}
	if val == nil || val.IsNull() {
		return ""
	}
	switch val.Type() {
	case "identifier":
		if elemOf == nil {
			return ""
		}
		return elemOf[ingest.NodeText(val, content)]
	case "array_creation_expression":
		// new A[]{...}[i] / new A[n][i] — element type from creation type.
		if typeN := ingest.ChildByField(val, "type"); typeN != nil {
			return javaTypeName(typeN, content)
		}
	case "method_invocation":
		// as.stream().toArray()[i] / as.stream().toArray(new A[0])[i] /
		// as.toArray(new A[0])[i] / var arr = as.stream().toArray(); arr[i] when
		// the root is still the call — element type from the stream/collection
		// pipeline (toArray is type-preserving).
		// Arrays.copyOf(as, n)[i] / Arrays.copyOfRange(as, from, to)[i] —
		// first-arg array element type (length/range only).
		return javaStreamPipelineElemType(val, content, elemOf, valOf)
	}
	return ""
}

// javaObjectsRequireNonNullElemType recovers T from Objects.requireNonNull(x[, msg]),
// requireNonNullElse(x, d), or requireNonNullElseGet(x, s) when x's type is
// statically recoverable without a scalar local type map: new T(...) / T.make() /
// cast / collection-accessor / field access. Bare typed locals use
// javaObjectsRequireNonNullArgIdent instead.
func javaObjectsRequireNonNullElemType(call, obj *grammar.Node, content []byte, elemOf, valOf, entryValOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf map[string]string) string {
	if call == nil || obj == nil {
		return ""
	}
	if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(obj, content) != "Objects" {
		return ""
	}
	first := javaFirstMethodArg(call)
	if first == nil {
		return ""
	}
	// Prefer collection/field accessors before javaInferExprType: the latter treats
	// any ident.method() as type ident (A.make factory convention), which would
	// mis-type as.get(0) as "as" instead of elemOf[as].
	if t := javaCollectionAccessElemType(first, content, elemOf, valOf, entryValOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, nil); t != "" {
		return t
	}
	if t := javaFieldAccessMemberType(first, content, compOf, nil); t != "" {
		return t
	}
	// new A() / A.make() / (A) x / ternary/switch of those.
	if t := javaInferExprType(first, content); t != "" {
		return t
	}
	return ""
}

// javaObjectsRequireNonNullArgIdent returns the first-arg identifier of
// Objects.requireNonNull(id[, msg]) / requireNonNullElse(id, d) /
// requireNonNullElseGet(id, s) when that arg is a bare identifier.
// Empty when the call is not one of those Objects methods or the arg is not an id.
func javaObjectsRequireNonNullArgIdent(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil {
		return ""
	}
	switch ingest.NodeText(nameN, content) {
	case "requireNonNull", "requireNonNullElse", "requireNonNullElseGet":
	default:
		return ""
	}
	obj := ingest.ChildByField(call, "object")
	if obj == nil {
		return ""
	}
	if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(obj, content) != "Objects" {
		return ""
	}
	first := javaFirstMethodArg(call)
	if first == nil || first.Type() != "identifier" {
		return ""
	}
	return ingest.NodeText(first, content)
}

// javaFirstMethodArg returns the first real argument of a method_invocation.
func javaFirstMethodArg(call *grammar.Node) *grammar.Node {
	if call == nil {
		return nil
	}
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() != "argument_list" {
			continue
		}
		al := call.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			ch := al.Child(j)
			switch ch.Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				return ch
			}
		}
	}
	return nil
}

// javaFunctionIdentityApplyType recovers T from Function.identity().apply(x) /
// Function.<T>identity().apply(x) / java.util.function.Function.identity().apply(x)
// when x peels to concrete T (new T() / typed local / pipeline). Identity is
// Function<T,T> so apply returns the arg type. Enables
// Function.identity().apply(new A()).run() under foreign same-leaf.
// Non-identity / multi-arg apply fail closed.
func javaFunctionIdentityApplyType(call *grammar.Node, content []byte, elemOf, valOf, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "apply" {
		return ""
	}
	// Exactly one apply arg.
	argCount := 0
	var firstArg *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() != "argument_list" {
			continue
		}
		al := call.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			ch := al.Child(j)
			switch ch.Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				argCount++
				if firstArg == nil {
					firstArg = ch
				}
			}
		}
	}
	if argCount != 1 || firstArg == nil {
		return ""
	}
	obj := ingest.ChildByField(call, "object")
	if obj == nil || obj.Type() != "method_invocation" {
		return ""
	}
	idName := ingest.ChildByField(obj, "name")
	if idName == nil || ingest.NodeText(idName, content) != "identity" {
		return ""
	}
	// Zero-arg identity() only.
	if javaMethodCallArgCount(obj) != 0 {
		return ""
	}
	idObj := ingest.ChildByField(obj, "object")
	if idObj == nil || !javaIsFunctionTypeName(idObj, content) {
		return ""
	}
	// Apply arg: new A() / ba.get() / typed local / collection access / pipeline.
	if t := javaObjectArgType(firstArg, content, compOf, typeMembers); t != "" {
		return t
	}
	if t := javaCollectionAccessElemType(firstArg, content, elemOf, valOf, nil, compOf, nil, nil, nil, nil, typeMembers); t != "" {
		return t
	}
	if t := javaInferExprType(firstArg, content); t != "" {
		return t
	}
	if firstArg.Type() == "identifier" && elemOf != nil {
		if t := elemOf[ingest.NodeText(firstArg, content)]; t != "" {
			return t
		}
	}
	return ""
}

// javaIsFunctionTypeName reports Function / java.util.function.Function receivers
// of Function.identity() (type_identifier, identifier, or scoped type).
func javaIsFunctionTypeName(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "identifier", "type_identifier":
		return ingest.NodeText(n, content) == "Function"
	case "scoped_type_identifier", "field_access":
		// java.util.function.Function or Function nested scope — leaf must be Function.
		return strings.HasSuffix(ingest.NodeText(n, content), "Function")
	case "generic_type":
		// Function<A,A> — peel type name.
		if name := ingest.ChildByField(n, "type"); name != nil {
			return javaIsFunctionTypeName(name, content)
		}
	}
	return false
}

// javaFunctionIdentityApplyArgIdent returns the bare identifier apply-arg of
// Function.identity().apply(id) when the receiver is Function.identity().
// Empty when not that shape or the arg is not an identifier.
func javaFunctionIdentityApplyArgIdent(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "apply" {
		return ""
	}
	obj := ingest.ChildByField(call, "object")
	if obj == nil || obj.Type() != "method_invocation" {
		return ""
	}
	idName := ingest.ChildByField(obj, "name")
	if idName == nil || ingest.NodeText(idName, content) != "identity" {
		return ""
	}
	if javaMethodCallArgCount(obj) != 0 {
		return ""
	}
	idObj := ingest.ChildByField(obj, "object")
	if idObj == nil || !javaIsFunctionTypeName(idObj, content) {
		return ""
	}
	first := javaFirstMethodArg(call)
	if first == nil || first.Type() != "identifier" {
		return ""
	}
	return ingest.NodeText(first, content)
}

// javaRecordComponentAccessType recovers T from ba.a() when ba is a record local
// with component a of type T (zero-arg accessor only; fail closed on args).
// Also peels same-file zero-arg methods (ba.get() / ba.self().a()) via typeMembers
// under foreign same-leaf methods.
func javaRecordComponentAccessType(call, obj *grammar.Node, method string, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if call == nil || method == "" || obj == nil {
		return ""
	}
	if javaMethodCallArgCount(call) != 0 {
		return ""
	}
	// Peel (ba.self()).get() — parenthesized zero-arg method receiver under
	// foreign same-leaf (bare ba.self().get already peels).
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		obj = inner
	}
	if obj == nil || obj.IsNull() {
		return ""
	}
	// ba.a() / ba.get() when ba is a known record/class local.
	// A.create() — static/instance zero-arg method on class name (typeMembers["A"]).
	if (obj.Type() == "identifier" || obj.Type() == "type_identifier") && (compOf != nil || typeMembers != nil) {
		name := ingest.NodeText(obj, content)
		if compOf != nil {
			if t := compOf[name+"."+method]; t != "" {
				return t
			}
		}
		// Class-qualified static factory: A.create() → typeMembers[A][create].
		// Prefer local compOf above so shadowed locals stay local peels.
		if typeMembers != nil {
			if comps := typeMembers[name]; comps != nil {
				if t := comps[method]; t != "" {
					return t
				}
			}
		}
	}
	// new BoxA(new A()).a() — component type from same-file record/class header.
	// Under foreign same-leaf methods (assigned ba.a() already peels via compOf).
	if obj.Type() == "object_creation_expression" && typeMembers != nil {
		if typeN := ingest.ChildByField(obj, "type"); typeN != nil {
			tn := javaTypeName(typeN, content)
			if tn != "" {
				if comps := typeMembers[tn]; comps != nil {
					return comps[method]
				}
			}
		}
	}
	// ((BoxA) x).a() — cast to known record/class type.
	if obj.Type() == "cast_expression" && typeMembers != nil {
		if typeN := ingest.ChildByField(obj, "type"); typeN != nil {
			tn := javaTypeName(typeN, content)
			if tn != "" {
				if comps := typeMembers[tn]; comps != nil {
					return comps[method]
				}
			}
		}
	}
	// oa.h.get() / oa.h.box.get() — field access peel then zero-arg method return
	// (dual-class; nested fields via typeMembers).
	// oa is a typed local with field h of type HolderA; typeMembers[HolderA][get]=A.
	if obj.Type() == "field_access" && typeMembers != nil {
		if ft := javaFieldAccessMemberType(obj, content, compOf, typeMembers); ft != "" {
			if comps := typeMembers[ft]; comps != nil {
				if t := comps[method]; t != "" {
					return t
				}
			}
		}
	}
	// ba.self().a() / ba.self().get() — peel zero-arg method return type then
	// look up member on that type (dual-class under foreign same-leaf).
	if obj.Type() == "method_invocation" && typeMembers != nil {
		if rt := javaRecordComponentAccessType(obj, ingest.ChildByField(obj, "object"),
			javaMethodInvocationName(obj, content), content, compOf, typeMembers); rt != "" {
			if comps := typeMembers[rt]; comps != nil {
				return comps[method]
			}
		}
	}
	// (c ? ba : ba).get() — both arms peel to the same method-return T under
	// foreign same-leaf (typed-local / method-return / ctor arms).
	if obj.Type() == "ternary_expression" {
		cons := ingest.ChildByField(obj, "consequence")
		alt := ingest.ChildByField(obj, "alternative")
		if cons != nil && alt != nil {
			t1 := javaRecordComponentAccessType(call, cons, method, content, compOf, typeMembers)
			t2 := javaRecordComponentAccessType(call, alt, method, content, compOf, typeMembers)
			if t1 != "" && t1 == t2 {
				return t1
			}
		}
	}
	return ""
}

// javaMethodInvocationName returns the method leaf of a method_invocation.
func javaMethodInvocationName(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil {
		return ""
	}
	return ingest.NodeText(nameN, content)
}

// javaOptionalOfMethodReturnType recovers T from Optional.of(ba.get()).get() /
// ofNullable(ba.get()).orElseThrow() when ba.get() is a same-file zero-arg method
// with concrete return T (typeMembers). Bare Optional.of(a) uses typedLocals
// via javaOptionalOfIdentUnwrap. Multi-arg / non-Optional / unknown methods fail closed.
func javaOptionalOfMethodReturnType(call *grammar.Node, content []byte, compOf map[string]string, typeMembers map[string]map[string]string) string {
	if call == nil || call.Type() != "method_invocation" || typeMembers == nil {
		return ""
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil {
		return ""
	}
	switch ingest.NodeText(nameN, content) {
	case "get", "orElseThrow", "orElse", "orElseGet":
		// ok — Optional unwrap
	default:
		return ""
	}
	method := ingest.NodeText(nameN, content)
	if method == "get" || method == "orElseThrow" {
		if !javaCallIsZeroArg(call) {
			return ""
		}
	}
	opt := ingest.ChildByField(call, "object")
	for opt != nil && !opt.IsNull() && opt.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(opt, "expression")
		if inner == nil {
			for i := uint32(0); i < opt.ChildCount(); i++ {
				ch := opt.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		opt = inner
	}
	if opt == nil || opt.Type() != "method_invocation" {
		return ""
	}
	optName := ingest.ChildByField(opt, "name")
	optRecv := ingest.ChildByField(opt, "object")
	if optName == nil || optRecv == nil {
		return ""
	}
	switch ingest.NodeText(optName, content) {
	case "of", "ofNullable":
		// ok
	default:
		return ""
	}
	if javaStaticFactoryReceiverName(optRecv, content) != "Optional" {
		return ""
	}
	first := javaFirstMethodArg(opt)
	if first == nil || first.Type() != "method_invocation" {
		return ""
	}
	// ba.get() / oa.h.get() — zero-arg product method return.
	return javaRecordComponentAccessType(first, ingest.ChildByField(first, "object"),
		javaMethodInvocationName(first, content), content, compOf, typeMembers)
}

// javaSameFileMethodReturns maps same-file class/record type → zero-arg method
// name → concrete return type leaf:
//
//	class HolderA { A get() { return item; } }
//	record BoxA(A a) { BoxA self() { return this; } }
//
// Enables ba.get().run() / ba.self().a().run() under foreign same-leaf methods.
// Methods with parameters fail closed (not a lone product accessor).
func javaSameFileMethodReturns(root *grammar.Node, content []byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "class_declaration" || n.Type() == "record_declaration" {
			nameN := ingest.ChildByField(n, "name")
			body := ingest.ChildByField(n, "body")
			if nameN != nil && body != nil {
				typeName := ingest.NodeText(nameN, content)
				methods := map[string]string{}
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					if ch == nil || ch.Type() != "method_declaration" {
						continue
					}
					// Zero formal parameters only.
					params := ingest.ChildByField(ch, "parameters")
					if params == nil {
						// Some grammars use formal_parameters.
						for j := uint32(0); j < ch.ChildCount(); j++ {
							c := ch.Child(j)
							if c != nil && (c.Type() == "formal_parameters" || c.Type() == "parameters") {
								params = c
								break
							}
						}
					}
					if params != nil {
						hasParam := false
						for j := uint32(0); j < params.ChildCount(); j++ {
							c := params.Child(j)
							if c == nil {
								continue
							}
							switch c.Type() {
							case "(", ")", ",", "comment":
								continue
							default:
								hasParam = true
							}
						}
						if hasParam {
							continue
						}
					}
					retN := ingest.ChildByField(ch, "type")
					mNameN := ingest.ChildByField(ch, "name")
					if retN == nil || mNameN == nil {
						continue
					}
					ret := javaTypeName(retN, content)
					mname := ingest.NodeText(mNameN, content)
					if ret == "" || mname == "" || ret == "void" {
						continue
					}
					// Skip primitive returns (not product method receivers).
					switch ret {
					case "boolean", "byte", "short", "int", "long", "float", "double", "char":
						continue
					}
					methods[mname] = ret
				}
				if len(methods) > 0 {
					out[typeName] = methods
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

// javaStreamReduceIdentityElemType recovers T from stream.reduce(identity, op[, comb]).
// Identity forms have ≥2 args and a non-lambda first arg. One-arg reduce returns
// Optional and is handled via pipeline orElse/ifPresent instead.
func javaStreamReduceIdentityElemType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || obj == nil {
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	nReal := 0
	var first *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			nReal++
			if first == nil {
				first = ch
			}
		}
	}
	// reduce(op) → Optional; reduce(identity, op) / reduce(identity, acc, comb) → T/U.
	if nReal < 2 || first == nil || first.Type() == "lambda_expression" {
		return ""
	}
	return javaStreamPipelineElemType(obj, content, elemOf, valOf)
}

// javaIsReducingMaxMinCollector reports Stream.collect args that yield Optional<T>
// or T of the stream element type: Collectors.reducing(...) / maxBy(...) / minBy(...)
// (or static-import reducing/maxBy/minBy). Type-changing 3-arg reducing (non-identity
// mapper) still reports true for pipeline peel — scalar identity return recovery is
// gated separately in javaStreamCollectReducingIdentityElemType.
func javaIsReducingMaxMinCollector(collectCall *grammar.Node, content []byte) bool {
	first := javaCollectFirstArg(collectCall)
	return javaIsReducingMaxMinCollectorExpr(first, content)
}

// javaIsReducingMaxMinCollectorExpr reports a reducing/maxBy/minBy collector expression.
func javaIsReducingMaxMinCollectorExpr(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil {
		return false
	}
	switch ingest.NodeText(nameN, content) {
	case "reducing", "maxBy", "minBy":
	default:
		return false
	}
	if obj := ingest.ChildByField(n, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return false
		}
	}
	// At least one arg (op / comparator / identity+op).
	return len(javaCallArgs(n)) >= 1
}

// javaStreamCollectCollectingAndThenElemType recovers T from
// stream.collect(Collectors.collectingAndThen(toList()/toSet()/toUnmodifiable…/
// toCollection(…), finisher)) when finisher clearly extracts one collection
// element: l -> l.get(0) / l.getFirst() / l.iterator().next() / List::getFirst.
// Result is scalar stream element T (not Collection). Identity finishers
// (xs -> xs / Collections::unmodifiableList) stay on the toList Collection path.
// Enables collect(...).run() / var xa = collect(...) under foreign same-leaf.
func javaStreamCollectCollectingAndThenElemType(collectCall, streamObj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if collectCall == nil || streamObj == nil {
		return ""
	}
	first := javaCollectFirstArg(collectCall)
	if first == nil || first.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(first, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "collectingAndThen" {
		return ""
	}
	if obj := ingest.ChildByField(first, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return ""
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return ""
		}
	}
	args := javaCallArgs(first)
	if len(args) != 2 {
		return ""
	}
	// Downstream must be toList/toSet/toUnmodifiable/toCollection.
	if !javaIsToListOrSetCollectorExpr(args[0], content) {
		return ""
	}
	// Finisher must extract one element of the collected Collection.
	if !javaIsCollectionElemFinisher(args[1], content) {
		return ""
	}
	return javaStreamPipelineElemType(streamObj, content, elemOf, valOf)
}

// javaIsCollectionElemFinisher reports finishers that yield one element of a
// List/Set/Collection: l -> l.get(0) / l.getFirst() / l.iterator().next() /
// List::getFirst. Other finishers (identity, unmodifiableList, …) fail closed.
func javaIsCollectionElemFinisher(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "lambda_expression":
		// Unary lambda: l -> <body>
		params := javaInferredLambdaParamNames(n, content)
		if len(params) != 1 || params[0] == "" {
			return false
		}
		body := javaLambdaExprBody(n)
		if body == nil {
			// Fall back to body field when helper is shape-specific.
			body = ingest.ChildByField(n, "body")
		}
		if body == nil {
			return false
		}
		return javaIsCollectionElemExtract(body, content, params[0])
	case "method_reference":
		// List::getFirst / Collection::getFirst (Java 21 SequencedCollection).
		var parts []*grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "::" {
				continue
			}
			parts = append(parts, ch)
		}
		if len(parts) < 2 {
			return false
		}
		obj, name := parts[0], parts[len(parts)-1]
		if name.Type() != "identifier" || ingest.NodeText(name, content) != "getFirst" {
			return false
		}
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return false
		}
		switch ingest.NodeText(obj, content) {
		case "List", "SequencedCollection", "Deque", "LinkedList", "ArrayList":
			return true
		}
		return false
	default:
		return false
	}
}

// javaIsCollectionElemExtract reports expressions that extract one element from
// collection param p: p.get(0) / p.getFirst() / p.iterator().next().
func javaIsCollectionElemExtract(n *grammar.Node, content []byte, param string) bool {
	if n == nil || param == "" {
		return false
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(n, "expression")
		if inner == nil {
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		n = inner
	}
	if n == nil || n.Type() != "method_invocation" {
		return false
	}
	nameN := ingest.ChildByField(n, "name")
	obj := ingest.ChildByField(n, "object")
	if nameN == nil || obj == nil {
		return false
	}
	name := ingest.NodeText(nameN, content)
	switch name {
	case "get":
		// p.get(0) — single integer-literal 0 arg.
		if obj.Type() != "identifier" || ingest.NodeText(obj, content) != param {
			return false
		}
		args := javaCallArgs(n)
		if len(args) != 1 {
			return false
		}
		return javaIsIntegerZero(args[0], content)
	case "getFirst":
		// p.getFirst() — zero-arg.
		if obj.Type() != "identifier" || ingest.NodeText(obj, content) != param {
			return false
		}
		return javaCallIsZeroArg(n)
	case "next":
		// p.iterator().next() — zero-arg next on iterator() of param.
		if !javaCallIsZeroArg(n) {
			return false
		}
		if obj.Type() != "method_invocation" {
			return false
		}
		itName := ingest.ChildByField(obj, "name")
		itObj := ingest.ChildByField(obj, "object")
		if itName == nil || itObj == nil || ingest.NodeText(itName, content) != "iterator" {
			return false
		}
		if !javaCallIsZeroArg(obj) {
			return false
		}
		return itObj.Type() == "identifier" && ingest.NodeText(itObj, content) == param
	default:
		return false
	}
}

// javaIsIntegerZero reports integer literal 0 (decimal).
func javaIsIntegerZero(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(n, "expression")
		if inner == nil {
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		n = inner
	}
	if n == nil {
		return false
	}
	if n.Type() == "decimal_integer_literal" || n.Type() == "integer_literal" || n.Type() == "decimal_floating_point_literal" {
		return ingest.NodeText(n, content) == "0"
	}
	// Some grammars use generic "number" / bare text "0".
	return ingest.NodeText(n, content) == "0"
}

// javaStreamCollectReducingIdentityElemType recovers T from
// stream.collect(Collectors.reducing(identity, op)) / reducing(identity, mapper, op)
// when the result is T (not Optional). One-arg reducing / maxBy / minBy return
// Optional and fail closed here. 3-arg requires identity mapper (a -> a) so U=T.
func javaStreamCollectReducingIdentityElemType(collectCall, streamObj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if collectCall == nil || streamObj == nil {
		return ""
	}
	first := javaCollectFirstArg(collectCall)
	if first == nil || first.Type() != "method_invocation" {
		return ""
	}
	nameN := ingest.ChildByField(first, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "reducing" {
		return ""
	}
	if obj := ingest.ChildByField(first, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return ""
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return ""
		}
	}
	args := javaCallArgs(first)
	switch len(args) {
	case 2:
		// reducing(identity, BinaryOperator) → T. First arg must not be the op lambda.
		if args[0].Type() == "lambda_expression" {
			return ""
		}
		return javaStreamPipelineElemType(streamObj, content, elemOf, valOf)
	case 3:
		// reducing(identity, mapper, BinaryOperator) → U. Identity mapper only (U=T).
		if args[0].Type() == "lambda_expression" {
			return ""
		}
		if !javaIsIdentityLambda(args[1], content) {
			return ""
		}
		return javaStreamPipelineElemType(streamObj, content, elemOf, valOf)
	default:
		// reducing(BinaryOperator) → Optional<T>; maxBy/minBy not here.
		return ""
	}
}

// javaBindCollectorsReducingLambdaParams binds BinaryOperator / identity-mapper
// params on collect(Collectors.reducing(...)) from the stream element type of the
// collect receiver. maxBy/minBy have no T BinaryOperator at the collector top level
// (Comparator only). Type-changing 3-arg mappers fail closed on the reducer.
func javaBindCollectorsReducingLambdaParams(collectCall *grammar.Node, content []byte, ourReceivers map[string]bool, elemOf, valOf map[string]string, out map[string]bool) {
	if collectCall == nil || out == nil {
		return
	}
	streamObj := ingest.ChildByField(collectCall, "object")
	et := javaStreamPipelineElemType(streamObj, content, elemOf, valOf)
	if et == "" || !ourReceivers[et] {
		return
	}
	first := javaCollectFirstArg(collectCall)
	if first == nil || first.Type() != "method_invocation" {
		return
	}
	nameN := ingest.ChildByField(first, "name")
	if nameN == nil || ingest.NodeText(nameN, content) != "reducing" {
		return
	}
	if obj := ingest.ChildByField(first, "object"); obj != nil {
		if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
			return
		}
		if ingest.NodeText(obj, content) != "Collectors" {
			return
		}
	}
	args := javaCallArgs(first)
	bindBi := func(n *grammar.Node) {
		if n == nil || n.Type() != "lambda_expression" {
			return
		}
		params := javaInferredLambdaParamNames(n, content)
		if len(params) != 2 {
			return
		}
		out[params[0]] = true
		out[params[1]] = true
	}
	bindUnary := func(n *grammar.Node) {
		if n == nil || n.Type() != "lambda_expression" {
			return
		}
		params := javaInferredLambdaParamNames(n, content)
		if len(params) != 1 {
			return
		}
		out[params[0]] = true
	}
	switch len(args) {
	case 1:
		// reducing(BinaryOperator<? super T>)
		bindBi(args[0])
	case 2:
		// reducing(identity, BinaryOperator)
		bindBi(args[1])
	case 3:
		// reducing(identity, mapper, BinaryOperator on U) — identity mapper only.
		if javaIsIdentityLambda(args[1], content) {
			bindUnary(args[1])
			bindBi(args[2])
		}
	}
}

// javaMethodCallArgCount returns the number of real arguments on a
// method_invocation (skips punctuation/comments in the argument_list).
// Missing or non-call nodes yield 0.
func javaMethodCallArgCount(call *grammar.Node) int {
	if call == nil {
		return 0
	}
	n := 0
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() != "argument_list" {
			continue
		}
		al := call.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			switch al.Child(j).Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				n++
			}
		}
	}
	return n
}

// javaCollectionsMinMaxElemType recovers T from Collections.min/max(coll[, cmp]).
// First argument is the collection; Comparator does not change the element type.
// Non-Collections receivers fail closed (Stream.min returns Optional, not T).
func javaCollectionsMinMaxElemType(call, obj *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
	if call == nil || obj == nil {
		return ""
	}
	if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(obj, content) != "Collections" {
		return ""
	}
	var args *grammar.Node
	for i := uint32(0); i < call.ChildCount(); i++ {
		if call.Child(i).Type() == "argument_list" {
			args = call.Child(i)
			break
		}
	}
	if args == nil {
		return ""
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			// Collections.min(as) / Collections.min(as, cmp) — element of first arg.
			return javaStreamPipelineElemType(ch, content, elemOf, valOf)
		}
	}
	return ""
}

// javaInferExprType recovers a simple type leaf from a var initializer expression.
// Covers new A(), A.make() / A.getInstance() static calls, (A) expr casts,
// homogeneous ternaries (f ? new A() : A.make()), and switch expressions whose
// arms all infer the same type (arrow and yield forms).
// Static Type.method() is treated as returning Type (common factory convention);
// fail closed on other shapes / mixed arms.
// Collection accessors (list.get / map.get / iterator.next) are handled separately
// via javaCollectionAccessElemType (needs elemOf/valOf).
func javaInferExprType(val *grammar.Node, content []byte) string {
	if val == nil {
		return ""
	}
	// Unwrap (new A()) / (A.make()).
	for val != nil && !val.IsNull() && val.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(val, "expression")
		if inner == nil {
			for i := uint32(0); i < val.ChildCount(); i++ {
				ch := val.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		if inner == nil {
			return ""
		}
		val = inner
	}
	if val == nil || val.IsNull() {
		return ""
	}
	switch val.Type() {
	case "object_creation_expression":
		if vt := ingest.ChildByField(val, "type"); vt != nil {
			return javaTypeName(vt, content)
		}
	case "method_invocation":
		// A.make() — object is the type name; treat return as that type.
		obj := ingest.ChildByField(val, "object")
		if obj == nil {
			return ""
		}
		if obj.Type() == "identifier" || obj.Type() == "type_identifier" {
			return ingest.NodeText(obj, content)
		}
	case "cast_expression":
		if typeN := ingest.ChildByField(val, "type"); typeN != nil {
			return javaTypeName(typeN, content)
		}
	case "ternary_expression":
		// f ? new A() : A.make() — both arms must agree.
		cons := ingest.ChildByField(val, "consequence")
		alt := ingest.ChildByField(val, "alternative")
		t1 := javaInferExprType(cons, content)
		t2 := javaInferExprType(alt, content)
		if t1 != "" && t1 == t2 {
			return t1
		}
		return ""
	case "switch_expression":
		return javaInferSwitchExprType(val, content)
	case "expression_statement":
		// switch arrow arms wrap values as expression_statement ("new A();").
		return javaInferExprType(javaFirstExprChild(val), content)
	}
	return ""
}

// javaInferSwitchExprType recovers the type of a switch expression when every
// arm yields the same inferable type (case 0 -> new A(); / yield A.make()).
// Mixed types or uninferable arms fail closed.
func javaInferSwitchExprType(sw *grammar.Node, content []byte) string {
	if sw == nil || sw.Type() != "switch_expression" {
		return ""
	}
	body := ingest.ChildByField(sw, "body")
	if body == nil {
		for i := uint32(0); i < sw.ChildCount(); i++ {
			if sw.Child(i).Type() == "switch_block" {
				body = sw.Child(i)
				break
			}
		}
	}
	if body == nil {
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < body.ChildCount(); i++ {
		ch := body.Child(i)
		var armExpr *grammar.Node
		switch ch.Type() {
		case "switch_rule":
			// case 0 -> new A();  /  case 0 -> { yield new A(); }
			armExpr = javaSwitchRuleResult(ch)
		case "switch_block_statement_group":
			// case 0: yield new A();
			armExpr = javaSwitchGroupYieldExpr(ch)
		default:
			continue
		}
		if armExpr == nil {
			return ""
		}
		tn := javaInferExprType(armExpr, content)
		if tn == "" {
			return ""
		}
		if !saw {
			elem = tn
			saw = true
			continue
		}
		if tn != elem {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// javaSwitchExprAllArmsRename reports whether every switch arm's result should
// rename as ours under foreign same-leaf methods (typed-local / construction
// arms that javaShouldRenameMemberAccess accepts). Empty / uninferable arms
// fail closed.
func javaSwitchExprAllArmsRename(sw *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf map[string]string, typeMembers map[string]map[string]string, implementsEdges map[string]map[string]bool) bool {
	if sw == nil || sw.Type() != "switch_expression" {
		return false
	}
	body := ingest.ChildByField(sw, "body")
	if body == nil {
		for i := uint32(0); i < sw.ChildCount(); i++ {
			if sw.Child(i).Type() == "switch_block" {
				body = sw.Child(i)
				break
			}
		}
	}
	if body == nil {
		return false
	}
	saw := false
	for i := uint32(0); i < body.ChildCount(); i++ {
		ch := body.Child(i)
		var armExpr *grammar.Node
		switch ch.Type() {
		case "switch_rule":
			armExpr = javaSwitchRuleResult(ch)
		case "switch_block_statement_group":
			armExpr = javaSwitchGroupYieldExpr(ch)
		default:
			continue
		}
		if armExpr == nil {
			return false
		}
		if !javaShouldRenameMemberAccess(armExpr, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, entryValOf, valOf, elemOf, compOf, windowStreamOf, windowOptOf, groupValOf, entryGroupOf, typeMembers, implementsEdges) {
			return false
		}
		saw = true
	}
	return saw
}

// javaSwitchExprTypedLocalType recovers T when every switch arm is an identifier
// already bound in typedLocals (our receivers). Used for var xa = switch (...) {
// case 0 -> a; default -> x; } under foreign same-leaf. Mixed / foreign / empty fail closed.
func javaSwitchExprTypedLocalType(sw *grammar.Node, content []byte, typedLocals map[string]bool) bool {
	if sw == nil || sw.Type() != "switch_expression" || typedLocals == nil {
		return false
	}
	body := ingest.ChildByField(sw, "body")
	if body == nil {
		for i := uint32(0); i < sw.ChildCount(); i++ {
			if sw.Child(i).Type() == "switch_block" {
				body = sw.Child(i)
				break
			}
		}
	}
	if body == nil {
		return false
	}
	saw := false
	for i := uint32(0); i < body.ChildCount(); i++ {
		ch := body.Child(i)
		var armExpr *grammar.Node
		switch ch.Type() {
		case "switch_rule":
			armExpr = javaSwitchRuleResult(ch)
		case "switch_block_statement_group":
			armExpr = javaSwitchGroupYieldExpr(ch)
		default:
			continue
		}
		if armExpr == nil {
			return false
		}
		// Unwrap parens.
		for armExpr != nil && armExpr.Type() == "parenthesized_expression" {
			inner := ingest.ChildByField(armExpr, "expression")
			if inner == nil {
				for j := uint32(0); j < armExpr.ChildCount(); j++ {
					c := armExpr.Child(j)
					if c.Type() == "(" || c.Type() == ")" {
						continue
					}
					inner = c
					break
				}
			}
			armExpr = inner
		}
		if armExpr == nil || armExpr.Type() != "identifier" {
			return false
		}
		if !typedLocals[ingest.NodeText(armExpr, content)] {
			return false
		}
		saw = true
	}
	return saw
}

// javaTernaryTypedLocalType reports whether both ternary arms are identifiers
// bound in typedLocals (our receivers). Used for var xa = c ? a : x under
// foreign same-leaf methods. Mixed / foreign fail closed.
func javaTernaryTypedLocalType(n *grammar.Node, content []byte, typedLocals map[string]bool) bool {
	if n == nil || n.Type() != "ternary_expression" || typedLocals == nil {
		return false
	}
	cons := ingest.ChildByField(n, "consequence")
	alt := ingest.ChildByField(n, "alternative")
	return javaIdentInTypedLocals(cons, content, typedLocals) && javaIdentInTypedLocals(alt, content, typedLocals)
}

// javaIdentInTypedLocals reports whether n (after paren unwrap) is an identifier
// present in typedLocals.
func javaIdentInTypedLocals(n *grammar.Node, content []byte, typedLocals map[string]bool) bool {
	if n == nil || typedLocals == nil {
		return false
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(n, "expression")
		if inner == nil {
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		n = inner
	}
	if n == nil || n.Type() != "identifier" {
		return false
	}
	return typedLocals[ingest.NodeText(n, content)]
}

// javaSwitchRuleResult returns the result expression of a switch_rule arm.
func javaSwitchRuleResult(rule *grammar.Node) *grammar.Node {
	if rule == nil || rule.Type() != "switch_rule" {
		return nil
	}
	// Children: switch_label, "->", expression_statement | block | throw_statement.
	var afterArrow *grammar.Node
	seenArrow := false
	for i := uint32(0); i < rule.ChildCount(); i++ {
		ch := rule.Child(i)
		if ch.Type() == "->" {
			seenArrow = true
			continue
		}
		if !seenArrow {
			continue
		}
		afterArrow = ch
		break
	}
	if afterArrow == nil {
		return nil
	}
	switch afterArrow.Type() {
	case "expression_statement":
		return javaFirstExprChild(afterArrow)
	case "block":
		// case 0 -> { yield new A(); }
		return javaBlockSoleYieldExpr(afterArrow)
	default:
		// Bare expression (some grammars) or throw — try infer, else fail closed.
		return afterArrow
	}
}

// javaSwitchGroupYieldExpr returns the expression of a sole yield in a
// classic switch_block_statement_group (case 0: yield new A();).
func javaSwitchGroupYieldExpr(group *grammar.Node) *grammar.Node {
	if group == nil {
		return nil
	}
	var yieldExpr *grammar.Node
	for i := uint32(0); i < group.ChildCount(); i++ {
		ch := group.Child(i)
		if ch.Type() != "yield_statement" {
			continue
		}
		if yieldExpr != nil {
			// Multiple yields — fail closed.
			return nil
		}
		yieldExpr = javaFirstExprChild(ch)
	}
	return yieldExpr
}

// javaBlockSoleYieldExpr returns the expression of a single yield_statement in a block.
func javaBlockSoleYieldExpr(block *grammar.Node) *grammar.Node {
	if block == nil || block.Type() != "block" {
		return nil
	}
	var yieldExpr *grammar.Node
	for i := uint32(0); i < block.ChildCount(); i++ {
		ch := block.Child(i)
		if ch.Type() == "{" || ch.Type() == "}" || ch.Type() == "comment" {
			continue
		}
		if ch.Type() != "yield_statement" {
			return nil
		}
		if yieldExpr != nil {
			return nil
		}
		yieldExpr = javaFirstExprChild(ch)
	}
	return yieldExpr
}

// javaFirstExprChild returns the first non-punctuation child of n (expression under
// expression_statement / yield_statement).
func javaFirstExprChild(n *grammar.Node) *grammar.Node {
	if n == nil {
		return nil
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "(", ")", "{", "}", ";", ":", "->", "yield", "comment", "case", "default", "switch_label":
			continue
		default:
			return ch
		}
	}
	return nil
}

// javaBindTypePattern records a type_pattern binding (Type name) when Type is ours.
func javaBindTypePattern(n *grammar.Node, content []byte, ourReceivers, out map[string]bool) {
	if n == nil || n.Type() != "type_pattern" {
		return
	}
	var typeN, nameN *grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "type_identifier", "generic_type", "scoped_type_identifier", "array_type", "integral_type", "floating_point_type", "boolean_type":
			typeN = ch
		case "identifier":
			nameN = ch
		}
	}
	if typeN == nil || nameN == nil {
		return
	}
	if tn := javaTypeName(typeN, content); ourReceivers[tn] {
		out[ingest.NodeText(nameN, content)] = true
	}
}

// javaBindCatchFormal records catch (Type e) when Type is a single our-receiver.
// Multi-catch (A | B e) is skipped: the static type is the lub, not either arm.
func javaBindCatchFormal(n *grammar.Node, content []byte, ourReceivers, out map[string]bool) {
	if n == nil || n.Type() != "catch_formal_parameter" {
		return
	}
	nameN := ingest.ChildByField(n, "name")
	if nameN == nil {
		nameN = ingest.ChildByType(n, "identifier")
	}
	if nameN == nil {
		return
	}
	var catchType *grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.Child(i).Type() == "catch_type" {
			catchType = n.Child(i)
			break
		}
	}
	if catchType == nil {
		return
	}
	var typeNames []string
	for i := uint32(0); i < catchType.ChildCount(); i++ {
		ch := catchType.Child(i)
		switch ch.Type() {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			if tn := javaTypeName(ch, content); tn != "" {
				typeNames = append(typeNames, tn)
			}
		}
	}
	if len(typeNames) == 1 && ourReceivers[typeNames[0]] {
		out[ingest.NodeText(nameN, content)] = true
	}
}

// javaBindRecordPatternComponent records A a inside Box(A a) / Holder(A a).
func javaBindRecordPatternComponent(n *grammar.Node, content []byte, ourReceivers, out map[string]bool) {
	if n == nil || n.Type() != "record_pattern_component" {
		return
	}
	var typeN, nameN *grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "type_identifier", "generic_type", "scoped_type_identifier", "array_type":
			typeN = ch
		case "identifier":
			nameN = ch
		}
	}
	if typeN == nil || nameN == nil {
		return
	}
	if tn := javaTypeName(typeN, content); ourReceivers[tn] {
		out[ingest.NodeText(nameN, content)] = true
	}
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
		// List<A> / Map.Entry<K,V> / java.util.List<A> — simple name of the head.
		// Scoped heads (Map.Entry) have no direct type_identifier child; walk the
		// head node so javaMapEntryDeclaredValueType can see "Entry".
		if name := ingest.ChildByField(typeN, "type"); name != nil {
			if tn := javaTypeName(name, content); tn != "" {
				return tn
			}
		}
		for i := uint32(0); i < typeN.ChildCount(); i++ {
			ch := typeN.Child(i)
			if ch.Type() == "type_arguments" {
				continue
			}
			if tn := javaTypeName(ch, content); tn != "" {
				return tn
			}
		}
		return ""
	case "scoped_type_identifier":
		// Map.Entry / java.util.List / Outer.Inner — simple name is the rightmost
		// type_identifier (Entry / List / Inner). Field "name" when present.
		if name := ingest.ChildByField(typeN, "name"); name != nil {
			return ingest.NodeText(name, content)
		}
		return javaScopedTypeSimpleName(typeN, content)
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

// javaScopedTypeSimpleName returns the rightmost type_identifier of a
// scoped_type_identifier chain (Map.Entry → Entry, java.util.List → List).
// Left segments are package/outer type names and are not the local type leaf.
func javaScopedTypeSimpleName(n *grammar.Node, content []byte) string {
	if n == nil || n.IsNull() {
		return ""
	}
	var last string
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "type_identifier" {
			last = ingest.NodeText(n, content)
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(n)
	return last
}
