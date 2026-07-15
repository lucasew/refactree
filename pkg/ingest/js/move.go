package js

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

func init() {
	ingest.RegisterMoveDriver("javascript", moveDriver{})
}

type moveDriver struct{}

func (moveDriver) Language() string { return "javascript" }

func (moveDriver) ExtractDecl(filePath string, entity ingest.Entity) (ingest.DeclExtract, error) {
	pf, err := ingest.ParseSourceFile(filePath, "")
	if err != nil {
		return ingest.DeclExtract{}, err
	}
	defer pf.Close()
	source, root := pf.Source, pf.Root

	declNode, includeExport := findJSDecl(root, entity.StartByte)
	if declNode == nil {
		return ingest.DeclExtract{}, fmt.Errorf("declaration not found in %s", filePath)
	}

	var start, end uint32
	if includeExport != nil {
		// Include the export_statement wrapping
		start = includeExport.StartByte()
		end = includeExport.EndByte()
	} else {
		start = declNode.StartByte()
		end = declNode.EndByte()
	}
	declText := string(source[start:end])

	// Ensure the declaration is exported for cross-file moves. In JS modules,
	// non-exported declarations are file-private; moving one to another file
	// requires exporting it so callers can import it.
	if includeExport == nil && !strings.HasPrefix(declText, "export ") {
		declText = "export " + declText
	}

	// Remove up to two trailing newlines.
	removeEnd := end
	for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
		removeEnd++
		if removeEnd-end >= 2 {
			break
		}
	}

	imports := jsImportsNeededByDecl(root, source, start, end)

	return ingest.DeclExtract{
		DeclText:    declText,
		Imports:     imports, // import statement texts for unused-import cleanup
		RemoveStart: start,
		RemoveEnd:   removeEnd,
	}, nil
}

func (moveDriver) InsertDecl(dstRelPath string, dstContent []byte, decl ingest.DeclExtract) ingest.Edit {
	insertAt := uint32(0)
	insertText := ""

	// Import insertion is handled by FinishCrossFileMove which has access to
	// both source and destination paths for correct relative path computation.
	if dstContent != nil {
		insertAt = uint32(len(dstContent))
		if len(dstContent) > 0 && dstContent[len(dstContent)-1] != '\n' {
			insertText += "\n"
		}
		if len(dstContent) > 0 {
			insertText += "\n"
		}
		insertText += decl.DeclText + "\n"
	} else {
		insertText = decl.DeclText + "\n"
	}

	return ingest.Edit{
		File:      dstRelPath,
		StartByte: insertAt,
		EndByte:   insertAt,
		NewText:   insertText,
	}
}

// rewriteImportPaths adjusts relative import paths in import statement strings
// when the declaration moves from srcDir to dstDir. Bare (non-relative) imports
// like 'react' or '@scope/pkg' are left unchanged.
func rewriteImportPaths(imports []string, srcDir, dstDir string) []string {
	if len(imports) == 0 || srcDir == dstDir {
		return imports
	}
	out := make([]string, len(imports))
	for i, imp := range imports {
		out[i] = rewriteOneImportPath(imp, srcDir, dstDir)
	}
	return out
}

func rewriteOneImportPath(stmt, srcDir, dstDir string) string {
	// Find the quoted module specifier in the import statement.
	for _, q := range []byte{'\'', '"'} {
		start := strings.IndexByte(stmt, q)
		if start < 0 {
			continue
		}
		end := strings.IndexByte(stmt[start+1:], q)
		if end < 0 {
			continue
		}
		end += start + 1
		modPath := stmt[start+1 : end]
		if !strings.HasPrefix(modPath, ".") {
			// Bare import — not relative, leave as-is.
			return stmt
		}
		// Resolve the absolute path, then recompute relative to dstDir.
		absPath := path.Join(srcDir, modPath)
		newRel, err := relPath(dstDir, absPath)
		if err != nil {
			return stmt
		}
		if !strings.HasPrefix(newRel, ".") {
			newRel = "./" + newRel
		}
		return stmt[:start+1] + newRel + stmt[end:]
	}
	return stmt
}

func (moveDriver) RewriteImports(fileRelPath string, content []byte, result *ingest.Result, oldRef, newRef ingest.Reference) []ingest.Edit {
	oldPath := strings.TrimPrefix(oldRef.Path, "./")
	newPath := strings.TrimPrefix(newRef.Path, "./")

	// For symbol-level moves (cross-file), replace the full import path.
	// JS imports reference file paths directly, so replace the old file path
	// with the new one in the import string.
	if oldRef.Symbol != "" {
		if oldPath != "" && newPath != "" && oldPath != newPath {
			return ingest.FindAllOccurrences(fileRelPath, content, oldPath, newPath)
		}
		return nil
	}

	// For package/directory moves, use whole-word replacement to avoid
	// corrupting identifiers that contain the directory name as substring.
	oldDir := oldPath
	newDir := newPath
	if oldDir == "" || newDir == "" || oldDir == newDir {
		return nil
	}
	oldBase := ingest.LastPathComponent(oldDir)
	newBase := ingest.LastPathComponent(newDir)
	if oldBase == newBase {
		return nil
	}
	return ingest.FindAllWholeWordOccurrences(fileRelPath, content, oldBase, newBase)
}

func (moveDriver) FinishCrossFileMove(rootDir string, result *ingest.Result, src, dst ingest.Reference, decl ingest.DeclExtract) ([]ingest.Edit, error) {
	srcRel := strings.TrimPrefix(src.Path, "./")
	dstRel := strings.TrimPrefix(dst.Path, "./")
	leaf := src.Symbol
	if leaf == "" {
		return nil, nil
	}

	var edits []ingest.Edit

	// 1. Strip unused imports from the source file (imports that only the
	//    moved declaration used).
	if srcContent, err := os.ReadFile(path.Join(rootDir, srcRel)); err == nil {
		edits = append(edits, stripUnusedJSImports(srcRel, srcContent, decl)...)
	}

	// 2. Carry needed imports to the destination with rewritten relative paths.
	if len(decl.Imports) > 0 {
		srcDir := path.Dir(srcRel)
		dstDir := path.Dir(dstRel)
		adjusted := rewriteImportPaths(decl.Imports, srcDir, dstDir)
		if dstContent, err := os.ReadFile(path.Join(rootDir, dstRel)); err == nil {
			edits = append(edits, jsImportInsertEdits(dstRel, dstContent, adjusted)...)
		}
	}

	// 3. Check whether the source file still uses the moved symbol.
	//    If so, add an import for it from the destination.
	srcRef := src.String()
	srcUsesMovedSymbol := false
	for _, rel := range result.Relations {
		if rel.Target != srcRef {
			continue
		}
		ref := ingest.ParseReference(rel.Reference)
		relFile := strings.TrimPrefix(ref.Path, "./")
		if relFile != srcRel {
			continue
		}
		// Skip spans inside the declaration being removed.
		if rel.StartByte >= decl.RemoveStart && rel.EndByte <= decl.RemoveEnd {
			continue
		}
		srcUsesMovedSymbol = true
		break
	}

	if srcUsesMovedSymbol {
		importPath := jsRelativeImportPath(srcRel, dstRel)
		importStmt := fmt.Sprintf("import { %s } from '%s';", leaf, importPath)
		if srcContent, err := os.ReadFile(path.Join(rootDir, srcRel)); err == nil {
			edits = append(edits, jsImportInsertEdits(srcRel, srcContent, []string{importStmt})...)
		}
	}

	// 4. Find same-file entities the moved declaration references.
	//    Export them from the source and import them in the destination.
	localDeps := findLocalDepsForDecl(result, src, decl)
	if len(localDeps) > 0 {
		var importNames []string
		for _, dep := range localDeps {
			importNames = append(importNames, dep)
			// Add export keyword to the entity in the source file.
			edits = append(edits, ensureExportedInFile(rootDir, result, srcRel, dep)...)
		}
		importPath := jsRelativeImportPath(dstRel, srcRel)
		var stmts []string
		stmts = append(stmts, fmt.Sprintf("import { %s } from '%s';", strings.Join(importNames, ", "), importPath))
		if dstContent, err := os.ReadFile(path.Join(rootDir, dstRel)); err == nil {
			edits = append(edits, jsImportInsertEdits(dstRel, dstContent, stmts)...)
		}
	}

	return edits, nil
}

// findLocalDepsForDecl returns entity names defined in the same file as src
// that the moved declaration references (excluding imports, which are handled
// separately via DeclExtract.Imports).
func findLocalDepsForDecl(result *ingest.Result, src ingest.Reference, decl ingest.DeclExtract) []string {
	srcRef := src.String()
	srcPath := src.Path
	// Build set of entity names defined in the source file.
	localEntities := map[string]string{} // symbol -> reference
	for _, ent := range result.Entities {
		ref := ingest.ParseReference(ent.Reference)
		if ref.Path == srcPath && ent.Reference != srcRef {
			localEntities[ref.Symbol] = ent.Reference
		}
	}
	if len(localEntities) == 0 {
		return nil
	}
	// Check which of these are referenced inside the declaration being moved.
	var deps []string
	seen := map[string]bool{}
	for _, rel := range result.Relations {
		ref := ingest.ParseReference(rel.Reference)
		if ref.Path != srcPath {
			continue
		}
		// Only consider relations within the declaration range.
		if rel.StartByte < decl.RemoveStart || rel.EndByte > decl.RemoveEnd {
			continue
		}
		targetRef := ingest.ParseReference(rel.Target)
		if targetRef.Path != srcPath {
			continue
		}
		sym := targetRef.Symbol
		if sym == "" || seen[sym] || sym == src.Symbol {
			continue
		}
		if _, ok := localEntities[sym]; ok {
			seen[sym] = true
			deps = append(deps, sym)
		}
	}
	return deps
}

// ensureExportedInFile adds "export " before a function/class declaration
// in the given file if it isn't already exported.
func ensureExportedInFile(rootDir string, result *ingest.Result, fileRel, symbol string) []ingest.Edit {
	content, err := os.ReadFile(path.Join(rootDir, fileRel))
	if err != nil {
		return nil
	}
	return addExportKeyword(fileRel, content, symbol)
}

// addExportKeyword finds a declaration of the given symbol in content and
// prepends "export " if it is not already exported.
func addExportKeyword(file string, content []byte, symbol string) []ingest.Edit {
	pf, err := ingest.ParseSource(content, file, "")
	if err != nil {
		return nil
	}
	defer pf.Close()
	root := pf.Root

	declTypes := map[string]bool{
		"function_declaration":           true,
		"generator_function_declaration": true,
		"class_declaration":              true,
		"abstract_class_declaration":     true,
		"interface_declaration":          true,
		"type_alias_declaration":         true,
		"enum_declaration":               true,
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if declTypes[child.Type()] {
			if n := ingest.ChildByField(child, "name"); n != nil && ingest.NodeText(n, content) == symbol {
				// Not already exported; add "export " before the declaration.
				return []ingest.Edit{{
					File:      file,
					StartByte: child.StartByte(),
					EndByte:   child.StartByte(),
					NewText:   "export ",
				}}
			}
		}
		// Already exported — nothing to do.
		if child.Type() == "export_statement" {
			for j := uint32(0); j < child.ChildCount(); j++ {
				inner := child.Child(j)
				if declTypes[inner.Type()] {
					if n := ingest.ChildByField(inner, "name"); n != nil && ingest.NodeText(n, content) == symbol {
						return nil // already exported
					}
				}
			}
		}
	}
	return nil
}

// jsImportStmt holds a parsed JS/TS import statement.
type jsImportStmt struct {
	// Full text of the import statement including trailing semicolon/newline.
	text string
	// Local names this import introduces.
	locals []string
	// startByte/endByte in the source file.
	startByte, endByte uint32
}

// parseJSImportStatements extracts import_statement nodes from the tree-sitter root.
func parseJSImportStatements(root *grammar.Node, source []byte) []jsImportStmt {
	var stmts []jsImportStmt
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Type() != "import_statement" {
			continue
		}
		text := ingest.NodeText(child, source)
		stmt := jsImportStmt{
			text:      text,
			startByte: child.StartByte(),
			endByte:   child.EndByte(),
		}
		clause := ingest.ChildByType(child, "import_clause")
		if clause != nil {
			collectImportLocals(clause, source, &stmt.locals)
		}
		stmts = append(stmts, stmt)
	}
	return stmts
}

func collectImportLocals(n *grammar.Node, source []byte, out *[]string) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Type() {
		case "identifier":
			*out = append(*out, ingest.NodeText(c, source))
		case "named_imports":
			for j := uint32(0); j < c.ChildCount(); j++ {
				spec := c.Child(j)
				if spec.Type() != "import_specifier" {
					continue
				}
				// Alias takes precedence as local name.
				if alias := ingest.ChildByField(spec, "alias"); alias != nil {
					*out = append(*out, ingest.NodeText(alias, source))
				} else if name := ingest.ChildByField(spec, "name"); name != nil {
					*out = append(*out, ingest.NodeText(name, source))
				}
			}
		case "namespace_import":
			for j := uint32(0); j < c.ChildCount(); j++ {
				id := c.Child(j)
				if id.Type() == "identifier" {
					*out = append(*out, ingest.NodeText(id, source))
				}
			}
		}
	}
}

// jsImportsNeededByDecl returns full import statement texts whose local names
// appear in the declaration spanning [declStart, declEnd).
func jsImportsNeededByDecl(root *grammar.Node, source []byte, declStart, declEnd uint32) []string {
	stmts := parseJSImportStatements(root, source)
	if len(stmts) == 0 {
		return nil
	}
	declText := string(source[declStart:declEnd])
	var out []string
	for _, stmt := range stmts {
		for _, local := range stmt.locals {
			if jsIdentUsed(declText, local) {
				out = append(out, stmt.text)
				break
			}
		}
	}
	return out
}

// jsIdentUsed checks if ident appears as a whole-word identifier in text.
func jsIdentUsed(text, ident string) bool {
	off := 0
	for {
		idx := strings.Index(text[off:], ident)
		if idx < 0 {
			return false
		}
		pos := off + idx
		end := pos + len(ident)
		before := pos == 0 || !isJSWordChar(text[pos-1])
		after := end >= len(text) || !isJSWordChar(text[end])
		if before && after {
			return true
		}
		off = end
	}
}

func isJSWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '$'
}

// jsImportInsertEdits produces an edit to insert import statements into a file.
func jsImportInsertEdits(file string, content []byte, stmts []string) []ingest.Edit {
	if len(stmts) == 0 {
		return nil
	}
	text := string(content)
	// Filter out imports already present.
	var missing []string
	for _, s := range stmts {
		if !strings.Contains(text, s) {
			missing = append(missing, s)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	// Find position after the last import statement.
	lines := strings.Split(text, "\n")
	offset := 0
	insertPos := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineEnd := offset + len(line) + 1 // +1 for \n
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "import\t") {
			insertPos = lineEnd
		}
		offset = lineEnd
	}
	block := strings.Join(missing, "\n") + "\n"
	return []ingest.Edit{{
		File:      file,
		StartByte: uint32(insertPos),
		EndByte:   uint32(insertPos),
		NewText:   block,
	}}
}

// stripUnusedJSImports removes import statements from the source file that were
// only used by the removed declaration.
func stripUnusedJSImports(file string, content []byte, decl ingest.DeclExtract) []ingest.Edit {
	if len(decl.Imports) == 0 {
		return nil
	}
	// Build set of import texts that the declaration used.
	declImports := map[string]bool{}
	for _, imp := range decl.Imports {
		declImports[imp] = true
	}
	// Mask out the declaration region to see what the rest of the file uses.
	masked := append([]byte(nil), content...)
	for i := decl.RemoveStart; i < decl.RemoveEnd && int(i) < len(masked); i++ {
		if masked[i] != '\n' {
			masked[i] = ' '
		}
	}

	// For each import used by the declaration, check if any of its local
	// names are still referenced in the rest of the file.
	pf, err := ingest.ParseSource(content, file, "")
	if err != nil {
		return nil
	}
	defer pf.Close()
	root := pf.Root

	stmts := parseJSImportStatements(root, content)

	// Also mask out all import statements so we only check usage in
	// the non-import body of the file.
	for _, stmt := range stmts {
		for i := stmt.startByte; i < stmt.endByte && int(i) < len(masked); i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	restText := string(masked)

	var edits []ingest.Edit
	for _, stmt := range stmts {
		if !declImports[stmt.text] {
			continue
		}
		// Check if any local name from this import is still used in the body.
		stillUsed := false
		for _, local := range stmt.locals {
			if jsIdentUsed(restText, local) {
				stillUsed = true
				break
			}
		}
		if stillUsed {
			continue
		}
		// Remove the entire import statement including trailing newline.
		removeEnd := stmt.endByte
		for removeEnd < uint32(len(content)) && content[removeEnd] == '\n' {
			removeEnd++
			break // remove at most one trailing newline
		}
		edits = append(edits, ingest.Edit{
			File:      file,
			StartByte: stmt.startByte,
			EndByte:   removeEnd,
			NewText:   "",
		})
	}
	return edits
}

// jsRelativeImportPath computes a relative import path from fromFile to toFile.
// Both paths are relative to the ingest root (no leading "./").
func jsRelativeImportPath(fromFile, toFile string) string {
	fromDir := path.Dir(fromFile)
	rel, err := relPath(fromDir, toFile)
	if err != nil {
		return "./" + toFile
	}
	// Strip file extension for the import path.
	for _, ext := range []string{".tsx", ".ts", ".jsx", ".js", ".mjs"} {
		if strings.HasSuffix(rel, ext) {
			rel = strings.TrimSuffix(rel, ext)
			break
		}
	}
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel
}

// relPath computes a relative path from base directory to target file.
func relPath(base, target string) (string, error) {
	baseParts := splitPath(base)
	targetParts := splitPath(target)

	// Find common prefix length.
	common := 0
	for common < len(baseParts) && common < len(targetParts) && baseParts[common] == targetParts[common] {
		common++
	}

	// Number of ".." segments needed.
	ups := len(baseParts) - common
	var parts []string
	for i := 0; i < ups; i++ {
		parts = append(parts, "..")
	}
	parts = append(parts, targetParts[common:]...)
	return strings.Join(parts, "/"), nil
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return nil
	}
	return strings.Split(p, "/")
}

// findJSDecl returns the declaration node whose name starts at nameStart.
// If the declaration is wrapped in an export_statement, the export node is
// returned as the second value so the caller can include it in the extraction.
func findJSDecl(root *grammar.Node, nameStart uint32) (decl *grammar.Node, export *grammar.Node) {
	declTypes := map[string]bool{
		"function_declaration":           true,
		"generator_function_declaration": true,
		"class_declaration":              true,
		"abstract_class_declaration":     true,
		"interface_declaration":          true,
		"type_alias_declaration":         true,
		"enum_declaration":               true,
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if declTypes[child.Type()] {
			if n := ingest.ChildByField(child, "name"); n != nil && n.StartByte() == nameStart {
				return child, nil
			}
		}
		if child.Type() == "export_statement" {
			for j := uint32(0); j < child.ChildCount(); j++ {
				inner := child.Child(j)
				if declTypes[inner.Type()] {
					if n := ingest.ChildByField(inner, "name"); n != nil && n.StartByte() == nameStart {
						return inner, child
					}
				}
			}
		}
	}
	return nil, nil
}

// spanOccupied reports whether [start,end) overlaps any already-covered rename span.
func spanOccupied(occ map[[2]uint32]bool, start, end uint32) bool {
	if occ == nil {
		return false
	}
	if occ[[2]uint32{start, end}] {
		return true
	}
	for k := range occ {
		if start < k[1] && end > k[0] {
			return true
		}
	}
	return false
}

// ExtraRenameEdits rewrites member-expression call sites when renaming a method
// (Class.method → Class.newName). Relation-based rename only covers entity/
// relation spans; instance receivers (this/params/locals) are not entities.
// Mirrors Go/Python ExtraRenameEdits for ECMA member expressions.
func (moveDriver) ExtraRenameEdits(rootDir string, result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) []ingest.Edit {
	if oldLeaf == "" || oldLeaf == newLeaf || len(sourceRefs) == 0 || rootDir == "" || result == nil {
		return nil
	}
	src := ingest.ParseReference(sourceRefs[0])
	if !strings.Contains(src.Symbol, ".") {
		return nil // only methods / nested symbols
	}

	ourReceivers := map[string]bool{}
	sourceSet := map[string]bool{}
	for _, s := range sourceRefs {
		sourceSet[s] = true
		ref := ingest.ParseReference(s)
		if recv, ok := jsMethodReceiver(ref.Symbol); ok {
			ourReceivers[recv] = true
		}
	}
	if len(ourReceivers) == 0 {
		return nil
	}

	foreignReceivers := map[string]bool{}
	for _, ent := range result.Entities {
		if sourceSet[ent.Reference] {
			continue
		}
		ref := ingest.ParseReference(ent.Reference)
		if ingest.SymbolLeaf(ref.Symbol) != oldLeaf {
			continue
		}
		if recv, ok := jsMethodReceiver(ref.Symbol); ok && !ourReceivers[recv] {
			foreignReceivers[recv] = true
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
	for _, rel := range result.Relations {
		if !sourceSet[rel.Target] {
			continue
		}
		ref := ingest.ParseReference(rel.Reference)
		mark(ref.Path, rel.StartByte, rel.EndByte)
	}

	var edits []ingest.Edit
	for _, f := range result.Files {
		if f.Language != "javascript" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		occ := occupied[rel]
		for _, e := range jsMethodAttrEdits(rel, content, oldLeaf, newLeaf, ourReceivers, foreignReceivers) {
			if spanOccupied(occ, e.StartByte, e.EndByte) {
				continue
			}
			edits = append(edits, e)
		}
	}
	return edits
}

// jsMethodReceiver returns the class/type name for "Class.method" symbols.
func jsMethodReceiver(symbol string) (string, bool) {
	if symbol == "" || !strings.Contains(symbol, ".") {
		return "", false
	}
	parts := strings.Split(symbol, ".")
	if len(parts) < 2 {
		return "", false
	}
	recv := strings.Join(parts[:len(parts)-1], ".")
	return recv, recv != ""
}

// jsMethodAttrEdits finds obj.oldLeaf property nodes to rewrite.
func jsMethodAttrEdits(fileRel string, content []byte, oldLeaf, newLeaf string, ourReceivers, foreignReceivers map[string]bool) []ingest.Edit {
	pf, err := ingest.ParseSource(content, fileRel, "javascript")
	if err != nil {
		return nil
	}
	defer pf.Close()

	typedLocals := jsTypedLocals(pf.Root, content, ourReceivers)

	var edits []ingest.Edit
	var walk func(n *grammar.Node, enclosingClass string)
	walk = func(n *grammar.Node, enclosingClass string) {
		if n == nil || n.IsNull() {
			return
		}
		classHere := enclosingClass
		switch n.Type() {
		case "class_declaration", "class", "abstract_class_declaration":
			if nameN := ingest.ChildByField(n, "name"); nameN != nil {
				classHere = ingest.NodeText(nameN, content)
			}
		}
		switch n.Type() {
		case "member_expression", "member_expression_optional", "optional_chain":
			obj := ingest.ChildByField(n, "object")
			prop := ingest.ChildByField(n, "property")
			if obj != nil && prop != nil && ingest.NodeText(prop, content) == oldLeaf {
				if jsShouldRenameMember(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals) {
					edits = append(edits, ingest.Edit{
						File:      fileRel,
						StartByte: prop.StartByte(),
						EndByte:   prop.EndByte(),
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

// jsShouldRenameMember decides whether obj.oldLeaf is a call on one of our receivers.
func jsShouldRenameMember(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool) bool {
	if obj == nil {
		return false
	}
	// this.x / super.x
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
	// Only simple identifiers: box.x, Box.x
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
	// Unique method leaf in the project graph: rewrite all simple member loads.
	return len(foreignReceivers) == 0
}

// jsTypedLocals maps local names constructed as ourReceivers (const b = new Box()).
// Also covers simple TS-style typed parameters when present as type annotations.
func jsTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) map[string]bool {
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
		case "lexical_declaration", "variable_declaration":
			// const box = new Box(); var box = new Box()
			for i := uint32(0); i < n.ChildCount(); i++ {
				child := n.Child(i)
				if child.Type() != "variable_declarator" {
					continue
				}
				nameN := ingest.ChildByField(child, "name")
				valN := ingest.ChildByField(child, "value")
				if nameN == nil || nameN.Type() != "identifier" || valN == nil {
					continue
				}
				if ctor := jsNewExpressionType(valN, content); ourReceivers[ctor] {
					out[ingest.NodeText(nameN, content)] = true
				}
			}
		case "required_parameter", "optional_parameter", "assignment_pattern":
			// TS: (b: Box) / (b: Box = ...)
			nameN := ingest.ChildByField(n, "pattern")
			if nameN == nil {
				nameN = ingest.ChildByField(n, "name")
			}
			if nameN == nil {
				nameN = ingest.ChildByType(n, "identifier")
			}
			typeN := ingest.ChildByField(n, "type")
			if nameN != nil && nameN.Type() == "identifier" && typeN != nil {
				if tn := jsTypeName(typeN, content); ourReceivers[tn] {
					out[ingest.NodeText(nameN, content)] = true
				}
			}
		case "formal_parameters":
			// plain JS has no types; walk children for TS parameters
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out
}

// jsNewExpressionType returns the constructor identifier for `new Box(...)`.
func jsNewExpressionType(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	if n.Type() != "new_expression" {
		return ""
	}
	ctor := ingest.ChildByField(n, "constructor")
	if ctor == nil {
		// some grammars use "function" or first named child
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c.Type() == "identifier" {
				return ingest.NodeText(c, content)
			}
		}
		return ""
	}
	if ctor.Type() == "identifier" {
		return ingest.NodeText(ctor, content)
	}
	return ""
}

// jsTypeName extracts a simple class name from a TS type annotation node.
func jsTypeName(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	// type_annotation wraps the type: `: Box`
	if typeN.Type() == "type_annotation" && typeN.ChildCount() > 0 {
		// skip `:`
		for i := uint32(0); i < typeN.ChildCount(); i++ {
			c := typeN.Child(i)
			if c.Type() == ":" {
				continue
			}
			typeN = c
			break
		}
	}
	switch typeN.Type() {
	case "type_identifier", "identifier":
		return ingest.NodeText(typeN, content)
	case "generic_type":
		if name := ingest.ChildByField(typeN, "name"); name != nil {
			return ingest.NodeText(name, content)
		}
	}
	return ""
}
