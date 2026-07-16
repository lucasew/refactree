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

	typedLocals := javaTypedLocals(pf.Root, content, ourSimple)

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
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, implementsEdges) {
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
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, implementsEdges) {
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
					if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, implementsEdges) {
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
func javaShouldRenameMemberAccess(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, implementsEdges map[string]map[string]bool) bool {
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
	if obj.Type() == "field_access" {
		text := ingest.NodeText(obj, content)
		if ourReceivers[text] || foreignReceivers[text] {
			return ourReceivers[text]
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
	// xs[0].helper() — recurse into array root (handles (xs)[0] / matrix[i][j]).
	if obj.Type() == "array_access" {
		arr := ingest.ChildByField(obj, "array")
		if arr == nil {
			return len(foreignReceivers) == 0
		}
		return javaShouldRenameMemberAccess(arr, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, implementsEdges)
	}
	if obj.Type() == "identifier" || obj.Type() == "type_identifier" {
		return javaRenameByTypeMaps(ingest.NodeText(obj, content), ourReceivers, foreignReceivers, typedLocals)
	}
	// Unknown / complex receivers without recoverable static type: unique-leaf only.
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
// Also binds untyped stream/collection lambda params when the pipeline element
// type is ours (List<A> as → as.stream().map(a -> a.m()) / as.iterator().forEachRemaining(a -> a.m())
// / List.of(new A()).forEach(a -> a.m()) / Stream.of(new A()).map(a -> a.m())
// / Map<K,A>.forEach((k,v) -> v.m()) / map.values().forEach(v -> v.m())
// types a/v as A), for (var a : as) loop variables from collection/array element types,
// and var locals from collection accessors (list.get(i) / map.get(k) / it.next() /
// list.iterator().next()).
func javaTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) map[string]bool {
	out := map[string]bool{}
	if root == nil || len(ourReceivers) == 0 {
		return out
	}
	// Collection/stream locals: name → element type leaf (List<A> as → "A").
	elemOf := map[string]string{}
	// Map-like locals: name → value type leaf (Map<K,A> m → "A").
	valOf := map[string]string{}
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
				if tn := javaTypeName(typeN, content); ourReceivers[tn] {
					out[name] = true
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
				if explicitOurs {
					out[name] = true
					continue
				}
				if !inferFromInit {
					continue
				}
				valN := ingest.ChildByField(c, "value")
				if vt := javaInferExprType(valN, content); ourReceivers[vt] {
					out[name] = true
				} else if et := javaCollectionAccessElemType(valN, content, elemOf, valOf); ourReceivers[et] {
					// var xa = as.get(0) / am.get("k") / as.iterator().next() / ia.next()
					out[name] = true
				}
			}
		case "enhanced_for_statement":
			// for (A a : as) — explicit type. for (var a : as) — element of collection.
			// Without var→elem binding, a.run() is skipped when foreign same-leaf methods exist.
			typeN := ingest.ChildByField(n, "type")
			nameN := ingest.ChildByField(n, "name")
			if typeN != nil && nameN != nil {
				name := ingest.NodeText(nameN, content)
				tn := javaTypeName(typeN, content)
				if ourReceivers[tn] {
					out[name] = true
				} else if tn == "var" {
					valN := ingest.ChildByField(n, "value")
					if et := javaStreamPipelineElemType(valN, content, elemOf, valOf); ourReceivers[et] {
						out[name] = true
					}
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
			// m.forEach((k,v) -> v.m()) — untyped lambda params.
			javaBindStreamLambdaParams(n, content, ourReceivers, elemOf, valOf, out)
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out
}

// javaRecordCollectionElem records name → element/value types for arrays and generics
// (List<A> as → elem "A", A[] xs → elem "A", Stream<A> s → elem "A",
// Map<K,A> m → elem "K" (first arg) and val "A" (second arg)).
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
		}
		// Map<K,V> / HashMap<K,V> — second type arg is the value type.
		if valOf != nil && len(args) >= 2 {
			valOf[name] = args[1]
		}
	}
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
// or Map.forEach((k,v) -> …) when the map value type is ours.
// Typed (A a) -> params are already handled via formal_parameter.
func javaBindStreamLambdaParams(call *grammar.Node, content []byte, ourReceivers map[string]bool, elemOf, valOf map[string]string, out map[string]bool) {
	if call == nil || call.Type() != "method_invocation" || out == nil {
		return
	}
	nameN := ingest.ChildByField(call, "name")
	if nameN == nil {
		return
	}
	method := ingest.NodeText(nameN, content)
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
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() != "lambda_expression" {
			continue
		}
		params := javaInferredLambdaParamNames(ch, content)
		switch len(params) {
		case 1:
			// Unary: stream/collection element or map.values() element.
			et := javaStreamPipelineElemType(obj, content, elemOf, valOf)
			if et != "" && ourReceivers[et] {
				out[params[0]] = true
			}
		case 2:
			// Map.forEach BiConsumer — second param is the map value type.
			if method != "forEach" {
				continue
			}
			vt := javaMapPipelineValueType(obj, content, valOf)
			if vt != "" && ourReceivers[vt] {
				out[params[1]] = true
			}
		}
	}
}

// javaStreamElementLambdaMethod reports methods whose (first) functional arg is
// applied to the stream/collection element type.
func javaStreamElementLambdaMethod(method string) bool {
	switch method {
	case "map", "mapToInt", "mapToLong", "mapToDouble",
		"flatMap", "flatMapToInt", "flatMapToLong", "flatMapToDouble",
		"filter", "peek", "forEach", "forEachOrdered", "forEachRemaining",
		"takeWhile", "dropWhile",
		"anyMatch", "allMatch", "noneMatch",
		"removeIf", "ifPresent":
		return true
	default:
		return false
	}
}

// javaStreamPipelineElemType recovers the element type of a stream pipeline object:
// as / as.stream() / as.iterator() / as.stream().filter(...) → elemOf[as],
// m.values() → valOf[m],
// List.of(new A()) / Stream.of(new A()) / Arrays.asList(new A()) → "A".
// Type-changing stages (map/flatMap) fail closed so later lambdas are not mis-typed.
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
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		switch name := ingest.NodeText(nameN, content); name {
		case "stream", "parallelStream", "iterator",
			"filter", "peek", "sorted", "distinct", "limit", "skip",
			"unordered", "sequential", "parallel", "onClose",
			"takeWhile", "dropWhile":
			return javaStreamPipelineElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "values":
			// m.values() — Collection of map values (valOf[m]).
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, valOf)
		case "of", "asList":
			// List.of(new A()) / Stream.of(new A(), new A()) / Arrays.asList(new A())
			// / Set.of(new A()) — element type from homogeneous new T(...) args.
			return javaStaticCollectionOfElemType(obj, content, name)
		default:
			// map/flatMap/… change the element type — fail closed.
			return ""
		}
	default:
		return ""
	}
}

// javaMapPipelineValueType recovers the value type of a Map-typed expression:
// m → valOf[m]. Fail closed on other shapes.
func javaMapPipelineValueType(obj *grammar.Node, content []byte, valOf map[string]string) string {
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
	if obj == nil || obj.IsNull() || obj.Type() != "identifier" || valOf == nil {
		return ""
	}
	return valOf[ingest.NodeText(obj, content)]
}

// javaStaticCollectionOfElemType recovers the element type of List/Stream/Set.of(...)
// and Arrays.asList(...) when every argument is `new T(...)` with the same T.
// Non-creation args and mixed types fail closed.
func javaStaticCollectionOfElemType(call *grammar.Node, content []byte, method string) string {
	if call == nil || call.Type() != "method_invocation" {
		return ""
	}
	recvN := ingest.ChildByField(call, "object")
	if recvN == nil {
		return ""
	}
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	recv := ingest.NodeText(recvN, content)
	switch method {
	case "of":
		switch recv {
		case "List", "Stream", "Set":
			// ok
		default:
			return ""
		}
	case "asList":
		if recv != "Arrays" {
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
//	ia.next()                 → elemOf[ia]   (Iterator<A>)
//	as.iterator().next()      → elemOf[as]   (via type-preserving iterator())
//
// Optional.get() also works when Optional<A> is tracked in elemOf (single type arg).
// Fail closed on other methods / unknown receivers.
func javaCollectionAccessElemType(val *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
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
	obj := ingest.ChildByField(val, "object")
	switch method := ingest.NodeText(nameN, content); method {
	case "get", "getOrDefault":
		// Map-like (2 type args recorded in valOf) → value type; else element type.
		// Identifier receiver only — chained gets fail closed.
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
		return ""
	case "next":
		// it.next() / as.iterator().next() — element of iterator or pipeline.
		return javaStreamPipelineElemType(obj, content, elemOf, valOf)
	default:
		return ""
	}
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
