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

	typedLocals, entryValOf, valOf := javaTypedLocals(pf.Root, content, ourSimple)

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
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, entryValOf, valOf, implementsEdges) {
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
				if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, entryValOf, valOf, implementsEdges) {
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
					if javaShouldRenameMemberAccess(obj, content, classHere, ourSimple, foreignSimple, typedLocals, entryValOf, valOf, implementsEdges) {
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
// valOf maps Map locals → value type leaf (for am.firstEntry().getValue().m()).
func javaShouldRenameMemberAccess(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, entryValOf, valOf map[string]string, implementsEdges map[string]map[string]bool) bool {
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
		return javaShouldRenameMemberAccess(arr, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, entryValOf, valOf, implementsEdges)
	}
	if obj.Type() == "identifier" || obj.Type() == "type_identifier" {
		return javaRenameByTypeMaps(ingest.NodeText(obj, content), ourReceivers, foreignReceivers, typedLocals)
	}
	// e.getValue().m() — Map.Entry local value type (entrySet for-var / forEach).
	// Map.entry(k, new A()).getValue().m() — creation value type (self-contained).
	// am.firstEntry().getValue().m() / am.pollFirstEntry().getValue().m() —
	// map value type via valOf[am] (NavigableMap entry accessors).
	if obj.Type() == "method_invocation" {
		nameN := ingest.ChildByField(obj, "name")
		if nameN != nil && ingest.NodeText(nameN, content) == "getValue" {
			if vt := javaEntryExprValueType(ingest.ChildByField(obj, "object"), content, nil, valOf, entryValOf); vt != "" {
				return javaRenameByTypeMaps(vt, ourReceivers, foreignReceivers, nil)
			}
		}
		// Unknown method receivers: unique-leaf only.
		return len(foreignReceivers) == 0
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
// / Arrays.stream(as).forEach(a -> a.m()) / Arrays.stream(new A[]{...}).map(a -> a.m())
// / as.stream().findFirst().ifPresent(a -> a.m()) / Optional.of(new A()).ifPresent(a -> a.m())
// / Optional.flatMap(a -> Optional.of(a)).ifPresent(x -> x.m()) / flatMap(...).orElse(d) /
// / Optional.map(a -> a).ifPresent(x -> x.m()) / map(...).orElse(d) /
// / Optional<A>.ifPresent(a -> a.m()) / opt.ifPresentOrElse(a -> a.m(), () -> {}) /
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
// entryValOf maps Map.Entry locals → value type leaf for e.getValue().m()
// (for (var e : m.entrySet()) / m.entrySet().forEach(e -> …) / Map.Entry<K,A> e /
// var ea = Map.entry(...) / var ea = am.firstEntry()).
// valOf maps Map locals → value type leaf (also returned for inline
// am.firstEntry().getValue().m() under foreign same-leaf methods).
func javaTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) (map[string]bool, map[string]string, map[string]string) {
	out := map[string]bool{}
	entryValOf := map[string]string{}
	valOf := map[string]string{}
	if root == nil || len(ourReceivers) == 0 {
		return out, entryValOf, valOf
	}
	// Collection/stream locals: name → element type leaf (List<A> as → "A").
	elemOf := map[string]string{}
	// groupingBy/partitioningBy maps: name → element type of each value list
	// (Map<K,List<T>> / Map<Boolean,List<T>> → "T").
	groupValOf := map[string]string{}
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
				if vt := javaInferExprType(valN, content); ourReceivers[vt] {
					out[name] = true
				} else if et := javaCollectionAccessElemType(valN, content, elemOf, valOf, entryValOf); ourReceivers[et] {
					// var xa = as.get(0) / am.get("k") / as.iterator().next() / ia.next()
					// / qa.poll() / qa.peek() / qa.take() / da.takeFirst()/takeLast()
					// / as.remove(0) / as.getFirst()
					// / as.removeFirst() / as.removeLast()
					// / e.getValue() when e is a Map.Entry local
					// / Map.entry(k, new A()).getValue() / am.firstEntry().getValue()
					// / am.pollFirstEntry().getValue() / am.ceilingEntry(k).getValue()
					out[name] = true
				} else if vt := javaEntryExprValueType(valN, content, elemOf, valOf, entryValOf); vt != "" {
					// var ea = Map.entry(k, new A()) / am.firstEntry() /
					// am.pollLastEntry() / am.floorEntry(k) — Entry of V;
					// track value T for later ea.getValue().m() (entry is not A itself).
					entryValOf[name] = vt
				} else if et := javaStreamPipelineElemType(valN, content, elemOf, valOf); et != "" {
					// var list = as.stream().toList() / collect(Collectors.toList()/toSet()) /
					// var s = as.stream() / var opt = as.stream().findFirst() —
					// track collection/stream/Optional element type for later
					// list.forEach / for (var a : list) / opt.ifPresent (not a scalar A).
					elemOf[name] = et
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
				} else if et := javaGroupingByMapGetElemType(valN, content, groupValOf); et != "" {
					// var g = m.get(k) when m is a groupingBy/partitioningBy map — g is List<T>.
					elemOf[name] = et
				}
			}
		case "enhanced_for_statement":
			// for (A a : as) — explicit type. for (var a : as) — element of collection.
			// Without var→elem binding, a.run() is skipped when foreign same-leaf methods exist.
			// for (var e : m.entrySet()) / for (Map.Entry<K,A> e : m.entrySet()) —
			// entry is not A; bind entryValOf for e.getValue().m().
			// for (var g : m.values()) when m is groupingBy/partitioningBy → g is List<T> (elemOf).
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
					if vt := javaEntrySetPipelineValueType(valN, content, elemOf, valOf); vt != "" {
						entryValOf[name] = vt
					}
					if et := javaGroupingByValuesGroupElemType(valN, content, elemOf, valOf, groupValOf); et != "" {
						elemOf[name] = et
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
			javaBindStreamLambdaParams(n, content, ourReceivers, elemOf, valOf, entryValOf, groupValOf, out)
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out, entryValOf, valOf
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
// or Map bi-lambdas (forEach/computeIfPresent/compute/replaceAll/merge) when the map
// value type is ours. entrySet pipelines bind entryValOf for e.getValue().m().
// groupingBy/partitioningBy maps bind elemOf for value-list params (List<T> groups).
// Typed (A a) -> params are already handled via formal_parameter.
func javaBindStreamLambdaParams(call *grammar.Node, content []byte, ourReceivers map[string]bool, elemOf, valOf, entryValOf, groupValOf map[string]string, out map[string]bool) {
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
			} else if et := javaGroupingByValuesGroupElemType(obj, content, elemOf, valOf, groupValOf); et != "" {
				// m.values().forEach(g -> …) / collect(groupingBy|partitioningBy).values().forEach —
				// param is List<T>, not T; track for nested forEach / for-var.
				if elemOf != nil {
					elemOf[params[0]] = et
				}
			}
			// m.entrySet().forEach(e -> e.getValue().m()) — param is Entry, not V.
			if entryValOf != nil {
				if vt := javaEntrySetPipelineValueType(obj, content, elemOf, valOf); vt != "" {
					entryValOf[params[0]] = vt
				}
			}
		case 2:
			// Map bi-lambdas — value type from valOf[map] / collect(toMap(...)).
			// forEach/computeIfPresent/compute/replaceAll: (K,V) → second is V.
			// merge: (V,V) → both params are V (BiFunction remapping).
			// groupingBy/partitioningBy maps: value is List<T> — bind elemOf on the value param.
			if !javaMapValueBiLambdaMethod(method) {
				continue
			}
			vt := javaMapPipelineValueType(obj, content, elemOf, valOf)
			if vt != "" && ourReceivers[vt] {
				if method == "merge" {
					out[params[0]] = true
					out[params[1]] = true
				} else {
					out[params[1]] = true
				}
				continue
			}
			if et := javaGroupingByMapGroupElemType(obj, content, elemOf, valOf, groupValOf); et != "" && elemOf != nil {
				// m.forEach((k,g) -> g.forEach(...)) / collect(groupingBy|partitioningBy).forEach —
				// value param is List<T>.
				if method == "merge" {
					// merge values would be List — fail closed (not a product case).
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
		"filter", "peek", "forEach", "forEachOrdered", "forEachRemaining",
		"takeWhile", "dropWhile",
		"anyMatch", "allMatch", "noneMatch",
		"removeIf", "ifPresent", "ifPresentOrElse",
		// Map value bi-lambdas (see javaMapValueBiLambdaMethod).
		"computeIfPresent", "compute", "replaceAll", "merge":
		return true
	default:
		return false
	}
}

// javaMapValueBiLambdaMethod reports Map methods whose bi-lambda args include the
// map value type: forEach/computeIfPresent/compute/replaceAll → (K,V), merge → (V,V).
func javaMapValueBiLambdaMethod(method string) bool {
	switch method {
	case "forEach", "computeIfPresent", "compute", "replaceAll", "merge":
		return true
	default:
		return false
	}
}

// javaStreamPipelineElemType recovers the element type of a stream pipeline object:
// as / as.stream() / as.iterator() / as.stream().filter(...) → elemOf[as],
// as.reversed() / as.reversed().stream() → elemOf[as] (SequencedCollection/List view),
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
// m.values() → valOf[m],
// List.of(new A()) / Stream.of(new A()) / Arrays.asList(new A()) → "A",
// Collections.singletonList(new A()) / Collections.singleton(new A()) → "A",
// Collections.nCopies(n, new A()) → "A",
// Collections.unmodifiableList/Set/Collection(as) / synchronizedList/Set/Collection(as) /
// checkedList/Set/Collection(as, …) → elemOf[as],
// Collections.list(Enumeration) / Collections.enumeration(coll) → enumeration/coll element type,
// List.copyOf(as) / Set.copyOf(as) → elemOf[as] (Collection of first-arg elements),
// Stream.concat(s1, s2) → element type when both stream args agree,
// Stream.generate(() -> new A()) → "A",
// Stream.iterate(new A(), …) / iterate(new A(), pred, …) → "A" (seed creation type),
// Stream.ofNullable(new A()) → "A",
// Arrays.stream(as) / Arrays.stream(new A[]{...}) → "A",
// Optional.of(new A()) / Optional.ofNullable(new A()) → "A",
// Optional.flatMap(a -> Optional.of(a)) / ofNullable(a) / Optional::of → same element
// when the mapper clearly rewraps T (see javaFlatMapResultElemType),
// Optional.map(a -> a) / map(a -> new A()) → same or known element
// when the mapper clearly yields T (see javaMapResultElemType).
// Type-changing stages (unknown map / flatMap mappers) fail closed so later
// lambdas are not mis-typed.
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
			// reversed() returns List/SequencedCollection of the same element type
			// (Java 21 SequencedCollection; order only, element type unchanged).
			"findFirst", "findAny", "min", "max", "reduce", "toList", "reversed":
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
			return javaStreamPipelineElemType(recv, content, elemOf, valOf)
		case "flatMap":
			// Optional.flatMap (and Stream.flatMap with the same rewrap shapes):
			// recover U from mapper when clearly Optional.of/ofNullable rewrap
			// or another tracked Optional/collection local. Unknown mappers fail closed.
			return javaFlatMapResultElemType(obj, content, elemOf, valOf)
		case "map":
			// Optional.map / Stream.map: recover U from mapper when clearly
			// identity (a -> a) or new T(...). Unknown/type-changing mappers fail closed.
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
			// Collection of the stream element type. Other collectors
			// (groupingBy, type-changing mapping/flatMapping, toMap, …) fail closed here
			// (toMap values recovered via javaMapPipelineValueType / javaToMapCollectValueType).
			if !javaIsToListOrSetCollector(obj, content) {
				return ""
			}
			return javaStreamPipelineElemType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "values":
			// m.values() / collect(toMap(...)).values() — Collection of map values.
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		case "of", "asList", "ofNullable", "singletonList", "singleton":
			// List.of(new A()) / Stream.of(new A(), new A()) / Arrays.asList(new A())
			// / Set.of(new A()) / Optional.of(new A()) / Optional.ofNullable(new A())
			// / Stream.ofNullable(new A())
			// / Collections.singletonList(new A()) / Collections.singleton(new A())
			// — element type from homogeneous new T(...) args.
			return javaStaticCollectionOfElemType(obj, content, name)
		case "nCopies":
			// Collections.nCopies(n, new A()) — List of T from the second arg.
			return javaCollectionsNCopiesElemType(obj, content)
		case "unmodifiableList", "synchronizedList", "checkedList",
			"unmodifiableSet", "synchronizedSet", "checkedSet",
			"unmodifiableCollection", "synchronizedCollection", "checkedCollection",
			"list", "enumeration":
			// Collections.unmodifiableList/Set/Collection(as) /
			// synchronizedList/Set/Collection(as) /
			// checkedList/Set/Collection(as, A.class) — Collection of first-arg
			// element type (Class arg on checked* ignored).
			// Collections.list(Enumeration) — ArrayList of enumeration elements.
			// Collections.enumeration(coll) — Enumeration of coll elements.
			return javaCollectionsListWrapperElemType(obj, content, elemOf, valOf)
		case "copyOf":
			// List.copyOf(coll) / Set.copyOf(coll) — Collection of first-arg element type
			// (unlike of/asList which take new T(...) args).
			return javaListSetCopyOfElemType(obj, content, elemOf, valOf)
		case "concat":
			// Stream.concat(s1, s2) — Stream of both args' element type when they agree.
			return javaStreamConcatElemType(obj, content, elemOf, valOf)
		case "generate":
			// Stream.generate(() -> new A()) — element type from supplier body.
			return javaStreamGenerateElemType(obj, content)
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

// javaMapResultElemType recovers U after Optional.map / Stream.map(mapper) when
// the mapper clearly yields a known element type:
//
//	oa.map(a -> a) → elem(oa)                         // identity
//	oa.map(a -> new A()) → A                          // object creation
//	as.stream().map(a -> a).forEach(...) → elem(as)   // same for Stream
//
// Expression-bodied lambdas only; blocks, method refs, and other mappers fail
// closed so type-changing maps stay unbound.
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
	if first == nil || first.Type() != "lambda_expression" {
		return ""
	}
	body := ingest.ChildByField(first, "body")
	if body == nil {
		return ""
	}
	params := javaInferredLambdaParamNames(first, content)
	param := ""
	if len(params) == 1 {
		param = params[0]
	}
	recv := ingest.ChildByField(call, "object")
	return javaMapMapperBodyElemType(body, param, recv, content, elemOf, valOf)
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

// javaFlatMapResultElemType recovers T after Optional.flatMap(mapper) when the
// mapper clearly yields Optional<T> of the same (or known) element:
//
//	oa.flatMap(a -> Optional.of(a)) / Optional.ofNullable(a) → elem(oa)
//	oa.flatMap(Optional::of) / Optional::ofNullable → elem(oa)
//	oa.flatMap(a -> other) when other is tracked in elemOf → elemOf[other]
//	oa.flatMap(a -> Optional.of(new A())) → A
//
// Expression-bodied lambdas and Optional::of / ofNullable only; blocks and
// other mappers fail closed so Stream.flatMap type changes stay unbound.
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
		if javaIsOptionalOfMethodRef(first, content) {
			return javaStreamPipelineElemType(recv, content, elemOf, valOf)
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
			if ingest.NodeText(obj, content) != "Optional" {
				return ""
			}
			// a -> Optional.of(a) / ofNullable(a) — same element as receiver.
			if arg := javaFirstCallArg(body); arg != nil && arg.Type() == "identifier" && param != "" && ingest.NodeText(arg, content) == param {
				return javaStreamPipelineElemType(recv, content, elemOf, valOf)
			}
			// a -> Optional.of(new A()) — element from creation args.
			return javaStaticCollectionOfElemType(body, content, name)
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	switch ingest.NodeText(recvN, content) {
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(recvN, content) != "Collections" {
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
// Collections.list(enumeration) / Collections.enumeration(coll).
// First arg's element type; the Class arg on checked* is ignored.
// Non-Collections receivers fail closed.
func javaCollectionsListWrapperElemType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
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
	if ingest.NodeText(recvN, content) != "Collections" {
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(recvN, content) != "Stream" {
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
	if ingest.NodeText(recvN, content) != "Stream" {
		return ""
	}
	args := javaCallArgs(call)
	if len(args) != 1 || args[0].Type() != "lambda_expression" {
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(recvN, content) != "Stream" {
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
	return ingest.NodeText(body, content) == params[0]
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

// javaIsGroupingByCollector reports Stream.collect(Collectors.groupingBy(classifier))
// / collect(groupingBy(classifier)) and the one-arg partitioningBy(predicate) twin
// (Map<Boolean, List<T>> with the same List-group shape). Multi-arg forms
// (downstream collectors like counting) fail closed.
func javaIsGroupingByCollector(collectCall *grammar.Node, content []byte) bool {
	first := javaCollectFirstArg(collectCall)
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
	// Exactly one real arg (the classifier). Extra downstream collector → fail closed.
	nReal := 0
	for i := uint32(0); i < first.ChildCount(); i++ {
		if first.Child(i).Type() != "argument_list" {
			continue
		}
		al := first.Child(i)
		for j := uint32(0); j < al.ChildCount(); j++ {
			switch al.Child(j).Type() {
			case "(", ")", ",", "comment":
				continue
			default:
				nReal++
			}
		}
	}
	return nReal == 1
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
// a groupingBy/partitioningBy map (value is List<T>).
func javaGroupingByMapGetElemType(val *grammar.Node, content []byte, groupValOf map[string]string) string {
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
	if val == nil || val.IsNull() || val.Type() != "method_invocation" || groupValOf == nil {
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
	obj := ingest.ChildByField(val, "object")
	if obj == nil || obj.Type() != "identifier" {
		return ""
	}
	return groupValOf[ingest.NodeText(obj, content)]
}

// javaIsArraysReceiver reports whether obj is the Arrays type name (static call site).
func javaIsArraysReceiver(obj *grammar.Node, content []byte) bool {
	if obj == nil || obj.IsNull() {
		return false
	}
	if obj.Type() != "identifier" && obj.Type() != "type_identifier" {
		return false
	}
	return ingest.NodeText(obj, content) == "Arrays"
}

// javaArraysStreamElemType recovers T from Arrays.stream(T[] arr[, from, to]).
// First argument is the array (identifier with elemOf, or new T[]{...}).
// Range bounds do not change the element type.
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
			return javaStreamPipelineElemType(ch, content, elemOf, valOf)
		}
	}
	return ""
}

// javaMapPipelineValueType recovers the value type of a Map-typed expression:
// m → valOf[m],
// stream.collect(toMap(key, a -> a[, …])) → stream element type,
// stream.collect(collectingAndThen(toMap(key, a -> a[, …]), finisher)) → same,
// Collections.unmodifiableMap/synchronizedMap/checkedMap(m[, …]) → valOf[m],
// Collections.singletonMap(k, new T(...)) → T,
// Map.of(k, new T(...), …) → T (homogeneous value creations),
// Map.ofEntries(Map.entry(k, new T(...)), …) → T (homogeneous entry values),
// Map.copyOf(m) → valOf[m].
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
			"unmodifiableNavigableMap", "synchronizedNavigableMap", "checkedNavigableMap":
			// Collections.*Map wrappers — value type of first-arg map (Class args ignored).
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
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaCollectionsMapWrapperValueType recovers V from
// Collections.unmodifiableMap/synchronizedMap/checkedMap(m[, keyClass, valueClass])
// (and Sorted/Navigable variants). First arg's map value type; Class args ignored.
// Non-Collections receivers fail closed.
func javaCollectionsMapWrapperValueType(call *grammar.Node, content []byte, elemOf, valOf map[string]string) string {
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
	if ingest.NodeText(recvN, content) != "Collections" {
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(recvN, content) != "Collections" {
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(recvN, content) != "Map" {
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(recvN, content) != "Map" {
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
//	Map.entry(k, new T(...))  → T (creation value; key ignored)
//	am.firstEntry()           → valOf[am] (NavigableMap entry endpoints)
//	am.lastEntry()            → valOf[am]
//	am.pollFirstEntry() / am.pollLastEntry() → valOf[am]
//	am.ceilingEntry(k) / am.floorEntry(k) / am.higherEntry(k) / am.lowerEntry(k) → valOf[am]
//
// Used for e.getValue() / var ea = Map.entry(...) / var ea = am.firstEntry() so
// method rename hits value call sites under foreign same-leaf methods.
// Unknown shapes fail closed.
func javaEntryExprValueType(obj *grammar.Node, content []byte, elemOf, valOf, entryValOf map[string]string) string {
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
	case "method_invocation":
		nameN := ingest.ChildByField(obj, "name")
		if nameN == nil {
			return ""
		}
		switch name := ingest.NodeText(nameN, content); name {
		case "entry":
			// Map.entry(k, new T(...)) — value type from second arg creation.
			return javaMapEntryCreationValueType(obj, content)
		case "firstEntry", "lastEntry",
			// NavigableMap poll/search entry accessors also return Map.Entry<K,V>;
			// V from map value type (same path as firstEntry/lastEntry).
			"pollFirstEntry", "pollLastEntry",
			"ceilingEntry", "floorEntry", "higherEntry", "lowerEntry":
			// NavigableMap.*Entry → Map.Entry<K,V>; V from map value type.
			return javaMapPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
		default:
			return ""
		}
	default:
		return ""
	}
}

// javaMapEntryCreationValueType recovers T from Map.entry(k, new T(...)).
// Key is ignored; only the second arg's creation type matters. Non-Map receivers,
// wrong arity, or non-creation values fail closed.
func javaMapEntryCreationValueType(call *grammar.Node, content []byte) string {
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
	if ingest.NodeText(recvN, content) != "Map" {
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
	if recvN.Type() != "identifier" && recvN.Type() != "type_identifier" {
		return ""
	}
	if ingest.NodeText(recvN, content) != "Map" {
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
		"filter", "peek", "sorted", "distinct", "limit", "skip",
		"unordered", "sequential", "parallel", "onClose",
		"takeWhile", "dropWhile":
		return javaEntrySetPipelineValueType(ingest.ChildByField(obj, "object"), content, elemOf, valOf)
	default:
		return ""
	}
}

// javaMapEntryDeclaredValueType recovers V from Map.Entry<K,V> / Entry<K,V> type nodes.
// Used for for (Map.Entry<K,A> e : …) and Map.Entry locals — value type for getValue().
func javaMapEntryDeclaredValueType(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	if javaTypeName(typeN, content) != "Entry" {
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
// Collections.singletonList(...), and Collections.singleton(...) when every
// argument is `new T(...)` with the same T. Non-creation args and mixed types
// fail closed.
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
		case "List", "Stream", "Set", "Optional":
			// ok
		default:
			return ""
		}
	case "ofNullable":
		// Optional.ofNullable(new T(...)) / Stream.ofNullable(new T(...)).
		if recv != "Optional" && recv != "Stream" {
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
//	ia.next()                 → elemOf[ia]   (Iterator<A>)
//	as.iterator().next()      → elemOf[as]   (via type-preserving iterator())
//	lia.previous()            → elemOf[lia]  (ListIterator<A>)
//	as.listIterator().next()/previous() → elemOf[as] (type-preserving listIterator())
//	ea.nextElement()          → elemOf[ea]   (Enumeration<A>)
//	vs.elements().nextElement() → elemOf[vs] (Vector; Hashtable uses valOf)
//	Collections.enumeration(as).nextElement() → elemOf[as]
//	oa.orElse(d) / oa.orElseGet(s) / oa.orElseThrow([s]) → elemOf[oa]
//	  (Optional<A>; also findFirst().orElse / findFirst().orElseThrow)
//	Collections.min(as) / Collections.max(as[, cmp]) → elemOf[as]
//	stream.reduce(identity, op[, combiner]) → stream element type (returns T/U)
//	e.getValue()              → entryValOf[e] (Map.Entry local from entrySet)
//	Map.entry(k, new T()).getValue() → T
//	am.firstEntry().getValue() / am.lastEntry().getValue() → valOf[am]
//	am.pollFirstEntry()/pollLastEntry()/ceilingEntry(k)/… .getValue() → valOf[am]
//
// Optional.get() also works when Optional<A> is tracked in elemOf (single type arg).
// Fail closed on other methods / unknown receivers.
// One-arg stream.reduce(BinaryOperator) returns Optional — use orElse/ifPresent
// (pipeline typing), not bare var of the element type.
func javaCollectionAccessElemType(val *grammar.Node, content []byte, elemOf, valOf, entryValOf map[string]string) string {
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
		"first", "last":
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
	case "getValue":
		// e.getValue() — Map.Entry local (entrySet for-var / forEach / var ea = …).
		// Map.entry(k, new T(...)).getValue() / am.firstEntry().getValue() — same V.
		return javaEntryExprValueType(obj, content, elemOf, valOf, entryValOf)
	case "next", "previous", "nextElement":
		// it.next() / as.iterator().next() — element of iterator or pipeline.
		// lia.previous() / as.listIterator().previous() — same E (ListIterator).
		// listIterator() is type-preserving in javaStreamPipelineElemType.
		// ea.nextElement() / vs.elements().nextElement() /
		// Collections.enumeration(as).nextElement() — same E (Enumeration).
		// elements()/enumeration() are type-preserving in javaStreamPipelineElemType.
		return javaStreamPipelineElemType(obj, content, elemOf, valOf)
	case "orElse", "orElseGet", "orElseThrow":
		// Optional.orElse / orElseGet / orElseThrow return T; receiver may be
		// Optional<A> local or a pipeline that yields Optional
		// (findFirst/findAny / Optional.of). Exception supplier on orElseThrow
		// does not change the value type leaf.
		return javaStreamPipelineElemType(obj, content, elemOf, valOf)
	case "min", "max":
		// Collections.min(coll) / Collections.max(coll[, cmp]) return the element type.
		// Stream.min/max return Optional — bind via orElse/ifPresent on the pipeline,
		// not as a bare var of the element type.
		return javaCollectionsMinMaxElemType(val, obj, content, elemOf, valOf)
	case "reduce":
		// Stream.reduce(identity, accumulator[, combiner]) returns the identity type
		// (T for BinaryOperator; U for the 3-arg form when identity is U — we recover
		// the stream element type, which matches the common T-identity product case).
		// One-arg reduce(BinaryOperator) returns Optional<T> — fail closed here.
		return javaStreamReduceIdentityElemType(val, obj, content, elemOf, valOf)
	default:
		return ""
	}
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
