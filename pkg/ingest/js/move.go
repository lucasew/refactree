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
	return ingest.IdentUsed(text, ident, ingest.IsIdentCharJava)
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
	ingest.MaskNonNewlinesInPlace(masked, int(decl.RemoveStart), int(decl.RemoveEnd))

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
		ingest.MaskNonNewlinesInPlace(masked, int(stmt.startByte), int(stmt.endByte))
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

	factories := jsSameFileFactoryReturns(pf.Root, content)
	generators := jsSameFileGeneratorYields(pf.Root, content)
	typedLocals, settledOf, genLocals, arrayLocals, entryLocals := jsTypedLocals(pf.Root, content, ourReceivers, factories, generators)
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
				if jsShouldRenameMember(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals) {
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
				if uniqueLeaf || jsShouldRenameMember(valN, content, classHere, ourReceivers, foreignReceivers, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals) {
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

// jsRenameByTypeMaps: our → rename; foreign → skip; typedLocals → rename; else unique-leaf only.
// typedLocals maps local name → concrete type leaf (only our-receiver locals).
func jsRenameByTypeMaps(name string, ourReceivers, foreignReceivers map[string]bool, typedLocals map[string]string) bool {
	if ourReceivers[name] {
		return true
	}
	if foreignReceivers[name] {
		return false
	}
	if typedLocals != nil {
		if t := typedLocals[name]; t != "" && ourReceivers[t] {
			return true
		}
	}
	return len(foreignReceivers) == 0
}

// jsShouldRenameMember decides whether obj.oldLeaf is a call on one of our receivers.
// factories maps same-file factory names → class leaf (makeA → A) for
// Promise.resolve(makeA()) peels under foreign same-leaf methods.
// settledOf maps Promise.allSettled / generator .next() / array iterator .next()
// result locals → value type leaf so r.value.run() peels under foreign same-leaf.
// generators maps same-file generator names → yield type leaf (genA → A).
// genLocals maps generator/array-iterator instance locals → yield/elem type
// (const g = genA() → g:"A", const ia = [new A()].values() → ia:"A").
// arrayLocals maps array locals → uniform element type (const as = [new A()] → as:"A").
// entryLocals maps Object.entries pair locals → value type leaf
// (const e = Object.entries({k: new A()})[0] → e:"A",
// for (const e of Object.entries({k: new A()})) → e:"A") so e[1].run() peels.
func jsShouldRenameMember(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers map[string]bool, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals map[string]string) bool {
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
	// super.x: parent impl. Rewrite when renaming Base.m in Child; leave alone when renaming Child.m.
	if obj.Type() == "super" {
		return enclosingClass == "" || !ourReceivers[enclosingClass]
	}
	if obj.Type() == "this" {
		return jsRenameByTypeMaps(enclosingClass, ourReceivers, foreignReceivers, nil)
	}
	if obj.Type() == "new_expression" {
		return jsRenameByTypeMaps(jsNewExpressionType(obj, content), ourReceivers, foreignReceivers, nil)
	}
	if obj.Type() == "identifier" {
		return jsRenameByTypeMaps(ingest.NodeText(obj, content), ourReceivers, foreignReceivers, typedLocals)
	}
	// await Promise.resolve(new A() / a / makeA()) / Promise.resolve(...) — identity peels
	// under foreign same-leaf methods (same leaf as const a = await …; a.run()).
	// Also Promise.race/any([new A()]) value peels (uniform array element type).
	// structuredClone(new A()) / Object.assign(new A()) — identity first-arg peels.
	if obj.Type() == "await_expression" || obj.Type() == "call_expression" {
		if t := jsPromiseResolveArgType(obj, content, typedLocals, factories); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsPromiseRaceValueType(obj, content, typedLocals, factories); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsIdentityCloneType(obj, content, typedLocals, factories); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// (await Promise.all([new A()]))[0].run() / [new A()][0].run() /
	// Array.from([new A()])[0].run() / Array.of(new A())[0].run() /
	// Object.values({k: new A()})[0].run() /
	// Object.entries({k: new A()})[0][1].run() /
	// e[1].run() after const e = Object.entries(...)[i] /
	// for (const e of Object.entries(...)) e[1].run() —
	// element / entries-value peel under foreign same-leaf.
	if obj.Type() == "subscript_expression" {
		if t := jsPromiseAllSubscriptType(obj, content, typedLocals, factories); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsObjectEntriesPairValueType(obj, content, typedLocals, factories, entryLocals); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsArrayElemSubscriptType(obj, content, arrayLocals, typedLocals, factories); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// r.value.run() / (await Promise.allSettled([new A()]))[0].value.run() /
	// genA().next().value.run() / (await agenA().next()).value.run() /
	// [new A()].values().next().value.run() /
	// [new A()][Symbol.iterator]().next().value.run() —
	// value peel under foreign same-leaf methods.
	if obj.Type() == "member_expression" || obj.Type() == "member_expression_optional" || obj.Type() == "optional_chain" {
		if t := jsPromiseAllSettledValueType(obj, content, typedLocals, settledOf, factories); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsGeneratorNextValueType(obj, content, generators, genLocals, arrayLocals, typedLocals, factories); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// Complex receivers: only when the method leaf is unique project-wide.
	switch obj.Type() {
	case "call_expression", "await_expression", "ternary_expression",
		"binary_expression", "member_expression",
		"member_expression_optional", "optional_chain",
		"subscript_expression", "assignment_expression":
		return len(foreignReceivers) == 0
	}
	return false
}

// jsTypedLocals maps local names → concrete type leaf for ourReceivers
// (const b = new Box() → "Box"). Also covers simple TS-style typed parameters
// and Promise.resolve then callback params.
// settledOf maps Promise.allSettled result locals / generator .next() /
// array-iterator .next() result locals → value type leaf
// (const [r] = await Promise.allSettled([new A()]) → r:"A",
// const ra = genA().next() → ra:"A",
// const ra = [new A()].values().next() → ra:"A") so r.value peels.
// factories maps same-file factory names → class leaf for Promise.resolve(makeA()).
// generators maps same-file generator names → yield type leaf (genA → A).
// genLocals maps generator/array-iterator instance locals → yield/elem type
// (const g = genA() → g:"A", const ia = [new A()].values() → ia:"A").
// arrayLocals maps array locals → uniform element type (const as = [new A()] → as:"A").
// entryLocals maps Object.entries pair locals → value type leaf
// (const e = Object.entries({k: new A()})[0] → e:"A") so e[1] peels.
func jsTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool, factories, generators map[string]string) (map[string]string, map[string]string, map[string]string, map[string]string, map[string]string) {
	out := map[string]string{}
	settledOf := map[string]string{}
	genLocals := map[string]string{}
	arrayLocals := map[string]string{}
	entryLocals := map[string]string{}
	if root == nil || len(ourReceivers) == 0 {
		return out, settledOf, genLocals, arrayLocals, entryLocals
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "lexical_declaration", "variable_declaration":
			// const box = new Box(); var box = new Box()
			// const a = await Promise.resolve(new A())
			// const a = await Promise.resolve(x) after const x = new A()
			// const a = await Promise.resolve(makeA()) after function makeA(){return new A()}
			// const [a] = await Promise.all([new A()])
			// const [r] = await Promise.allSettled([new A()]) — r is settled; r.value is A
			// const [{value: a}] = await Promise.allSettled([new A()]) — a is A
			// const r = (await Promise.allSettled([new A()]))[0] — r settled
			// const ga = genA() — generator local of yield type A
			// const ra = genA().next() / await agenA().next() — next result; ra.value is A
			// const a = genA().next().value — a is A
			// const as = [new A()] — array local of element type A
			// const ia = [new A()].values() / [new A()][Symbol.iterator]() — iterator of A
			// const ra = [new A()].values().next() — next result; ra.value is A
			// const a = [new A()].values().next().value — a is A
			for i := uint32(0); i < n.ChildCount(); i++ {
				child := n.Child(i)
				if child.Type() != "variable_declarator" {
					continue
				}
				nameN := ingest.ChildByField(child, "name")
				valN := ingest.ChildByField(child, "value")
				if nameN == nil || valN == nil {
					continue
				}
				if nameN.Type() == "identifier" {
					if ctor := jsNewExpressionType(valN, content); ourReceivers[ctor] {
						out[ingest.NodeText(nameN, content)] = ctor
					} else if t := jsPromiseResolveArgType(valN, content, out, factories); ourReceivers[t] {
						// await Promise.resolve(new A() / a / makeA()) — resolved value is A.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseRaceValueType(valN, content, out, factories); ourReceivers[t] {
						// await Promise.race/any([new A()]) — value is A when elems agree.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseAllSettledSubscriptType(valN, content, out, factories); ourReceivers[t] {
						// const r = (await Promise.allSettled([new A()]))[0]
						settledOf[ingest.NodeText(nameN, content)] = t
					} else if t := jsIdentityCloneType(valN, content, out, factories); ourReceivers[t] {
						// const a = structuredClone(new A()) / Object.assign(new A()[, …])
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsArraySourceElemType(valN, content, arrayLocals, out, factories); ourReceivers[t] {
						// const as = [new A()] / Array.from([new A()]) /
						// Array.of(new A()) / Object.values({k: new A()}) —
						// array local of element type A.
						arrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectEntriesPairValueType(valN, content, out, factories, entryLocals); ourReceivers[t] {
						// const a = Object.entries({k: new A()})[0][1] /
						// const a = e[1] after entry-local e
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectEntriesPairSubscriptType(valN, content, out, factories); t != "" {
						// const e = Object.entries({k: new A()})[0] — pair local;
						// e[1].run() peels via entryLocals. Bind foreign value types too
						// so dual-class B rebinds overwrite a prior A under the same name
						// (rename path fails closed via foreignReceivers).
						entryLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsArrayElemSubscriptType(valN, content, arrayLocals, out, factories); ourReceivers[t] {
						// const a = [new A()][0] / Array.from([new A()])[0] /
						// Array.of(new A())[0] / Object.values({k: new A()})[0]
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsIteratorSourceYieldType(valN, content, generators, genLocals, arrayLocals, out, factories); ourReceivers[t] {
						// const ga = genA() / const ia = [new A()].values() /
						// const ia = [new A()][Symbol.iterator]() — iterator of A.
						genLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsGeneratorNextResultType(valN, content, generators, genLocals, arrayLocals, out, factories); ourReceivers[t] {
						// const ra = genA().next() / [new A()].values().next() /
						// ia.next() — IteratorResult; ra.value peels via settledOf.
						settledOf[ingest.NodeText(nameN, content)] = t
					} else if t := jsGeneratorNextValueType(valN, content, generators, genLocals, arrayLocals, out, factories); ourReceivers[t] {
						// const a = genA().next().value / [new A()].values().next().value
						out[ingest.NodeText(nameN, content)] = t
					}
				} else if nameN.Type() == "array_pattern" {
					// const [a] = await Promise.all([new A()]) — bind pattern names to elem type.
					if t := jsPromiseAllElemType(valN, content, out, factories); ourReceivers[t] {
						jsBindArrayPatternNames(nameN, content, t, out)
					} else if t := jsPromiseAllSettledElemType(valN, content, out, factories); ourReceivers[t] {
						// const [r] = await Promise.allSettled([new A()])
						// const [{value: a}] / [{value}] = await Promise.allSettled([new A()])
						jsBindAllSettledArrayPattern(nameN, content, t, out, settledOf)
					} else if t := jsObjectEntriesPairSubscriptType(valN, content, out, factories); ourReceivers[t] {
						// const [, a] = Object.entries({k: new A()})[0] — value slot is A.
						jsBindEntriesArrayPattern(nameN, content, t, out)
					} else if valN.Type() == "identifier" && entryLocals != nil {
						// const [, a] = e after const e = Object.entries({k: new A()})[0]
						if t := entryLocals[ingest.NodeText(valN, content)]; ourReceivers[t] {
							jsBindEntriesArrayPattern(nameN, content, t, out)
						}
					}
				}
			}
		case "for_in_statement":
			// for (const a of genA()) / for await (const a of agenA()) /
			// for (const a of [new A()]) / for (const a of as) /
			// for (const a of as.values()) / for (const a of Array.of(new A())) /
			// for (const [, a] of Object.entries({k: new A()})) /
			// for (const e of Object.entries({k: new A()})) — pair local; e[1] peels —
			// bind left when right peels to a concrete element/yield type.
			// Only `of` (not `in`); left is bare identifier or entries value pattern.
			if op := ingest.ChildByField(n, "operator"); op == nil || ingest.NodeText(op, content) != "of" {
				break
			}
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left == nil || right == nil {
				break
			}
			if left.Type() == "identifier" {
				if t := jsForOfElemType(right, content, generators, genLocals, arrayLocals, out, factories); ourReceivers[t] {
					out[ingest.NodeText(left, content)] = t
				} else if t := jsObjectEntriesValueType(right, content, out, factories); t != "" {
					// for (const e of Object.entries({k: new A()})) — pair of value T;
					// e itself is not T (e[1] peels via entryLocals). Bind foreign too
					// so dual-class B rebinds fail closed on rename.
					entryLocals[ingest.NodeText(left, content)] = t
				}
			} else if left.Type() == "array_pattern" {
				// for (const [, a] of Object.entries({k: new A()})) — value slot only.
				if t := jsObjectEntriesValueType(right, content, out, factories); ourReceivers[t] {
					jsBindEntriesArrayPattern(left, content, t, out)
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
					out[ingest.NodeText(nameN, content)] = tn
				}
			}
		case "formal_parameters":
			// plain JS has no types; walk children for TS parameters
		case "call_expression":
			// Promise.resolve(new A() / a / makeA()).then(a => a.run()) — bind then callback param.
			// Promise.allSettled([new A()]).then(([r]) => r.value.run()) — settled params.
			jsBindPromiseThenParams(n, content, ourReceivers, out, settledOf, factories)
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out, settledOf, genLocals, arrayLocals, entryLocals
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

// jsSameFileFactoryReturns maps same-file factory names → class leaf recovered
// from body-only `return new T()` / expression-body arrow `() => new T()`.
// function makeA(){ return new A() } / const makeA = () => new A() /
// const makeA = function(){ return new A() }. Mixed/non-new returns fail closed.
// Same-file name → last wins (nested decls included when walked).
func jsSameFileFactoryReturns(root *grammar.Node, content []byte) map[string]string {
	out := map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "function_declaration", "generator_function_declaration":
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.Type() == "identifier" {
				name := ingest.NodeText(nameN, content)
				if name != "" {
					if t := jsFuncBodyNewReturn(n, content); t != "" {
						out[name] = t
					}
				}
			}
		case "lexical_declaration", "variable_declaration":
			for i := uint32(0); i < n.ChildCount(); i++ {
				child := n.Child(i)
				if child == nil || child.Type() != "variable_declarator" {
					continue
				}
				nameN := ingest.ChildByField(child, "name")
				valN := ingest.ChildByField(child, "value")
				if nameN == nil || nameN.Type() != "identifier" || valN == nil {
					continue
				}
				switch valN.Type() {
				case "arrow_function", "function_expression", "generator_function":
					if t := jsFuncBodyNewReturn(valN, content); t != "" {
						out[ingest.NodeText(nameN, content)] = t
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

// jsFuncBodyNewReturn recovers T when every return in fn is `return new T()`
// (or arrow expression body is `new T()`). Nested function bodies are skipped.
// Zero, mixed, or non-new returns fail closed ("").
func jsFuncBodyNewReturn(fn *grammar.Node, content []byte) string {
	if fn == nil {
		return ""
	}
	// Arrow expression body: () => new A()
	if fn.Type() == "arrow_function" {
		if body := ingest.ChildByField(fn, "body"); body != nil && body.Type() != "statement_block" {
			return jsNewExpressionType(body, content)
		}
	}
	body := ingest.ChildByField(fn, "body")
	if body == nil || body.Type() != "statement_block" {
		return ""
	}
	const fail = "-"
	found := ""
	saw := false
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() || found == fail {
			return
		}
		switch n.Type() {
		case "function_declaration", "generator_function_declaration",
			"function_expression", "generator_function", "arrow_function",
			"method_definition", "class_declaration", "class":
			// Nested scopes: do not harvest their returns for the outer factory.
			return
		case "return_statement":
			var expr *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == nil || ch.Type() == "return" || ch.Type() == ";" {
					continue
				}
				expr = ch
				break
			}
			t := jsNewExpressionType(expr, content)
			if t == "" {
				found = fail
				return
			}
			if !saw {
				found = t
				saw = true
			} else if found != t {
				found = fail
			}
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(body)
	if !saw || found == fail {
		return ""
	}
	return found
}

// jsFactoryCallReturnType recovers T from makeA() / makeA(...) when makeA is
// listed in factories (same-file body return new T()). Callee must be a bare
// identifier; member callees / unknown names fail closed.
func jsFactoryCallReturnType(call *grammar.Node, content []byte, factories map[string]string) string {
	if call == nil || call.Type() != "call_expression" || factories == nil {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" {
		return ""
	}
	return factories[ingest.NodeText(fn, content)]
}

// jsPromiseResolveArgType recovers T from Promise.resolve(new T()) /
// Promise.resolve(a) when a is a typed local / Promise.resolve(makeA()) when
// makeA is a same-file factory returning new T() / await Promise.resolve(...).
// Enables (await Promise.resolve(new A())).run(),
// const a = new A(); Promise.resolve(a).then(x => x.run()),
// Promise.resolve(makeA()).then(x => x.run()), and
// const a = await Promise.resolve(new A()); a.run() under foreign same-leaf.
// Non-Promise receivers / multi-arg / unknown args fail closed.
// typedLocals maps local → type leaf (may be nil).
// factories maps factory name → class leaf (may be nil).
func jsPromiseResolveArgType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	// Peel await.
	if n.Type() == "await_expression" {
		var arg *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch == nil || ch.Type() == "await" {
				continue
			}
			arg = ch
			break
		}
		return jsPromiseResolveArgType(arg, content, typedLocals, factories)
	}
	if n.Type() == "parenthesized_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return jsPromiseResolveArgType(ch, content, typedLocals, factories)
		}
		return ""
	}
	if n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "member_expression" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil {
		return ""
	}
	if obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Promise" {
		return ""
	}
	if prop.Type() != "property_identifier" || ingest.NodeText(prop, content) != "resolve" {
		return ""
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return ""
	}
	var first *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		c := args.Child(i)
		if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
			continue
		}
		count++
		if first == nil {
			first = c
		}
	}
	if count != 1 || first == nil {
		return ""
	}
	if t := jsNewExpressionType(first, content); t != "" {
		return t
	}
	// Promise.resolve(a) after const a = new A() — peel typed local.
	if first.Type() == "identifier" && typedLocals != nil {
		if t := typedLocals[ingest.NodeText(first, content)]; t != "" {
			return t
		}
	}
	// Promise.resolve(makeA()) after function makeA(){ return new A() }.
	if first.Type() == "call_expression" && factories != nil {
		if t := jsFactoryCallReturnType(first, content, factories); t != "" {
			return t
		}
	}
	return ""
}

// jsBindPromiseThenParams types the first parameter of
// Promise.resolve(new A() / a / makeA()).then(x => …) / .then(function(x) { … })
// and Promise.all([new A()]).then(([a]) => …) array-destructure params when the
// call receiver peels to a concrete our-receiver type. Also
// Promise.allSettled([new A()]).then(([r]) => r.value.run()) /
// .then(([{value}]) => value.run()) settled peels. Under foreign same-leaf
// methods, only our-receiver resolved values bind so B is preserved.
// out maps param name → type leaf; settledOf maps settled-result locals → value type.
func jsBindPromiseThenParams(call *grammar.Node, content []byte, ourReceivers map[string]bool, out, settledOf, factories map[string]string) {
	if call == nil || call.Type() != "call_expression" || out == nil {
		return
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "member_expression" {
		return
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || prop.Type() != "property_identifier" || ingest.NodeText(prop, content) != "then" {
		return
	}
	obj := ingest.ChildByField(fn, "object")
	// Promise.resolve(new T() / typed local / factory) → scalar param type.
	resolveT := jsPromiseResolveArgType(obj, content, out, factories)
	// Promise.race/any([new T(), …]) → scalar value type when elems agree.
	raceT := jsPromiseRaceValueType(obj, content, out, factories)
	// Promise.all([new T(), …]) → array element type for [a] destructure.
	allT := jsPromiseAllElemType(obj, content, out, factories)
	// Promise.allSettled([new T(), …]) → fulfilled value type for [r] / [{value}].
	settledT := jsPromiseAllSettledElemType(obj, content, out, factories)
	if !ourReceivers[resolveT] && !ourReceivers[raceT] && !ourReceivers[allT] && !ourReceivers[settledT] {
		return
	}
	args := ingest.ChildByField(call, "arguments")
	if args == nil {
		return
	}
	var cb *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		c := args.Child(i)
		if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
			continue
		}
		cb = c
		break
	}
	if cb == nil {
		return
	}
	// Collect first formal parameter node (identifier or array_pattern).
	var param *grammar.Node
	switch cb.Type() {
	case "arrow_function":
		if p := ingest.ChildByField(cb, "parameter"); p != nil {
			param = p
		} else if p := ingest.ChildByField(cb, "parameters"); p != nil {
			param = jsFirstFormalParamNode(p)
		} else {
			for i := uint32(0); i < cb.ChildCount(); i++ {
				ch := cb.Child(i)
				if ch.Type() == "identifier" || ch.Type() == "array_pattern" {
					param = ch
					break
				}
				if ch.Type() == "formal_parameters" {
					param = jsFirstFormalParamNode(ch)
					break
				}
			}
		}
	case "function_expression", "function_declaration":
		if p := ingest.ChildByField(cb, "parameters"); p != nil {
			param = jsFirstFormalParamNode(p)
		}
	}
	if param == nil {
		return
	}
	// Promise.resolve / Promise.race|any → bare identifier param.
	scalarT := ""
	if ourReceivers[resolveT] {
		scalarT = resolveT
	} else if ourReceivers[raceT] {
		scalarT = raceT
	}
	if scalarT != "" && param.Type() == "identifier" {
		name := ingest.NodeText(param, content)
		if name != "" && name != "_" {
			out[name] = scalarT
		}
		return
	}
	// Promise.all → array_pattern param ([a] / [a, b] when elems agree).
	if ourReceivers[allT] && param.Type() == "array_pattern" {
		jsBindArrayPatternNames(param, content, allT, out)
		return
	}
	// Promise.allSettled → array_pattern ([r] settled / [{value}] typed).
	if ourReceivers[settledT] && param.Type() == "array_pattern" {
		jsBindAllSettledArrayPattern(param, content, settledT, out, settledOf)
	}
}

// jsFirstFormalParamNode returns the first formal parameter node (identifier,
// array_pattern, or TS parameter wrapper's pattern/name).
func jsFirstFormalParamNode(params *grammar.Node) *grammar.Node {
	if params == nil {
		return nil
	}
	for i := uint32(0); i < params.ChildCount(); i++ {
		ch := params.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		switch ch.Type() {
		case "identifier", "array_pattern":
			return ch
		}
		// TS required_parameter / optional_parameter
		if nameN := ingest.ChildByField(ch, "pattern"); nameN != nil {
			return nameN
		}
		if nameN := ingest.ChildByField(ch, "name"); nameN != nil {
			return nameN
		}
		if nameN := ingest.ChildByType(ch, "identifier"); nameN != nil {
			return nameN
		}
		if nameN := ingest.ChildByType(ch, "array_pattern"); nameN != nil {
			return nameN
		}
	}
	return nil
}

// jsBindArrayPatternNames binds each simple identifier slot in an array_pattern
// to typeLeaf (Promise.all element type when all elements agree). Nested patterns
// / rest / defaults fail closed for that slot only.
func jsBindArrayPatternNames(pattern *grammar.Node, content []byte, typeLeaf string, out map[string]string) {
	if pattern == nil || pattern.Type() != "array_pattern" || typeLeaf == "" || out == nil {
		return
	}
	for i := uint32(0); i < pattern.ChildCount(); i++ {
		ch := pattern.Child(i)
		if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
			continue
		}
		if ch.Type() == "identifier" {
			name := ingest.NodeText(ch, content)
			if name != "" && name != "_" {
				out[name] = typeLeaf
			}
		}
		// Skip rest_pattern / assignment_pattern / nested array_pattern (fail closed).
	}
}

// jsPromiseAllElemType recovers T from Promise.all([new T() / a / makeT(), …])
// when every array element peels to the same concrete type T. Enables
// Promise.all([new A()]).then(([a]) => a.run()), const [a] = await Promise.all([new A()]),
// and (await Promise.all([new A()]))[0].run() under foreign same-leaf methods.
// Non-Promise.all / non-array / mixed / empty args fail closed.
func jsPromiseAllElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseArrayElemType(n, content, typedLocals, factories, "all")
}

// jsPromiseAllSettledElemType recovers T from Promise.allSettled([new T(), …])
// when every array element peels to the same concrete type T. The result array
// holds settled objects {status, value}; T is the fulfilled value type (not the
// settled wrapper). Enables Promise.allSettled([new A()]).then(([r]) => r.value.run()),
// const [r] = await Promise.allSettled([new A()]); r.value.run(), and
// (await Promise.allSettled([new A()]))[0].value.run() under foreign same-leaf.
// Non-allSettled / non-array / mixed / empty args fail closed.
func jsPromiseAllSettledElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseArrayElemType(n, content, typedLocals, factories, "allSettled")
}

// jsPromiseRaceValueType recovers T from Promise.race([new T(), …]) /
// Promise.any([new T(), …]) when every array element peels to the same T.
// The settled value is T (scalar), not an array — unlike Promise.all.
// Enables Promise.race([new A()]).then(a => a.run()) and
// (await Promise.race([new A()])).run() under foreign same-leaf methods.
func jsPromiseRaceValueType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if t := jsPromiseArrayElemType(n, content, typedLocals, factories, "race"); t != "" {
		return t
	}
	return jsPromiseArrayElemType(n, content, typedLocals, factories, "any")
}

// jsPromiseArrayElemType recovers the uniform element type of
// Promise.<method>([…]) for method in {all, race, any}.
func jsPromiseArrayElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, method string) string {
	if n == nil || method == "" {
		return ""
	}
	if n.Type() == "await_expression" {
		var arg *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch == nil || ch.Type() == "await" {
				continue
			}
			arg = ch
			break
		}
		return jsPromiseArrayElemType(arg, content, typedLocals, factories, method)
	}
	if n.Type() == "parenthesized_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return jsPromiseArrayElemType(ch, content, typedLocals, factories, method)
		}
		return ""
	}
	if n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "member_expression" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil {
		return ""
	}
	if obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Promise" {
		return ""
	}
	if prop.Type() != "property_identifier" || ingest.NodeText(prop, content) != method {
		return ""
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return ""
	}
	var first *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		c := args.Child(i)
		if c == nil || c.Type() == "(" || c.Type() == ")" || c.Type() == "," {
			continue
		}
		count++
		if first == nil {
			first = c
		}
	}
	if count != 1 || first == nil || first.Type() != "array" {
		return ""
	}
	// All array elements must peel to the same concrete type.
	found := ""
	saw := false
	for i := uint32(0); i < first.ChildCount(); i++ {
		el := first.Child(i)
		if el == nil || el.Type() == "[" || el.Type() == "]" || el.Type() == "," {
			continue
		}
		t := jsExprConcreteType(el, content, typedLocals, factories)
		if t == "" {
			return ""
		}
		if !saw {
			found = t
			saw = true
		} else if found != t {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// jsPromiseAllSubscriptType recovers T from (await Promise.all([new T()]))[i]
// when the indexed object peels to a uniform Promise.all element type. Index
// must be a numeric literal (any index; elems already agree).
func jsPromiseAllSubscriptType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	return jsPromiseAllElemType(obj, content, typedLocals, factories)
}

// jsPromiseAllSettledSubscriptType recovers T from
// (await Promise.allSettled([new T()]))[i] when the indexed object peels to a
// uniform allSettled value type. Index must be a numeric literal.
func jsPromiseAllSettledSubscriptType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	return jsPromiseAllSettledElemType(obj, content, typedLocals, factories)
}

// jsPromiseAllSettledValueType recovers T from r.value /
// (await Promise.allSettled([new T()]))[i].value when the object peels to a
// settled result of fulfilled value type T. Property must be bare "value".
func jsPromiseAllSettledValueType(n *grammar.Node, content []byte, typedLocals, settledOf, factories map[string]string) string {
	if n == nil {
		return ""
	}
	if n.Type() != "member_expression" && n.Type() != "member_expression_optional" && n.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(n, "property")
	if prop == nil || (prop.Type() != "property_identifier" && prop.Type() != "identifier") {
		return ""
	}
	if ingest.NodeText(prop, content) != "value" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
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
		return ""
	}
	// r.value after const [r] = await Promise.allSettled([new A()])
	if obj.Type() == "identifier" && settledOf != nil {
		if t := settledOf[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	// (await Promise.allSettled([new A()]))[0].value
	if t := jsPromiseAllSettledSubscriptType(obj, content, typedLocals, factories); t != "" {
		return t
	}
	return ""
}

// jsBindAllSettledArrayPattern binds slots of an array_pattern from
// Promise.allSettled results:
//
//	[r]           → settledOf[r] = valueType (r.value peels)
//	[{value: a}]  → out[a] = valueType
//	[{value}]     → out[value] = valueType (shorthand)
//
// Nested / rest / other object keys fail closed for that slot only.
func jsBindAllSettledArrayPattern(pattern *grammar.Node, content []byte, valueType string, out, settledOf map[string]string) {
	if pattern == nil || pattern.Type() != "array_pattern" || valueType == "" {
		return
	}
	for i := uint32(0); i < pattern.ChildCount(); i++ {
		ch := pattern.Child(i)
		if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
			continue
		}
		switch ch.Type() {
		case "identifier":
			// [r] — settled result local; .value peels via settledOf.
			name := ingest.NodeText(ch, content)
			if name != "" && name != "_" && settledOf != nil {
				settledOf[name] = valueType
			}
		case "object_pattern":
			// [{value: a}] / [{value}] — bind the value property to T in out.
			jsBindAllSettledValueObjectPattern(ch, content, valueType, out)
		}
		// Skip rest_pattern / nested array_pattern / assignment_pattern (fail closed).
	}
}

// jsBindAllSettledValueObjectPattern binds value / value: name from a settled
// result object pattern into out as the fulfilled value type.
func jsBindAllSettledValueObjectPattern(pattern *grammar.Node, content []byte, valueType string, out map[string]string) {
	if pattern == nil || pattern.Type() != "object_pattern" || valueType == "" || out == nil {
		return
	}
	for i := uint32(0); i < pattern.ChildCount(); i++ {
		ch := pattern.Child(i)
		if ch == nil || ch.Type() == "{" || ch.Type() == "}" || ch.Type() == "," {
			continue
		}
		switch ch.Type() {
		case "shorthand_property_identifier_pattern":
			// {value} — local name is "value".
			if ingest.NodeText(ch, content) == "value" {
				out["value"] = valueType
			}
		case "pair_pattern":
			// {value: a}
			key := ingest.ChildByField(ch, "key")
			val := ingest.ChildByField(ch, "value")
			if key == nil || val == nil {
				continue
			}
			if ingest.NodeText(key, content) != "value" {
				continue
			}
			if val.Type() == "identifier" {
				name := ingest.NodeText(val, content)
				if name != "" && name != "_" {
					out[name] = valueType
				}
			}
		}
	}
}

// jsExprConcreteType peels new T() / typed local / factory call to a class leaf.
func jsExprConcreteType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return jsExprConcreteType(ch, content, typedLocals, factories)
		}
		return ""
	}
	if t := jsNewExpressionType(n, content); t != "" {
		return t
	}
	if n.Type() == "identifier" && typedLocals != nil {
		if t := typedLocals[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	if n.Type() == "call_expression" {
		return jsFactoryCallReturnType(n, content, factories)
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

// jsSameFileGeneratorYields maps same-file generator names → yield type leaf
// recovered from body-only `yield new T()` / `yield x` after `x = new T()`.
// function* genA(){ yield new A() } / async function* agenA(){ yield new A() } /
// const genA = function*(){ yield new A() }. Mixed/non-new yields and yield*
// fail closed. Same-file name → last wins.
func jsSameFileGeneratorYields(root *grammar.Node, content []byte) map[string]string {
	out := map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "generator_function_declaration":
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.Type() == "identifier" {
				name := ingest.NodeText(nameN, content)
				if name != "" {
					if t := jsFuncBodyYieldNew(n, content); t != "" {
						out[name] = t
					}
				}
			}
		case "lexical_declaration", "variable_declaration":
			for i := uint32(0); i < n.ChildCount(); i++ {
				child := n.Child(i)
				if child == nil || child.Type() != "variable_declarator" {
					continue
				}
				nameN := ingest.ChildByField(child, "name")
				valN := ingest.ChildByField(child, "value")
				if nameN == nil || nameN.Type() != "identifier" || valN == nil {
					continue
				}
				if valN.Type() == "generator_function" {
					if t := jsFuncBodyYieldNew(valN, content); t != "" {
						out[ingest.NodeText(nameN, content)] = t
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

// jsFuncBodyYieldNew recovers T when every yield in fn is `yield new T()`
// or `yield x` after a local `x = new T()` assignment. Nested function/class
// bodies are skipped. yield* / zero / mixed / non-new yields fail closed.
func jsFuncBodyYieldNew(fn *grammar.Node, content []byte) string {
	if fn == nil {
		return ""
	}
	body := ingest.ChildByField(fn, "body")
	if body == nil || body.Type() != "statement_block" {
		return ""
	}
	localCtor := map[string]string{}
	const fail = "-"
	found := ""
	saw := false
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() || found == fail {
			return
		}
		switch n.Type() {
		case "function_declaration", "generator_function_declaration",
			"function_expression", "generator_function", "arrow_function",
			"method_definition", "class_declaration", "class":
			// Nested scopes: do not harvest their yields for the outer generator.
			return
		case "variable_declarator":
			nameN := ingest.ChildByField(n, "name")
			valN := ingest.ChildByField(n, "value")
			if nameN != nil && nameN.Type() == "identifier" && valN != nil {
				if t := jsNewExpressionType(valN, content); t != "" {
					localCtor[ingest.NodeText(nameN, content)] = t
				}
			}
		case "yield_expression":
			// yield new T() / yield x — reject yield*.
			var expr *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == nil {
					continue
				}
				switch ch.Type() {
				case "yield":
					continue
				case "*":
					found = fail
					return
				default:
					if expr == nil {
						expr = ch
					}
				}
			}
			t := ""
			if expr != nil {
				if ctor := jsNewExpressionType(expr, content); ctor != "" {
					t = ctor
				} else if expr.Type() == "identifier" {
					t = localCtor[ingest.NodeText(expr, content)]
				}
			}
			if t == "" {
				found = fail
				return
			}
			if !saw {
				found = t
				saw = true
			} else if found != t {
				found = fail
			}
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(body)
	if !saw || found == fail {
		return ""
	}
	return found
}

// jsGeneratorCallYieldType recovers T from genA() / agenA() when the callee is
// a same-file generator yielding T, or from a typed generator local (ga after
// const ga = genA()). Non-generator calls fail closed.
func jsGeneratorCallYieldType(n *grammar.Node, content []byte, generators, genLocals map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		n = inner
	}
	if n == nil {
		return ""
	}
	if n.Type() == "identifier" && genLocals != nil {
		return genLocals[ingest.NodeText(n, content)]
	}
	if n.Type() != "call_expression" || generators == nil {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "identifier" {
		return ""
	}
	return generators[ingest.NodeText(fn, content)]
}

// jsUniformArrayElemType recovers T from [new T() / a / makeT(), …] when every
// element peels to the same concrete type T. Empty / mixed arrays fail closed.
func jsUniformArrayElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		n = inner
	}
	if n == nil || n.Type() != "array" {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		el := n.Child(i)
		if el == nil || el.Type() == "[" || el.Type() == "]" || el.Type() == "," {
			continue
		}
		t := jsExprConcreteType(el, content, typedLocals, factories)
		if t == "" {
			return ""
		}
		if !saw {
			found = t
			saw = true
		} else if found != t {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// jsArraySourceElemType recovers T from an array-like expression whose elements
// uniformly peel to T: array literal, array local, Array.from([…]) (no mapfn),
// Array.of(…), or Object.values({…}) when all property values agree.
// Object.entries is not an element source of T (yields [key, value] pairs).
func jsArraySourceElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		n = inner
	}
	if n == nil {
		return ""
	}
	if t := jsUniformArrayElemType(n, content, typedLocals, factories); t != "" {
		return t
	}
	if n.Type() == "identifier" && arrayLocals != nil {
		if t := arrayLocals[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	if t := jsArrayFromElemType(n, content, arrayLocals, typedLocals, factories); t != "" {
		return t
	}
	if t := jsArrayOfElemType(n, content, typedLocals, factories); t != "" {
		return t
	}
	if t := jsObjectValuesElemType(n, content, typedLocals, factories); t != "" {
		return t
	}
	return ""
}

// jsArrayFromElemType recovers T from Array.from(arrLike) when the first arg
// peels to uniform element type T and no mapfn is present (single-arg only).
func jsArrayFromElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string) string {
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil || args == nil {
		return ""
	}
	// Array.from
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil ||
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Array" ||
		ingest.NodeText(prop, content) != "from" {
		return ""
	}
	// Single positional arg only (no mapfn).
	var first *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if first == nil {
			first = ch
		}
	}
	if count != 1 || first == nil {
		return ""
	}
	return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories)
}

// jsArrayOfElemType recovers T from Array.of(x, …) when every positional arg
// peels to the same concrete type T. Empty / mixed args fail closed.
// Enables Array.of(new A())[0].run() / for (const a of Array.of(new A())) under
// foreign same-leaf methods.
func jsArrayOfElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil || args == nil {
		return ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil ||
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Array" ||
		ingest.NodeText(prop, content) != "of" {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		t := jsExprConcreteType(ch, content, typedLocals, factories)
		if t == "" {
			return ""
		}
		if !saw {
			found = t
			saw = true
		} else if found != t {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// jsObjectValuesElemType recovers T from Object.values({…}) when every property
// value peels to the same concrete type T. Non-object-literal / mixed / empty
// / method / spread entries fail closed.
func jsObjectValuesElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsObjectStaticValuesCallType(n, content, typedLocals, factories, "values")
}

// jsObjectEntriesValueType recovers T from Object.entries({…}) when every
// property value peels to the same concrete type T. The call yields [key, value]
// pairs — T is the value type (not the pair). Same object-literal rules as values.
func jsObjectEntriesValueType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsObjectStaticValuesCallType(n, content, typedLocals, factories, "entries")
}

// jsObjectStaticValuesCallType recovers uniform property-value type T from
// Object.values/Object.entries({…}) (single object-literal arg).
func jsObjectStaticValuesCallType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, method string) string {
	if n == nil || n.Type() != "call_expression" || method == "" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil || args == nil {
		return ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil ||
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != method {
		return ""
	}
	var first *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if first == nil {
			first = ch
		}
	}
	if count != 1 || first == nil {
		return ""
	}
	// Peel parens around object literal.
	for first != nil && first.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < first.ChildCount(); i++ {
			ch := first.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		first = inner
	}
	if first == nil || first.Type() != "object" {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < first.ChildCount(); i++ {
		ch := first.Child(i)
		if ch == nil || ch.Type() == "{" || ch.Type() == "}" || ch.Type() == "," {
			continue
		}
		var val *grammar.Node
		switch ch.Type() {
		case "pair":
			val = ingest.ChildByField(ch, "value")
		case "shorthand_property_identifier":
			// {a} after const a = new A() — value is the identifier itself.
			val = ch
		default:
			// method_definition / spread_element / … fail closed.
			return ""
		}
		if val == nil {
			return ""
		}
		// shorthand is property_identifier-like; peel as identifier via text.
		t := ""
		if val.Type() == "shorthand_property_identifier" {
			if typedLocals != nil {
				t = typedLocals[ingest.NodeText(val, content)]
			}
		} else {
			t = jsExprConcreteType(val, content, typedLocals, factories)
		}
		if t == "" {
			return ""
		}
		if !saw {
			found = t
			saw = true
		} else if found != t {
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// jsObjectEntriesPairSubscriptType recovers T from Object.entries({…})[i] when
// the pair value type is T. Index must be a numeric literal. The pair itself is
// not T — use for destructure binding (const [, a] = …[i]) only.
func jsObjectEntriesPairSubscriptType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	return jsObjectEntriesValueType(ingest.ChildByField(n, "object"), content, typedLocals, factories)
}

// jsObjectEntriesPairValueType recovers T from Object.entries({…})[i][1] when
// the pair value type is T, or from e[1] when e is an entry-local pair of value T
// (const e = Object.entries({…})[i] / for (const e of Object.entries({…}))).
// Outer index must be the number 1 (value slot); inner index any numeric literal.
// Enables Object.entries({k: new A()})[0][1].run() / e[1].run() under foreign
// same-leaf methods. Key slot e[0] fails closed.
func jsObjectEntriesPairValueType(n *grammar.Node, content []byte, typedLocals, factories, entryLocals map[string]string) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" || ingest.NodeText(idx, content) != "1" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	// e[1] after const e = Object.entries(...)[i] / for (const e of Object.entries(...)).
	if obj != nil && obj.Type() == "identifier" && entryLocals != nil {
		if t := entryLocals[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	return jsObjectEntriesPairSubscriptType(obj, content, typedLocals, factories)
}

// jsBindEntriesArrayPattern binds the value slot (index 1) of an array_pattern
// from Object.entries pairs: [, a] / [k, a] → a:T. Key slot (index 0) is left
// unbound (string key). Nested / rest / defaults fail closed for that slot.
func jsBindEntriesArrayPattern(pattern *grammar.Node, content []byte, valueType string, out map[string]string) {
	if pattern == nil || pattern.Type() != "array_pattern" || valueType == "" || out == nil {
		return
	}
	slot := 0
	expectElem := true
	for i := uint32(0); i < pattern.ChildCount(); i++ {
		ch := pattern.Child(i)
		if ch == nil || ch.Type() == "[" || ch.Type() == "]" {
			continue
		}
		// Commas separate slots; a comma while still expecting an element is a hole.
		if ch.Type() == "," {
			if expectElem {
				slot++
			}
			expectElem = true
			continue
		}
		if slot == 1 && ch.Type() == "identifier" {
			name := ingest.NodeText(ch, content)
			if name != "" && name != "_" {
				out[name] = valueType
			}
		}
		// Only value slot binds; other slots (key / rest / nested) ignored.
		slot++
		expectElem = false
	}
}

// jsArrayElemSubscriptType recovers T from arr[i] / Array.from([new T()])[i] /
// Array.of(new T())[i] / Object.values({k: new T()})[i] when the indexed object
// peels to a uniform array element type T. Index must be a numeric literal.
// Object.entries pairs are not element sources of T (use pair-value peels).
func jsArrayElemSubscriptType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	return jsArraySourceElemType(ingest.ChildByField(n, "object"), content, arrayLocals, typedLocals, factories)
}

// jsIsSymbolIteratorIndex reports whether n is Symbol.iterator (member form).
func jsIsSymbolIteratorIndex(n *grammar.Node, content []byte) bool {
	if n == nil || (n.Type() != "member_expression" && n.Type() != "member_expression_optional" && n.Type() != "optional_chain") {
		return false
	}
	obj := ingest.ChildByField(n, "object")
	prop := ingest.ChildByField(n, "property")
	if obj == nil || prop == nil {
		return false
	}
	return obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Symbol" &&
		ingest.NodeText(prop, content) == "iterator"
}

// jsCallIsZeroArg reports whether a call_expression has no positional args.
func jsCallIsZeroArg(n *grammar.Node) bool {
	if n == nil || n.Type() != "call_expression" {
		return false
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return true
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		return false
	}
	return true
}

// jsArrayIteratorYieldType recovers T from arr.values() / arr[Symbol.iterator]()
// when arr peels to a uniform array element type T. Zero-arg calls only.
func jsArrayIteratorYieldType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		n = inner
	}
	if n == nil || n.Type() != "call_expression" || !jsCallIsZeroArg(n) {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "member_expression", "member_expression_optional", "optional_chain":
		// arr.values()
		prop := ingest.ChildByField(fn, "property")
		if prop == nil || ingest.NodeText(prop, content) != "values" {
			return ""
		}
		return jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories)
	case "subscript_expression":
		// arr[Symbol.iterator]()
		if !jsIsSymbolIteratorIndex(ingest.ChildByField(fn, "index"), content) {
			return ""
		}
		return jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories)
	}
	return ""
}

// jsIteratorSourceYieldType recovers T from a generator call/local or an array
// iterator call (arr.values() / arr[Symbol.iterator]()) / array-iterator local.
func jsIteratorSourceYieldType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories map[string]string) string {
	if t := jsGeneratorCallYieldType(n, content, generators, genLocals); t != "" {
		return t
	}
	return jsArrayIteratorYieldType(n, content, arrayLocals, typedLocals, factories)
}

// jsForOfElemType recovers T from the right-hand side of for…of / for await…of:
// generator call/local, array literal, array local, or array iterator call.
func jsForOfElemType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories map[string]string) string {
	if t := jsArraySourceElemType(n, content, arrayLocals, typedLocals, factories); t != "" {
		return t
	}
	return jsIteratorSourceYieldType(n, content, generators, genLocals, arrayLocals, typedLocals, factories)
}

// jsGeneratorNextResultType recovers T from genA().next() / ga.next() /
// await agenA().next() / [new A()].values().next() /
// [new A()][Symbol.iterator]().next() / ia.next() when the receiver peels to an
// iterator yielding T. Only zero-arg .next() peels (fail closed on arguments).
func jsGeneratorNextResultType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	// Unwrap (await gen().next()) / await gen().next() / (gen().next()).
	for n != nil {
		switch n.Type() {
		case "parenthesized_expression":
			var inner *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
			n = inner
			continue
		case "await_expression":
			// await gen().next() — peel through await for async generators.
			var arg *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == nil || ch.Type() == "await" {
					continue
				}
				arg = ch
				break
			}
			n = arg
			continue
		}
		break
	}
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	// Zero-arg next() only.
	if !jsCallIsZeroArg(n) {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "next" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	return jsIteratorSourceYieldType(obj, content, generators, genLocals, arrayLocals, typedLocals, factories)
}

// jsGeneratorNextValueType recovers T from genA().next().value /
// ga.next().value / (await agenA().next()).value /
// [new A()].values().next().value / [new A()][Symbol.iterator]().next().value.
// ra.value is handled via settledOf in jsPromiseAllSettledValueType.
// Property must be bare "value".
func jsGeneratorNextValueType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	if n.Type() != "member_expression" && n.Type() != "member_expression_optional" && n.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(n, "property")
	if prop == nil || (prop.Type() != "property_identifier" && prop.Type() != "identifier") {
		return ""
	}
	if ingest.NodeText(prop, content) != "value" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	return jsGeneratorNextResultType(obj, content, generators, genLocals, arrayLocals, typedLocals, factories)
}

// jsIdentityCloneType recovers T from structuredClone(x) / Object.assign(x[, …])
// when the first positional arg peels to T (new T() / typed local / factory).
// structuredClone returns a structured copy of its argument; Object.assign
// returns its first argument (target). Extra assign sources ignored.
// Non-matching callees / missing first arg fail closed.
func jsIdentityCloneType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil {
		return ""
	}
	// await structuredClone(...) — peel through await.
	if n.Type() == "await_expression" {
		var arg *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch == nil || ch.Type() == "await" {
				continue
			}
			arg = ch
			break
		}
		return jsIdentityCloneType(arg, content, typedLocals, factories)
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		n = inner
	}
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil || args == nil {
		return ""
	}
	ok := false
	switch fn.Type() {
	case "identifier":
		// structuredClone(x)
		if ingest.NodeText(fn, content) == "structuredClone" {
			ok = true
		}
	case "member_expression", "member_expression_optional", "optional_chain":
		// Object.assign(x[, …])
		prop := ingest.ChildByField(fn, "property")
		obj := ingest.ChildByField(fn, "object")
		if prop != nil && obj != nil &&
			ingest.NodeText(prop, content) == "assign" &&
			obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
			ok = true
		}
	}
	if !ok {
		return ""
	}
	// First positional argument.
	var first *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		first = ch
		break
	}
	if first == nil {
		return ""
	}
	return jsExprConcreteType(first, content, typedLocals, factories)
}
