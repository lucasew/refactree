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


// ExpandRenameSources ties TypeScript interface methods to implementors:
//
//  1. Renaming Iface.method expands Class.method for classes that implement Iface.
//  2. Renaming Class.method expands Iface.method for interfaces Class implements.
//
// Type-alias object types are structural — no expand (call sites use ExtraRename).
func (moveDriver) ExpandRenameSources(rootDir string, result *ingest.Result, sourceRef string) []string {
	src := ingest.ParseReference(sourceRef)
	if src.Symbol == "" || !strings.Contains(src.Symbol, ".") {
		return nil
	}
	leaf := ingest.SymbolLeaf(src.Symbol)
	recv, ok := jsMethodReceiver(src.Symbol)
	if !ok || leaf == "" || recv == "" || result == nil {
		return nil
	}

	// Types whose Type.leaf entities should join the rename source set.
	related := map[string]bool{}
	if jsTypeIsInterface(rootDir, result, recv) {
		for c := range jsClassesImplementing(rootDir, result, recv) {
			related[c] = true
		}
	} else {
		for iface := range jsInterfacesImplementedBy(rootDir, result, recv) {
			related[iface] = true
		}
	}
	if len(related) == 0 {
		return nil
	}

	var extra []string
	for _, ent := range result.Entities {
		if ent.Reference == sourceRef {
			continue
		}
		ref := ingest.ParseReference(ent.Reference)
		if ref.Provider != "path" || ref.Symbol == "" {
			continue
		}
		if ingest.SymbolLeaf(ref.Symbol) != leaf {
			continue
		}
		entRecv, isMethod := jsMethodReceiver(ref.Symbol)
		if !isMethod || !related[entRecv] {
			continue
		}
		extra = append(extra, ent.Reference)
	}
	return extra
}

// jsInterfacesImplementedBy returns interface names that className lists in its
// implements clause across javascript files in result.
func jsInterfacesImplementedBy(rootDir string, result *ingest.Result, className string) map[string]bool {
	out := map[string]bool{}
	if className == "" || result == nil {
		return out
	}
	for _, f := range result.Files {
		if f.Language != "javascript" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		pf, err := ingest.ParseSource(content, rel, "javascript")
		if err != nil {
			continue
		}
		var walk func(n *grammar.Node)
		walk = func(n *grammar.Node) {
			if n == nil {
				return
			}
			if n.Type() == "class_declaration" || n.Type() == "abstract_class_declaration" || n.Type() == "class" {
				nameN := ingest.ChildByField(n, "name")
				if nameN != nil && ingest.NodeText(nameN, content) == className {
					for iface := range jsClassImplementsNames(n, content) {
						out[iface] = true
					}
				}
			}
			for i := uint32(0); i < n.ChildCount(); i++ {
				walk(n.Child(i))
			}
		}
		walk(pf.Root)
		pf.Close()
	}
	return out
}

// jsTypeIsInterface reports whether typeName is declared as an interface in
// any javascript file of result.
func jsTypeIsInterface(rootDir string, result *ingest.Result, typeName string) bool {
	if typeName == "" || result == nil {
		return false
	}
	for _, f := range result.Files {
		if f.Language != "javascript" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if jsTypeIsInterfaceInFile(content, rel, typeName) {
			return true
		}
	}
	return false
}

func jsTypeIsInterfaceInFile(content []byte, fileRel, typeName string) bool {
	if typeName == "" {
		return false
	}
	pf, err := ingest.ParseSource(content, fileRel, "javascript")
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
		if n.Type() == "interface_declaration" {
			if nameN := ingest.ChildByField(n, "name"); nameN != nil && ingest.NodeText(nameN, content) == typeName {
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

// jsClassesImplementing returns simple class names that declare
// `implements … Iface …` for ifaceName across javascript files in result.
func jsClassesImplementing(rootDir string, result *ingest.Result, ifaceName string) map[string]bool {
	out := map[string]bool{}
	if ifaceName == "" || result == nil {
		return out
	}
	for _, f := range result.Files {
		if f.Language != "javascript" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		pf, err := ingest.ParseSource(content, rel, "javascript")
		if err != nil {
			continue
		}
		var walk func(n *grammar.Node)
		walk = func(n *grammar.Node) {
			if n == nil {
				return
			}
			if n.Type() == "class_declaration" || n.Type() == "abstract_class_declaration" || n.Type() == "class" {
				if jsClassImplements(n, content, ifaceName) {
					if nameN := ingest.ChildByField(n, "name"); nameN != nil {
						if cn := ingest.NodeText(nameN, content); cn != "" {
							out[cn] = true
						}
					}
				}
			}
			for i := uint32(0); i < n.ChildCount(); i++ {
				walk(n.Child(i))
			}
		}
		walk(pf.Root)
		pf.Close()
	}
	return out
}

// jsClassImplements reports whether class node lists ifaceName in its
// implements clause (class_heritage / implements_clause).
func jsClassImplements(class *grammar.Node, content []byte, ifaceName string) bool {
	if ifaceName == "" {
		return false
	}
	return jsClassImplementsNames(class, content)[ifaceName]
}

// jsClassImplementsNames returns interface type names listed on the class
// implements clause.
func jsClassImplementsNames(class *grammar.Node, content []byte) map[string]bool {
	out := map[string]bool{}
	if class == nil {
		return out
	}
	var collect func(n *grammar.Node)
	collect = func(n *grammar.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "type_identifier", "identifier":
			if name := ingest.NodeText(n, content); name != "" {
				out[name] = true
			}
			return
		case "generic_type":
			if name := ingest.ChildByField(n, "name"); name != nil {
				if t := ingest.NodeText(name, content); t != "" {
					out[t] = true
				}
			}
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			collect(n.Child(i))
		}
	}
	// Prefer class_heritage / implements_clause children so we do not match
	// the class name itself or body type annotations.
	for i := uint32(0); i < class.ChildCount(); i++ {
		ch := class.Child(i)
		switch ch.Type() {
		case "class_heritage":
			// Only the implements_clause branch — not extends.
			for j := uint32(0); j < ch.ChildCount(); j++ {
				inner := ch.Child(j)
				if inner.Type() == "implements_clause" {
					collect(inner)
				}
			}
		case "implements_clause":
			collect(ch)
		default:
			// Bare `implements` keyword sibling form.
			if ch.Type() == "implements" {
				for j := i + 1; j < class.ChildCount(); j++ {
					sib := class.Child(j)
					if sib.Type() == "{" || sib.Type() == "class_body" || sib.Type() == "extends" || sib.Type() == "class_heritage" {
						break
					}
					collect(sib)
				}
			}
		}
	}
	return out
}

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
		before := pos == 0 || !ingest.IsIdentCharJava(text[pos-1])
		after := end >= len(text) || !ingest.IsIdentCharJava(text[end])
		if before && after {
			return true
		}
		off = end
	}
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

// ExtraRenameEdits rewrites member-expression call sites when renaming a method
// (Class.method → Class.newName). Relation-based rename only covers entity/
// relation spans; instance receivers (this/params/locals) are not entities.
func (moveDriver) ExtraRenameEdits(rootDir string, result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) []ingest.Edit {
	if oldLeaf == "" || oldLeaf == newLeaf || len(sourceRefs) == 0 || rootDir == "" || result == nil {
		return nil
	}
	src := ingest.ParseReference(sourceRefs[0])
	if !strings.Contains(src.Symbol, ".") {
		return jsShorthandPropertyAccessEdits(rootDir, result, sourceRefs, oldLeaf, newLeaf)
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

	occupied := ingest.MarkEntityRelationSpans(result, sourceSet)

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
			if ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
				continue
			}
			edits = append(edits, e)
		}
	}
	return edits
}

// jsShorthandPropertyAccessEdits covers function/value renames where object
// shorthand ({ helper }) rewrites the property key with the value. Locals
// bound to such objects still have o.helper / { helper: h } = o under the old
// key after the shorthand rewrite — rename those access sites too.
func jsShorthandPropertyAccessEdits(rootDir string, result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) []ingest.Edit {
	sourceSet := map[string]bool{}
	for _, s := range sourceRefs {
		sourceSet[s] = true
	}
	// Only when relation rename actually touches a shorthand use of this symbol.
	hasShorthandUse := false
	for _, reln := range result.Relations {
		if !sourceSet[reln.Target] {
			continue
		}
		ref := ingest.ParseReference(reln.Reference)
		fileRel := strings.TrimPrefix(ref.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(fileRel)))
		if err != nil || int(reln.EndByte) > len(content) {
			continue
		}
		// Heuristic: the usage span text is oldLeaf and sits inside `{ … }`.
		if string(content[reln.StartByte:reln.EndByte]) != oldLeaf {
			continue
		}
		if jsSpanIsObjectShorthand(content, reln.StartByte, reln.EndByte) {
			hasShorthandUse = true
			break
		}
	}
	if !hasShorthandUse {
		return nil
	}

	occupied := ingest.MarkEntityRelationSpans(result, sourceSet)
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
		for _, e := range jsShorthandLocalPropertyEdits(rel, content, oldLeaf, newLeaf) {
			if ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
				continue
			}
			edits = append(edits, e)
		}
	}
	return edits
}

// jsSpanIsObjectShorthand reports whether [start:end) is a shorthand property
// name inside an object literal (not a longhand key or a pattern binding).

// jsSpanIsObjectShorthand reports whether [start:end) is a shorthand property
// name inside an object literal (not a longhand key or a pattern binding).
func jsSpanIsObjectShorthand(content []byte, start, end uint32) bool {
	pf, err := ingest.ParseSource(content, "file.js", "javascript")
	if err != nil {
		return false
	}
	defer pf.Close()
	var found bool
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() || found {
			return
		}
		if n.Type() == "shorthand_property_identifier" &&
			n.StartByte() == start && n.EndByte() == end {
			found = true
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(pf.Root)
	return found
}

// jsShorthandLocalPropertyEdits finds locals assigned an object literal that
// contains shorthand oldLeaf, then rewrites .oldLeaf member accesses and
// object-pattern keys on those locals (and nested property paths).

// jsShorthandLocalPropertyEdits finds locals assigned an object literal that
// contains shorthand oldLeaf, then rewrites .oldLeaf member accesses and
// object-pattern keys on those locals (and nested property paths).
func jsShorthandLocalPropertyEdits(fileRel string, content []byte, oldLeaf, newLeaf string) []ingest.Edit {
	pf, err := ingest.ParseSource(content, fileRel, "javascript")
	if err != nil {
		return nil
	}
	defer pf.Close()

	// binding → true when the bound object literal includes shorthand oldLeaf
	// either at the top level or nested under property paths we track.
	// For nested `{ nested: { helper } }` we record binding "o" with path
	// prefix "nested" so o.nested.helper rewrites.
	type pathKey struct {
		binding string
		// dotted path under binding to the object that has the shorthand key,
		// empty when the shorthand is a direct property of the binding.
		prefix string
	}
	shorthandLocals := map[pathKey]bool{}

	// Collect (binding, prefix) for every shorthand occurrence under an object
	// assigned to a local. prefix is the path of pair keys from the binding root
	// to the object that owns the shorthand.
	var collectShorthandPaths func(obj *grammar.Node, binding, prefix string)
	collectShorthandPaths = func(obj *grammar.Node, binding, prefix string) {
		if obj == nil || obj.Type() != "object" || binding == "" {
			return
		}
		for i := uint32(0); i < obj.ChildCount(); i++ {
			ch := obj.Child(i)
			switch ch.Type() {
			case "shorthand_property_identifier":
				if ingest.NodeText(ch, content) == oldLeaf {
					shorthandLocals[pathKey{binding, prefix}] = true
				}
			case "pair":
				key := ingest.ChildByField(ch, "key")
				val := ingest.ChildByField(ch, "value")
				if key == nil || val == nil {
					continue
				}
				keyName := ingest.NodeText(key, content)
				// Only simple identifier keys form a stable path.
				if key.Type() != "property_identifier" && key.Type() != "identifier" {
					continue
				}
				next := keyName
				if prefix != "" {
					next = prefix + "." + keyName
				}
				collectShorthandPaths(val, binding, next)
			}
		}
	}

	var walkDecl func(n *grammar.Node)
	walkDecl = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "variable_declarator" {
			nameN := ingest.ChildByField(n, "name")
			valN := ingest.ChildByField(n, "value")
			if nameN != nil && nameN.Type() == "identifier" && valN != nil && valN.Type() == "object" {
				collectShorthandPaths(valN, ingest.NodeText(nameN, content), "")
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walkDecl(n.Child(i))
		}
	}
	walkDecl(pf.Root)
	if len(shorthandLocals) == 0 {
		return nil
	}

	var edits []ingest.Edit
	seen := map[[2]uint32]bool{}
	add := func(start, end uint32) {
		key := [2]uint32{start, end}
		if seen[key] {
			return
		}
		seen[key] = true
		edits = append(edits, ingest.Edit{
			File:      fileRel,
			StartByte: start,
			EndByte:   end,
			NewText:   newLeaf,
		})
	}

	// member chain path: o.nested.helper → binding o, prefix nested, prop helper
	memberPath := func(n *grammar.Node) (binding, prefix, prop string, propNode *grammar.Node, ok bool) {
		if n == nil {
			return "", "", "", nil, false
		}
		propN := ingest.ChildByField(n, "property")
		obj := ingest.ChildByField(n, "object")
		if propN == nil || obj == nil || ingest.NodeText(propN, content) != oldLeaf {
			return "", "", "", nil, false
		}
		// Walk object chain leftward collecting property names until identifier.
		var parts []string
		cur := obj
		for cur != nil {
			switch cur.Type() {
			case "identifier":
				binding = ingest.NodeText(cur, content)
				// parts are outermost-first? we walked inward so reverse
				for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
					parts[i], parts[j] = parts[j], parts[i]
				}
				prefix = strings.Join(parts, ".")
				return binding, prefix, oldLeaf, propN, true
			case "member_expression", "member_expression_optional", "optional_chain":
				p := ingest.ChildByField(cur, "property")
				if p == nil {
					return "", "", "", nil, false
				}
				parts = append(parts, ingest.NodeText(p, content))
				cur = ingest.ChildByField(cur, "object")
			default:
				return "", "", "", nil, false
			}
		}
		return "", "", "", nil, false
	}

	var walkUses func(n *grammar.Node)
	walkUses = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "member_expression", "member_expression_optional", "optional_chain":
			if binding, prefix, _, propN, ok := memberPath(n); ok {
				if shorthandLocals[pathKey{binding, prefix}] {
					add(propN.StartByte(), propN.EndByte())
				}
			}
		case "variable_declarator":
			// const { helper: h } = o  /  const { helper } = o.nested
			nameN := ingest.ChildByField(n, "name")
			valN := ingest.ChildByField(n, "value")
			if nameN != nil && nameN.Type() == "object_pattern" && valN != nil {
				binding, prefix := "", ""
				switch valN.Type() {
				case "identifier":
					binding = ingest.NodeText(valN, content)
				case "member_expression", "member_expression_optional", "optional_chain":
					// o.nested → binding o, prefix nested
					var parts []string
					cur := valN
					for cur != nil {
						if cur.Type() == "identifier" {
							binding = ingest.NodeText(cur, content)
							for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
								parts[i], parts[j] = parts[j], parts[i]
							}
							prefix = strings.Join(parts, ".")
							break
						}
						if cur.Type() == "member_expression" || cur.Type() == "member_expression_optional" || cur.Type() == "optional_chain" {
							p := ingest.ChildByField(cur, "property")
							if p == nil {
								break
							}
							parts = append(parts, ingest.NodeText(p, content))
							cur = ingest.ChildByField(cur, "object")
							continue
						}
						break
					}
				}
				if binding != "" && shorthandLocals[pathKey{binding, prefix}] {
					jsCollectObjectPatternMethodEdits(nameN, content, oldLeaf, newLeaf, func(start, end uint32, text string) {
						add(start, end)
					})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walkUses(n.Child(i))
		}
	}
	walkUses(pf.Root)
	return edits
}

// jsMethodReceiver returns the class/type name for "Class.method" symbols.

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
	// Unique method leaf: ExtraRename already rewrites every simple obj.oldLeaf.
	// Apply the same aggressiveness to object-pattern property keys.
	uniqueLeaf := len(foreignReceivers) == 0

	var edits []ingest.Edit
	// Spans already scheduled (dedupe pattern keys when both declarator + pattern walk hit).
	seen := map[[2]uint32]bool{}
	addEdit := func(start, end uint32, text string) {
		key := [2]uint32{start, end}
		if seen[key] {
			return
		}
		seen[key] = true
		edits = append(edits, ingest.Edit{
			File:      fileRel,
			StartByte: start,
			EndByte:   end,
			NewText:   text,
		})
	}

	var walk func(n *grammar.Node, enclosingClass string, block *grammar.Node)
	walk = func(n *grammar.Node, enclosingClass string, block *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		classHere := enclosingClass
		blockHere := block
		switch n.Type() {
		case "class_declaration", "class", "abstract_class_declaration":
			if nameN := ingest.ChildByField(n, "name"); nameN != nil {
				classHere = ingest.NodeText(nameN, content)
			}
		case "statement_block":
			blockHere = n
		}
		switch n.Type() {
		case "member_expression", "member_expression_optional", "optional_chain":
			obj := ingest.ChildByField(n, "object")
			prop := ingest.ChildByField(n, "property")
			if obj != nil && prop != nil && ingest.NodeText(prop, content) == oldLeaf {
				if jsShouldRenameMember(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals) {
					addEdit(prop.StartByte(), prop.EndByte(), newLeaf)
				}
			}
		case "private_property_identifier":
			// Brand checks: `#helper in this` — bare private id, not member_expression.
			// Only rewrite inside a class we own (private names are class-scoped).
			if strings.HasPrefix(oldLeaf, "#") && ingest.NodeText(n, content) == oldLeaf {
				if classHere != "" && ourReceivers[classHere] {
					addEdit(n.StartByte(), n.EndByte(), newLeaf)
				}
			}
		case "variable_declarator":
			// const { helper } = b  /  const { helper: h } = b
			nameN := ingest.ChildByField(n, "name")
			valN := ingest.ChildByField(n, "value")
			if nameN != nil && nameN.Type() == "object_pattern" && valN != nil {
				if uniqueLeaf || jsShouldRenameMember(valN, content, classHere, ourReceivers, foreignReceivers, typedLocals) {
					jsCollectObjectPatternMethodEdits(nameN, content, oldLeaf, newLeaf, addEdit)
					// Shorthand `{ helper }` also renames the local binding — rewrite
					// bare oldLeaf identifiers later in the same statement_block.
					if blockHere != nil {
						jsCollectShorthandBindingUses(blockHere, nameN, content, oldLeaf, newLeaf, addEdit)
					}
				}
			}
		case "function_declaration", "generator_function_declaration",
			"function_expression", "arrow_function", "method_definition":
			// Parameter destructure: function use({ helper }) { … }
			// formal_parameters is a sibling of body, so handle both here.
			// Only when the method leaf is unique (no RHS type on the pattern).
			if uniqueLeaf {
				params := ingest.ChildByField(n, "parameters")
				body := ingest.ChildByField(n, "body")
				if params != nil {
					jsForEachObjectPattern(params, func(pattern *grammar.Node) {
						jsCollectObjectPatternMethodEdits(pattern, content, oldLeaf, newLeaf, addEdit)
						if body != nil && body.Type() == "statement_block" {
							jsCollectShorthandBindingUses(body, pattern, content, oldLeaf, newLeaf, addEdit)
						}
					})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), classHere, blockHere)
		}
	}
	walk(pf.Root, "", nil)
	return edits
}

// jsForEachObjectPattern invokes fn for every object_pattern under n.
func jsForEachObjectPattern(n *grammar.Node, fn func(*grammar.Node)) {
	if n == nil {
		return
	}
	if n.Type() == "object_pattern" {
		fn(n)
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		jsForEachObjectPattern(n.Child(i), fn)
	}
}

// jsCollectObjectPatternMethodEdits rewrites property keys named oldLeaf in an object_pattern.
func jsCollectObjectPatternMethodEdits(pattern *grammar.Node, content []byte, oldLeaf, newLeaf string, add func(start, end uint32, text string)) {
	if pattern == nil || pattern.Type() != "object_pattern" {
		return
	}
	for i := uint32(0); i < pattern.ChildCount(); i++ {
		ch := pattern.Child(i)
		switch ch.Type() {
		case "shorthand_property_identifier_pattern":
			if ingest.NodeText(ch, content) == oldLeaf {
				add(ch.StartByte(), ch.EndByte(), newLeaf)
			}
		case "pair_pattern":
			if key := ingest.ChildByField(ch, "key"); key != nil && ingest.NodeText(key, content) == oldLeaf {
				add(key.StartByte(), key.EndByte(), newLeaf)
			}
		case "object_pattern":
			jsCollectObjectPatternMethodEdits(ch, content, oldLeaf, newLeaf, add)
		case "assignment_pattern":
			// `{ helper = def }` — left is the binding/property name.
			if left := ingest.ChildByField(ch, "left"); left != nil {
				switch left.Type() {
				case "shorthand_property_identifier_pattern", "identifier":
					if ingest.NodeText(left, content) == oldLeaf {
						add(left.StartByte(), left.EndByte(), newLeaf)
					}
				case "object_pattern":
					jsCollectObjectPatternMethodEdits(left, content, oldLeaf, newLeaf, add)
				}
			}
		}
	}
}

// jsCollectShorthandBindingUses rewrites bare identifier uses of a shorthand
// destructure binding after the pattern inside block (same statement_block).
func jsCollectShorthandBindingUses(block, pattern *grammar.Node, content []byte, oldLeaf, newLeaf string, add func(start, end uint32, text string)) {
	if block == nil || pattern == nil {
		return
	}
	// Only when the pattern actually introduced a shorthand binding named oldLeaf.
	hasShorthand := false
	var after uint32
	var scanPattern func(*grammar.Node)
	scanPattern = func(p *grammar.Node) {
		if p == nil {
			return
		}
		if p.Type() == "shorthand_property_identifier_pattern" && ingest.NodeText(p, content) == oldLeaf {
			hasShorthand = true
			if p.EndByte() > after {
				after = p.EndByte()
			}
		}
		// assignment_pattern left shorthand also binds locally under the old name
		// before we rewrite it — treat like shorthand.
		if p.Type() == "assignment_pattern" {
			if left := ingest.ChildByField(p, "left"); left != nil {
				if (left.Type() == "shorthand_property_identifier_pattern" || left.Type() == "identifier") &&
					ingest.NodeText(left, content) == oldLeaf {
					hasShorthand = true
					if left.EndByte() > after {
						after = left.EndByte()
					}
				}
			}
		}
		for i := uint32(0); i < p.ChildCount(); i++ {
			scanPattern(p.Child(i))
		}
	}
	scanPattern(pattern)
	if !hasShorthand {
		return
	}

	var walk func(*grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		// Do not walk into nested function bodies (new scopes).
		switch n.Type() {
		case "function_declaration", "generator_function_declaration",
			"function_expression", "arrow_function", "method_definition",
			"class_declaration", "class", "abstract_class_declaration":
			return
		case "identifier":
			if n.StartByte() >= after && ingest.NodeText(n, content) == oldLeaf {
				add(n.StartByte(), n.EndByte(), newLeaf)
			}
			return
		case "property_identifier", "shorthand_property_identifier",
			"shorthand_property_identifier_pattern", "private_property_identifier":
			// Property names / patterns handled elsewhere.
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(block)
}

// jsShouldRenameMember decides whether obj.oldLeaf is a call on one of our receivers.
func jsShouldRenameMember(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool) bool {
	if obj == nil {
		return false
	}
	// Unwrap (new Box()) so parenthesized new expressions still match.
	for obj != nil && obj.Type() == "parenthesized_expression" {
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
		return false
	}
	// super.x targets a parent implementation, not the enclosing class's own
	// override. When renaming Base.m, rewrite super.m in Child even if Child
	// also defines m; when renaming Child.m, leave super.m alone.
	// Mirrors pythonShouldRenameAttr's super() handling.
	if obj.Type() == "super" {
		if enclosingClass != "" && ourReceivers[enclosingClass] {
			return false
		}
		return true
	}
	// this.x
	if obj.Type() == "this" {
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
	// new Box().m / new Box(1).m — mirror Java object_creation_expression.
	if obj.Type() == "new_expression" {
		ctor := jsNewExpressionType(obj, content)
		if ctor == "" {
			return len(foreignReceivers) == 0
		}
		if ourReceivers[ctor] {
			return true
		}
		if foreignReceivers[ctor] {
			return false
		}
		return len(foreignReceivers) == 0
	}
	// Simple identifiers: box.x, Box.x
	if obj.Type() == "identifier" {
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
	// Complex expression receivers — call chains (b.next().m), await, ternary,
	// nullish/binary, subscript — only when the method leaf is unique project-wide.
	// Without static return types we cannot disambiguate foreign same-leaf methods.
	switch obj.Type() {
	case "call_expression", "await_expression", "ternary_expression",
		"binary_expression", "member_expression",
		"member_expression_optional", "optional_chain",
		"subscript_expression", "assignment_expression":
		return len(foreignReceivers) == 0
	}
	return false
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
