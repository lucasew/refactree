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
	classFields := jsClassFieldIndex(pf.Root, content)
	methodReturns := jsClassMethodReturns(pf.Root, content, classFields)
	// Second pass: field inits that are method-return (mrA = new BoxA().get())
	// need methodReturns, which depends on Class()-only field index above.
	jsEnhanceClassFieldsMethodReturn(pf.Root, content, classFields, methodReturns)
	// Generators after methodReturns so yield new BoxA().get() peels under foreign same-leaf.
	generators := jsSameFileGeneratorYields(pf.Root, content, classFields, methodReturns)
	typedLocals, settledOf, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals := jsTypedLocals(pf.Root, content, ourReceivers, factories, generators, classFields, methodReturns)
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
				if jsShouldRenameMember(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals, classFields, methodReturns) {
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
				if uniqueLeaf || jsShouldRenameMember(valN, content, classHere, ourReceivers, foreignReceivers, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals, classFields, methodReturns) {
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
// entryLocals maps Object.entries / Array.entries / Map.entries pair locals → value type leaf
// (const e = Object.entries({k: new A()})[0] → e:"A",
// for (const e of Object.entries({k: new A()})) → e:"A",
// const e = [new A()].entries().next().value → e:"A") so e[1].run() peels.
// entryArrayLocals maps pair-array / entries-iterator locals → value type leaf
// (const es = Object.entries({k: new A()}) → es:"A",
// const ia = [new A()].entries() → ia:"A") so es[i][1] / ia.next().value[1] peels.
// entryNextLocals maps entries-iterator .next() result locals → pair value type
// (const ra = [new A()].entries().next() → ra:"A") so ra.value[1].run() peels.
// mapLocals maps Map locals → uniform value type (const ma = new Map([[k, new A()]]) → ma:"A").
// objValueLocals maps plain-object locals → uniform property value type
// (const o = Object.fromEntries([[k, new A()]]) → o:"A") so o.k.run() peels.
// groupByLocals maps Object.groupBy result locals → element T
// (const ga = Object.groupBy([new A()], …) → ga:"A") so ga[k][0].run() peels.
// groupMapLocals maps Map.groupBy result locals → element T
// (const ma = Map.groupBy([new A()], …) → ma:"A") so ma.get(k)[0].run() peels.
// groupEntryLocals maps groupBy-entries pair locals → element T
// (for (const e of Object.entries(ga)) → e:"A") so e[1][0].run() peels
// (pair value is T[], not T — unlike scalar entryLocals).
// groupEntryArrayLocals maps groupBy-entries array/iterator locals → element T
// (const es = Object.entries(ga) → es:"A") so es[i][1][0].run() peels.
func jsShouldRenameMember(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers map[string]bool, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) bool {
	if obj == nil {
		return false
	}
	extra := jsExtraLocals{
		objValue: objValueLocals, groupBy: groupByLocals, groupMap: groupMapLocals,
		groupEntry: groupEntryLocals, groupEntryArray: groupEntryArrayLocals,
		mapLocals: mapLocals, setLocals: setLocals, entryArrayLocals: entryArrayLocals,
		classFields: classFields, methodReturns: methodReturns,
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
		// new Proxy(a, {}) / new Proxy(new A(), {}) / new Proxy(new BoxA().get(), {}) —
		// identity of target (method-return via Ex).
		if t := jsProxyTargetTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		return jsRenameByTypeMaps(jsNewExpressionType(obj, content), ourReceivers, foreignReceivers, nil)
	}
	if obj.Type() == "identifier" {
		return jsRenameByTypeMaps(ingest.NodeText(obj, content), ourReceivers, foreignReceivers, typedLocals)
	}
	// await Promise.resolve(new A() / a / makeA() / ba.get()) / Promise.resolve(...) —
	// identity peels under foreign same-leaf methods (same leaf as const a = await …; a.run()).
	// Also Promise.race/any([new A() / ba.get()]) value peels (uniform array element type).
	// structuredClone(new A()) / Object.assign(new A()) — identity first-arg peels.
	if obj.Type() == "await_expression" || obj.Type() == "call_expression" {
		if t := jsPromiseResolveArgTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsPromiseRaceValueTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsIdentityCloneTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsMapGetValueTypeEx(obj, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns); t != "" {
			// new Map([[k, new A()]]).get(k) / ma.get(k) / new Map(pa).get(k) /
			// new Map([[k, ba.get()]]).get(k) — uniform value type (method-return too).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsArrayAtElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
			// [new A()].at(0) / as.at(-1) / Array.from([new A()]).at(0) —
			// element peel under foreign same-leaf (same as arr[i]).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsArrayPopShiftElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
			// [new A()].pop() / as.shift() / Array.from([new A()]).pop() —
			// element peel under foreign same-leaf (same as arr[i] / arr.at(i)).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsArrayFindElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
			// [new A()].find(pred) / as.findLast(pred) — element peel under
			// foreign same-leaf (uniform array T; predicate ignored).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsArrayReduceElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
			// [new A()].reduce((a,b)=>a) / as.reduceRight((a,b)=>b, new A()) —
			// identity reduce/reduceRight peels to element T under foreign same-leaf.
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsIteratorFindElemType(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); t != "" {
			// Iterator.from([new A()]).find(pred) / Iterator.from([new BoxA().get()]).find —
			// yield peel under foreign same-leaf (method-return via methodReturns).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsIteratorReduceElemType(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); t != "" {
			// Iterator.from([new A()]).reduce((a,x) => x) / method-return yield peel.
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsPromiseThenIdentityType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			// Promise.resolve(new A() / new BoxA().get()).then(x => x) /
			// await …then(x => x) — identity then peels to resolved value T
			// under foreign same-leaf (method-return via methodReturns).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsPromiseFinallyType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			// Promise.resolve(new A() / new BoxA().get()).finally(() => {}) /
			// await …finally — finally is identity for the resolved value
			// under foreign same-leaf (method-return via methodReturns).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsPromiseCatchType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			// Promise.resolve(new A() / new BoxA().get()).catch(() => null) /
			// await …catch — catch is identity for the fulfilled value under
			// foreign same-leaf (rejection handler ignored; method-return via
			// methodReturns; same leaf as finally).
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsPromiseTryTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			// Promise.try(() => new A() / new BoxA().get()) / await Promise.try(...) — sole-return peel.
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsWeakRefDerefTypeEx(obj, content, typedLocals, factories, genLocals, classFields, methodReturns); t != "" {
			// new WeakRef(new A()).deref() / new WeakRef(new BoxA().get()).deref() /
			// wa.deref() after const wa = new WeakRef(...) — referent peel under foreign same-leaf.
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsReflectGetTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
			// Reflect.get({k: new A()}, "k") / Reflect.get({k: new BoxA().get()}, "k") /
			// Reflect.get(oa, "k") — property value peel under foreign same-leaf.
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsReflectConstructType(obj, content); t != "" {
			// Reflect.construct(A, []).run() — constructed instance peel.
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// (await Promise.all([new A()]))[0].run() / [new A()][0].run() /
	// Array.from([new A()])[0].run() / Array.of(new A())[0].run() /
	// Object.values({k: new A()})[0].run() /
	// Object.entries({k: new A()})[0][1].run() /
	// es[0][1].run() after const es = Object.entries(...) /
	// [...Object.entries({k: new A()})][0][1].run() /
	// e[1].run() after const e = Object.entries(...)[i] /
	// for (const e of Object.entries(...)) e[1].run() /
	// Object.fromEntries([[k, new A()]])["k"].run() / o["k"].run() —
	// element / entries-value / fromEntries-prop peel under foreign same-leaf.
	if obj.Type() == "subscript_expression" {
		if t := jsPromiseAllSubscriptTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsObjectEntriesPairValueTypeEx(obj, content, typedLocals, factories, entryLocals, entryArrayLocals, entryNextLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsArrayElemSubscriptType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsObjectFromEntriesPropTypeEx(obj, content, typedLocals, factories, entryArrayLocals, objValueLocals, mapLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// r.value.run() / (await Promise.allSettled([new A()]))[0].value.run() /
	// genA().next().value.run() / (await agenA().next()).value.run() /
	// [new A()].values().next().value.run() /
	// [new A()][Symbol.iterator]().next().value.run() /
	// Iterator.from([new BoxA().get()]).next().value.run() /
	// Object.fromEntries([[k, new A()]]).k.run() / o.k.run() after fromEntries local —
	// Object.fromEntries([[k, new BoxA().get()]]).k.run() method-return peels —
	// value peel under foreign same-leaf methods.
	if obj.Type() == "member_expression" || obj.Type() == "member_expression_optional" || obj.Type() == "optional_chain" {
		if t := jsPromiseAllSettledValueTypeEx(obj, content, typedLocals, settledOf, factories, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsGeneratorNextValueTypeEx(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsObjectFromEntriesPropTypeEx(obj, content, typedLocals, factories, entryArrayLocals, objValueLocals, mapLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// ma.get(k) ?? new A() / ma.get(k) || new A() — nullish/or default peels
	// under foreign same-leaf when both arms agree on T (or one is null/undefined).
	if obj.Type() == "binary_expression" {
		if t := jsNullishOrDefaultTypeEx(obj, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// (c ? new A() : new A()).run() / (c ? a : x).run() — both arms agree on T.
	// (c ? new BoxA().get() : new BoxA().get()).helper() — method-return arms
	// under foreign same-leaf (jsTernaryExprLeafType peels ba.get() / new T()).
	if obj.Type() == "ternary_expression" {
		if t := jsTernaryExprLeafType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		cons := ingest.ChildByField(obj, "consequence")
		alt := ingest.ChildByField(obj, "alternative")
		if cons != nil && alt != nil &&
			jsShouldRenameMember(cons, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals, classFields, methodReturns) &&
			jsShouldRenameMember(alt, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, settledOf, factories, generators, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals, classFields, methodReturns) {
			return true
		}
	}
	// {...{k: new A()}}.k.run() / {...oa}.k.run() / {...{k: ba.get()}}.k —
	// object spread property peels when spread sources agree on uniform value T
	// (same leaf as Object.assign; method-return via extra).
	if obj.Type() == "member_expression" || obj.Type() == "member_expression_optional" || obj.Type() == "optional_chain" {
		if t := jsObjectSpreadPropTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		// structuredClone({k: new A()}).k / structuredClone(oa).k — clone preserves
		// uniform object value type (same leaf as oa.k / {...oa}.k).
		if t := jsStructuredCloneObjectPropTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		// new Proxy({k: new A()}, {}).k / Object.freeze({k: new A()}).k /
		// Object.create(oa).k / Object.create({k: ba.get()}).k — identity object
		// wrappers preserve prop value T (method-return via Ex).
		if t := jsIdentityObjectPropTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// structuredClone({k: new A()})["k"] / new Proxy({k: new A()}, {})["k"] /
	// Object.create({k: ba.get()})["k"] — bracket form of object-prop peels.
	if obj.Type() == "subscript_expression" {
		if t := jsStructuredCloneObjectPropTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
		if t := jsIdentityObjectPropTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// new BoxA().a.helper() / BoxA.sa.helper() / ba.a.helper() /
	// this.#a.helper() / this.a.helper() / new OuterA().box.a.helper() —
	// class field peels from same-file field initializers (new T()) under
	// foreign same-leaf (private # too; nested field paths).
	if obj.Type() == "member_expression" || obj.Type() == "member_expression_optional" || obj.Type() == "optional_chain" {
		if t := jsClassFieldAccessType(obj, content, typedLocals, classFields, methodReturns, enclosingClass); t != "" {
			return jsRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil)
		}
	}
	// new BoxA().get().helper() / A.create().helper() / this.#get().helper() —
	// zero-arg same-file method return peels under foreign same-leaf.
	if obj.Type() == "call_expression" {
		if t := jsMethodCallReturnType(obj, content, typedLocals, classFields, methodReturns, enclosingClass); t != "" {
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
// entryLocals maps Object.entries / Array.entries / Map.entries pair locals → value type leaf
// (const e = Object.entries({k: new A()})[0] → e:"A",
// const e = [new A()].entries().next().value → e:"A") so e[1] peels.
// entryArrayLocals maps pair-array / entries-iterator locals → value type leaf
// (const es = Object.entries({k: new A()}) → es:"A",
// const ia = [new A()].entries() → ia:"A") so es[i][1] / ia.next().value[1] peels.
// entryNextLocals maps entries-iterator .next() result locals → pair value type
// (const ra = [new A()].entries().next() → ra:"A") so ra.value[1] peels.
// mapLocals maps Map locals → uniform value type
// (const ma = new Map([[k, new A()]]) → ma:"A") so ma.entries() / for-of peels.
// objValueLocals maps plain-object locals → uniform property value type
// (const o = Object.fromEntries([[k, new A()]]) → o:"A") so o.k.run() peels.
func jsTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool, factories, generators map[string]string, classFields, methodReturns map[string]map[string]string) (map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string, map[string]string) {
	out := map[string]string{}
	settledOf := map[string]string{}
	genLocals := map[string]string{}
	arrayLocals := map[string]string{}
	entryLocals := map[string]string{}
	entryArrayLocals := map[string]string{}
	entryNextLocals := map[string]string{}
	mapLocals := map[string]string{}
	objValueLocals := map[string]string{}
	// setLocals maps Set locals → uniform element type (const sa = new Set([new A()]) → sa:"A").
	// Returned for set.values/keys/entries member peels under foreign same-leaf methods.
	setLocals := map[string]string{}
	// groupByLocals / groupMapLocals: Object.groupBy / Map.groupBy result locals → element T.
	groupByLocals := map[string]string{}
	groupMapLocals := map[string]string{}
	// groupEntryLocals: [key, T[]] pair locals from groupBy entries → element T.
	groupEntryLocals := map[string]string{}
	// groupEntryArrayLocals: Object.entries(groupBy) / ma.entries() locals → element T.
	groupEntryArrayLocals := map[string]string{}
	extra := jsExtraLocals{
		objValue: objValueLocals, groupBy: groupByLocals, groupMap: groupMapLocals,
		groupEntry: groupEntryLocals, groupEntryArray: groupEntryArrayLocals,
		mapLocals: mapLocals, setLocals: setLocals, entryArrayLocals: entryArrayLocals,
		classFields: classFields, methodReturns: methodReturns,
	}
	if root == nil || len(ourReceivers) == 0 {
		return out, settledOf, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals
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
					if ctor := jsNewExpressionType(valN, content); ctor != "" && (ourReceivers[ctor] ||
						(classFields != nil && classFields[ctor] != nil) ||
						(methodReturns != nil && methodReturns[ctor] != nil)) {
						// const a = new A() — our receiver.
						// const oa = new OuterA() — non-our class with fields/methods so
						// oa.box.a.helper() peels under foreign same-leaf (dual-class).
						out[ingest.NodeText(nameN, content)] = ctor
					} else if t := jsMethodCallReturnType(valN, content, out, classFields, methodReturns, ""); t != "" {
						// const a = new BoxA().get() / A.create() — method return T.
						// Bind foreign too so dual-class B rebinds fail closed.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsClassFieldAccessType(valN, content, out, classFields, methodReturns, ""); t != "" {
						// const xa = new BoxA().a / oa.box — field peel under foreign same-leaf.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsTernaryExprLeafType(valN, content, out, factories, classFields, methodReturns); t != "" {
						// const xa = c ? a : x / c ? new A() : new A() — both arms agree.
						// const mrA = c ? new BoxA().get() : new BoxA().get() — method-return
						// arms under foreign same-leaf (inline already renames via dual
						// shouldRenameMember on arms; Class peels via jsTernaryExprType).
						// Bind foreign too so dual-class B rebinds fail closed.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectSpreadPropTypeEx(valN, content, out, factories, objValueLocals, classFields, methodReturns); t != "" {
						// const a = {...{k: new A()}}.k / {...oa}.k / {...{k: ba.get()}}.k
						// — uniform spread value T (method-return via Ex). Bind foreign too.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsStructuredCloneObjectPropTypeEx(valN, content, out, factories, objValueLocals, classFields, methodReturns); t != "" {
						// const a = structuredClone({k: new A()}).k / structuredClone(oa).k
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsIdentityObjectPropTypeEx(valN, content, out, factories, objValueLocals, classFields, methodReturns); t != "" {
						// const a = new Proxy({k: new A()}, {}).k / Object.freeze({k: new A()}).k /
						// Object.create({k: ba.get()}).k — method-return prop peels.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseResolveArgTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// await Promise.resolve(new A() / a / makeA() / ba.get()) — resolved value is A.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseRaceValueTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// await Promise.race/any([new A() / ba.get()]) — value is A when elems agree.
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseAllSettledSubscriptTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const r = (await Promise.allSettled([new A() / ba.get()]))[0]
						settledOf[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseAllSettledValueTypeEx(valN, content, out, settledOf, factories, classFields, methodReturns); ourReceivers[t] {
						// const a = ra.value after const [ra] = await Promise.allSettled([new A()])
						// / const a = (await Promise.allSettled([new A() / ba.get()]))[0].value
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsIdentityCloneTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const a = structuredClone(new A()) / Object.assign(new A()[, …]) /
						// Object.create(a) / Object.create(new A())
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsProxyTargetTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const pa = new Proxy(a, {}) / new Proxy(new A(), {})
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsWeakRefTargetTypeEx(valN, content, out, factories, classFields, methodReturns); t != "" {
						// const wa = new WeakRef(new A()) / new WeakRef(a) — WeakRef holder;
						// wa.deref() peels via genLocals["@weakref."+name]. Bind foreign too
						// for shadowing (wb after wa).
						genLocals["@weakref."+ingest.NodeText(nameN, content)] = t
					} else if t := jsNestedArrayFlatElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const aa = [[new A()]] — nested array local of element type A.
						// Stored under @nested so aa[0][0].run() / aa.flat()[0].run() peel
						// without treating aa[0] as scalar A. Bind foreign too for shadowing.
						arrayLocals["@nested."+ingest.NodeText(nameN, content)] = t
					} else if t := jsNestedIdentityMapThenFlatElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const ma = aa.map(xs => xs) / aa.map(xs => xs.map(x => x)) when
						// aa is [[new A()]] — identity map preserves nested shape; store
						// @nested so ma.flat()[0].run() peels under foreign same-leaf.
						// Bind foreign too for shadowing (mb after ma).
						arrayLocals["@nested."+ingest.NodeText(nameN, content)] = t
					} else if t := jsSetValueType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const sa = new Set([new A()]) / new Set(as) — Set of element T.
						// Bind before jsArraySourceElemType: array-source now peels new Set
						// (for Array.from(new Set) / [...xs]), which would otherwise steal
						// the binding into arrayLocals and leave setLocals empty so
						// xs.values().next().value / [...xs.values()] under-rename.
						// Bind foreign too so dual-class B rebinds fail closed.
						setLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsIteratorSourceYieldType(valN, content, generators, genLocals, arrayLocals, out, factories, mapLocals, entryArrayLocals, setLocals); ourReceivers[t] {
						// const ga = genA() / const ia = [new A()].values() /
						// const ia = new Set([new A()]).values() / sa.keys() /
						// const ia = ma.values() — iterator of A.
						// Bind before jsArraySourceElemType: array-source now peels
						// map/set/arr.values() as element sources (for [...ma.values()]),
						// which would otherwise steal into arrayLocals and break
						// ia.next().value peels via genLocals.
						genLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsArraySourceElemType(valN, content, arrayLocals, out, factories, extra); ourReceivers[t] {
						// const as = [new A()] / Array.from([new A()]) /
						// Array.of(new A()) / Object.values({k: new A()}) —
						// array local of element type A.
						arrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectEntriesPairValueTypeEx(valN, content, out, factories, entryLocals, entryArrayLocals, entryNextLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); ourReceivers[t] {
						// const a = Object.entries({k: new A()})[0][1] /
						// const a = es[0][1] / [...Object.entries(...)][0][1] /
						// const a = e[1] after entry-local e /
						// const a = [new A()].entries().next().value[1]
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectEntriesPairSubscriptTypeEx(valN, content, out, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
						// const e = Object.entries({k: new A()})[0] /
						// const e = es[0] after entries-array local /
						// const e = [...Object.entries(...)][0] — pair local;
						// e[1].run() peels via entryLocals. Bind foreign value types too
						// so dual-class B rebinds overwrite a prior A under the same name
						// (rename path fails closed via foreignReceivers).
						entryLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectEntriesPairAtTypeEx(valN, content, out, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
						// const e = [...ma].at(0) / es.at(0) — pair local of value T
						// (same leaf as es[0]); e[1].run() peels via entryLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsEntriesNextPairType(valN, content, out, factories, arrayLocals, entryArrayLocals, entryNextLocals, mapLocals, setLocals); t != "" {
						// const e = [new A()].entries().next().value /
						// const e = ia.next().value / ra.value — pair local of value T.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsEntriesNextResultType(valN, content, out, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals); t != "" {
						// const ra = [new A()].entries().next() / ia.next() —
						// IteratorResult whose .value is pair of value T.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryNextLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsMapValueTypeEx(valN, content, out, factories, entryArrayLocals, classFields, methodReturns); t != "" {
						// const ma = new Map([[k, new A()]]) / new WeakMap([[k, ba.get()]]) /
						// new Map(pa) — Map/WeakMap of value T (method-return too). Bind BEFORE
						// jsObjectEntriesArraySourceType:
						// entries-array peels now accept bare Map / new Map (default
						// iterator is entries) for [...ma][i][1] / Array.from(ma)[i][1],
						// which would otherwise steal into entryArrayLocals and leave
						// mapLocals empty so ma.get(k) under-renames.
						// Bind foreign too so dual-class B rebinds fail closed.
						mapLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectEntriesArraySourceTypeEx(valN, content, out, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
						// const es = Object.entries({k: new A()}) /
						// const es = [...Object.entries({k: new A()})] /
						// const es = [...ma] after map local —
						// array of [key, value] pairs of value T; es[i][1] peels.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryArrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsArrayEntriesValueType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const ia = [new A()].entries() / as.entries() —
						// entries-iterator of pairs of value T; ia.next().value[1] peels.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryArrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsMapEntriesValueType(valN, content, out, factories, mapLocals, entryArrayLocals); t != "" {
						// const ie = new Map([[k, new A()]]).entries() / ma.entries()
						entryArrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsMapSymbolIteratorEntriesType(valN, content, out, factories, mapLocals, entryArrayLocals); t != "" {
						// const ia = ma[Symbol.iterator]() / new Map([[k, new A()]])[Symbol.iterator]()
						// — Map default iterator yields entries pairs of value T.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryArrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsSetEntriesValueType(valN, content, arrayLocals, out, factories, setLocals); t != "" {
						// const ie = new Set([new A()]).entries() / sa.entries()
						// pairs of [T, T]; ie.next().value[1] peels.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryArrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsPairArrayValueType(valN, content, out, factories); t != "" {
						// const pa = [["k", new A()]] — pair-array local of value T;
						// new Map(pa).get(k) peels via entryArrayLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						entryArrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectFromEntriesValueTypeEx(valN, content, out, factories, entryArrayLocals, mapLocals, classFields, methodReturns); t != "" {
						// const o = Object.fromEntries([[k, new A()]]) / Object.fromEntries(pa) /
						// Object.fromEntries([[k, new BoxA().get()]]) method-return pairs /
						// Object.fromEntries(ma) after Map.set / new Map — plain object of
						// property values T; o.k peels via objValueLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						objValueLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectAssignValueTypeEx(valN, content, out, factories, objValueLocals, classFields, methodReturns); t != "" {
						// const o = Object.assign({}, {k: new A()}, …) /
						// Object.assign({}, {k: new BoxA().get()}) — plain object of
						// uniform property values T (method-return via methodReturns);
						// Object.values(o) peels via objValueLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						objValueLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsIdentityObjectValueType(valN, content, out, factories, objValueLocals); t != "" {
						// const pa = new Proxy({k: new A()}, {}) / Object.freeze(oa) /
						// Object.create(oa) — plain object of uniform property values T;
						// pa.k peels via objValueLocals. Bind foreign too for shadowing.
						objValueLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectLiteralNestedArrayElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const oa = {k: [new A()]} — property values are arrays of T
						// (groupBy-like shape); oa[k][0] / Object.values(oa)[0][0] peel
						// via groupByLocals. Bind foreign too for dual-class shadowing.
						groupByLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectLiteralValueTypeEx(valN, content, out, factories, extra); t != "" {
						// const oa = {k: new A()} / {k: new BoxA().get()} — plain object
						// of uniform property values T (method-return via extra);
						// Object.values(oa)[0] / oa.k peel via objValueLocals.
						// Nested-array shape handled above. Bind foreign too for shadowing.
						objValueLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectGroupByElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const ga = Object.groupBy([new A()], fn) — groups of T;
						// ga[k][0].run() peels via groupByLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						groupByLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectValuesGroupByElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const va = Object.values(ga) / Object.values(Object.groupBy(...)) —
						// array of group arrays T[]; va[i][0].run() peels via
						// groupByLocals["@values."+va] (not arrayLocals — elements are
						// T[], not T). Bind foreign too so dual-class B rebinds fail closed.
						groupByLocals["@values."+ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectFromEntriesGroupByElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const oa = Object.fromEntries(Object.entries(ga)) /
						// Object.fromEntries(Object.entries(Object.groupBy(...))) —
						// reconstructs groupBy shape (property values T[]);
						// oa[k][0].run() peels via groupByLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						groupByLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsMapGroupByElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const ma = Map.groupBy([new A()], fn) — Map of T[] groups;
						// ma.get(k)[0].run() peels via groupMapLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						groupMapLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsGroupByEntriesPairSubscriptElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const e = Object.entries(ga)[0] / [...ma.entries()][0] —
						// pair of group-array T[]; e[1][0] peels via groupEntryLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						groupEntryLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsGroupByEntriesNextPairElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const e = ma.entries().next().value /
						// Map.groupBy(...).entries().next().value — pair of T[].
						// Bind foreign too so dual-class B rebinds fail closed.
						groupEntryLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsGroupByEntriesSourceElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const es = Object.entries(ga) / const ie = ma.entries() /
						// const es = [...Object.entries(ga)] — array/iterator of
						// [key, T[]] pairs; es[i][1][0] peels via groupEntryArrayLocals.
						// Bind foreign too so dual-class B rebinds fail closed.
						// Note: does not match bare Map.groupBy / ga (those are
						// groupMap/groupBy locals, not entries sources).
						groupEntryArrayLocals[ingest.NodeText(nameN, content)] = t
					} else if t := jsObjectFromEntriesPropTypeEx(valN, content, out, factories, entryArrayLocals, objValueLocals, mapLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = Object.fromEntries([[k, new A()]]).k /
						// const a = Object.fromEntries([[k, new BoxA().get()]]).k /
						// const a = Object.fromEntries(...)["k"] / o.k after fromEntries local /
						// Object.fromEntries(ma).k after Map.set / new Map
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsMapGetValueTypeEx(valN, content, out, factories, mapLocals, entryArrayLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = new Map([[k, new A()]]).get(k) / ma.get(k) / new Map(pa).get(k) /
						// const a = new Map([[k, new BoxA().get()]]).get(k) /
						// const a = new WeakMap([[k, ba.get()]]).get(k) — method-return
						// Map/WeakMap value under foreign same-leaf (inline already peels
						// via jsMapGetValueTypeEx in shouldRenameMember; Class peels via
						// jsMapGetValueType).
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsNullishOrDefaultTypeEx(valN, content, out, factories, mapLocals, entryArrayLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = ma.get(k) ?? new A() / null ?? new BoxA().get() /
						// null || new BoxA().get() / true && new BoxA().get()
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsArrayElemSubscriptType(valN, content, arrayLocals, out, factories, extra); ourReceivers[t] {
						// const a = [new A()][0] / Array.from([new A()])[0] /
						// Array.of(new A())[0] / Object.values({k: new A()})[0]
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsArrayAtElemType(valN, content, arrayLocals, out, factories, extra); ourReceivers[t] {
						// const a = [new A()].at(0) / as.at(-1) /
						// Array.from([new A()]).at(0) / Object.values({k: new A()}).at(0)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsArrayPopShiftElemType(valN, content, arrayLocals, out, factories, extra); ourReceivers[t] {
						// const a = [new A()].pop() / as.shift() /
						// Array.from([new A()]).pop() / [new A()].slice().pop()
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsArrayFindElemType(valN, content, arrayLocals, out, factories, extra); ourReceivers[t] {
						// const a = [new A()].find(pred) / as.findLast(pred)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsArrayReduceElemType(valN, content, arrayLocals, out, factories, extra); ourReceivers[t] {
						// const a = [new A()].reduce((a,b)=>a) / as.reduce((a,b)=>b)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsIteratorFindElemType(valN, content, generators, genLocals, arrayLocals, out, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = Iterator.from([new A()]).find(pred) /
						// Iterator.from([new BoxA().get()]).find(pred)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsIteratorReduceElemType(valN, content, generators, genLocals, arrayLocals, out, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = Iterator.from([new A()]).reduce((a,x) => x) /
						// Iterator.from([new BoxA().get()]).reduce(...)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseThenIdentityType(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const a = await Promise.resolve(new A() / new BoxA().get()).then(x => x)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseFinallyType(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const a = await Promise.resolve(new A() / new BoxA().get()).finally(() => {})
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseCatchType(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const a = await Promise.resolve(new A() / new BoxA().get()).catch(() => null)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsPromiseTryTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const a = await Promise.try(() => new A() / new BoxA().get())
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsWeakRefDerefTypeEx(valN, content, out, factories, genLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = new WeakRef(new A()).deref() / wa.deref() after
						// const wa = new WeakRef(...)
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsReflectGetTypeEx(valN, content, out, factories, objValueLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = Reflect.get({k: new A()}, "k") / Reflect.get(oa, "k")
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsReflectConstructType(valN, content); ourReceivers[t] {
						// const a = Reflect.construct(A, [])
						out[ingest.NodeText(nameN, content)] = t
					} else if t := jsGeneratorNextResultTypeEx(valN, content, generators, genLocals, arrayLocals, out, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); ourReceivers[t] {
						// const ra = genA().next() / [new A()].values().next() /
						// ia.next() / Iterator.from([ba.get()]).next() —
						// IteratorResult; ra.value peels via settledOf.
						settledOf[ingest.NodeText(nameN, content)] = t
					} else if t := jsGeneratorNextValueTypeEx(valN, content, generators, genLocals, arrayLocals, out, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); ourReceivers[t] {
						// const a = genA().next().value / [new A()].values().next().value /
						// new Set([new A()]).values().next().value /
						// Iterator.from([new BoxA().get()]).next().value
						out[ingest.NodeText(nameN, content)] = t
					}
				} else if nameN.Type() == "array_pattern" {
					// const [a] = await Promise.all([new A() / ba.get()]) — bind pattern names to elem type.
					if t := jsPromiseAllElemTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						jsBindArrayPatternNames(nameN, content, t, out)
					} else if t := jsPromiseAllSettledElemTypeEx(valN, content, out, factories, classFields, methodReturns); ourReceivers[t] {
						// const [r] = await Promise.allSettled([new A() / ba.get()])
						// const [{value: a}] / [{value}] = await Promise.allSettled([new A()])
						jsBindAllSettledArrayPattern(nameN, content, t, out, settledOf)
					} else if t := jsObjectEntriesPairSubscriptTypeEx(valN, content, out, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); ourReceivers[t] {
						// const [, a] = Object.entries({k: new A()})[0] /
						// const [, a] = es[0] / [...Object.entries(...)][0] — value slot is A.
						jsBindEntriesArrayPattern(nameN, content, t, out)
					} else if t := jsEntriesNextPairType(valN, content, out, factories, arrayLocals, entryArrayLocals, entryNextLocals, mapLocals, setLocals); ourReceivers[t] {
						// const [, a] = [new A()].entries().next().value /
						// const [, a] = ra.value after entries next result
						jsBindEntriesArrayPattern(nameN, content, t, out)
					} else if t := jsArraySourceElemType(valN, content, arrayLocals, out, factories, extra); t != "" {
						// const [a] = ma.get("k") / const [a] = [new A()] /
						// const [a] = as / const [a] = Object.values(ga)[0] —
						// group-array / array-source element T.
						// Bind foreign too so dual-class B rebinds fail closed.
						jsBindArrayPatternNames(nameN, content, t, out)
					} else if valN.Type() == "identifier" && entryLocals != nil {
						// const [, a] = e after const e = Object.entries({k: new A()})[0]
						// / after const e = [new A()].entries().next().value
						if t := entryLocals[ingest.NodeText(valN, content)]; ourReceivers[t] {
							jsBindEntriesArrayPattern(nameN, content, t, out)
						}
					}
				} else if nameN.Type() == "object_pattern" {
					// const {k: a} = {k: new A()} / const {k: a} = oa after
					// objValue local of uniform property value T.
					// Bind foreign too so dual-class B rebinds fail closed.
					// const {k: a} = {k: new BoxA().get()} — method-return values.
					if t := jsObjectLiteralValueTypeEx(valN, content, out, factories, extra); t != "" {
						jsBindObjectPatternUniformValues(nameN, content, t, out)
					} else if t := jsObjectLiteralValueType(valN, content, out, factories); t != "" {
						jsBindObjectPatternUniformValues(nameN, content, t, out)
					} else if t := jsObjectSpreadValueTypeEx(valN, content, out, factories, objValueLocals, classFields, methodReturns); t != "" {
						// const {k: a} = {...{k: new A()}} / {...oa} /
						// {...{k: new BoxA().get()}} — method-return spread values.
						jsBindObjectPatternUniformValues(nameN, content, t, out)
					} else if valN.Type() == "identifier" && objValueLocals != nil {
						if t := objValueLocals[ingest.NodeText(valN, content)]; t != "" {
							jsBindObjectPatternUniformValues(nameN, content, t, out)
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
			// for (const [, a] of es) / for (const e of es) after entries-array local —
			// for (const [, a] of [new A()].entries()) / for (const e of arr.entries()) —
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
				if t := jsForOfElemType(right, content, generators, genLocals, arrayLocals, out, factories, mapLocals, entryArrayLocals, setLocals, extra); ourReceivers[t] {
					out[ingest.NodeText(left, content)] = t
				} else if t := jsSetSourceValueType(right, content, arrayLocals, out, factories, setLocals); ourReceivers[t] {
					// for (const a of new Set([new A()])) / for (const a of sa)
					out[ingest.NodeText(left, content)] = t
				} else if t := jsMapGroupByValuesElemType(right, content, arrayLocals, out, factories, extra); t != "" {
					// for (const g of ma.values()) / Map.groupBy(...).values() —
					// yields group arrays T[]; bind as array local of T.
					// Bind foreign too so dual-class B rebinds fail closed.
					arrayLocals[ingest.NodeText(left, content)] = t
				} else if t := jsObjectValuesGroupByElemType(right, content, arrayLocals, out, factories, extra); t != "" {
					// for (const g of Object.values(ga)) /
					// for (const g of Object.values(Object.groupBy(...))) —
					// yields group arrays T[]; bind as array local of T.
					// Bind foreign too so dual-class B rebinds fail closed.
					arrayLocals[ingest.NodeText(left, content)] = t
				} else if t := jsGroupByEntriesIterableElemType(right, content, arrayLocals, out, factories, extra); t != "" {
					// for (const e of Object.entries(ga)) /
					// for (const e of ma.entries()) / for (const e of ma) —
					// pair of group-array T[]; e[1][0] peels via groupEntryLocals
					// (not scalar entryLocals — pair value is T[], not T).
					// Bind foreign too so dual-class B rebinds fail closed.
					groupEntryLocals[ingest.NodeText(left, content)] = t
				} else if t := jsEntriesIterableValueTypeEx(right, content, out, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
					// for (const e of Object.entries({k: new A()})) /
					// for (const e of es) / for (const e of [new A()].entries()) /
					// for (const e of new Map([[k, new A()]]).entries()) /
					// for (const e of new Map([[k, new A()]])) —
					// pair of value T; e itself is not T (e[1] peels via entryLocals).
					// Bind foreign too so dual-class B rebinds fail closed on rename.
					entryLocals[ingest.NodeText(left, content)] = t
				}
			} else if left.Type() == "array_pattern" {
				// for (const [, a] of Object.entries({k: new A()})) /
				// for (const [, a] of es) / for (const [, a] of [new A()].entries()) /
				// for (const [, a] of new Map([[k, new A()]]).entries()) /
				// for (const [, a] of new Map([[k, new A()]])) —
				// value slot only.
				// GroupBy entries first: value slot is T[] (group array), not T.
				if t := jsGroupByEntriesIterableElemType(right, content, arrayLocals, out, factories, extra); t != "" {
					// for (const [, g] of Object.entries(ga)) /
					// for (const [, g] of ma.entries()) / for (const [, g] of ma) —
					// g is array of T; g[0].run() peels via arrayLocals.
					// Bind foreign too so dual-class B rebinds fail closed.
					jsBindEntriesArrayPattern(left, content, t, arrayLocals)
				} else if t := jsEntriesIterableValueTypeEx(right, content, out, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, objValueLocals, extra); ourReceivers[t] {
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
			// Promise.resolve(new A() / a / makeA() / ba.get()).then(a => a.run()) — bind then callback param.
			// Promise.allSettled([new A() / ba.get()]).then(([r]) => r.value.run()) — settled params.
			jsBindPromiseThenParams(n, content, ourReceivers, out, settledOf, factories, classFields, methodReturns)
			// new Map([[k, new A()]]).forEach((v) => v.run()) /
			// [new A()].forEach((v) => v.run()) — bind forEach value/elem param.
			jsBindForEachParams(n, content, ourReceivers, out, arrayLocals, factories, mapLocals, entryArrayLocals, setLocals, extra)
			// xs.push(new A()) / xs.unshift(new A()) / xs.splice(0,0,new A()) /
			// xs.push(...[new A()]) / xs.fill(new A()) — bare array mutation.
			// Bind arrayLocals so xs[0].run() / for (const a of xs) peel under
			// foreign same-leaf. Foreign elems too for shadowing.
			if local, et := jsArrayMutationElemType(n, content, arrayLocals, out, factories, extra); local != "" && et != "" {
				arrayLocals[local] = et
			}
			// xs.add(new A()) / xs.add(new BoxA().get()) — bare Set mutation after
			// new Set() / empty ctor. Bind setLocals so for (const a of xs) /
			// xs.values().next().value peel under foreign same-leaf. Foreign elems
			// too for shadowing.
			if local, et := jsSetAddMutationElemTypeEx(n, content, out, factories, classFields, methodReturns); local != "" && et != "" {
				setLocals[local] = et
			}
			// m.set(k, new A()) / wm.set(k, new A()) — bare Map/WeakMap mutation.
			// Bind mapLocals so m.get(k) / [...m.values()] peel under foreign
			// same-leaf. Foreign values too for shadowing.
			if local, et := jsMapSetMutationElemTypeEx(n, content, out, factories, classFields, methodReturns); local != "" && et != "" {
				// m.set(k, new A()) / m.set(k, new BoxA().get()) — Class + method-return.
				mapLocals[local] = et
			}
		case "assignment_expression":
			// xs = [...xs, new A()] / xs = xs.concat([new A()]) /
			// xs = xs.toSpliced(0,0,new A()) / xs = [new A()] —
			// rebind arrayLocals from RHS (self-target untyped arms are wildcards).
			// Foreign too for shadowing (ys after xs).
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left != nil && left.Type() == "identifier" && right != nil {
				name := ingest.NodeText(left, content)
				if t := jsArrayAssignSourceElemType(right, content, arrayLocals, out, factories, extra, name); t != "" {
					arrayLocals[name] = t
				}
			}
			// xs[0] = new A() / xs[0] = new BoxA().get() — index assign binds
			// arrayLocals so xs[0].run() / for (const a of xs) peel under foreign
			// same-leaf. Numeric index only; non-ident receivers / non-concrete
			// RHS fail closed. Foreign too. Method-return via extra.
			if local, et := jsArrayIndexAssignElemTypeEx(n, content, out, factories, classFields, methodReturns); local != "" && et != "" {
				arrayLocals[local] = et
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out, settledOf, genLocals, arrayLocals, entryLocals, entryArrayLocals, entryNextLocals, mapLocals, objValueLocals, setLocals, groupByLocals, groupMapLocals, groupEntryLocals, groupEntryArrayLocals
}

// jsClassFieldIndex maps same-file class name → field name → value type leaf
// from field initializers that are `new T(...)`:
//
//	class BoxA { a = new A(); static sa = new A(); #pa = new A(); }
//
// Enables new BoxA().a.helper() / BoxA.sa.helper() under foreign same-leaf.
// Private fields use the private name text including '#'. Other initializers
// fail closed.
func jsClassFieldIndex(root *grammar.Node, content []byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "class_declaration" || n.Type() == "class" {
			nameN := ingest.ChildByField(n, "name")
			body := ingest.ChildByField(n, "body")
			if nameN == nil || body == nil {
				// class expression may lack name; skip
			} else {
				typeName := ingest.NodeText(nameN, content)
				fields := map[string]string{}
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					if ch == nil {
						continue
					}
					// field_definition: [static] name = value
					if ch.Type() != "field_definition" && ch.Type() != "public_field_definition" {
						continue
					}
					var prop *grammar.Node
					var val *grammar.Node
					for j := uint32(0); j < ch.ChildCount(); j++ {
						c := ch.Child(j)
						if c == nil {
							continue
						}
						switch c.Type() {
						case "property_identifier", "private_property_identifier", "identifier":
							if prop == nil {
								prop = c
							}
						case "new_expression":
							val = c
						}
					}
					// Prefer field-named children when present.
					if p := ingest.ChildByField(ch, "property"); p != nil {
						prop = p
					}
					if v := ingest.ChildByField(ch, "value"); v != nil {
						val = v
					}
					if prop == nil || val == nil {
						continue
					}
					fname := ingest.NodeText(prop, content)
					if fname == "" {
						continue
					}
					if tn := jsNewExpressionType(val, content); tn != "" {
						fields[fname] = tn
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

// jsEnhanceClassFieldsMethodReturn fills classFields entries for field
// initializers that peel as method-return leaves (mrA = new BoxA().get() → A)
// after methodReturns is built. Class()-only peels live in jsClassFieldIndex.
// Enables new HolderM().mrA.run() / h.mrA.run() under foreign same-leaf.
// Existing Class() fields are left unchanged; empty peels fail closed.
func jsEnhanceClassFieldsMethodReturn(root *grammar.Node, content []byte, classFields, methodReturns map[string]map[string]string) {
	if root == nil || classFields == nil || methodReturns == nil {
		return
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "class_declaration" || n.Type() == "class" {
			nameN := ingest.ChildByField(n, "name")
			body := ingest.ChildByField(n, "body")
			if nameN != nil && body != nil {
				typeName := ingest.NodeText(nameN, content)
				if typeName != "" {
					fields := classFields[typeName]
					if fields == nil {
						fields = map[string]string{}
					}
					for i := uint32(0); i < body.ChildCount(); i++ {
						ch := body.Child(i)
						if ch == nil {
							continue
						}
						if ch.Type() != "field_definition" && ch.Type() != "public_field_definition" {
							continue
						}
						prop := ingest.ChildByField(ch, "property")
						val := ingest.ChildByField(ch, "value")
						if prop == nil || val == nil {
							// Fall back to child scan (same as jsClassFieldIndex).
							for j := uint32(0); j < ch.ChildCount(); j++ {
								c := ch.Child(j)
								if c == nil {
									continue
								}
								switch c.Type() {
								case "property_identifier", "private_property_identifier", "identifier":
									if prop == nil {
										prop = c
									}
								case "call_expression", "new_expression":
									if val == nil {
										val = c
									}
								}
							}
						}
						if prop == nil || val == nil {
							continue
						}
						fname := ingest.NodeText(prop, content)
						if fname == "" {
							continue
						}
						// Already Class()-indexed — leave alone.
						if fields[fname] != "" {
							continue
						}
						// new BoxA().get() / ba.get() — method-return leaf.
						if tn := jsExprLeafType(val, content, nil, nil, classFields, methodReturns); tn != "" {
							fields[fname] = tn
						}
					}
					if len(fields) > 0 {
						classFields[typeName] = fields
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
}

// jsClassFieldAccessType recovers T from new BoxA().a / BoxA.sa / ba.a /
// this.#a / this.a / new OuterA().box.a when the field is indexed from same-file
// class field initializers (new T()). typedLocals may supply ba → BoxA for
// instance field peels; enclosingClass supplies this → BoxA inside BoxA methods
// (private # too). methodReturns peels new BoxA().get().a-style bases.
func jsClassFieldAccessType(obj *grammar.Node, content []byte, typedLocals map[string]string, classFields, methodReturns map[string]map[string]string, enclosingClass string) string {
	if obj == nil || classFields == nil {
		return ""
	}
	if obj.Type() != "member_expression" && obj.Type() != "member_expression_optional" && obj.Type() != "optional_chain" {
		return ""
	}
	base := ingest.ChildByField(obj, "object")
	prop := ingest.ChildByField(obj, "property")
	if base == nil || prop == nil {
		return ""
	}
	fname := ingest.NodeText(prop, content)
	if fname == "" {
		return ""
	}
	var typeName string
	switch base.Type() {
	case "new_expression":
		typeName = jsNewExpressionType(base, content)
	case "this":
		// this.#a / this.a / this.#sa (static method) inside enclosing class.
		typeName = enclosingClass
	case "identifier":
		// BoxA.sa / BoxA.#sa (static on class name) or ba.a (typed local).
		name := ingest.NodeText(base, content)
		if _, isClass := classFields[name]; isClass {
			typeName = name
		} else if methodReturns != nil {
			if _, isClass := methodReturns[name]; isClass {
				typeName = name
			}
		}
		if typeName == "" && typedLocals != nil {
			typeName = typedLocals[name]
		}
	case "member_expression", "member_expression_optional", "optional_chain":
		// new OuterA().box.a — nested field path (outer type then field).
		typeName = jsClassFieldAccessType(base, content, typedLocals, classFields, methodReturns, enclosingClass)
	case "call_expression":
		// new BoxA().get().… — method-return base then field (rare product form).
		typeName = jsMethodCallReturnType(base, content, typedLocals, classFields, methodReturns, enclosingClass)
	default:
		return ""
	}
	if typeName == "" {
		return ""
	}
	if fields := classFields[typeName]; fields != nil {
		return fields[fname]
	}
	return ""
}

// jsClassMethodReturns maps same-file class → method name → return type leaf.
// Private methods keep the '#' name. Static methods included (A.create() → A).
//
// Harvest order (two-pass so method-return factory bodies peel even when BoxA
// is declared after A — same leaf as pythonSameFileFuncReturnTypes):
//  1. TS return type annotations (`static fromBox(ba: BoxA): A`) and zero-arg
//     body peels (`return new T()` / `return this.field` / `return this`).
//  2. Body peels that need other methods' returns (`return new BoxA().get()` /
//     `return ba.get()` with typed param ba: BoxA / `return x` after assign).
// Methods with parameters fail closed unless annotation or body peels to a
// concrete leaf (A.fromBox(ba).run() under foreign same-leaf).
// Enables new BoxA().get().helper() / A.create().helper() / A.fromBox(ba).run()
// / this.#get().helper() under foreign same-leaf methods.
func jsClassMethodReturns(root *grammar.Node, content []byte, classFields map[string]map[string]string) map[string]map[string]string {
	out := map[string]map[string]string{}
	if root == nil {
		return out
	}
	// Collect class bodies for two passes.
	type classBody struct {
		name string
		body *grammar.Node
	}
	var classes []classBody
	var collect func(n *grammar.Node)
	collect = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "class_declaration" || n.Type() == "class" {
			nameN := ingest.ChildByField(n, "name")
			body := ingest.ChildByField(n, "body")
			if nameN != nil && body != nil {
				if typeName := ingest.NodeText(nameN, content); typeName != "" {
					classes = append(classes, classBody{name: typeName, body: body})
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			collect(n.Child(i))
		}
	}
	collect(root)

	harvestMethod := func(ch *grammar.Node, typeName string, withMethodReturns bool) (mname, ret string) {
		if ch == nil || ch.Type() != "method_definition" {
			return "", ""
		}
		prop := ingest.ChildByField(ch, "name")
		if prop == nil {
			// property / private name as first identifier-like child.
			for j := uint32(0); j < ch.ChildCount(); j++ {
				c := ch.Child(j)
				if c == nil {
					continue
				}
				switch c.Type() {
				case "property_identifier", "private_property_identifier", "identifier":
					prop = c
				}
				if prop != nil {
					break
				}
			}
		}
		if prop == nil {
			return "", ""
		}
		mname = ingest.NodeText(prop, content)
		if mname == "" || mname == "constructor" {
			return "", ""
		}
		// Pass 1 preference: TS return type annotation (`: A`).
		if !withMethodReturns {
			if t := jsMethodReturnTypeAnnotation(ch, content); t != "" {
				return mname, t
			}
			// Zero-arg body: return new T() / this.field / this (no methodReturns).
			if jsMethodDefinitionZeroArg(ch) {
				if t := jsMethodBodyReturnType(ch, content, typeName, classFields, nil); t != "" {
					return mname, t
				}
			}
			return mname, ""
		}
		// Pass 2: body peels with methodReturns (method-return / typed-param).
		// Skip if already filled.
		if methods := out[typeName]; methods != nil && methods[mname] != "" {
			return mname, ""
		}
		if t := jsMethodBodyReturnType(ch, content, typeName, classFields, out); t != "" {
			return mname, t
		}
		return mname, ""
	}

	// Pass 1: annotations + zero-arg Class/field peels.
	for _, cl := range classes {
		methods := out[cl.name]
		if methods == nil {
			methods = map[string]string{}
		}
		for i := uint32(0); i < cl.body.ChildCount(); i++ {
			ch := cl.body.Child(i)
			mname, ret := harvestMethod(ch, cl.name, false)
			if mname != "" && ret != "" {
				methods[mname] = ret
			}
		}
		if len(methods) > 0 {
			out[cl.name] = methods
		}
	}
	// Pass 2: method-return / typed-param body peels (needs pass-1 index).
	for _, cl := range classes {
		methods := out[cl.name]
		if methods == nil {
			methods = map[string]string{}
		}
		for i := uint32(0); i < cl.body.ChildCount(); i++ {
			ch := cl.body.Child(i)
			mname, ret := harvestMethod(ch, cl.name, true)
			if mname != "" && ret != "" {
				methods[mname] = ret
			}
		}
		if len(methods) > 0 {
			out[cl.name] = methods
		}
	}
	return out
}

// jsMethodReturnTypeAnnotation recovers T from a TS method return type
// annotation: `static fromBox(ba: BoxA): A` / `get(): A`. Empty when absent
// or not a simple type_identifier (generics peel via jsTypeName).
func jsMethodReturnTypeAnnotation(method *grammar.Node, content []byte) string {
	if method == nil {
		return ""
	}
	// Field "return_type" when present; else sibling type_annotation after params.
	if rt := ingest.ChildByField(method, "return_type"); rt != nil {
		return jsTypeName(rt, content)
	}
	for i := uint32(0); i < method.ChildCount(); i++ {
		ch := method.Child(i)
		if ch != nil && ch.Type() == "type_annotation" {
			return jsTypeName(ch, content)
		}
	}
	return ""
}

// jsMethodDefinitionZeroArg reports whether a method_definition has no formal params.
func jsMethodDefinitionZeroArg(method *grammar.Node) bool {
	if method == nil {
		return false
	}
	params := ingest.ChildByField(method, "parameters")
	if params == nil {
		for i := uint32(0); i < method.ChildCount(); i++ {
			c := method.Child(i)
			if c != nil && (c.Type() == "formal_parameters" || c.Type() == "parameters") {
				params = c
				break
			}
		}
	}
	if params == nil {
		// No params node — treat as zero-arg (getter/setter forms vary).
		return true
	}
	for i := uint32(0); i < params.ChildCount(); i++ {
		c := params.Child(i)
		if c == nil {
			continue
		}
		switch c.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			return false
		}
	}
	return true
}

// jsMethodBodyReturnType recovers T when every return in method is
// `return new T()` / `return this.field` / `return this` / `return ba.get()`
// (typed param or local) / `return new BoxA().get()` (methodReturns) /
// `return x` after `const x = …` local assignment.
// Nested function bodies are skipped. Zero/mixed/unknown returns fail closed.
// methodReturns may be nil (pass-1 Class/field peels only).
func jsMethodBodyReturnType(method *grammar.Node, content []byte, className string, classFields, methodReturns map[string]map[string]string) string {
	if method == nil {
		return ""
	}
	body := ingest.ChildByField(method, "body")
	if body == nil || body.Type() != "statement_block" {
		return ""
	}
	// Seed typed params so return ba.get() peels under foreign same-leaf.
	localCtor := map[string]string{}
	jsHarvestMethodTypedParams(method, content, localCtor)

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
			return
		case "lexical_declaration", "variable_declaration":
			// const x = new T() / ba.get() / new BoxA().get() — track for return x.
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
				name := ingest.NodeText(nameN, content)
				if name == "" {
					continue
				}
				if t := jsMethodBodyExprLeafType(valN, content, className, localCtor, classFields, methodReturns); t != "" {
					localCtor[name] = t
				}
			}
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
			t := jsMethodBodyExprLeafType(expr, content, className, localCtor, classFields, methodReturns)
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

// jsHarvestMethodTypedParams seeds localCtor from TS formal params:
// `static fromBox(ba: BoxA)` → localCtor["ba"]="BoxA". Plain JS has none.
func jsHarvestMethodTypedParams(method *grammar.Node, content []byte, localCtor map[string]string) {
	if method == nil || localCtor == nil {
		return
	}
	params := ingest.ChildByField(method, "parameters")
	if params == nil {
		for i := uint32(0); i < method.ChildCount(); i++ {
			c := method.Child(i)
			if c != nil && (c.Type() == "formal_parameters" || c.Type() == "parameters") {
				params = c
				break
			}
		}
	}
	if params == nil {
		return
	}
	for i := uint32(0); i < params.ChildCount(); i++ {
		ch := params.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "required_parameter", "optional_parameter", "assignment_pattern":
			nameN := ingest.ChildByField(ch, "pattern")
			if nameN == nil {
				nameN = ingest.ChildByField(ch, "name")
			}
			if nameN == nil {
				nameN = ingest.ChildByType(ch, "identifier")
			}
			typeN := ingest.ChildByField(ch, "type")
			if nameN != nil && nameN.Type() == "identifier" && typeN != nil {
				if tn := jsTypeName(typeN, content); tn != "" {
					if name := ingest.NodeText(nameN, content); name != "" {
						localCtor[name] = tn
					}
				}
			}
		}
	}
}

// jsMethodBodyExprLeafType recovers T for a return/assign RHS inside a method
// body: new T() / this.field / this / typed local / ba.get() / new BoxA().get().
func jsMethodBodyExprLeafType(expr *grammar.Node, content []byte, className string, localCtor map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if expr == nil {
		return ""
	}
	if nt := jsNewExpressionType(expr, content); nt != "" {
		return nt
	}
	if expr.Type() == "identifier" {
		name := ingest.NodeText(expr, content)
		if localCtor != nil {
			if t := localCtor[name]; t != "" {
				return t
			}
		}
		return ""
	}
	if expr.Type() == "this" {
		return className
	}
	if expr.Type() == "member_expression" || expr.Type() == "member_expression_optional" || expr.Type() == "optional_chain" {
		// return this.#a / return this.a
		base := ingest.ChildByField(expr, "object")
		prop := ingest.ChildByField(expr, "property")
		if base != nil && base.Type() == "this" && prop != nil && classFields != nil {
			if fields := classFields[className]; fields != nil {
				return fields[ingest.NodeText(prop, content)]
			}
		}
	}
	if expr.Type() == "call_expression" && methodReturns != nil {
		// return ba.get() / return new BoxA().get() — peel via methodReturns.
		if t := jsMethodCallReturnType(expr, content, localCtor, classFields, methodReturns, className); t != "" {
			return t
		}
	}
	return ""
}

// jsMethodCallReturnType recovers T from new BoxA().get() / A.create() /
// A.fromBox(ba) / this.#get() / ba.get() when the method is indexed in
// methodReturns. Arguments are ignored once the method return leaf is known
// (factory/with-arg peels; unknown methods fail closed via missing index).
func jsMethodCallReturnType(call *grammar.Node, content []byte, typedLocals map[string]string, classFields, methodReturns map[string]map[string]string, enclosingClass string) string {
	if call == nil || call.Type() != "call_expression" || methodReturns == nil {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	base := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if base == nil || prop == nil {
		return ""
	}
	mname := ingest.NodeText(prop, content)
	if mname == "" {
		return ""
	}
	// Peel (new BoxA().self()).get() — parenthesized method receiver under
	// foreign same-leaf (bare new BoxA().self().get already peels).
	for base != nil && base.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < base.ChildCount(); i++ {
			ch := base.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		if inner == nil {
			return ""
		}
		base = inner
	}
	if base == nil {
		return ""
	}
	var typeName string
	switch base.Type() {
	case "new_expression":
		typeName = jsNewExpressionType(base, content)
	case "this":
		typeName = enclosingClass
	case "identifier":
		name := ingest.NodeText(base, content)
		if _, isClass := methodReturns[name]; isClass {
			typeName = name
		} else if classFields != nil {
			if _, isClass := classFields[name]; isClass {
				typeName = name
			}
		}
		if typeName == "" && typedLocals != nil {
			typeName = typedLocals[name]
		}
	case "member_expression", "member_expression_optional", "optional_chain":
		// new OuterA().box.get() — field peel then method.
		typeName = jsClassFieldAccessType(base, content, typedLocals, classFields, methodReturns, enclosingClass)
	case "call_expression":
		// ba.self().get() — nested method return then method.
		typeName = jsMethodCallReturnType(base, content, typedLocals, classFields, methodReturns, enclosingClass)
	case "ternary_expression":
		// (c ? ba : ba).get() — both arms agree on object type T then T.m
		// (typed local / new / factory; factories nil is fine for new/local).
		typeName = jsTernaryExprType(base, content, typedLocals, nil)
	default:
		return ""
	}
	if typeName == "" {
		return ""
	}
	if methods := methodReturns[typeName]; methods != nil {
		return methods[mname]
	}
	return ""
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
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseResolveArgType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseResolveArgTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsPromiseResolveArgTypeEx also peels Promise.resolve(new BoxA().get()) /
// Promise.resolve(ba.get()) method-return args under foreign same-leaf
// (Class peels via non-Ex path).
func jsPromiseResolveArgTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		return jsPromiseResolveArgTypeEx(arg, content, typedLocals, factories, classFields, methodReturns)
	}
	if n.Type() == "parenthesized_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return jsPromiseResolveArgTypeEx(ch, content, typedLocals, factories, classFields, methodReturns)
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
	// new T() / typed local / factory / zero-arg method-return (ba.get()).
	return jsExprLeafType(first, content, typedLocals, factories, classFields, methodReturns)
}

// jsBindForEachParams types the first parameter of array/Map/Set element
// callbacks when the receiver peels to value/element T (our receivers only):
//   - forEach: Map value / array element / Set element
//   - some / every / filter / map / find / findLast / flatMap: array element only
//
// Enables new Map([[k, new A()]]).forEach((v) => v.run()),
// [new A()].forEach((v) => v.run()), new Set([new A()]).forEach((v) => v.run()),
// [new A()].some/every/filter/map/find((v) => v.run()), and
// [new A()].flatMap((v) => … v.run() …) under foreign same-leaf methods;
// B preserved. Only the value/element slot binds (key / index / thisArg ignored).
func jsBindForEachParams(call *grammar.Node, content []byte, ourReceivers map[string]bool, out, arrayLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, extra jsExtraLocals) {
	if call == nil || call.Type() != "call_expression" || out == nil {
		return
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil {
		return
	}
	method := ingest.NodeText(prop, content)
	// Methods whose first callback param is the element/value.
	arrayOnly := false
	switch method {
	case "forEach":
		// Map / array / Set
	case "some", "every", "filter", "map", "find", "findLast", "flatMap":
		arrayOnly = true
	default:
		return
	}
	obj := ingest.ChildByField(fn, "object")
	// Map value type, array element type, or Set element type — our receivers only.
	valueT := ""
	if !arrayOnly {
		if t := jsMapSourceValueType(obj, content, out, factories, mapLocals, entryArrayLocals); ourReceivers[t] {
			valueT = t
		} else if t := jsSetSourceValueType(obj, content, arrayLocals, out, factories, setLocals); ourReceivers[t] {
			valueT = t
		}
	}
	if valueT == "" {
		if t := jsArraySourceElemType(obj, content, arrayLocals, out, factories, extra); ourReceivers[t] {
			valueT = t
		}
	}
	// Iterator.from([new A()]).forEach/some/every/find/map — yield peel when the
	// receiver is an iterator of T (not an array source).
	if valueT == "" {
		if t := jsIteratorSourceYieldType(obj, content, nil, nil, arrayLocals, out, factories, mapLocals, entryArrayLocals, setLocals); ourReceivers[t] {
			valueT = t
		}
	}
	if valueT == "" {
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
	param := jsCallbackFirstParam(cb)
	if param == nil || param.Type() != "identifier" {
		return
	}
	name := ingest.NodeText(param, content)
	if name != "" && name != "_" {
		out[name] = valueT
	}
}

// jsCallbackFirstParam returns the first formal parameter of an arrow/function
// callback (identifier, array_pattern, or TS-wrapped pattern).
func jsCallbackFirstParam(cb *grammar.Node) *grammar.Node {
	if cb == nil {
		return nil
	}
	switch cb.Type() {
	case "arrow_function":
		if p := ingest.ChildByField(cb, "parameter"); p != nil {
			return p
		}
		if p := ingest.ChildByField(cb, "parameters"); p != nil {
			return jsFirstFormalParamNode(p)
		}
		for i := uint32(0); i < cb.ChildCount(); i++ {
			ch := cb.Child(i)
			if ch.Type() == "identifier" || ch.Type() == "array_pattern" {
				return ch
			}
			if ch.Type() == "formal_parameters" {
				return jsFirstFormalParamNode(ch)
			}
		}
	case "function_expression", "function_declaration":
		if p := ingest.ChildByField(cb, "parameters"); p != nil {
			return jsFirstFormalParamNode(p)
		}
	}
	return nil
}

// jsBindPromiseThenParams types the first parameter of
// Promise.resolve(new A() / a / makeA() / ba.get()).then(x => …) / .then(function(x) { … })
// and Promise.all([new A() / ba.get()]).then(([a]) => …) array-destructure params when the
// call receiver peels to a concrete our-receiver type. Also
// Promise.allSettled([new A() / ba.get()]).then(([r]) => r.value.run()) /
// .then(([{value}]) => value.run()) settled peels. Under foreign same-leaf
// methods, only our-receiver resolved values bind so B is preserved.
// out maps param name → type leaf; settledOf maps settled-result locals → value type.
func jsBindPromiseThenParams(call *grammar.Node, content []byte, ourReceivers map[string]bool, out, settledOf, factories map[string]string, classFields, methodReturns map[string]map[string]string) {
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
	// Promise.resolve(new T() / typed local / factory / method-return) → scalar param type.
	resolveT := jsPromiseResolveArgTypeEx(obj, content, out, factories, classFields, methodReturns)
	// Promise.try(() => new T()) / Promise.try(() => a) → scalar param type.
	if resolveT == "" {
		resolveT = jsPromiseTryTypeEx(obj, content, out, factories, classFields, methodReturns)
	}
	// Promise.resolve(...).then(x => x) identity chain → scalar value type
	// (method-return via methodReturns under foreign same-leaf).
	if resolveT == "" {
		resolveT = jsPromiseThenIdentityType(obj, content, out, factories, classFields, methodReturns)
	}
	// Promise.resolve(...).finally(fn) / .catch(fn) identity stages → scalar value
	// (method-return via methodReturns under foreign same-leaf).
	if resolveT == "" {
		resolveT = jsPromiseFinallyType(obj, content, out, factories, classFields, methodReturns)
	}
	if resolveT == "" {
		resolveT = jsPromiseCatchType(obj, content, out, factories, classFields, methodReturns)
	}
	// Promise.race/any([new T() / ba.get(), …]) → scalar value type when elems agree.
	raceT := jsPromiseRaceValueTypeEx(obj, content, out, factories, classFields, methodReturns)
	// Promise.all([new T() / ba.get(), …]) → array element type for [a] destructure.
	allT := jsPromiseAllElemTypeEx(obj, content, out, factories, classFields, methodReturns)
	// Promise.allSettled([new T() / ba.get(), …]) → fulfilled value type for [r] / [{value}].
	settledT := jsPromiseAllSettledElemTypeEx(obj, content, out, factories, classFields, methodReturns)
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
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseAllElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseAllElemTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsPromiseAllElemTypeEx also peels Promise.all([new BoxA().get()]) method-return
// elements under foreign same-leaf.
func jsPromiseAllElemTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	return jsPromiseArrayElemTypeEx(n, content, typedLocals, factories, "all", classFields, methodReturns)
}

// jsPromiseAllSettledElemType recovers T from Promise.allSettled([new T(), …])
// when every array element peels to the same concrete type T. The result array
// holds settled objects {status, value}; T is the fulfilled value type (not the
// settled wrapper). Enables Promise.allSettled([new A()]).then(([r]) => r.value.run()),
// const [r] = await Promise.allSettled([new A()]); r.value.run(), and
// (await Promise.allSettled([new A()]))[0].value.run() under foreign same-leaf.
// Non-allSettled / non-array / mixed / empty args fail closed.
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseAllSettledElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseAllSettledElemTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsPromiseAllSettledElemTypeEx also peels Promise.allSettled([ba.get()])
// method-return elements under foreign same-leaf.
func jsPromiseAllSettledElemTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	return jsPromiseArrayElemTypeEx(n, content, typedLocals, factories, "allSettled", classFields, methodReturns)
}

// jsPromiseRaceValueType recovers T from Promise.race([new T(), …]) /
// Promise.any([new T(), …]) when every array element peels to the same T.
// The settled value is T (scalar), not an array — unlike Promise.all.
// Enables Promise.race([new A()]).then(a => a.run()) and
// (await Promise.race([new A()])).run() under foreign same-leaf methods.
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseRaceValueType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseRaceValueTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsPromiseRaceValueTypeEx also peels Promise.race/any([new BoxA().get()])
// method-return elements under foreign same-leaf.
func jsPromiseRaceValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if t := jsPromiseArrayElemTypeEx(n, content, typedLocals, factories, "race", classFields, methodReturns); t != "" {
		return t
	}
	return jsPromiseArrayElemTypeEx(n, content, typedLocals, factories, "any", classFields, methodReturns)
}

// jsPromiseArrayElemType recovers the uniform element type of
// Promise.<method>([…]) for method in {all, race, any, allSettled}.
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseArrayElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, method string) string {
	return jsPromiseArrayElemTypeEx(n, content, typedLocals, factories, method, nil, nil)
}

// jsPromiseArrayElemTypeEx also peels method-return array elements
// (new BoxA().get() / ba.get()) under foreign same-leaf.
func jsPromiseArrayElemTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, method string, classFields, methodReturns map[string]map[string]string) string {
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
		return jsPromiseArrayElemTypeEx(arg, content, typedLocals, factories, method, classFields, methodReturns)
	}
	if n.Type() == "parenthesized_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return jsPromiseArrayElemTypeEx(ch, content, typedLocals, factories, method, classFields, methodReturns)
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
	// Promise.resolve(new T() / ba.get()) is accepted as an element of
	// race/any/all (thenable of T) — same leaf as bare new T() under foreign same-leaf.
	found := ""
	saw := false
	for i := uint32(0); i < first.ChildCount(); i++ {
		el := first.Child(i)
		if el == nil || el.Type() == "[" || el.Type() == "]" || el.Type() == "," {
			continue
		}
		t := jsExprLeafType(el, content, typedLocals, factories, classFields, methodReturns)
		if t == "" {
			t = jsPromiseResolveArgTypeEx(el, content, typedLocals, factories, classFields, methodReturns)
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

// jsPromiseAllSubscriptType recovers T from (await Promise.all([new T()]))[i]
// when the indexed object peels to a uniform Promise.all element type. Index
// must be a numeric literal (any index; elems already agree).
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseAllSubscriptType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseAllSubscriptTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsPromiseAllSubscriptTypeEx also peels (await Promise.all([ba.get()]))[i]
// method-return elements under foreign same-leaf.
func jsPromiseAllSubscriptTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	return jsPromiseAllElemTypeEx(obj, content, typedLocals, factories, classFields, methodReturns)
}

// jsPromiseAllSettledSubscriptType recovers T from
// (await Promise.allSettled([new T()]))[i] when the indexed object peels to a
// uniform allSettled value type. Index must be a numeric literal.
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseAllSettledSubscriptType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseAllSettledSubscriptTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsPromiseAllSettledSubscriptTypeEx also peels method-return allSettled elems.
func jsPromiseAllSettledSubscriptTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	return jsPromiseAllSettledElemTypeEx(obj, content, typedLocals, factories, classFields, methodReturns)
}

// jsPromiseAllSettledValueType recovers T from r.value /
// (await Promise.allSettled([new T()]))[i].value when the object peels to a
// settled result of fulfilled value type T. Property must be bare "value".
// Class()/local/factory only — method-return peels via Ex.
func jsPromiseAllSettledValueType(n *grammar.Node, content []byte, typedLocals, settledOf, factories map[string]string) string {
	return jsPromiseAllSettledValueTypeEx(n, content, typedLocals, settledOf, factories, nil, nil)
}

// jsPromiseAllSettledValueTypeEx also peels method-return allSettled value.
func jsPromiseAllSettledValueTypeEx(n *grammar.Node, content []byte, typedLocals, settledOf, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	// (await Promise.allSettled([new A() / ba.get()]))[0].value
	if t := jsPromiseAllSettledSubscriptTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
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

// jsExprConcreteType peels new T() / typed local / factory call / ternary to a class leaf.
func jsExprConcreteType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsExprLeafType(n, content, typedLocals, factories, nil, nil)
}

// jsExprLeafType peels new T() / typed local / factory / ternary / zero-arg method
// return (ba.get() → A) under foreign same-leaf. methodReturns/classFields may be
// nil (Class()/local/factory only — same as historical jsExprConcreteType).
func jsExprLeafType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			return jsExprLeafType(ch, content, typedLocals, factories, classFields, methodReturns)
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
		if t := jsFactoryCallReturnType(n, content, factories); t != "" {
			return t
		}
		// ba.get() / new BoxA().get() — zero-arg product method return.
		return jsMethodCallReturnType(n, content, typedLocals, classFields, methodReturns, "")
	}
	if n.Type() == "ternary_expression" {
		return jsTernaryExprLeafType(n, content, typedLocals, factories, classFields, methodReturns)
	}
	return ""
}

// jsTernaryExprLeafType is jsTernaryExprType with method-return arms.
func jsTernaryExprLeafType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil || n.Type() != "ternary_expression" {
		return ""
	}
	cons := ingest.ChildByField(n, "consequence")
	alt := ingest.ChildByField(n, "alternative")
	t1 := jsExprLeafType(cons, content, typedLocals, factories, classFields, methodReturns)
	t2 := jsExprLeafType(alt, content, typedLocals, factories, classFields, methodReturns)
	if t1 != "" && t1 == t2 {
		return t1
	}
	if t1 != "" && jsIsNullUndefinedLiteral(alt, content) {
		return t1
	}
	if t2 != "" && jsIsNullUndefinedLiteral(cons, content) {
		return t2
	}
	return ""
}

// jsTernaryExprType recovers T from (c ? a : b) when both arms peel to the same
// Class leaf (new T() / typed local / factory / nested ternary), or one arm peels
// to T and the other is null/undefined. Enables (c ? new A() : null).run() /
// const a = c ? new A() : null under foreign same-leaf. Mixed fail closed.
func jsTernaryExprType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	if n == nil || n.Type() != "ternary_expression" {
		return ""
	}
	cons := ingest.ChildByField(n, "consequence")
	alt := ingest.ChildByField(n, "alternative")
	t1 := jsExprConcreteType(cons, content, typedLocals, factories)
	t2 := jsExprConcreteType(alt, content, typedLocals, factories)
	if t1 != "" && t1 == t2 {
		return t1
	}
	if t1 != "" && jsIsNullUndefinedLiteral(alt, content) {
		return t1
	}
	if t2 != "" && jsIsNullUndefinedLiteral(cons, content) {
		return t2
	}
	return ""
}

// jsBindObjectPatternUniformValues binds each simple property binding in an
// object_pattern to the same value type T when the RHS is a uniform-value object
// (object literal / spread / objValue local). Shorthand {k} and {k: a} both bind.
// Nested / rest / default patterns fail closed for that slot only.
func jsBindObjectPatternUniformValues(pattern *grammar.Node, content []byte, valueType string, out map[string]string) {
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
			// {a} — local name is property name.
			name := ingest.NodeText(ch, content)
			if name != "" && name != "_" {
				out[name] = valueType
			}
		case "pair_pattern":
			// {k: a}
			val := ingest.ChildByField(ch, "value")
			if val != nil && val.Type() == "identifier" {
				name := ingest.NodeText(val, content)
				if name != "" && name != "_" {
					out[name] = valueType
				}
			}
		}
		// rest_pattern / nested object_pattern / assignment_pattern — skip slot.
	}
}

// jsObjectSpreadValueType recovers T from {...{k: new T()}} / {...oa} when every
// spread source peels to the same uniform property value type T. Non-spread
// properties / mixed leaves / empty fail closed.
func jsObjectSpreadValueType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsObjectSpreadValueTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsObjectSpreadValueTypeEx peels {...{k: new BoxA().get()}} method-return
// property values under foreign same-leaf (Class peels via non-Ex path).
func jsObjectSpreadValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if n == nil || n.Type() != "object" {
		return ""
	}
	extra := jsExtraLocals{classFields: classFields, methodReturns: methodReturns}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil || ch.Type() == "{" || ch.Type() == "}" || ch.Type() == "," {
			continue
		}
		if ch.Type() != "spread_element" {
			// Non-spread property — fail closed (mixed shape).
			return ""
		}
		// spread_element: "..." + expression
		var arg *grammar.Node
		for j := uint32(0); j < ch.ChildCount(); j++ {
			c := ch.Child(j)
			if c == nil || c.Type() == "..." {
				continue
			}
			arg = c
			break
		}
		if arg == nil {
			return ""
		}
		t := ""
		if arg.Type() == "object" {
			t = jsObjectLiteralValueTypeEx(arg, content, typedLocals, factories, extra)
		} else if arg.Type() == "identifier" {
			if typedLocals != nil {
				// plain typed local is not object-of-T; prefer objValueLocals.
			}
			if objValueLocals != nil {
				t = objValueLocals[ingest.NodeText(arg, content)]
			}
		}
		if t == "" {
			// Also peel nested spreads / Object.assign sources.
			if arg.Type() == "object" {
				t = jsObjectSpreadValueTypeEx(arg, content, typedLocals, factories, objValueLocals, classFields, methodReturns)
			}
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

// jsObjectSpreadPropType recovers T from {...src}.prop / {...src}["k"] when the
// object peels via jsObjectSpreadValueType to uniform value T. Property key shape
// is free (any identifier / string); value type is T for all keys when uniform.
func jsObjectSpreadPropType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsObjectSpreadPropTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsObjectSpreadPropTypeEx peels {...{k: ba.get()}}.k method-return props.
func jsObjectSpreadPropTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
		return ""
	}
	var obj *grammar.Node
	switch n.Type() {
	case "member_expression", "member_expression_optional", "optional_chain":
		obj = ingest.ChildByField(n, "object")
	case "subscript_expression":
		obj = ingest.ChildByField(n, "object")
	default:
		return ""
	}
	return jsObjectSpreadValueTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns)
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
// recovered from body-only `yield new T()` / `yield new BoxA().get()` /
// `yield x` after `x = new T()` / `x = new BoxA().get()`.
// function* genA(){ yield new A() } / async function* agenA(){ yield new A() } /
// const genA = function*(){ yield new A() }. Mixed/non-leaf yields and yield*
// fail closed. Same-file name → last wins.
// classFields/methodReturns peel method-return yields under foreign same-leaf.
func jsSameFileGeneratorYields(root *grammar.Node, content []byte, classFields, methodReturns map[string]map[string]string) map[string]string {
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
					if t := jsFuncBodyYieldNew(n, content, classFields, methodReturns); t != "" {
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
					if t := jsFuncBodyYieldNew(valN, content, classFields, methodReturns); t != "" {
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

// jsFuncBodyYieldNew recovers T when every yield in fn is `yield new T()` /
// `yield new BoxA().get()` / `yield x` after a local `x = new T()` or
// `x = new BoxA().get()` assignment. Nested function/class bodies are skipped.
// yield* / zero / mixed / non-leaf yields fail closed.
// methodReturns peels product method-return yields under foreign same-leaf.
func jsFuncBodyYieldNew(fn *grammar.Node, content []byte, classFields, methodReturns map[string]map[string]string) string {
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
				// new T() / new BoxA().get() / factories — leaf for yield x.
				if t := jsExprLeafType(valN, content, localCtor, nil, classFields, methodReturns); t != "" {
					localCtor[ingest.NodeText(nameN, content)] = t
				}
			}
		case "yield_expression":
			// yield new T() / yield new BoxA().get() / yield x — reject yield*.
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
				t = jsExprLeafType(expr, content, localCtor, nil, classFields, methodReturns)
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

// jsUniformArrayElemType recovers T from [new T() / a / makeT() / ba.get(), …]
// when every element peels to the same concrete type T (Class()/local/factory/
// zero-arg method return via extra.methodReturns). Empty / mixed arrays fail closed.
func jsUniformArrayElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsUniformArrayElemTypeEx(n, content, typedLocals, factories, jsExtraLocals{})
}

// jsUniformArrayElemTypeEx is jsUniformArrayElemType with method-return peels.
func jsUniformArrayElemTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		t := jsExprLeafType(el, content, typedLocals, factories, extra.classFields, extra.methodReturns)
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

// jsExtraLocals carries object/group peel maps threaded through array-source peels.
// Nil maps fail closed. Used for Object.values(assignLocal), Object.groupBy locals,
// and Map.groupBy peels under foreign same-leaf methods.
// groupBy also stores Object.values(groupBy) locals under "@values.<name>" so
// va[i][0].run() peels after const va = Object.values(ga) (va[i] is T[]).
// mapLocals / setLocals / entryArray enable map.values() / set sources as
// array-element peels ([...ma.values()][0] / Array.from(xs) / …).
type jsExtraLocals struct {
	objValue         map[string]string // Object.fromEntries / Object.assign → value T
	groupBy          map[string]string // Object.groupBy result → element T; @values.name → T
	groupMap         map[string]string // Map.groupBy result → element T
	groupEntry       map[string]string // [key, T[]] pair local from groupBy entries → T
	groupEntryArray  map[string]string // Object.entries(groupBy) / ma.entries() local → T
	mapLocals        map[string]string // Map local → value T (ma.set / new Map)
	setLocals        map[string]string // Set local → element T (xs.add / new Set)
	entryArrayLocals map[string]string // pair-array local for new Map(pa)
	// classFields / methodReturns peel ba.get() / new BoxA().get() as array/object
	// element leaves under foreign same-leaf (jsExprConcreteType alone is Class()-only).
	classFields   map[string]map[string]string
	methodReturns map[string]map[string]string
}

// jsArraySourceElemType recovers T from an array-like expression whose elements
// uniformly peel to T: array literal, array local, spread-array identity
// ([...arr] / [...arr, new T()]), Array.from([…]) (no mapfn), Array.of(…),
// Object.values({…}) when all property values agree, identity array methods
// (slice / concat / toSpliced / toReversed / toSorted / with / flat / filter /
// identity map), map.values() (Map value iterator), or Set sources
// (new Set / set local / set.values()/keys()) when dual-class solid.
// Object.entries is not an element source of T (yields [key, value] pairs).
// Bare Map locals are not element sources of T (iterate [k,v] pairs).
func jsArraySourceElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if n == nil {
		return ""
	}
	// Unwrap (await Array.fromAsync(...)) / (arr) so await peels match call forms.
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
	if n == nil {
		return ""
	}
	if t := jsUniformArrayElemTypeEx(n, content, typedLocals, factories, extra); t != "" {
		return t
	}
	if t := jsArraySpreadElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if n.Type() == "identifier" && arrayLocals != nil {
		if t := arrayLocals[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	// aa[0] when aa: [[new A()]] (@nested) — expression is array of A (not A).
	// Enables aa[0][0].run() / [[new A()]][0][0].run() under foreign same-leaf.
	if t := jsNestedArrayIndexSourceElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if t := jsArrayFromElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if t := jsArrayOfElemType(n, content, typedLocals, factories, extra); t != "" {
		return t
	}
	if t := jsObjectValuesElemType(n, content, typedLocals, factories, extra); t != "" {
		return t
	}
	if t := jsObjectGroupByGroupArrayType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		// Object.groupBy(arr, fn)[key] / Object.values(Object.groupBy(arr, fn))[i]
		// — group is array of T (element type of arr).
		return t
	}
	if t := jsMapGroupByGroupArrayType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		// Map.groupBy(arr, fn).get(k) / ma.get(k) — group is array of T.
		return t
	}
	if t := jsMapGroupByValuesNextValueGroupElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		// ma.values().next().value / Map.groupBy(...).values().next().value —
		// next group array of T (not scalar T).
		return t
	}
	if t := jsGroupByEntriesPairGroupArrayType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		// Object.entries(groupBy)[i][1] / [...Map.groupBy.entries()][i][1] —
		// pair value is group array of T (not scalar T).
		return t
	}
	if t := jsArrayIdentityElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if t := jsArrayFlatElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// as.values() / as[Symbol.iterator]() — array iterator as element source of T
	// when as peels via arrayLocals / array literal (not Map/Set — those below).
	if t := jsArrayIteratorYieldType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// ma.values() / new Map([[k, new A()]]).values() — Map value iterator as
	// array element source of T. Enables [...ma.values()][0].run() /
	// Array.from(ma.values())[0].run() / [...ma.values()].at(0).run() /
	// ma.values().forEach(a => a.run()) under foreign same-leaf (including
	// after ma.set(k, new A()) / new Map([[k, new BoxA().get()]])).
	// Bare Map is not an element source of T.
	if t := jsMapValuesYieldTypeEx(n, content, typedLocals, factories, extra.mapLocals, extra.entryArrayLocals, extra.classFields, extra.methodReturns); t != "" {
		return t
	}
	// xs.values() / xs.keys() / xs[Symbol.iterator]() — Set iterators yield T
	// (method-return via extra: new Set([ba.get()]).keys()).
	if t := jsSetIteratorYieldTypeEx(n, content, arrayLocals, typedLocals, factories, extra.setLocals, extra); t != "" {
		return t
	}
	// new Set([new A()]) / set local — Array.from(xs) / [...xs] element source.
	// Bare Map intentionally excluded (entries, not values).
	if t := jsSetSourceValueTypeEx(n, content, arrayLocals, typedLocals, factories, extra.setLocals, extra); t != "" {
		return t
	}
	// structuredClone(arr) / structuredClone([new A()]) — identity of array source.
	if t := jsStructuredCloneArrayElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// iter.take(n).toArray() / iter.toArray() — materialize iterator of T as array.
	if t := jsIteratorToArrayElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// arr.values().take(n) / .drop(n) / .filter(pred) / .flatMap(x => [x]) as
	// array source of T (spread / Array.from / toArray of iterator helpers),
	// including Iterator.from([new BoxA().get()]).flatMap(...).toArray().
	if t := jsIteratorHelperYieldType(n, content, nil, nil, arrayLocals, typedLocals, factories, extra.mapLocals, extra.entryArrayLocals, extra.setLocals, extra.classFields, extra.methodReturns); t != "" {
		return t
	}
	return ""
}

// jsObjectGroupByGroupArrayType recovers T from a group-array expression whose
// elements are the Object.groupBy iterable's element type:
//
//	Object.groupBy(arr, fn)[key] / Object.groupBy(arr, fn).key
//	Object.values(Object.groupBy(arr, fn))[i]
//	Object.fromEntries(Object.entries(groupBy))[key] / .key
//
// Key / index ignored (all groups are T[]). Callback ignored for typing.
// Enables Object.groupBy([new A()], x => "k")["k"][0].run() and
// Object.values(Object.groupBy([new A()], x => "k"))[0][0].run() under foreign
// same-leaf methods. Unknown iterable / non-groupBy fail closed.
func jsObjectGroupByGroupArrayType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	var obj *grammar.Node
	switch n.Type() {
	case "subscript_expression":
		if ingest.ChildByField(n, "index") == nil {
			return ""
		}
		obj = ingest.ChildByField(n, "object")
	case "member_expression", "member_expression_optional", "optional_chain":
		prop := ingest.ChildByField(n, "property")
		if prop == nil || (prop.Type() != "property_identifier" && prop.Type() != "identifier") {
			return ""
		}
		obj = ingest.ChildByField(n, "object")
	default:
		return ""
	}
	if obj == nil {
		return ""
	}
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
	// Object.groupBy(arr, fn)[key] / .key
	if t := jsObjectGroupByElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// ga[key] / ga.key after const ga = Object.groupBy(arr, fn) /
	// after const ga = Object.fromEntries(Object.entries(groupBy))
	if obj.Type() == "identifier" && extra.groupBy != nil {
		if t := extra.groupBy[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
		// va[i] after const va = Object.values(ga) / Object.values(Object.groupBy(...))
		// — va[i] is group array of T (same as Object.values(ga)[i]).
		if t := extra.groupBy["@values."+ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	// Object.fromEntries(Object.entries(ga))[key] / .key
	if t := jsObjectFromEntriesGroupByElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Object.values(Object.groupBy(arr, fn))[i]
	return jsObjectValuesGroupByElemType(obj, content, arrayLocals, typedLocals, factories, extra)
}

// jsObjectGroupByElemType recovers T from Object.groupBy(iterable, keyfn) when
// iterable peels to uniform element type T. Keyfn ignored (not type-changing).
// Result is Record of T[] — use jsObjectGroupByGroupArrayType for group access.
func jsObjectGroupByElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != "groupBy" {
		return ""
	}
	// Object.groupBy(iterable, keyfn) — require ≥1 arg; keyfn optional for typing.
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
	if count < 1 || first == nil {
		return ""
	}
	return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories, extra)
}

// jsMapGroupByElemType recovers T from Map.groupBy(iterable, keyfn) when
// iterable peels to uniform element type T. Keyfn ignored (not type-changing).
// Result is Map of T[] — use jsMapGroupByGroupArrayType for .get(k) group access.
func jsMapGroupByElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Map" ||
		ingest.NodeText(prop, content) != "groupBy" {
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
	if count < 1 || first == nil {
		return ""
	}
	return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories, extra)
}

// jsMapGroupByGroupArrayType recovers T from a group-array expression whose
// elements are the Map.groupBy iterable's element type:
//
//	Map.groupBy(arr, fn).get(k)
//	ma.get(k) after const ma = Map.groupBy(arr, fn)
//
// Key ignored (all groups are T[]). Enables
// Map.groupBy([new A()], x => "k").get("k")[0].run() under foreign same-leaf
// methods. Unknown iterable / non-groupBy fail closed.
func jsMapGroupByGroupArrayType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "get" {
		return ""
	}
	// Require ≥1 arg for .get(k).
	argCount := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		argCount++
	}
	if argCount < 1 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
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
	// Map.groupBy(arr, fn).get(k)
	if t := jsMapGroupByElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// ma.get(k) after const ma = Map.groupBy(...)
	if obj.Type() == "identifier" && extra.groupMap != nil {
		if t := extra.groupMap[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsGroupByEntriesPairGroupArrayType recovers T from a group-array expression
// that is the value slot of an entries pair over Object.groupBy / Map.groupBy
// results (pair values are T[], not T):
//
//	Object.entries(Object.groupBy(arr, fn))[i][1]
//	Object.entries(ga)[i][1] after const ga = Object.groupBy(...)
//	[...Map.groupBy(arr, fn).entries()][i][1]
//	[...ma.entries()][i][1] after const ma = Map.groupBy(...)
//	e[1] after for (const e of Object.entries(ga)) / ma.entries()
//	Map.groupBy(...).entries().next().value[1] / ma.entries().next().value[1]
//
// Enables Object.entries(ga)[0][1][0].run() and [...ma.entries()][0][1][0].run()
// under foreign same-leaf methods. Scalar Object.entries / Map.entries (value T)
// stay on the pair-value path; this path is group arrays only. Unknown /
// non-groupBy fail closed.
func jsGroupByEntriesPairGroupArrayType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" || ingest.NodeText(idx, content) != "1" {
		return ""
	}
	pair := ingest.ChildByField(n, "object")
	for pair != nil && pair.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < pair.ChildCount(); i++ {
			ch := pair.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		pair = inner
	}
	if pair == nil {
		return ""
	}
	// e[1] after for (const e of Object.entries(ga)) / const e = ma.entries().next().value
	if pair.Type() == "identifier" && extra.groupEntry != nil {
		if t := extra.groupEntry[ingest.NodeText(pair, content)]; t != "" {
			return t
		}
	}
	// Map.groupBy(...).entries().next().value[1] / ma.entries().next().value[1]
	if t := jsGroupByEntriesNextPairElemType(pair, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Object.entries(ga)[i][1] / [...ma.entries()][i][1]
	if pair.Type() != "subscript_expression" {
		return ""
	}
	pidx := ingest.ChildByField(pair, "index")
	if pidx == nil || pidx.Type() != "number" {
		return ""
	}
	return jsGroupByEntriesSourceElemType(ingest.ChildByField(pair, "object"), content, arrayLocals, typedLocals, factories, extra)
}

// jsGroupByEntriesSourceElemType recovers T from an expression that yields
// [key, T[]] pairs over a groupBy result: Object.entries(groupBy),
// Map.groupBy(...).entries() / ma.entries(), a groupEntryArray local, or a
// single-spread copy [...Object.entries(groupBy)] / [...ma.entries()].
func jsGroupByEntriesSourceElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	// const es = Object.entries(ga) / const ie = ma.entries()
	if n.Type() == "identifier" && extra.groupEntryArray != nil {
		if t := extra.groupEntryArray[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	// Object.entries(Object.groupBy(...)) / Object.entries(ga)
	if first := jsObjectStaticCallSingleArg(n, content, "entries"); first != nil {
		if t := jsObjectGroupByElemType(first, content, arrayLocals, typedLocals, factories, extra); t != "" {
			return t
		}
		if first.Type() == "identifier" && extra.groupBy != nil {
			if t := extra.groupBy[ingest.NodeText(first, content)]; t != "" {
				return t
			}
		}
		return ""
	}
	// Map.groupBy(...).entries() / ma.entries() — zero-arg only.
	if n.Type() == "call_expression" && jsCallIsZeroArg(n) {
		fn := ingest.ChildByField(n, "function")
		if fn != nil && (fn.Type() == "member_expression" || fn.Type() == "member_expression_optional" || fn.Type() == "optional_chain") {
			prop := ingest.ChildByField(fn, "property")
			if prop != nil && ingest.NodeText(prop, content) == "entries" {
				obj := ingest.ChildByField(fn, "object")
				// Object.entries handled above — fail closed here.
				if obj != nil && obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
					return ""
				}
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
				if t := jsMapGroupByElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
					return t
				}
				if obj != nil && obj.Type() == "identifier" && extra.groupMap != nil {
					if t := extra.groupMap[ingest.NodeText(obj, content)]; t != "" {
						return t
					}
				}
			}
		}
	}
	// [...Object.entries(groupBy)] / [...ma.entries()] — single spread.
	if n.Type() == "array" {
		var spreadArg *grammar.Node
		count := 0
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
				continue
			}
			count++
			if ch.Type() != "spread_element" {
				return ""
			}
			var arg *grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				c := ch.Child(j)
				if c == nil || c.Type() == "..." {
					continue
				}
				arg = c
				break
			}
			if arg == nil {
				return ""
			}
			spreadArg = arg
		}
		if count != 1 || spreadArg == nil {
			return ""
		}
		return jsGroupByEntriesSourceElemType(spreadArg, content, arrayLocals, typedLocals, factories, extra)
	}
	return ""
}

// jsGroupByEntriesIterableElemType recovers T from an iterable of [key, T[]]
// pairs over a groupBy result — used for for-of binding (pair local or
// destructured group array). Covers Object.entries(groupBy), Map.groupBy
// .entries() / ma.entries(), spread copies, and Map.groupBy / ma default
// iterators (Map yields entries). Scalar entries sources fail closed.
func jsGroupByEntriesIterableElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if t := jsGroupByEntriesSourceElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Map.groupBy(arr, fn) / ma after const ma = Map.groupBy(...) —
	// default iterator yields [key, T[]] entries.
	if t := jsMapGroupByElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if n != nil && n.Type() == "identifier" && extra.groupMap != nil {
		if t := extra.groupMap[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsGroupByEntriesPairSubscriptElemType recovers T from a single entries-pair
// subscript over a groupBy entries source: Object.entries(ga)[i] /
// [...ma.entries()][i]. The expression is a [key, T[]] pair (not T) — use for
// groupEntryLocals binding so e[1][0] peels.
func jsGroupByEntriesPairSubscriptElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	return jsGroupByEntriesSourceElemType(ingest.ChildByField(n, "object"), content, arrayLocals, typedLocals, factories, extra)
}

// jsGroupByEntriesNextPairElemType recovers T from a groupBy-entries iterator
// next().value pair expression (pair value is T[], not T):
//
//	Map.groupBy(arr, fn).entries().next().value
//	ma.entries().next().value after const ma = Map.groupBy(...)
//
// Enables …next().value[1][0].run() under foreign same-leaf methods.
// Non-groupBy entries iterators fail closed.
func jsGroupByEntriesNextPairElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	// …entries().next().value — require zero-arg next() on a groupBy entries source.
	if obj == nil || obj.Type() != "call_expression" || !jsCallIsZeroArg(obj) {
		return ""
	}
	fn := ingest.ChildByField(obj, "function")
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	nprop := ingest.ChildByField(fn, "property")
	if nprop == nil || ingest.NodeText(nprop, content) != "next" {
		return ""
	}
	return jsGroupByEntriesSourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
}

// jsObjectFromEntriesGroupByElemType recovers T from Object.fromEntries(iterable)
// when iterable peels to [key, T[]] pairs over a groupBy result:
//
//	Object.entries(groupBy) / Map.groupBy.entries / spread of those
//	bare Map.groupBy(...) / ma after const ma = Map.groupBy(...) — Map is
//	iterable of [key, T[]] pairs (same shape as .entries())
//
// Result is Record of T[] — same shape as Object.groupBy — so property access
// peels via jsObjectGroupByGroupArrayType. Scalar fromEntries (value T) stays
// on the objValue path. Unknown fail closed.
func jsObjectFromEntriesGroupByElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	first := jsObjectStaticCallSingleArg(n, content, "fromEntries")
	if first == nil {
		return ""
	}
	// Object.entries(ga) / ma.entries() / [...ma.entries()]
	if t := jsGroupByEntriesSourceElemType(first, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Object.fromEntries(Map.groupBy(arr, fn)) — Map iterates [key, T[]] pairs.
	if t := jsMapGroupByElemType(first, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Object.fromEntries(ma) after const ma = Map.groupBy(...)
	if first.Type() == "identifier" && extra.groupMap != nil {
		if t := extra.groupMap[ingest.NodeText(first, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsMapGroupByValuesNextValueGroupElemType recovers T from a group-array
// expression that is the .value of Map.groupBy values-iterator .next():
//
//	Map.groupBy(arr, fn).values().next().value
//	ma.values().next().value after const ma = Map.groupBy(...)
//
// Yields group arrays T[] (not scalar T) so [0].run() peels via
// jsArraySourceElemType. Zero-arg values/next only. Non-groupBy fail closed.
func jsMapGroupByValuesNextValueGroupElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	// .value on a .next() result
	if n == nil || (n.Type() != "member_expression" && n.Type() != "member_expression_optional" && n.Type() != "optional_chain") {
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
	// .next() zero-arg on values() iterator
	if obj == nil || obj.Type() != "call_expression" || !jsCallIsZeroArg(obj) {
		return ""
	}
	fn := ingest.ChildByField(obj, "function")
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	nprop := ingest.ChildByField(fn, "property")
	if nprop == nil || ingest.NodeText(nprop, content) != "next" {
		return ""
	}
	return jsMapGroupByValuesElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
}

// jsMapGroupByValuesElemType recovers T from Map.groupBy(...).values() /
// ma.values() after const ma = Map.groupBy(...) when the groupBy iterable peels
// to T. Yields group arrays T[] (not T) — use for for-of arrayLocals binding
// so g[0].run() peels. Zero-arg only. Non-groupBy maps fail closed.
func jsMapGroupByValuesElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "values" {
		return ""
	}
	// Object.values is handled elsewhere — fail closed here.
	obj := ingest.ChildByField(fn, "object")
	if obj != nil && obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
		return ""
	}
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
	if t := jsMapGroupByElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if obj != nil && obj.Type() == "identifier" && extra.groupMap != nil {
		if t := extra.groupMap[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsObjectValuesGroupByElemType recovers T from Object.values(Object.groupBy(…))
// / Object.values(ga) after const ga = Object.groupBy(...) /
// Object.values({k: [new A()]}) / Object.values(oa) after const oa = {k: [new A()]}
// when group property values are arrays of T. Object.values yields T[][] —
// callers use [i] group access via jsObjectGroupByGroupArrayType, or for-of
// binding of group arrays.
func jsObjectValuesGroupByElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != "values" {
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
	// Object.values(Object.groupBy(arr, fn))
	if t := jsObjectGroupByElemType(first, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Object.values({k: [new A()], …}) — property values are arrays of T.
	if t := jsObjectLiteralNestedArrayElemType(first, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Object.values(ga) after const ga = Object.groupBy(...) /
	// after const ga = Object.fromEntries(Object.entries(groupBy)) /
	// after const oa = {k: [new A()]} (groupByLocals nested-array object).
	if first.Type() == "identifier" && extra.groupBy != nil {
		if t := extra.groupBy[ingest.NodeText(first, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsObjectLiteralNestedArrayElemType recovers T from an object literal whose every
// property value is an array of uniform element type T:
//
//	{k: [new A()]} / {k: [new A()], j: [new A()]} → "A"
//
// Enables Object.values({k: [new A()]})[0][0].run() and const oa = {k: [new A()]};
// Object.values(oa)[0][0].run() under foreign same-leaf (groupBy-like peels).
// Scalar property values / mixed / empty / methods / spreads fail closed.
func jsObjectLiteralNestedArrayElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if n == nil || n.Type() != "object" {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil || ch.Type() == "{" || ch.Type() == "}" || ch.Type() == "," {
			continue
		}
		if ch.Type() != "pair" {
			// method / spread / shorthand — fail closed (not array-of-T groups).
			return ""
		}
		val := ingest.ChildByField(ch, "value")
		if val == nil {
			return ""
		}
		// Property value must be an array source of T (literal [new A()] / as local).
		t := jsArraySourceElemType(val, content, arrayLocals, typedLocals, factories, extra)
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

// jsArraySpreadElemType recovers T from an array literal that includes spread
// elements when every element is either spread of an array-source of T or a
// concrete expression of type T. Enables [...[new A()]][0].run() /
// [...as][0].run() / [...[new A()], new A()][0].run() /
// [...[], new A()][0].run() under foreign same-leaf methods.
// Empty array spreads are wildcards; mixed / non-array spreads fail closed.
// selfTarget is empty for expression peels; see jsArraySpreadAssignElemType
// for assignment self-target untyped arms.
func jsArraySpreadElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	return jsArraySpreadElemTypeSelf(n, content, arrayLocals, typedLocals, factories, extra, "")
}

// jsArraySpreadElemTypeSelf is jsArraySpreadElemType with optional selfTarget:
// when selfTarget is set (assignment `xs = [...xs, new A()]`), an untyped
// identifier spread equal to selfTarget is a wildcard (same as empty []).
func jsArraySpreadElemTypeSelf(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals, selfTarget string) string {
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
	sawSpread := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		el := n.Child(i)
		if el == nil || el.Type() == "[" || el.Type() == "]" || el.Type() == "," {
			continue
		}
		var t string
		if el.Type() == "spread_element" {
			sawSpread = true
			// spread_element: "..." + expression (first non-"..." child).
			var arg *grammar.Node
			for j := uint32(0); j < el.ChildCount(); j++ {
				ch := el.Child(j)
				if ch == nil || ch.Type() == "..." {
					continue
				}
				arg = ch
				break
			}
			if arg == nil {
				return ""
			}
			// Empty [] spread is a wildcard ([...[], new A()]).
			if jsIsEmptyArrayLiteral(arg) {
				continue
			}
			// xs = [...xs, new A()] — untyped self spread is wildcard.
			if selfTarget != "" && arg.Type() == "identifier" {
				name := ingest.NodeText(arg, content)
				if name == selfTarget && (arrayLocals == nil || arrayLocals[name] == "") {
					continue
				}
			}
			t = jsArraySourceElemType(arg, content, arrayLocals, typedLocals, factories, extra)
		} else {
			// Class() / typed local / factory / method-return (new BoxA().get()).
			t = jsExprLeafType(el, content, typedLocals, factories, extra.classFields, extra.methodReturns)
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
	if !saw || !sawSpread {
		// No elements, or no spreads (pure literals handled by jsUniformArrayElemType).
		return ""
	}
	return found
}

// jsArrayIdentityElemType recovers T from arr.slice(…) / arr.concat(…) /
// arr.toSpliced(…) / arr.toReversed() / arr.toSorted(…) / arr.copyWithin(…) /
// arr.fill(val) / arr.with(i, val) / arr.filter(pred) / arr.map(x => x) /
// arr.flatMap(x => [x]) when the receiver peels to uniform element type T and
// any inserted/concatenated/replaced values also peel to T. fill is value-typed
// (overwrite; prior receiver content discarded).
// Enables [new A()].slice()[0].run() / as.concat([new A()])[0].run() /
// [new A()].toSpliced(0, 0)[0].run() / [new A()].toReversed()[0].run() /
// [new A()].toSorted()[0].run() / [new A()].copyWithin(0)[0].run() /
// [null].fill(new A())[0].run() / [new A()].with(0, new A())[0].run() /
// [new A()].filter(pred)[0].run() / [new A()].map(x => x)[0].run() /
// [new A()].flatMap(x => [x])[0].run() under foreign same-leaf methods.
// Mixed concat/toSpliced/with inserts and non-identity map/flatMap fail closed.
// filter predicate body ignored (not type-changing for uniform arrays).
func jsArrayIdentityElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil {
		return ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil {
		return ""
	}
	name := ingest.NodeText(prop, content)
	obj := ingest.ChildByField(fn, "object")
	switch name {
	case "slice", "toReversed", "toSorted", "copyWithin":
		// slice / toReversed / toSorted / copyWithin yield elements of the same
		// type as the receiver (comparator on toSorted / copyWithin indices
		// ignored; not type-changing).
		return jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
	case "filter":
		// arr.filter(pred) — same element type as receiver; predicate ignored
		// (not type-changing for uniform arrays). ≥1 positional arg required.
		if args == nil {
			return ""
		}
		count := 0
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			count++
		}
		if count < 1 {
			return ""
		}
		return jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
	case "map":
		// arr.map(fn) only when fn is identity of its first param (x => x).
		// Non-identity map would transform type — fail closed.
		if args == nil {
			return ""
		}
		var cb *grammar.Node
		count := 0
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			count++
			if cb == nil {
				cb = ch
			}
		}
		if count < 1 || cb == nil || !jsIsIdentityCallback(cb, content) {
			return ""
		}
		return jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
	case "flatMap":
		// arr.flatMap(fn):
		//   - x => [x] on array of T — one-level flatten of [T] yields T
		//   - xs => xs on nested array of T (const aa = [[new A()]]) — identity
		//     flatten one level (same leaf as aa.flat()[0])
		//   - () => [new T()] / (_v) => [new T()] — sole return array of uniform T
		//     (same leaf as Array.from({length}, () => new T()); receiver ignored)
		// Non-identity / unknown receivers fail closed.
		if args == nil {
			return ""
		}
		var cb *grammar.Node
		count := 0
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			count++
			if cb == nil {
				cb = ch
			}
		}
		if count < 1 || cb == nil {
			return ""
		}
		if jsIsIdentityArrayCallback(cb, content) {
			return jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
		}
		// Nested identity: aa.flatMap(xs => xs) when aa is [[new A()], …].
		// Nested identity-map: aa.flatMap(xs => xs.map(x => x)) — same leaf as
		// identity flatten / aa.flat()[0] under foreign same-leaf.
		if jsIsIdentityCallback(cb, content) || jsIsNestedIdentityMapCallback(cb, content) {
			if obj != nil && obj.Type() == "identifier" && arrayLocals != nil {
				if t := arrayLocals["@nested."+ingest.NodeText(obj, content)]; t != "" {
					return t
				}
			}
			return jsNestedArrayFlatElemType(obj, content, arrayLocals, typedLocals, factories, extra)
		}
		// Non-identity sole-return array of uniform T: [0].flatMap(() => [new A()]) /
		// [0].flatMap(() => [new BoxA().get()]) method-return under foreign same-leaf.
		if t := jsFlatMapSoleArrayReturnElemType(cb, content, typedLocals, factories, extra.classFields, extra.methodReturns); t != "" {
			return t
		}
		return ""
	case "fill":
		// arr.fill(val[, start[, end]]) overwrites selected slots with val.
		// Element type is val's type (prior content discarded). Enables
		// [null].fill(new A())[0].run() / new Array(1).fill(new A())[0].run() /
		// Array(1).fill(new BoxA().get())[0].run() under foreign same-leaf.
		// Non-concrete val fails closed.
		var val *grammar.Node
		pos := 0
		if args != nil {
			for i := uint32(0); i < args.ChildCount(); i++ {
				ch := args.Child(i)
				if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
					continue
				}
				pos++
				if pos == 1 {
					val = ch
				}
			}
		}
		if pos < 1 || val == nil {
			return ""
		}
		// Class/typed-local via concrete path; method-return when extra.methodReturns set.
		return jsExprLeafType(val, content, typedLocals, factories, extra.classFields, extra.methodReturns)
	case "concat":
		// [].concat([new A()]) / [new A()].concat() / as.concat([new A()]) —
		// empty-array receiver is a wildcard (type from args). Mixed inserts fail closed.
		return jsArrayConcatElemType(obj, args, content, arrayLocals, typedLocals, factories, extra, "")
	case "toSpliced":
		// toSpliced(start, deleteCount, ...items) — typed receiver + items of T;
		// empty-array receiver is a wildcard (type from inserts). Zero items is
		// identity of a typed receiver. Self-target assign uses
		// jsArrayAssignSourceElemType → jsArrayToSplicedElemType(selfTarget).
		return jsArrayToSplicedElemType(obj, args, content, arrayLocals, typedLocals, factories, extra, "")
	case "with":
		// arr.with(i, val) — typed receiver + val of T; empty-array receiver is
		// a wildcard (type from val). Self-target assign uses
		// jsArrayAssignSourceElemType → jsArrayWithElemType(selfTarget).
		return jsArrayWithElemType(obj, args, content, arrayLocals, typedLocals, factories, extra, "")
	}
	return ""
}

// jsArrayFlatElemType recovers T from arr.flat() / arr.flat(1) when:
//  1. arr peels to uniform element type T (flat is a no-op for non-array elems), or
//  2. arr is an array of array-sources of uniform element type T (one-level flat).
//
// Depth absent or literal 1 only; deeper / non-numeric depth fail closed.
// Enables [[new A()]].flat()[0].run() / [new A()].flat()[0].run() under foreign
// same-leaf methods.
func jsArrayFlatElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil {
		return ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "flat" {
		return ""
	}
	// Zero args or single numeric literal depth 1.
	var depthArg *grammar.Node
	count := 0
	if args != nil {
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			count++
			if depthArg == nil {
				depthArg = ch
			}
		}
	}
	if count > 1 {
		return ""
	}
	if count == 1 {
		if depthArg == nil || depthArg.Type() != "number" || ingest.NodeText(depthArg, content) != "1" {
			return ""
		}
	}
	obj := ingest.ChildByField(fn, "object")
	// Identity: arr of T (elements are not nested arrays of interest).
	if t := jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Nested local: const aa = [[new A()]]; aa.flat()[0].run() — @nested peels A.
	if obj != nil && obj.Type() == "identifier" && arrayLocals != nil {
		if t := arrayLocals["@nested."+ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	// Nested identity map then flat:
	//   aa.map(xs => xs).flat()[0]
	//   aa.map(xs => xs.map(x => x)).flat()[0]
	//   [[new A()]].map(xs => xs).flat()[0]
	// Same leaf as aa.flat()[0] under foreign same-leaf. Non-identity map fails closed.
	if t := jsNestedIdentityMapThenFlatElemType(obj, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// One-level nest: [[new T()], …] / [as, …] when every element peels to array of T.
	return jsNestedArrayFlatElemType(obj, content, arrayLocals, typedLocals, factories, extra)
}

// jsNestedIdentityMapThenFlatElemType recovers T from arr.map(cb) when:
//   - cb is identity (xs => xs) or nested identity-map (xs => xs.map(x => x)), and
//   - arr is a one-level nested array of T (const aa = [[new A()]] / [[new A()]]).
//
// Identity map preserves the nested shape; callers (flat) peel one level to T.
// Enables aa.map(xs => xs).flat()[0].run() under foreign same-leaf methods.
// Type-changing map / non-nested receivers fail closed.
func jsNestedIdentityMapThenFlatElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "map" {
		return ""
	}
	var cb *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if cb == nil {
			cb = ch
		}
	}
	// Sole identity / nested-identity-map callback. thisArg / multi-arg fail closed.
	if count != 1 || cb == nil {
		return ""
	}
	if !jsIsIdentityCallback(cb, content) && !jsIsNestedIdentityMapCallback(cb, content) {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
	// Nested local: const aa = [[new A()]].
	if obj.Type() == "identifier" && arrayLocals != nil {
		if t := arrayLocals["@nested."+ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	// Inline nest: [[new A()]].map(...).
	return jsNestedArrayFlatElemType(obj, content, arrayLocals, typedLocals, factories, extra)
}

// jsNestedArrayIndexSourceElemType recovers T from aa[i] / [[new A()]][i] when the
// receiver is a one-level nested array of T (elements are arrays of T). The
// subscript expression is itself an array of T (not T), so further [j] / .at(j)
// peels T. Numeric index only. Unknown / non-nested fail closed.
func jsNestedArrayIndexSourceElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	if obj == nil {
		return ""
	}
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
	// Nested local: const aa = [[new A()]].
	if obj.Type() == "identifier" && arrayLocals != nil {
		if t := arrayLocals["@nested."+ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	// Inline nest: [[new A()]][0] — array-of-arrays literal.
	return jsNestedArrayFlatElemType(obj, content, arrayLocals, typedLocals, factories, extra)
}

// jsNestedArrayFlatElemType recovers T from an array whose every element peels
// to uniform array element type T (array-of-arrays flat one level).
func jsNestedArrayFlatElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		t := jsArraySourceElemType(el, content, arrayLocals, typedLocals, factories, extra)
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

// jsArrayFromElemType recovers T from Array.from(arrLike) when:
//   - 1-arg: first arg peels to uniform element type T, or
//   - 2-arg identity mapfn (x => x): same as 1-arg (type-preserving), or
//   - 2-arg non-identity mapfn whose sole return peels to concrete T
//     (Array.from({length: n}, () => new A()) / (_v, i) => a0) — mapfn return
//     is the element type under foreign same-leaf.
//
// thisArg (3rd arg) / unknown mapfn returns / untyped first arg fail closed.
func jsArrayFromElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil || args == nil {
		return ""
	}
	// Array.from / Array.fromAsync
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil ||
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Array" {
		return ""
	}
	switch ingest.NodeText(prop, content) {
	case "from", "fromAsync":
		// ok
	default:
		return ""
	}
	// 1-arg: Array.from(arr). 2-arg: Array.from(arr, mapfn).
	var first, second *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if first == nil {
			first = ch
		} else if second == nil {
			second = ch
		}
	}
	if first == nil {
		return ""
	}
	switch count {
	case 1:
		// Array.from(arr) — no mapfn; element type of first arg.
		return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories, extra)
	case 2:
		if second == nil {
			return ""
		}
		// Array.from(arr, x => x) — identity mapfn preserves first-arg elements.
		if jsIsIdentityCallback(second, content) {
			return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories, extra)
		}
		// Array.from({length: n}, () => new A()) / (_v, i) => a0 /
		// Array.from([0], () => new BoxA().get()) — non-identity mapfn: sole
		// return peels to concrete element T (array-like length objects have
		// no element source; mapfn creates each slot). Method-return peels
		// under foreign same-leaf when extra.methodReturns is set.
		return jsArrayFromMapfnReturnType(second, content, typedLocals, factories, extra.classFields, extra.methodReturns)
	default:
		return ""
	}
}

// jsArrayFromMapfnReturnType recovers T from an Array.from mapfn whose sole
// return expression peels to a concrete class leaf (new T() / typed local /
// factory / ternary / zero-arg method return ba.get() / new BoxA().get()).
// Enables Array.from({length: n}, () => new A())[0].run() and
// Array.from([0], () => new BoxA().get())[0].run() under foreign same-leaf.
// Multi-statement / untyped returns fail closed.
func jsArrayFromMapfnReturnType(cb *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	ret := jsCallbackSoleReturnExpr(cb, content)
	if ret == nil {
		return ""
	}
	return jsExprLeafType(ret, content, typedLocals, factories, classFields, methodReturns)
}

// jsArrayOfElemType recovers T from Array.of(x, …) when every positional arg
// peels to the same concrete type T. Empty / mixed args fail closed.
// Enables Array.of(new A())[0].run() / for (const a of Array.of(new A())) under
// foreign same-leaf methods.
func jsArrayOfElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		t := jsExprLeafType(ch, content, typedLocals, factories, extra.classFields, extra.methodReturns)
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
// value peels to the same concrete type T. Also peels Object.values(
// Object.fromEntries([[k, new T()]])) when fromEntries pairs agree on T,
// Object.values(Object.assign(…)) when assign sources' property values agree,
// and Object.values(o) when o is an objValueLocal (fromEntries / assign).
// Non-object-literal / mixed / empty / method / spread entries fail closed.
func jsObjectValuesElemType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if t := jsObjectStaticValuesCallTypeEx(n, content, typedLocals, factories, "values", extra); t != "" {
		return t
	}
	// Object.values(Object.fromEntries(...)) — values are fromEntries pair values.
	// Method-return pairs via extra.methodReturns (Object.fromEntries([[k, ba.get()]])).
	// mapLocals enables Object.values(Object.fromEntries(ma)) after Map.set / new Map.
	if t := jsObjectValuesFromEntriesTypeEx(n, content, typedLocals, factories, extra.entryArrayLocals, extra.mapLocals, extra.classFields, extra.methodReturns); t != "" {
		return t
	}
	// Object.values(Object.assign({}, {k: new T()}, …)) — merged property values.
	if t := jsObjectValuesAssignType(n, content, typedLocals, factories, extra); t != "" {
		return t
	}
	// Object.values(o) after const o = Object.assign(...) / Object.fromEntries(...)
	return jsObjectValuesLocalType(n, content, extra.objValue)
}

// jsObjectValuesLocalType recovers T from Object.values(ident) when ident is an
// objValueLocal of uniform property value type T (fromEntries / assign).
func jsObjectValuesLocalType(n *grammar.Node, content []byte, objValueLocals map[string]string) string {
	if n == nil || n.Type() != "call_expression" || objValueLocals == nil {
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
		ingest.NodeText(prop, content) != "values" {
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
	if first == nil || first.Type() != "identifier" {
		return ""
	}
	return objValueLocals[ingest.NodeText(first, content)]
}

// jsObjectValuesAssignType recovers T from Object.values(Object.assign(…)) when
// Object.assign's object-literal sources agree on uniform property value type T.
func jsObjectValuesAssignType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != "values" {
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
	return jsObjectAssignValueTypeEx(first, content, typedLocals, factories, extra.objValue, extra.classFields, extra.methodReturns)
}

// jsObjectAssignValueType recovers T from Object.assign(…sources) when every
// property value across all object-literal arguments peels to the same concrete
// type T. Empty object literals ({}) contribute nothing (common target). At
// least one property value required. Identifier sources peel via objValueLocals
// (Object.assign(oa) after const oa = {k: new A()}). Mixed / method / spread
// entries fail closed.
// Enables Object.values(Object.assign({}, {k: new A()}))[0].run() and
// Object.assign(oa).k.run() under foreign same-leaf methods. Object.assign(new A())
// identity target peels stay in jsIdentityCloneType (first-arg return).
// Method-return property values use jsObjectAssignValueTypeEx.
func jsObjectAssignValueType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsObjectAssignValueTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsObjectAssignValueTypeEx peels Object.assign({}, {k: new BoxA().get()}) via
// method-return property values under foreign same-leaf (Class peels via
// jsObjectAssignValueType / jsExprConcreteType).
func jsObjectAssignValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != "assign" {
		return ""
	}
	found := ""
	saw := false
	argCount := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		argCount++
		arg := ch
		for arg != nil && arg.Type() == "parenthesized_expression" {
			var inner *grammar.Node
			for j := uint32(0); j < arg.ChildCount(); j++ {
				c := arg.Child(j)
				if c.Type() == "(" || c.Type() == ")" {
					continue
				}
				inner = c
				break
			}
			arg = inner
		}
		if arg == nil {
			return ""
		}
		if arg.Type() == "identifier" {
			// Object.assign(oa[, …]) — plain-object local of uniform value T.
			if objValueLocals == nil {
				return ""
			}
			t := objValueLocals[ingest.NodeText(arg, content)]
			if t == "" {
				return ""
			}
			if !saw {
				found = t
				saw = true
			} else if found != t {
				return ""
			}
			continue
		}
		if arg.Type() != "object" {
			// Non-object-literal source (call / new) — fail closed for values
			// peels (identity first-arg return is jsIdentityCloneType).
			return ""
		}
		// Collect property values; empty {} is fine (no contribution).
		for j := uint32(0); j < arg.ChildCount(); j++ {
			propN := arg.Child(j)
			if propN == nil || propN.Type() == "{" || propN.Type() == "}" || propN.Type() == "," {
				continue
			}
			var val *grammar.Node
			switch propN.Type() {
			case "pair":
				val = ingest.ChildByField(propN, "value")
			case "shorthand_property_identifier":
				val = propN
			default:
				return ""
			}
			if val == nil {
				return ""
			}
			t := ""
			if val.Type() == "shorthand_property_identifier" {
				if typedLocals != nil {
					t = typedLocals[ingest.NodeText(val, content)]
				}
			} else {
				// Class/typed-local peels via jsExprConcreteType path inside
				// jsExprLeafType; method-return ba.get() when methodReturns set.
				t = jsExprLeafType(val, content, typedLocals, factories, classFields, methodReturns)
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
	}
	if argCount < 1 || !saw {
		return ""
	}
	return found
}

// jsObjectValuesFromEntriesType recovers T from Object.values(Object.fromEntries(…))
// / Object.values(o) when o is a fromEntries object-value local.
// mapLocals enables Object.values(Object.fromEntries(ma)) after Map.set / new Map.
func jsObjectValuesFromEntriesType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, mapLocals map[string]string) string {
	return jsObjectValuesFromEntriesTypeEx(n, content, typedLocals, factories, entryArrayLocals, mapLocals, nil, nil)
}

// jsObjectValuesFromEntriesTypeEx peels Object.values(Object.fromEntries([[k, ba.get()]]))
// via method-return pair values under foreign same-leaf.
func jsObjectValuesFromEntriesTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, mapLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != "values" {
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
	return jsObjectFromEntriesValueTypeEx(first, content, typedLocals, factories, entryArrayLocals, mapLocals, classFields, methodReturns)
}

// jsObjectFromEntriesValueType recovers T from Object.fromEntries(iterable) when
// the iterable peels to pairs of uniform value type T (pair-array literal,
// pair-array local, Object.entries({…}), Map local / new Map, or map.entries()).
// Empty / mixed fail closed.
func jsObjectFromEntriesValueType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, mapLocals map[string]string) string {
	return jsObjectFromEntriesValueTypeEx(n, content, typedLocals, factories, entryArrayLocals, mapLocals, nil, nil)
}

// jsObjectFromEntriesValueTypeEx peels Object.fromEntries([[k, new BoxA().get()]])
// via method-return pair values (same leaf as Map([[k, method]]) / entries method-return).
func jsObjectFromEntriesValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, mapLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != "fromEntries" {
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
	// Pair-array / Object.entries / entryArrayLocals first (method-return via Ex).
	if t := jsMapIterableValueTypeEx(first, content, typedLocals, factories, entryArrayLocals, classFields, methodReturns); t != "" {
		return t
	}
	// Object.fromEntries(ma) / Object.fromEntries(new Map(...)) — Map default
	// iterator is entries of value T (same leaf as ma.get(k); method-return Ex).
	if t := jsMapSourceValueTypeEx(first, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns); t != "" {
		return t
	}
	// Object.fromEntries(ma.entries()) / Object.fromEntries(new Map([[k, ba.get()]]).entries())
	// — explicit entries iterator of value T.
	if t := jsMapEntriesValueTypeEx(first, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns); t != "" {
		return t
	}
	// Object.fromEntries([...ma]) / Object.fromEntries([...ma.entries()]) —
	// spread materializes entries pairs of value T.
	if t := jsObjectEntriesArraySourceTypeEx(first, content, typedLocals, factories, entryArrayLocals, nil, mapLocals, nil, nil, jsExtraLocals{
		classFields: classFields, methodReturns: methodReturns, mapLocals: mapLocals, entryArrayLocals: entryArrayLocals,
	}); t != "" {
		return t
	}
	return ""
}

// jsObjectFromEntriesPropType recovers T from Object.fromEntries(…).k /
// Object.fromEntries(…)["k"] / o.k / o["k"] when the object peels to a
// fromEntries result (or objValueLocal) of uniform property value type T.
// Any property key is accepted (all values are T). mapLocals enables
// Object.fromEntries(ma).k after Map.set / new Map.
func jsObjectFromEntriesPropType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, objValueLocals, mapLocals map[string]string) string {
	return jsObjectFromEntriesPropTypeEx(n, content, typedLocals, factories, entryArrayLocals, objValueLocals, mapLocals, nil, nil)
}

// jsObjectFromEntriesPropTypeEx peels Object.fromEntries([[k, method]]).k under
// foreign same-leaf via methodReturns (Class()/local peels via non-Ex path).
func jsObjectFromEntriesPropTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, objValueLocals, mapLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	var obj *grammar.Node
	switch n.Type() {
	case "member_expression", "member_expression_optional", "optional_chain":
		// o.k / Object.fromEntries(...).k — property must be a simple identifier.
		prop := ingest.ChildByField(n, "property")
		if prop == nil {
			return ""
		}
		// Reject private / computed-looking forms; bare name only.
		if prop.Type() != "property_identifier" && prop.Type() != "identifier" {
			return ""
		}
		obj = ingest.ChildByField(n, "object")
	case "subscript_expression":
		// o["k"] / Object.fromEntries(...)[0] — any index; values are uniform T.
		// Still require a present index node.
		if ingest.ChildByField(n, "index") == nil {
			return ""
		}
		obj = ingest.ChildByField(n, "object")
	default:
		return ""
	}
	if obj == nil {
		return ""
	}
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
	if t := jsObjectFromEntriesValueTypeEx(obj, content, typedLocals, factories, entryArrayLocals, mapLocals, classFields, methodReturns); t != "" {
		return t
	}
	// Object.assign({}, {k: new A()}) / Object.assign({}, {k: new BoxA().get()}) /
	// Object.assign(oa) — property values T (method-return via methodReturns).
	if t := jsObjectAssignValueTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
		return t
	}
	if obj.Type() == "identifier" && objValueLocals != nil {
		if t := objValueLocals[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsObjectEntriesValueType recovers T from Object.entries({…}) when every
// property value peels to the same concrete type T. The call yields [key, value]
// pairs — T is the value type (not the pair). Same object-literal rules as values.
// Also peels Object.entries(Object.assign(…)) / Object.entries(Object.fromEntries(…))
// and Object.entries(o) when o is an objValueLocal (assign / fromEntries) — dual of
// Object.values peels under foreign same-leaf methods.
func jsObjectEntriesValueType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsObjectEntriesValueTypeEx(n, content, typedLocals, factories, objValueLocals, jsExtraLocals{})
}

// jsObjectEntriesValueTypeEx peels Object.entries({k: new BoxA().get()}) via
// method-return property values (same leaf as Object.values method-return).
func jsObjectEntriesValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, extra jsExtraLocals) string {
	if t := jsObjectStaticValuesCallTypeEx(n, content, typedLocals, factories, "entries", extra); t != "" {
		return t
	}
	// Object.entries(Object.assign(...)) / Object.entries(Object.fromEntries(...)) /
	// Object.entries(o) after const o = Object.assign(...) / Object.fromEntries(...).
	first := jsObjectStaticCallSingleArg(n, content, "entries")
	if first == nil {
		return ""
	}
	if t := jsObjectAssignValueTypeEx(first, content, typedLocals, factories, objValueLocals, extra.classFields, extra.methodReturns); t != "" {
		return t
	}
	if t := jsObjectFromEntriesValueType(first, content, typedLocals, factories, nil, nil); t != "" {
		return t
	}
	if first.Type() == "identifier" && objValueLocals != nil {
		return objValueLocals[ingest.NodeText(first, content)]
	}
	return ""
}

// jsObjectStaticCallSingleArg returns the sole real argument of Object.<method>(arg)
// after peeling parentheses, or nil when the shape does not match.
func jsObjectStaticCallSingleArg(n *grammar.Node, content []byte, method string) *grammar.Node {
	if n == nil || n.Type() != "call_expression" || method == "" {
		return nil
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil || args == nil {
		return nil
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return nil
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil ||
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" ||
		ingest.NodeText(prop, content) != method {
		return nil
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
		return nil
	}
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
	return first
}

// jsObjectStaticValuesCallType recovers uniform property-value type T from
// Object.values/Object.entries({…}) (single object-literal arg).
func jsObjectStaticValuesCallType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, method string) string {
	return jsObjectStaticValuesCallTypeEx(n, content, typedLocals, factories, method, jsExtraLocals{})
}

// jsObjectStaticValuesCallTypeEx peels Object.values/keys with method-return property values.
func jsObjectStaticValuesCallTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, method string, extra jsExtraLocals) string {
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
			t = jsExprLeafType(val, content, typedLocals, factories, extra.classFields, extra.methodReturns)
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

// jsObjectEntriesPairSubscriptType recovers T from Object.entries({…})[i] /
// es[i] after const es = Object.entries(...) /
// [...Object.entries({…})][i] / [...arr.entries()][i] / [...map.entries()][i]
// when the pair value type is T. Index must be a numeric literal. The pair
// itself is not T — use for destructure binding (const [, a] = …[i]) only.
func jsObjectEntriesPairSubscriptType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string) string {
	return jsObjectEntriesPairSubscriptTypeEx(n, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, jsExtraLocals{})
}

func jsObjectEntriesPairSubscriptTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string, extra jsExtraLocals) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	return jsObjectEntriesArraySourceTypeEx(ingest.ChildByField(n, "object"), content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra)
}

// jsObjectEntriesArraySourceType recovers value type T from an expression that
// yields [key, value] pairs of value T: Object.entries({…}), an entries-array
// / entries-iterator local (const es = Object.entries(...) /
// const ia = arr.entries()), a single-spread copy
// [...Object.entries(...)] / [...es] / [...arr.entries()] / [...map.entries()] /
// [...set.entries()] / [...ma] (bare Map default iterator is entries) /
// Array.from(ma) / Array.from(map.entries()), arr.entries(), map.entries(),
// set.entries(), or a bare Map source (new Map([[k, new T()]]) / map local).
// Used for es[i] / [...entries][i] peels.
// Bare Map is intentionally NOT an array-element source of T (pairs, not values);
// only entries peels ([…ma][i][1] / Array.from(ma)[i][1]) use this path.
func jsObjectEntriesArraySourceType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string) string {
	return jsObjectEntriesArraySourceTypeEx(n, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, jsExtraLocals{})
}

// jsObjectEntriesArraySourceTypeEx peels Object.entries method-return values via extra.
func jsObjectEntriesArraySourceTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string, extra jsExtraLocals) string {
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
	if t := jsObjectEntriesValueTypeEx(n, content, typedLocals, factories, objValueLocals, extra); t != "" {
		return t
	}
	if t := jsArrayEntriesValueType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if t := jsMapEntriesValueTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, extra.classFields, extra.methodReturns); t != "" {
		return t
	}
	if t := jsSetEntriesValueTypeEx(n, content, arrayLocals, typedLocals, factories, setLocals, extra); t != "" {
		return t
	}
	if t := jsPairArrayValueTypeEx(n, content, typedLocals, factories, extra.classFields, extra.methodReturns); t != "" {
		// [[k, new T()], …] / [[k, ba.get()]] — inline pair-array of value T.
		return t
	}
	if n.Type() == "identifier" && entryArrayLocals != nil {
		if t := entryArrayLocals[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	// Bare Map / new Map([[k, new T()]]) — default iterator yields [k, v] pairs.
	// Enables [...ma][0][1].run() / Array.from(ma)[0][1].run() after ma.set /
	// new Map([[k, new A()]]) / new Map([[k, ba.get()]]) under foreign same-leaf.
	if t := jsMapSourceValueTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, extra.classFields, extra.methodReturns); t != "" {
		return t
	}
	// Array.from(entriesIterable) — materializes [k,v][] of value T. 1-arg or
	// identity mapfn only. First arg peels as entries source (bare Map, .entries(),
	// Object.entries, pair array, …). Non-entries first args fail closed here
	// (array-element Array.from stays on jsArrayFromElemType).
	if t := jsArrayFromEntriesSourceTypeEx(n, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
		return t
	}
	// [...Object.entries({…})] / [...es] / [...arr.entries()] / [...map.entries()] /
	// [...ma] — single spread of entries pair source (bare Map via mapSource above).
	if n.Type() == "array" {
		var spreadArg *grammar.Node
		count := 0
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
				continue
			}
			count++
			if ch.Type() != "spread_element" {
				return ""
			}
			// spread_element: "..." + expression (first non-"..." child).
			var arg *grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				c := ch.Child(j)
				if c == nil || c.Type() == "..." {
					continue
				}
				arg = c
				break
			}
			if arg == nil {
				return ""
			}
			spreadArg = arg
		}
		if count != 1 || spreadArg == nil {
			return ""
		}
		return jsObjectEntriesArraySourceTypeEx(spreadArg, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra)
	}
	return ""
}

// jsArrayFromEntriesSourceType recovers T from Array.from(entriesIterable)
// when the first arg peels as an entries pair source of value T (bare Map,
// map.entries(), Object.entries, pair array, …). 1-arg or identity mapfn only.
// Enables Array.from(ma)[0][1].run() under foreign same-leaf. Non-entries
// first args fail closed (element Array.from stays on jsArrayFromElemType).
func jsArrayFromEntriesSourceType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string) string {
	return jsArrayFromEntriesSourceTypeEx(n, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, jsExtraLocals{})
}

func jsArrayFromEntriesSourceTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string, extra jsExtraLocals) string {
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
		ingest.NodeText(prop, content) != "from" {
		return ""
	}
	var first, second *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if first == nil {
			first = ch
		} else if second == nil {
			second = ch
		}
	}
	if first == nil {
		return ""
	}
	switch count {
	case 1:
		// Array.from(entries) — no mapfn.
	case 2:
		if second == nil || !jsIsIdentityCallback(second, content) {
			return ""
		}
	default:
		return ""
	}
	return jsObjectEntriesArraySourceTypeEx(first, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra)
}

// jsEntriesIterableValueType recovers T from an iterable of [key, value] pairs
// of value T: Object.entries({…}), entries-array/iterator local,
// [...Object.entries(...)], arr.entries(), map.entries(), set.entries(), or a
// Map source (new Map([[k, new T()]]) / map local — default iterator yields
// entries).
func jsEntriesIterableValueType(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, objValueLocals map[string]string) string {
	return jsEntriesIterableValueTypeEx(n, content, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, objValueLocals, jsExtraLocals{})
}

func jsEntriesIterableValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, objValueLocals map[string]string, extra jsExtraLocals) string {
	if t := jsObjectEntriesArraySourceTypeEx(n, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
		return t
	}
	return jsMapSourceValueType(n, content, typedLocals, factories, mapLocals, entryArrayLocals)
}

// jsArrayEntriesValueType recovers T from arr.entries() when arr peels to a
// uniform array element type T (pairs are [index, T]). Zero-arg only.
func jsArrayEntriesValueType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "entries" {
		return ""
	}
	// Object.entries is handled by jsObjectEntriesValueType — fail closed here so
	// static Object.entries is not double-routed through array element peels.
	obj := ingest.ChildByField(fn, "object")
	if obj != nil && obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
		return ""
	}
	return jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
}

// jsPairArrayValueType recovers T from [[k, v], …] when every element is a
// 2-slot pair array whose value peels to the same concrete type T. Empty /
// mixed / non-pair elems fail closed. Enables const pa = [["k", new A()]] and
// new Map([[k, new A()]]) pair peels under foreign same-leaf methods.
func jsPairArrayValueType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
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
		// Each element must be a 2-slot pair array [key, value].
		if el.Type() != "array" {
			return ""
		}
		var slots []*grammar.Node
		for j := uint32(0); j < el.ChildCount(); j++ {
			ch := el.Child(j)
			if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
				continue
			}
			slots = append(slots, ch)
		}
		if len(slots) != 2 {
			return ""
		}
		t := jsExprLeafType(slots[1], content, typedLocals, factories, nil, nil)
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

// jsPairArrayValueTypeEx peels pair values including zero-arg method returns.
func jsPairArrayValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		if el.Type() != "array" {
			return ""
		}
		var slots []*grammar.Node
		for j := uint32(0); j < el.ChildCount(); j++ {
			ch := el.Child(j)
			if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
				continue
			}
			slots = append(slots, ch)
		}
		if len(slots) != 2 {
			return ""
		}
		t := jsExprLeafType(slots[1], content, typedLocals, factories, classFields, methodReturns)
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

// jsMapIterableValueType recovers T from a Map/WeakMap constructor iterable:
// pair-array literal [[k, v], …], pair-array local (entryArrayLocals), or
// Object.entries({…}) when property values agree.
func jsMapIterableValueType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals map[string]string) string {
	return jsMapIterableValueTypeEx(n, content, typedLocals, factories, entryArrayLocals, nil, nil)
}

func jsMapIterableValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if t := jsPairArrayValueTypeEx(n, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	// Object.entries({k: new A()}) / Object.entries({k: ba.get()}) — method-return
	// via Ex (Class peels via non-Ex path with nil methodReturns).
	if t := jsObjectEntriesValueTypeEx(n, content, typedLocals, factories, nil, jsExtraLocals{
		classFields: classFields, methodReturns: methodReturns,
	}); t != "" {
		// new Map(Object.entries({k: new A()})) /
		// Object.fromEntries(Object.entries({k: ba.get()}))
		return t
	}
	if n.Type() == "identifier" && entryArrayLocals != nil {
		if t := entryArrayLocals[ingest.NodeText(n, content)]; t != "" {
			// new Map(pa) after const pa = [["k", new A()]] /
			// const pa = Object.entries({k: new A()})
			return t
		}
	}
	return ""
}

// jsMapValueType recovers T from new Map(iterable) / new WeakMap(iterable) when
// the iterable peels to pairs of uniform value type T. Iterable may be a pair-
// array literal, pair-array/entries local, or Object.entries({…}). Empty /
// mixed / other ctors fail closed. WeakMap shares the same pair-value peel.
func jsMapValueType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals map[string]string) string {
	return jsMapValueTypeEx(n, content, typedLocals, factories, entryArrayLocals, nil, nil)
}

func jsMapValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if n == nil || n.Type() != "new_expression" {
		return ""
	}
	ctor := ingest.ChildByField(n, "constructor")
	if ctor == nil {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c.Type() == "identifier" {
				ctor = c
				break
			}
		}
	}
	if ctor == nil || ctor.Type() != "identifier" {
		return ""
	}
	switch ingest.NodeText(ctor, content) {
	case "Map", "WeakMap":
		// ok
	default:
		return ""
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
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
	return jsMapIterableValueTypeEx(first, content, typedLocals, factories, entryArrayLocals, classFields, methodReturns)
}

// jsSetValueType recovers T from new Set(iterable) when the single iterable arg
// peels to a uniform array element type T. Empty / multi-arg / non-array
// iterables fail closed. Enables new Set([new A()]).forEach((v) => v.run()).
func jsSetValueType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if n == nil || n.Type() != "new_expression" {
		return ""
	}
	ctor := ingest.ChildByField(n, "constructor")
	if ctor == nil {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c.Type() == "identifier" {
				ctor = c
				break
			}
		}
	}
	if ctor == nil || ctor.Type() != "identifier" || ingest.NodeText(ctor, content) != "Set" {
		return ""
	}
	// new Set(iterable) — single positional arg only.
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
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
	return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories, extra)
}

// jsSetSourceValueType recovers T from new Set([new T()]) / new Set(as) or a
// set local bound to uniform element type T.
func jsSetSourceValueType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories, setLocals map[string]string) string {
	return jsSetSourceValueTypeEx(n, content, arrayLocals, typedLocals, factories, setLocals, jsExtraLocals{setLocals: setLocals})
}

func jsSetSourceValueTypeEx(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories, setLocals map[string]string, extra jsExtraLocals) string {
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
	if t := jsSetValueType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	if n.Type() == "identifier" && setLocals != nil {
		if t := setLocals[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsSetIteratorYieldType recovers T from set.values() / set.keys() /
// set[Symbol.iterator]() when set peels to a uniform element type T.
// For Set, keys() yields the same elements as values(). Zero-arg only.
// Enables new Set([new A()]).values().next().value.run() /
// sa.keys().next().value.run() under foreign same-leaf methods.
func jsSetIteratorYieldType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories, setLocals map[string]string) string {
	return jsSetIteratorYieldTypeEx(n, content, arrayLocals, typedLocals, factories, setLocals, jsExtraLocals{setLocals: setLocals})
}

// jsSetIteratorYieldTypeEx peels new Set([ba.get()]).keys() / .values() method-return
// elements under foreign same-leaf.
func jsSetIteratorYieldTypeEx(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories, setLocals map[string]string, extra jsExtraLocals) string {
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
	// Prefer caller's extra (methodReturns); fall back to setLocals only.
	ex := extra
	if ex.setLocals == nil {
		ex.setLocals = setLocals
	}
	switch fn.Type() {
	case "member_expression", "member_expression_optional", "optional_chain":
		// set.values() / set.keys()
		prop := ingest.ChildByField(fn, "property")
		if prop == nil {
			return ""
		}
		name := ingest.NodeText(prop, content)
		if name != "values" && name != "keys" {
			return ""
		}
		return jsSetSourceValueTypeEx(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, setLocals, ex)
	case "subscript_expression":
		// set[Symbol.iterator]()
		if !jsIsSymbolIteratorIndex(ingest.ChildByField(fn, "index"), content) {
			return ""
		}
		return jsSetSourceValueTypeEx(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, setLocals, ex)
	}
	return ""
}

// jsSetEntriesValueType recovers T from set.entries() when set peels to a
// uniform element type T (pairs are [T, T]; value slot [1] peels as T).
// Zero-arg only. Enables new Set([new A()]).entries().next().value[1].run() /
// for (const [, a] of sa.entries()) under foreign same-leaf methods.
func jsSetEntriesValueType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories, setLocals map[string]string) string {
	return jsSetEntriesValueTypeEx(n, content, arrayLocals, typedLocals, factories, setLocals, jsExtraLocals{setLocals: setLocals})
}

// jsSetEntriesValueTypeEx peels new Set([ba.get()]).entries() method-return
// elements under foreign same-leaf.
func jsSetEntriesValueTypeEx(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories, setLocals map[string]string, extra jsExtraLocals) string {
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
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "entries" {
		return ""
	}
	// Object.entries is handled elsewhere — fail closed here.
	obj := ingest.ChildByField(fn, "object")
	if obj != nil && obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
		return ""
	}
	ex := extra
	if ex.setLocals == nil {
		ex.setLocals = setLocals
	}
	return jsSetSourceValueTypeEx(obj, content, arrayLocals, typedLocals, factories, setLocals, ex)
}

// jsArrayFindElemType recovers T from arr.find(pred) / arr.findLast(pred) when
// the receiver peels to a uniform array element type T. Requires at least one
// positional arg (predicate); predicate body ignored (not type-changing for
// uniform arrays). Enables [new A()].find(x => x).run() /
// as.findLast(() => true).run() under foreign same-leaf methods.
func jsArrayFindElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil {
		return ""
	}
	name := ingest.NodeText(prop, content)
	if name != "find" && name != "findLast" {
		return ""
	}
	// At least one positional arg (predicate).
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
	}
	if count < 1 {
		return ""
	}
	return jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
}

// jsMapSourceValueType recovers T from new Map([[k, new T()]]) / new Map(pa) or
// a map local bound to uniform value type T.
func jsMapSourceValueType(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string) string {
	return jsMapSourceValueTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, nil, nil)
}

func jsMapSourceValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if t := jsMapValueTypeEx(n, content, typedLocals, factories, entryArrayLocals, classFields, methodReturns); t != "" {
		return t
	}
	if n.Type() == "identifier" && mapLocals != nil {
		if t := mapLocals[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsMapEntriesValueType recovers T from map.entries() when map peels to a Map
// of uniform value type T (pairs are [key, T]). Zero-arg only.
func jsMapEntriesValueType(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string) string {
	return jsMapEntriesValueTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, nil, nil)
}

// jsMapEntriesValueTypeEx peels new Map([[k, ba.get()]]).entries() method-return
// values under foreign same-leaf.
func jsMapEntriesValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "entries" {
		return ""
	}
	// Object.entries is handled elsewhere — fail closed here.
	obj := ingest.ChildByField(fn, "object")
	if obj != nil && obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
		return ""
	}
	return jsMapSourceValueTypeEx(obj, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns)
}

// jsMapValuesYieldType recovers T from map.values() when map peels to a Map of
// uniform value type T. Zero-arg only. Yields T directly (not pairs).
// Method-return Map values use jsMapValuesYieldTypeEx.
func jsMapValuesYieldType(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string) string {
	return jsMapValuesYieldTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, nil, nil)
}

// jsMapValuesYieldTypeEx peels map.values() with method-return Map values
// (new Map([[k, new BoxA().get()]]).values()) under foreign same-leaf.
func jsMapValuesYieldTypeEx(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "values" {
		return ""
	}
	// Object.values is handled by jsObjectValuesElemType — fail closed here.
	obj := ingest.ChildByField(fn, "object")
	if obj != nil && obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
		return ""
	}
	return jsMapSourceValueTypeEx(obj, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns)
}

// jsMapGetValueType recovers T from map.get(key) when map peels to a Map of
// uniform value type T. Requires exactly one positional arg (any key expression).
func jsMapGetValueType(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string) string {
	return jsMapGetValueTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, nil, nil)
}

func jsMapGetValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "get" {
		return ""
	}
	// Exactly one positional arg.
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
	}
	if count != 1 {
		return ""
	}
	return jsMapSourceValueTypeEx(ingest.ChildByField(fn, "object"), content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns)
}

// jsMapSymbolIteratorEntriesType recovers T from ma[Symbol.iterator]() /
// new Map([[k, new T()]])[Symbol.iterator]() when the receiver peels to a Map of
// uniform value type T. Zero-arg only. Map's default iterator yields [k, v] pairs
// (same as .entries()). Enables ma[Symbol.iterator]().next().value[1].run() under
// foreign same-leaf. Array/Set Symbol.iterator stay on element peels.
func jsMapSymbolIteratorEntriesType(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string) string {
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
	if fn == nil || fn.Type() != "subscript_expression" {
		return ""
	}
	if !jsIsSymbolIteratorIndex(ingest.ChildByField(fn, "index"), content) {
		return ""
	}
	return jsMapSourceValueType(ingest.ChildByField(fn, "object"), content, typedLocals, factories, mapLocals, entryArrayLocals)
}

// jsObjectEntriesPairAtType recovers T from es.at(i) / [...ma].at(0) /
// Array.from(ma).at(0) when the receiver peels as an entries-array source of
// value T. Single numeric arg only (incl. -1). The expression itself is a pair
// of value T (not T) — use for entryLocals and pair-value peels via [1].
// Enables [...ma].at(0)[1].run() under foreign same-leaf (es[0][1] already worked).
func jsObjectEntriesPairAtType(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string) string {
	return jsObjectEntriesPairAtTypeEx(n, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, jsExtraLocals{})
}

func jsObjectEntriesPairAtTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string, extra jsExtraLocals) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "at" {
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
	if count != 1 || first == nil || !jsIsNumericIndexArg(first, content) {
		return ""
	}
	return jsObjectEntriesArraySourceTypeEx(ingest.ChildByField(fn, "object"), content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra)
}

// jsNullishOrDefaultType recovers T from left ?? right / left || right / left && right
// when both arms peel to the same concrete type T, or (for ?? / ||) one arm peels
// to T and the other is null/undefined, or (for &&) left is boolean true and right
// peels to T. Arms peel via map.get value type or concrete new/local/factory.
// Enables (ma.get(k) ?? new A()).run() / const a = ma.get(k) || new A() /
// (true && new A()).run() / const a = true && new A() under foreign same-leaf.
// Mismatched arms (get A ?? new B()) fail closed.
func jsNullishOrDefaultType(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string) string {
	return jsNullishOrDefaultTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, nil, nil)
}

// jsNullishOrDefaultTypeEx peels nullish/or/and with method-return arms.
func jsNullishOrDefaultTypeEx(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if n == nil || n.Type() != "binary_expression" {
		return ""
	}
	op := ingest.ChildByField(n, "operator")
	if op == nil {
		return ""
	}
	opText := ingest.NodeText(op, content)
	switch opText {
	case "??", "||", "&&":
		// ok
	default:
		return ""
	}
	left := ingest.ChildByField(n, "left")
	right := ingest.ChildByField(n, "right")
	lt := jsNullishArmTypeEx(left, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns)
	rt := jsNullishArmTypeEx(right, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns)
	if lt != "" && rt != "" {
		if lt == rt {
			return lt
		}
		return ""
	}
	if opText == "&&" {
		// true && new A() → A. (new A() && true) stays boolean — fail closed.
		if rt != "" && jsIsTrueLiteral(left, content) {
			return rt
		}
		return ""
	}
	if lt != "" && jsIsNullUndefinedLiteral(right, content) {
		return lt
	}
	if rt != "" && jsIsNullUndefinedLiteral(left, content) {
		return rt
	}
	return ""
}

// jsIsTrueLiteral reports the boolean true literal (true / (true)).
func jsIsTrueLiteral(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
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
		return false
	}
	// tree-sitter-javascript: true is "true" node type or identifier/boolean.
	if n.Type() == "true" {
		return true
	}
	return ingest.NodeText(n, content) == "true"
}

// jsNullishArmType recovers T from one arm of ?? / ||: map.get value, new T(),
// typed local, or factory call. Empty / mixed fail closed at the binary level.
func jsNullishArmType(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string) string {
	return jsNullishArmTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, nil, nil)
}

func jsNullishArmTypeEx(n *grammar.Node, content []byte, typedLocals, factories, mapLocals, entryArrayLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if t := jsMapGetValueTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns); t != "" {
		return t
	}
	return jsExprLeafType(n, content, typedLocals, factories, classFields, methodReturns)
}

// jsIsNullUndefinedLiteral reports null / undefined (identifier or keyword form).
func jsIsNullUndefinedLiteral(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
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
		return false
	}
	switch n.Type() {
	case "null":
		return true
	case "undefined":
		return true
	case "identifier":
		return ingest.NodeText(n, content) == "undefined"
	}
	return false
}

// jsEntriesIteratorSourceType recovers T from an entries-iterator expression
// that yields [key, value] pairs of value T: arr.entries(), map.entries(), or
// an entries-iterator / pair-array local (entryArrayLocals).
func jsEntriesIteratorSourceType(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals map[string]string) string {
	return jsEntriesIteratorSourceTypeEx(n, content, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, jsExtraLocals{})
}

// jsEntriesIteratorSourceTypeEx peels map.entries() / set.entries() with
// method-return Map/Set elements under foreign same-leaf.
func jsEntriesIteratorSourceTypeEx(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals map[string]string, extra jsExtraLocals) string {
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
	ex := extra
	if ex.mapLocals == nil {
		ex.mapLocals = mapLocals
	}
	if ex.setLocals == nil {
		ex.setLocals = setLocals
	}
	if ex.entryArrayLocals == nil {
		ex.entryArrayLocals = entryArrayLocals
	}
	if t := jsArrayEntriesValueType(n, content, arrayLocals, typedLocals, factories, ex); t != "" {
		return t
	}
	if t := jsMapEntriesValueTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, ex.classFields, ex.methodReturns); t != "" {
		return t
	}
	// ma[Symbol.iterator]() — Map default iterator yields [k,v] pairs of value T
	// (same leaf as ma.entries()). Enables ma[Symbol.iterator]().next().value[1].run()
	// under foreign same-leaf. Bare Map is already entries via mapSource elsewhere.
	if t := jsMapSymbolIteratorEntriesType(n, content, typedLocals, factories, mapLocals, entryArrayLocals); t != "" {
		return t
	}
	if t := jsSetEntriesValueTypeEx(n, content, arrayLocals, typedLocals, factories, setLocals, ex); t != "" {
		return t
	}
	if n.Type() == "identifier" && entryArrayLocals != nil {
		if t := entryArrayLocals[ingest.NodeText(n, content)]; t != "" {
			return t
		}
	}
	return ""
}

// jsEntriesNextResultType recovers T from arr.entries().next() / map.entries().next() /
// ia.next() when the receiver peels to an entries-iterator of pair value T.
// Only zero-arg .next() peels. The result is an IteratorResult whose .value is
// a pair of value T (not T itself).
func jsEntriesNextResultType(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals map[string]string) string {
	return jsEntriesNextResultTypeEx(n, content, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, jsExtraLocals{})
}

func jsEntriesNextResultTypeEx(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals map[string]string, extra jsExtraLocals) string {
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
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "next" {
		return ""
	}
	return jsEntriesIteratorSourceTypeEx(ingest.ChildByField(fn, "object"), content, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, extra)
}

// jsEntriesNextPairType recovers T from arr.entries().next().value /
// map.entries().next().value / ia.next().value / ra.value after
// const ra = arr.entries().next(). The expression itself is a pair of value T
// (not T) — use for entryLocals binding and pair-value peels via [1].
func jsEntriesNextPairType(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, entryNextLocals, mapLocals, setLocals map[string]string) string {
	return jsEntriesNextPairTypeEx(n, content, typedLocals, factories, arrayLocals, entryArrayLocals, entryNextLocals, mapLocals, setLocals, jsExtraLocals{})
}

func jsEntriesNextPairTypeEx(n *grammar.Node, content []byte, typedLocals, factories, arrayLocals, entryArrayLocals, entryNextLocals, mapLocals, setLocals map[string]string, extra jsExtraLocals) string {
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
	// ra.value after const ra = arr.entries().next()
	if obj != nil && obj.Type() == "identifier" && entryNextLocals != nil {
		if t := entryNextLocals[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	return jsEntriesNextResultTypeEx(obj, content, typedLocals, factories, arrayLocals, entryArrayLocals, mapLocals, setLocals, extra)
}

// jsObjectEntriesPairValueType recovers T from Object.entries({…})[i][1] /
// es[i][1] / [...Object.entries({…})][i][1] / [...arr.entries()][i][1] /
// [...map.entries()][i][1] / arr.entries().next().value[1] /
// map.entries().next().value[1] when the pair value type is T, or from e[1]
// when e is an entry-local pair of value T
// (const e = Object.entries({…})[i] / for (const e of Object.entries({…})) /
// for (const e of arr.entries()) / const e = arr.entries().next().value).
// Outer index must be the number 1 (value slot); inner index any numeric literal.
// Enables Object.entries({k: new A()})[0][1].run() / es[0][1].run() /
// [...Object.entries(...)][0][1].run() / e[1].run() /
// [new A()].entries().next().value[1].run() under foreign same-leaf methods.
// Key slot e[0] fails closed.
func jsObjectEntriesPairValueType(n *grammar.Node, content []byte, typedLocals, factories, entryLocals, entryArrayLocals, entryNextLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string) string {
	return jsObjectEntriesPairValueTypeEx(n, content, typedLocals, factories, entryLocals, entryArrayLocals, entryNextLocals, arrayLocals, mapLocals, setLocals, objValueLocals, jsExtraLocals{})
}

func jsObjectEntriesPairValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, entryLocals, entryArrayLocals, entryNextLocals, arrayLocals, mapLocals, setLocals, objValueLocals map[string]string, extra jsExtraLocals) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" || ingest.NodeText(idx, content) != "1" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	// e[1] after const e = Object.entries(...)[i] / for (const e of Object.entries(...)) /
	// const e = arr.entries().next().value.
	if obj != nil && obj.Type() == "identifier" && entryLocals != nil {
		if t := entryLocals[ingest.NodeText(obj, content)]; t != "" {
			return t
		}
	}
	// arr.entries().next().value[1] / ra.value[1] after entries next result /
	// map.entries().next().value[1] with method-return Map values via extra.
	if t := jsEntriesNextPairTypeEx(obj, content, typedLocals, factories, arrayLocals, entryArrayLocals, entryNextLocals, mapLocals, setLocals, extra); t != "" {
		return t
	}
	// [...ma].at(0)[1] / es.at(i)[1] — pair from entries-array .at (same leaf as es[i][1]).
	if t := jsObjectEntriesPairAtTypeEx(obj, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra); t != "" {
		return t
	}
	return jsObjectEntriesPairSubscriptTypeEx(obj, content, typedLocals, factories, entryArrayLocals, arrayLocals, mapLocals, setLocals, objValueLocals, extra)
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
func jsArrayElemSubscriptType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
	if n == nil || n.Type() != "subscript_expression" {
		return ""
	}
	idx := ingest.ChildByField(n, "index")
	if idx == nil || idx.Type() != "number" {
		return ""
	}
	return jsArraySourceElemType(ingest.ChildByField(n, "object"), content, arrayLocals, typedLocals, factories, extra)
}

// jsIsNumericIndexArg reports whether n is a numeric index expression suitable
// for array element peels: a number literal or unary +/- of a number.
func jsIsNumericIndexArg(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
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
		return false
	}
	if n.Type() == "number" {
		return true
	}
	if n.Type() == "unary_expression" {
		// -1 / +0 — operator must be +/- and argument a number.
		op := ingest.ChildByField(n, "operator")
		arg := ingest.ChildByField(n, "argument")
		if op == nil || arg == nil {
			return false
		}
		ot := ingest.NodeText(op, content)
		return (ot == "-" || ot == "+") && arg.Type() == "number"
	}
	return false
}

// jsIsEmptyArrayLiteral reports [] (no elements). Parenthesized forms accepted.
func jsIsEmptyArrayLiteral(n *grammar.Node) bool {
	if n == nil {
		return false
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
		return false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		el := n.Child(i)
		if el == nil || el.Type() == "[" || el.Type() == "]" || el.Type() == "," {
			continue
		}
		return false
	}
	return true
}

// jsArrayConcatElemType recovers T from arr.concat(…) when the receiver and
// each arg peel to uniform element type T. Empty-array receiver is a wildcard
// (type from args). When selfTarget is set (assignment `xs = xs.concat(…)`),
// an untyped identifier receiver equal to selfTarget is also a wildcard.
// Zero-arg concat is identity of a typed receiver. Mixed inserts fail closed.
func jsArrayConcatElemType(obj, args *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals, selfTarget string) string {
	recvT := jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
	wildRecv := false
	if recvT == "" {
		if jsIsEmptyArrayLiteral(obj) {
			wildRecv = true
		} else if selfTarget != "" && obj != nil && obj.Type() == "identifier" {
			name := ingest.NodeText(obj, content)
			if name == selfTarget && (arrayLocals == nil || arrayLocals[name] == "") {
				wildRecv = true
			}
		}
		if !wildRecv {
			return ""
		}
	}
	// Each arg must be element T or array-source of T (zero args = identity of recv).
	// Empty array literal args are no-ops (same as zero args) so
	// [new A()].concat([])[0].run() / [new BoxA().get()].concat([]) peels.
	// Element peels: Class/typed-local via concrete; method-return via leaf type.
	argT := ""
	sawArg := false
	if args != nil {
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			// [].concat([]) / [new A()].concat([]) — empty array is a no-op insert.
			if jsIsEmptyArrayLiteral(ch) {
				continue
			}
			var t string
			// Class/typed-local; method-return when extra.methodReturns set.
			if ct := jsExprLeafType(ch, content, typedLocals, factories, extra.classFields, extra.methodReturns); ct != "" {
				t = ct
			} else if at := jsArraySourceElemType(ch, content, arrayLocals, typedLocals, factories, extra); at != "" {
				t = at
			} else {
				return ""
			}
			if !sawArg {
				argT = t
				sawArg = true
			} else if argT != t {
				return ""
			}
		}
	}
	if recvT != "" {
		if sawArg && argT != recvT {
			return ""
		}
		return recvT
	}
	// Wildcard receiver — need at least one typed arg.
	if !sawArg {
		return ""
	}
	return argT
}

// jsArrayMutationElemType recovers (local, T) from bare array mutations when
// the receiver is an identifier and inserted/filled values peel to uniform T:
//
//	xs.push(new A()) / xs.unshift(new A()) / xs.push(new A(), new A())
//	xs.push(new BoxA().get()) / xs.unshift(ba.get()) — method-return inserts
//	xs.push(...[new A()]) / xs.push(...as)  — spread of array-source of T
//	xs.splice(0, 0, new A()) / xs.splice(i, d, new A(), new A())
//	xs.fill(new A()) / xs.fill(new A(), 0, 1)
//
// Enables xs[0].run() / for (const a of xs) under foreign same-leaf.
// Mixed types / non-ident receivers / non-array spreads fail closed.
func jsArrayMutationElemType(call *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) (local, classType string) {
	if call == nil || call.Type() != "call_expression" {
		return "", ""
	}
	fn := ingest.ChildByField(call, "function")
	args := ingest.ChildByField(call, "arguments")
	if fn == nil || args == nil {
		return "", ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return "", ""
	}
	prop := ingest.ChildByField(fn, "property")
	obj := ingest.ChildByField(fn, "object")
	if prop == nil || obj == nil || obj.Type() != "identifier" {
		return "", ""
	}
	method := ingest.NodeText(prop, content)
	switch method {
	case "push", "unshift", "splice", "fill":
		// ok
	default:
		return "", ""
	}
	// fill(val[, start[, end]]) — type from first arg only (overwrite).
	if method == "fill" {
		var val *grammar.Node
		pos := 0
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			pos++
			if pos == 1 {
				val = ch
			}
		}
		if pos < 1 || val == nil {
			return "", ""
		}
		// Class() / typed local / factory / method-return (ba.get()).
		t := jsExprLeafType(val, content, typedLocals, factories, extra.classFields, extra.methodReturns)
		if t == "" {
			return "", ""
		}
		return ingest.NodeText(obj, content), t
	}
	// push/unshift: all args are inserts.
	// splice(start, deleteCount, ...items): items after the first two.
	skip := 0
	if method == "splice" {
		skip = 2
	}
	found := ""
	saw := false
	pos := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		pos++
		if pos <= skip {
			continue
		}
		var t string
		if ch.Type() == "spread_element" {
			// xs.push(...[new A()]) / xs.push(...as) — spread of array-source of T.
			var arg *grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				c := ch.Child(j)
				if c == nil || c.Type() == "..." {
					continue
				}
				arg = c
				break
			}
			if arg == nil {
				return "", ""
			}
			t = jsArraySourceElemType(arg, content, arrayLocals, typedLocals, factories, extra)
		} else {
			// Class() / typed local / factory / method-return (new BoxA().get()).
			t = jsExprLeafType(ch, content, typedLocals, factories, extra.classFields, extra.methodReturns)
		}
		if t == "" {
			return "", ""
		}
		if !saw {
			found = t
			saw = true
		} else if found != t {
			return "", ""
		}
	}
	if !saw {
		return "", ""
	}
	return ingest.NodeText(obj, content), found
}

// jsSetAddMutationElemType recovers (local, T) from bare Set mutations when the
// receiver is an identifier and the inserted value peels to concrete T:
//
//	xs.add(new A()) / xs.add(a) after const a = new A()
//	xs.add(new BoxA().get()) / xs.add(ba.get()) — method-return inserts
//
// Enables for (const a of xs) / xs.values().next().value.run() under foreign
// same-leaf after const xs = new Set() / empty ctor. Non-ident receivers /
// non-concrete args fail closed. Multi-arg add is not a Set method shape.
// Method-return peels use jsSetAddMutationElemTypeEx.
func jsSetAddMutationElemType(call *grammar.Node, content []byte, typedLocals, factories map[string]string) (local, classType string) {
	return jsSetAddMutationElemTypeEx(call, content, typedLocals, factories, nil, nil)
}

// jsSetAddMutationElemTypeEx also peels xs.add(new BoxA().get()) / xs.add(ba.get())
// method-return values under foreign same-leaf.
func jsSetAddMutationElemTypeEx(call *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) (local, classType string) {
	if call == nil || call.Type() != "call_expression" {
		return "", ""
	}
	fn := ingest.ChildByField(call, "function")
	args := ingest.ChildByField(call, "arguments")
	if fn == nil || args == nil {
		return "", ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return "", ""
	}
	prop := ingest.ChildByField(fn, "property")
	obj := ingest.ChildByField(fn, "object")
	if prop == nil || obj == nil || obj.Type() != "identifier" {
		return "", ""
	}
	if ingest.NodeText(prop, content) != "add" {
		return "", ""
	}
	var val *grammar.Node
	pos := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		pos++
		if pos == 1 {
			val = ch
		}
	}
	if pos != 1 || val == nil {
		return "", ""
	}
	t := jsExprLeafType(val, content, typedLocals, factories, classFields, methodReturns)
	if t == "" {
		return "", ""
	}
	return ingest.NodeText(obj, content), t
}

// jsMapSetMutationElemType recovers (local, T) from bare Map/WeakMap mutations
// when the receiver is an identifier and the value peels to concrete T:
//
// m.set(k, new A()) / wm.set(k, new A()) — bare Map/WeakMap mutation.
//
// Enables m.get(k).run() / [...m.values()][0].run() under foreign same-leaf
// after const m = new Map() / new WeakMap(). Key shape free; non-ident
// receivers / missing value / non-concrete value fail closed.
// Method-return peels use jsMapSetMutationElemTypeEx.
func jsMapSetMutationElemType(call *grammar.Node, content []byte, typedLocals, factories map[string]string) (local, classType string) {
	return jsMapSetMutationElemTypeEx(call, content, typedLocals, factories, nil, nil)
}

// jsMapSetMutationElemTypeEx also peels m.set(k, new BoxA().get()) /
// m.set(k, ba.get()) method-return values under foreign same-leaf.
func jsMapSetMutationElemTypeEx(call *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) (local, classType string) {
	if call == nil || call.Type() != "call_expression" {
		return "", ""
	}
	fn := ingest.ChildByField(call, "function")
	args := ingest.ChildByField(call, "arguments")
	if fn == nil || args == nil {
		return "", ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return "", ""
	}
	prop := ingest.ChildByField(fn, "property")
	obj := ingest.ChildByField(fn, "object")
	if prop == nil || obj == nil || obj.Type() != "identifier" {
		return "", ""
	}
	if ingest.NodeText(prop, content) != "set" {
		return "", ""
	}
	var val *grammar.Node
	pos := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		pos++
		if pos == 2 {
			val = ch
		}
	}
	if pos < 2 || val == nil {
		return "", ""
	}
	t := jsExprLeafType(val, content, typedLocals, factories, classFields, methodReturns)
	if t == "" {
		return "", ""
	}
	return ingest.NodeText(obj, content), t
}

// jsObjectLiteralValueType recovers uniform property-value type T from a plain
// object literal {k: new T() / a, …}. Empty objects fail closed. Method /
// spread / nested-array property values fail closed (nested arrays use
// jsObjectLiteralNestedArrayElemType). Enables const oa = {k: new A()};
// Object.values(oa)[0].run() / oa.k.run() under foreign same-leaf.
func jsObjectLiteralValueType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsObjectLiteralValueTypeEx(n, content, typedLocals, factories, jsExtraLocals{})
}

// jsObjectLiteralValueTypeEx is jsObjectLiteralValueType with method-return peels
// (new BoxA().get() → A) via jsExprLeafType when extra.methodReturns is set.
// Enables const {k: a} = {k: new BoxA().get()} under foreign same-leaf.
func jsObjectLiteralValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if n == nil || n.Type() != "object" {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil || ch.Type() == "{" || ch.Type() == "}" || ch.Type() == "," {
			continue
		}
		var val *grammar.Node
		switch ch.Type() {
		case "pair":
			val = ingest.ChildByField(ch, "value")
		case "shorthand_property_identifier":
			val = ch
		default:
			// method_definition / spread_element — fail closed.
			return ""
		}
		if val == nil {
			return ""
		}
		t := ""
		if val.Type() == "shorthand_property_identifier" {
			if typedLocals != nil {
				t = typedLocals[ingest.NodeText(val, content)]
			}
		} else if extra.methodReturns != nil || extra.classFields != nil {
			t = jsExprLeafType(val, content, typedLocals, factories, extra.classFields, extra.methodReturns)
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

// jsArrayAssignSourceElemType recovers T from an assignment RHS used to rebind
// an array local: [new A()] / [...xs, new A()] / xs.concat([new A()]) /
// [].concat(new A()) / xs.toSpliced(0,0,new A()) / [].toSpliced(0,0,new A()) /
// xs.with(0, new A()) / [].with(0, new A()) with selfTarget wildcards for
// untyped self arms. Foreign leaves returned too so dual-class B rebinds shadow A.
func jsArrayAssignSourceElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals, selfTarget string) string {
	if n == nil {
		return ""
	}
	// Prefer ordinary array-source peels (already-typed spreads/concats/literals).
	if t := jsArraySourceElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	// Spread with self-target / empty wildcards (xs = [...xs, new A()]).
	if t := jsArraySpreadElemTypeSelf(n, content, arrayLocals, typedLocals, factories, extra, selfTarget); t != "" {
		return t
	}
	// Concat / toSpliced / with with self-target / empty receiver
	// (xs = xs.concat([new A()]) / xs = xs.toSpliced(0,0,new A()) /
	//  xs = xs.with(0, new A())).
	if n.Type() == "call_expression" {
		fn := ingest.ChildByField(n, "function")
		if fn != nil && (fn.Type() == "member_expression" || fn.Type() == "member_expression_optional" || fn.Type() == "optional_chain") {
			if prop := ingest.ChildByField(fn, "property"); prop != nil {
				switch ingest.NodeText(prop, content) {
				case "concat":
					return jsArrayConcatElemType(ingest.ChildByField(fn, "object"), ingest.ChildByField(n, "arguments"), content, arrayLocals, typedLocals, factories, extra, selfTarget)
				case "toSpliced":
					return jsArrayToSplicedElemType(ingest.ChildByField(fn, "object"), ingest.ChildByField(n, "arguments"), content, arrayLocals, typedLocals, factories, extra, selfTarget)
				case "with":
					return jsArrayWithElemType(ingest.ChildByField(fn, "object"), ingest.ChildByField(n, "arguments"), content, arrayLocals, typedLocals, factories, extra, selfTarget)
				}
			}
		}
	}
	return ""
}

// jsArrayToSplicedElemType recovers T from arr.toSpliced(start, deleteCount, ...items)
// when the receiver and each insert peel to uniform element type T. Empty-array
// receiver is a wildcard (type from inserts). Nullish-only receivers
// ([null] / [undefined]) are also wildcards — same overwrite/replace peel as
// arr.with (e.g. [null].toSpliced(0,1,new A())[0]). When selfTarget is set
// (assignment `xs = xs.toSpliced(…)`), an untyped identifier receiver equal to
// selfTarget is also a wildcard. Zero-item toSpliced is identity of a typed
// receiver only. Mixed inserts fail closed.
//
// Insert peels via jsExprLeafType so method-return args work under foreign same-leaf:
//
//	[null].toSpliced(0, 1, new BoxA().get())[0].run()
//
// (Class peels via new A() already worked; with/fill use the same leaf path.)
func jsArrayToSplicedElemType(obj, args *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals, selfTarget string) string {
	recvT := jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
	wildRecv := false
	if recvT == "" {
		if jsIsEmptyArrayLiteral(obj) {
			wildRecv = true
		} else if jsArrayIsNullishOnly(obj, content) {
			// [null].toSpliced(0,1,new A()) / [undefined].toSpliced(...) —
			// nullish slots are wildcards (overwrite/replace like with/fill).
			wildRecv = true
		} else if selfTarget != "" && obj != nil && obj.Type() == "identifier" {
			name := ingest.NodeText(obj, content)
			if name == selfTarget && (arrayLocals == nil || arrayLocals[name] == "") {
				wildRecv = true
			}
		}
		if !wildRecv {
			return ""
		}
	}
	// toSpliced(start, deleteCount, ...items) — items after first two positionals.
	itemT := ""
	sawItem := false
	if args != nil {
		pos := 0
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			pos++
			if pos <= 2 {
				continue
			}
			// Class/typed-local via concrete path; method-return when
			// extra.methodReturns set (same leaf as arr.with / arr.fill).
			t := jsExprLeafType(ch, content, typedLocals, factories, extra.classFields, extra.methodReturns)
			if t == "" {
				return ""
			}
			if !sawItem {
				itemT = t
				sawItem = true
			} else if itemT != t {
				return ""
			}
		}
	}
	if recvT != "" {
		if sawItem && itemT != recvT {
			return ""
		}
		return recvT
	}
	// Wildcard receiver — need at least one typed insert.
	if !sawItem {
		return ""
	}
	return itemT
}

// jsArrayWithElemType recovers T from arr.with(i, val) when the receiver and
// val peel to uniform element type T. Empty-array receiver is a wildcard (type
// from val). When selfTarget is set (assignment `xs = xs.with(…)`), an untyped
// identifier receiver equal to selfTarget is also a wildcard. Numeric index
// required; non-concrete val / mixed types fail closed.
//
// Val peels via jsExprLeafType so method-return args work under foreign same-leaf:
//
//	[].with(0, new BoxA().get())[0].run()
//	[new BoxA().get()].with(0, new BoxA().get())[0].run()
//
// (Class peels via new A() already worked; fill uses the same leaf path.)
func jsArrayWithElemType(obj, args *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals, selfTarget string) string {
	recvT := jsArraySourceElemType(obj, content, arrayLocals, typedLocals, factories, extra)
	wildRecv := false
	if recvT == "" {
		if jsIsEmptyArrayLiteral(obj) {
			wildRecv = true
		} else if jsArrayIsNullishOnly(obj, content) {
			// [null].with(0, new A()) / [undefined].with(i, new A()) —
			// nullish slots are wildcards (overwrite semantics like fill).
			wildRecv = true
		} else if selfTarget != "" && obj != nil && obj.Type() == "identifier" {
			name := ingest.NodeText(obj, content)
			if name == selfTarget && (arrayLocals == nil || arrayLocals[name] == "") {
				wildRecv = true
			}
		}
		if !wildRecv {
			return ""
		}
	}
	var idx, val *grammar.Node
	pos := 0
	if args != nil {
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			pos++
			if pos == 1 {
				idx = ch
			} else if pos == 2 {
				val = ch
			}
		}
	}
	if pos != 2 || idx == nil || val == nil || !jsIsNumericIndexArg(idx, content) {
		return ""
	}
	// Class/typed-local via concrete path; method-return when extra.methodReturns set
	// (same leaf as arr.fill(new BoxA().get())).
	valT := jsExprLeafType(val, content, typedLocals, factories, extra.classFields, extra.methodReturns)
	if valT == "" {
		return ""
	}
	if recvT != "" {
		if valT != recvT {
			return ""
		}
		return recvT
	}
	return valT
}

// jsArrayIndexAssignElemType recovers (local, T) from xs[i] = new T() when the
// left is a numeric-index subscript of an identifier and the RHS peels to a
// concrete class leaf. Enables xs[0].run() / for (const a of xs) under foreign
// same-leaf after index assign. Non-numeric index / non-ident receiver /
// non-concrete RHS fail closed. Method-return peels use jsArrayIndexAssignElemTypeEx.
func jsArrayIndexAssignElemType(assign *grammar.Node, content []byte, typedLocals, factories map[string]string) (local, classType string) {
	return jsArrayIndexAssignElemTypeEx(assign, content, typedLocals, factories, nil, nil)
}

// jsArrayIndexAssignElemTypeEx also peels xs[i] = new BoxA().get() / xs[i] = ba.get()
// method-return RHS under foreign same-leaf.
func jsArrayIndexAssignElemTypeEx(assign *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) (local, classType string) {
	if assign == nil || assign.Type() != "assignment_expression" {
		return "", ""
	}
	left := ingest.ChildByField(assign, "left")
	right := ingest.ChildByField(assign, "right")
	if left == nil || right == nil || left.Type() != "subscript_expression" {
		return "", ""
	}
	obj := ingest.ChildByField(left, "object")
	idx := ingest.ChildByField(left, "index")
	if obj == nil || obj.Type() != "identifier" || !jsIsNumericIndexArg(idx, content) {
		return "", ""
	}
	// Class() / typed local / factory / method-return (new BoxA().get()).
	t := jsExprLeafType(right, content, typedLocals, factories, classFields, methodReturns)
	if t == "" {
		return "", ""
	}
	return ingest.NodeText(obj, content), t
}

// jsArrayPopShiftElemType recovers T from arr.pop() / arr.shift() when the
// receiver peels to a uniform array element type T. Zero-arg only.
// Enables [new A()].pop().run() / as.shift().run() under foreign same-leaf methods.
func jsArrayPopShiftElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil {
		return ""
	}
	name := ingest.NodeText(prop, content)
	if name != "pop" && name != "shift" {
		return ""
	}
	return jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
}

// jsArrayAtElemType recovers T from arr.at(i) / Array.from([new T()]).at(i) /
// Array.of(new T()).at(i) / Object.values({k: new T()}).at(i) when the receiver
// peels to a uniform array element type T. Single numeric arg only (incl. -1).
// Enables [new A()].at(0).run() under foreign same-leaf methods.
func jsArrayAtElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "at" {
		return ""
	}
	// Exactly one positional numeric arg.
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
	if count != 1 || first == nil || !jsIsNumericIndexArg(first, content) {
		return ""
	}
	return jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
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
func jsArrayIteratorYieldType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
		return jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
	case "subscript_expression":
		// arr[Symbol.iterator]()
		if !jsIsSymbolIteratorIndex(ingest.ChildByField(fn, "index"), content) {
			return ""
		}
		return jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
	}
	return ""
}

// jsIteratorSourceYieldType recovers T from a generator call/local or an array
// iterator call (arr.values() / arr[Symbol.iterator]()) / array-iterator local /
// map.values() / set.values() / set.keys() / set[Symbol.iterator]() /
// iter.take(n) / iter.drop(n) / iter.filter(pred) when the source peels to
// uniform element/value type T.
func jsIteratorSourceYieldType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string) string {
	return jsIteratorSourceYieldTypeEx(n, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, nil, nil)
}

// jsIteratorSourceYieldTypeEx threads methodReturns for Iterator.from / Map.values
// method-return peels under foreign same-leaf.
func jsIteratorSourceYieldTypeEx(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if t := jsGeneratorCallYieldType(n, content, generators, genLocals); t != "" {
		return t
	}
	if t := jsIteratorFromYieldType(n, content, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns); t != "" {
		// Iterator.from([new A()]) / Iterator.from([new BoxA().get()]) — iterable peel.
		return t
	}
	if t := jsArrayIteratorYieldType(n, content, arrayLocals, typedLocals, factories, jsExtraLocals{classFields: classFields, methodReturns: methodReturns}); t != "" {
		return t
	}
	if t := jsMapValuesYieldTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsSetIteratorYieldTypeEx(n, content, arrayLocals, typedLocals, factories, setLocals, jsExtraLocals{
		setLocals: setLocals, classFields: classFields, methodReturns: methodReturns,
	}); t != "" {
		return t
	}
	// arr.values().take(1) / .drop(0) / .filter(pred) / flatMap — identity iterator
	// helpers (method-return via classFields/methodReturns).
	return jsIteratorHelperYieldType(n, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
}

// jsForOfElemType recovers T from the right-hand side of for…of / for await…of:
// generator call/local, array literal, array local, array iterator call,
// map.values(), or set.values()/keys() when source peels to uniform type T.
func jsForOfElemType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, extra jsExtraLocals) string {
	if t := jsArraySourceElemType(n, content, arrayLocals, typedLocals, factories, extra); t != "" {
		return t
	}
	return jsIteratorSourceYieldType(n, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals)
}

// jsGeneratorNextResultType recovers T from genA().next() / ga.next() /
// await agenA().next() / [new A()].values().next() /
// [new A()][Symbol.iterator]().next() / ia.next() /
// map.values().next() / set.values().next() / set.keys().next() when the
// receiver peels to an iterator yielding T.
// Only zero-arg .next() peels (fail closed on arguments).
func jsGeneratorNextResultType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string) string {
	return jsGeneratorNextResultTypeEx(n, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, nil, nil)
}

// jsGeneratorNextResultTypeEx peels Iterator.from([ba.get()]).next() /
// set.keys().next() method-return yields under foreign same-leaf.
func jsGeneratorNextResultTypeEx(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	return jsIteratorSourceYieldTypeEx(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
}

// jsGeneratorNextValueType recovers T from genA().next().value /
// ga.next().value / (await agenA().next()).value /
// [new A()].values().next().value / [new A()][Symbol.iterator]().next().value /
// map.values().next().value / set.values().next().value / set.keys().next().value.
// ra.value is handled via settledOf in jsPromiseAllSettledValueType.
// Property must be bare "value".
func jsGeneratorNextValueType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string) string {
	return jsGeneratorNextValueTypeEx(n, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, nil, nil)
}

// jsGeneratorNextValueTypeEx peels Iterator.from([ba.get()]).next().value /
// set.keys().next().value method-return yields under foreign same-leaf.
func jsGeneratorNextValueTypeEx(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	return jsGeneratorNextResultTypeEx(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
}

// jsStructuredCloneArrayElemType recovers T from structuredClone(arr) when arr
// peels as an array source of T. Enables structuredClone([new A()])[0].run() /
// structuredClone(as)[0].run() under foreign same-leaf. Non-array args fail closed
// (scalar structuredClone stays on jsIdentityCloneType).
func jsStructuredCloneArrayElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	args := ingest.ChildByField(n, "arguments")
	if fn == nil || args == nil || fn.Type() != "identifier" ||
		ingest.NodeText(fn, content) != "structuredClone" {
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
	return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories, extra)
}

// jsIteratorHelperYieldType recovers T from iter.take(n) / iter.drop(n) /
// iter.filter(pred) / iter.map(x => x) / iter.flatMap(() => [new T()]) when the
// receiver peels as an iterator yielding T (or flatMap sole-return peels T).
// Limit / predicate args ignored (not type-changing). Unknown receivers fail closed.
// methodReturns peels Iterator.from([new BoxA().get()]).flatMap(x => [x]).
func jsIteratorHelperYieldType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if n == nil || n.Type() != "call_expression" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil {
		return ""
	}
	name := ingest.NodeText(prop, content)
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return ""
	}
	var firstArg *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if firstArg == nil {
			firstArg = ch
		}
	}
	if count < 1 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	switch name {
	case "take", "drop", "filter":
		// ok — identity iterator helpers (predicate/limit ignored)
	case "map":
		// iter.map(x => x) identity only — non-identity map changes yield type.
		if firstArg == nil || !jsIsIdentityCallback(firstArg, content) {
			return ""
		}
	case "flatMap":
		// iter.flatMap(x => [x]) — one-level flatten of yield T → T
		// iter.flatMap(() => [new T()]) — sole-return array of uniform T
		// (receiver ignored for sole-return type; same leaf as Array.flatMap sole return).
		// Array.prototype.flatMap stays on jsArrayIdentityElemType so
		// const as = [0].flatMap(() => [new A()]) binds as arrayLocals (as[0].run()).
		// methodReturns peels [new BoxA().get()].values().flatMap(x => [x]) and
		// iter.flatMap(() => [new BoxA().get()]) under foreign same-leaf.
		if firstArg == nil {
			return ""
		}
		if !jsLooksLikeIteratorReceiver(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns) {
			return ""
		}
		if jsIsIdentityArrayCallback(firstArg, content) {
			return jsIteratorSourceYieldTypeEx(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
		}
		if t := jsFlatMapSoleArrayReturnElemType(firstArg, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return t
		}
		return ""
	default:
		return ""
	}
	return jsIteratorSourceYieldTypeEx(obj, content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
}

// jsLooksLikeIteratorReceiver reports whether n is an iterator source (not a
// plain array). Used to keep Array.flatMap on the array-identity path while
// Iterator.from(...).flatMap / values().flatMap peels as iterator helpers.
// Structural: Iterator.from, generator/array-iterator/map.values/set locals,
// identity iterator helpers — even when the yield leaf is non-class (numbers).
// methodReturns peels [new BoxA().get()].values() under foreign same-leaf.
func jsLooksLikeIteratorReceiver(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) bool {
	if n == nil {
		return false
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
		return false
	}
	// Bound iterator local (const ia = Iterator.from(...)/arr.values()/…).
	if n.Type() == "identifier" && genLocals != nil {
		if genLocals[ingest.NodeText(n, content)] != "" {
			return true
		}
	}
	// Iterator.from(...) — structural even when iterable elems are non-class.
	if jsIsIteratorFromCall(n, content) {
		return true
	}
	// arr.values() / map.values() / set.values() / generator call — yield peels.
	if t := jsGeneratorCallYieldType(n, content, generators, genLocals); t != "" {
		return true
	}
	if t := jsArrayIteratorYieldType(n, content, arrayLocals, typedLocals, factories, jsExtraLocals{classFields: classFields, methodReturns: methodReturns}); t != "" {
		return true
	}
	if t := jsMapValuesYieldTypeEx(n, content, typedLocals, factories, mapLocals, entryArrayLocals, classFields, methodReturns); t != "" {
		return true
	}
	if t := jsSetIteratorYieldTypeEx(n, content, arrayLocals, typedLocals, factories, setLocals, jsExtraLocals{
		setLocals: setLocals, classFields: classFields, methodReturns: methodReturns,
	}); t != "" {
		return true
	}
	// Chained identity helpers: iter.take(n).flatMap(...).
	if n.Type() == "call_expression" {
		fn := ingest.ChildByField(n, "function")
		if fn != nil && (fn.Type() == "member_expression" || fn.Type() == "member_expression_optional" || fn.Type() == "optional_chain") {
			prop := ingest.ChildByField(fn, "property")
			if prop != nil {
				switch ingest.NodeText(prop, content) {
				case "take", "drop", "filter", "map", "flatMap":
					return jsLooksLikeIteratorReceiver(ingest.ChildByField(fn, "object"), content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
				}
			}
		}
	}
	return false
}

// jsIsIteratorFromCall reports Iterator.from(...) structurally (args ignored).
func jsIsIteratorFromCall(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "call_expression" {
		return false
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return false
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil {
		return false
	}
	return obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Iterator" &&
		ingest.NodeText(prop, content) == "from"
}

// jsIteratorFindElemType recovers T from iter.find(pred) / iter.findLast(pred)
// when the receiver peels as an iterator yielding T. Predicate ignored (not
// type-changing for uniform yields). Enables
// Iterator.from([new A()]).find(x => true).run() /
// Iterator.from([new BoxA().get()]).find(x => true).run() under foreign same-leaf.
func jsIteratorFindElemType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil {
		return ""
	}
	name := ingest.NodeText(prop, content)
	if name != "find" && name != "findLast" {
		return ""
	}
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
	}
	if count < 1 {
		return ""
	}
	return jsIteratorSourceYieldTypeEx(ingest.ChildByField(fn, "object"), content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
}

// jsIteratorReduceElemType recovers T from iter.reduce((a,x) => x) /
// iter.reduce((a,x) => a, init) when the receiver peels as an iterator yielding T
// and the callback is identity of first or second formal (same shapes as
// Array.prototype.reduce). Enables Iterator.from([new A()]).reduce((a,x) => x).run()
// under foreign same-leaf. Non-identity reducers fail closed.
func jsIteratorReduceElemType(n *grammar.Node, content []byte, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "reduce" {
		return ""
	}
	var cb, init *grammar.Node
	pos := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		pos++
		if pos == 1 {
			cb = ch
		} else if pos == 2 {
			init = ch
		}
	}
	if pos < 1 || pos > 2 || cb == nil {
		return ""
	}
	idx := jsCallbackReturnedParamIndex(cb, content)
	if idx != 0 && idx != 1 {
		return ""
	}
	t := jsIteratorSourceYieldTypeEx(ingest.ChildByField(fn, "object"), content, generators, genLocals, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals, classFields, methodReturns)
	if t == "" {
		if init != nil && idx == 0 {
			if it := jsExprLeafType(init, content, typedLocals, factories, classFields, methodReturns); it != "" {
				return it
			}
		}
		return ""
	}
	if init != nil && jsExprLeafType(init, content, typedLocals, factories, classFields, methodReturns) != t {
		return ""
	}
	return t
}

// jsReflectConstructType recovers T from Reflect.construct(T, args) when the
// first positional arg is a bare class identifier T. Args ignored for typing.
// Enables Reflect.construct(A, []).run() / const a = Reflect.construct(A, [])
// under foreign same-leaf. Non-Reflect / non-identifier target fail closed.
func jsReflectConstructType(n *grammar.Node, content []byte) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Reflect" ||
		ingest.NodeText(prop, content) != "construct" {
		return ""
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
	if first == nil || first.Type() != "identifier" {
		return ""
	}
	name := ingest.NodeText(first, content)
	if name == "" || name == "_" {
		return ""
	}
	return name
}

// jsPromiseTryType recovers T from Promise.try(() => new T()) / Promise.try(() => a)
// when the sole callback return peels to concrete T (new / local / factory).
// Enables Promise.try(() => new A()).then(x => x.run()) under foreign same-leaf.
// Multi-arg / non-Promise / non-sole-return fail closed.
// Method-return peels use jsPromiseTryTypeEx.
func jsPromiseTryType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsPromiseTryTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsPromiseTryTypeEx also peels Promise.try(() => new BoxA().get()) /
// Promise.try(() => ba.get()) method-return callback bodies under foreign same-leaf.
func jsPromiseTryTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
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
		return jsPromiseTryTypeEx(arg, content, typedLocals, factories, classFields, methodReturns)
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
	if fn == nil || args == nil || fn.Type() != "member_expression" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil ||
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Promise" ||
		ingest.NodeText(prop, content) != "try" {
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
	ret := jsCallbackSoleReturnExpr(first, content)
	if ret == nil {
		return ""
	}
	return jsExprLeafType(ret, content, typedLocals, factories, classFields, methodReturns)
}

// jsIteratorToArrayElemType recovers T from iter.toArray() when the receiver
// peels as an iterator yielding T. Zero-arg only. Enables
// aa.values().toArray()[0].run() / aa.values().take(1).toArray()[0].run() under
// foreign same-leaf methods.
func jsIteratorToArrayElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "toArray" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	return jsIteratorSourceYieldTypeEx(obj, content, nil, nil, arrayLocals, typedLocals, factories, extra.mapLocals, extra.entryArrayLocals, extra.setLocals, extra.classFields, extra.methodReturns)
}

// jsStructuredCloneObjectPropType recovers T from structuredClone({k: new T()}).k /
// structuredClone(oa).k / structuredClone({k: new T()})["k"] when the clone arg
// peels as a uniform-value object of T (literal / objValue local). Non-clone /
// non-prop / mixed values fail closed. Class()-only peels (nil methodReturns).
func jsStructuredCloneObjectPropType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsStructuredCloneObjectPropTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsStructuredCloneObjectPropTypeEx peels structuredClone({k: new BoxA().get()}).k
// method-return props under foreign same-leaf (Class peels via non-Ex path).
func jsStructuredCloneObjectPropTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
		return ""
	}
	var obj *grammar.Node
	switch n.Type() {
	case "member_expression", "member_expression_optional", "optional_chain", "subscript_expression":
		obj = ingest.ChildByField(n, "object")
	default:
		return ""
	}
	if obj == nil {
		return ""
	}
	// structuredClone(x) — peel await/parens via jsStructuredCloneObjectValueTypeEx.
	return jsStructuredCloneObjectValueTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns)
}

// jsStructuredCloneObjectValueType recovers uniform property value T from
// structuredClone(objectLiteral | objValueLocal). Scalar clone peels stay on
// jsIdentityCloneType. Non-object args fail closed. Class()-only peels.
func jsStructuredCloneObjectValueType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsStructuredCloneObjectValueTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsStructuredCloneObjectValueTypeEx peels structuredClone({k: new BoxA().get()})
// method-return object literals under foreign same-leaf.
func jsStructuredCloneObjectValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
		return ""
	}
	// await structuredClone(...)
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
		return jsStructuredCloneObjectValueTypeEx(arg, content, typedLocals, factories, objValueLocals, classFields, methodReturns)
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
	if fn.Type() != "identifier" || ingest.NodeText(fn, content) != "structuredClone" {
		return ""
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
	if first == nil {
		return ""
	}
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
	if first == nil {
		return ""
	}
	if first.Type() == "object" {
		return jsObjectLiteralValueTypeEx(first, content, typedLocals, factories, jsExtraLocals{
			classFields:   classFields,
			methodReturns: methodReturns,
		})
	}
	if first.Type() == "identifier" && objValueLocals != nil {
		return objValueLocals[ingest.NodeText(first, content)]
	}
	return ""
}

// jsIdentityCloneType recovers T from structuredClone(x) / Object.assign(x[, …]) /
// Object.create(x) / Object.freeze(x) / Object.seal(x) / Object.preventExtensions(x)
// when the first positional arg peels to T (new T() / typed local / factory /
// Class.prototype for create). structuredClone returns a structured copy;
// Object.assign returns its first argument (target); freeze/seal/preventExtensions
// return the same object; Object.create returns an object whose [[Prototype]] is
// the first arg (identity for method peels when the proto is an instance /
// Class.prototype). Extra assign sources ignored. Object.create with property
// descriptors (2nd arg) fails closed. Non-matching callees / missing first arg
// fail closed.
func jsIdentityCloneType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsIdentityCloneTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsIdentityCloneTypeEx peels structuredClone(new BoxA().get()) / Object.assign /
// freeze/seal with method-return first args under foreign same-leaf.
func jsIdentityCloneTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		return jsIdentityCloneTypeEx(arg, content, typedLocals, factories, classFields, methodReturns)
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
	isCreate := false
	switch fn.Type() {
	case "identifier":
		// structuredClone(x)
		if ingest.NodeText(fn, content) == "structuredClone" {
			ok = true
		}
	case "member_expression", "member_expression_optional", "optional_chain":
		// Object.assign(x[, …]) / Object.create(x) /
		// Object.freeze(x) / Object.seal(x) / Object.preventExtensions(x)
		prop := ingest.ChildByField(fn, "property")
		obj := ingest.ChildByField(fn, "object")
		if prop != nil && obj != nil &&
			obj.Type() == "identifier" && ingest.NodeText(obj, content) == "Object" {
			switch ingest.NodeText(prop, content) {
			case "assign", "freeze", "seal", "preventExtensions":
				ok = true
			case "create":
				ok = true
				isCreate = true
			}
		}
	}
	if !ok {
		return ""
	}
	// First positional argument; Object.create rejects a second (props) arg.
	var first *grammar.Node
	posCount := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		posCount++
		if first == nil {
			first = ch
		}
	}
	if first == nil {
		return ""
	}
	if isCreate && posCount != 1 {
		return ""
	}
	// Class/typed-local peels; method-return when methodReturns set.
	if t := jsExprLeafType(first, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	// Object.create(A.prototype) — prototype of Class is identity for method peels.
	if isCreate {
		return jsPrototypeClassType(first, content)
	}
	return ""
}

// jsIdentityObjectPropType recovers T from new Proxy({k: new A()}, {}).k /
// Object.freeze({k: new A()}).k / Object.create(oa).k / Object.seal(oa)["k"] when
// the wrapper peels via jsIdentityObjectValueType to uniform property value T.
// Scalar identity peels stay on jsIdentityCloneType / jsProxyTargetType.
func jsIdentityObjectPropType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsIdentityObjectPropTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsIdentityObjectPropTypeEx peels Object.create({k: ba.get()}).k method-return
// props under foreign same-leaf (Class peels via non-Ex path).
func jsIdentityObjectPropTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
		return ""
	}
	var obj *grammar.Node
	switch n.Type() {
	case "member_expression", "member_expression_optional", "optional_chain", "subscript_expression":
		obj = ingest.ChildByField(n, "object")
	default:
		return ""
	}
	if obj == nil {
		return ""
	}
	return jsIdentityObjectValueTypeEx(obj, content, typedLocals, factories, objValueLocals, classFields, methodReturns)
}

// jsIdentityObjectValueType recovers uniform property value T from
// new Proxy(objectLiteral | objValueLocal) / Object.freeze|seal|preventExtensions|create
// of the same. Object.assign stays on jsObjectAssignValueType; structuredClone on
// jsStructuredCloneObjectValueType. Object.create with property descriptors (2nd
// arg) fails closed. Non-object first args fail closed (scalar identity peels
// stay on jsIdentityCloneType / jsProxyTargetType).
func jsIdentityObjectValueType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsIdentityObjectValueTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsIdentityObjectValueTypeEx peels Object.create({k: ba.get()}) method-return
// object literals under foreign same-leaf.
func jsIdentityObjectValueTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	var first *grammar.Node
	switch n.Type() {
	case "new_expression":
		// new Proxy(target[, handler])
		ctor := ingest.ChildByField(n, "constructor")
		if ctor == nil || ctor.Type() != "identifier" || ingest.NodeText(ctor, content) != "Proxy" {
			return ""
		}
		args := ingest.ChildByField(n, "arguments")
		if args == nil {
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch != nil && ch.Type() == "arguments" {
					args = ch
					break
				}
			}
		}
		if args == nil {
			return ""
		}
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			first = ch
			break
		}
	case "call_expression":
		// Object.freeze/seal/preventExtensions/create(x)
		fn := ingest.ChildByField(n, "function")
		args := ingest.ChildByField(n, "arguments")
		if fn == nil || args == nil {
			return ""
		}
		if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
			return ""
		}
		prop := ingest.ChildByField(fn, "property")
		obj := ingest.ChildByField(fn, "object")
		if prop == nil || obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Object" {
			return ""
		}
		isCreate := false
		switch ingest.NodeText(prop, content) {
		case "freeze", "seal", "preventExtensions":
		case "create":
			isCreate = true
		default:
			return ""
		}
		posCount := 0
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			posCount++
			if first == nil {
				first = ch
			}
		}
		if isCreate && posCount != 1 {
			// Object.create(proto, props) — descriptors fail closed.
			return ""
		}
	default:
		return ""
	}
	if first == nil {
		return ""
	}
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
	if first == nil {
		return ""
	}
	if first.Type() == "object" {
		return jsObjectLiteralValueTypeEx(first, content, typedLocals, factories, jsExtraLocals{
			classFields: classFields, methodReturns: methodReturns,
		})
	}
	if first.Type() == "identifier" && objValueLocals != nil {
		return objValueLocals[ingest.NodeText(first, content)]
	}
	return ""
}

// jsWeakRefTargetType recovers T from new WeakRef(x) when x peels to T
// (new T() / typed local / factory). Used to bind WeakRef holder locals so
// wa.deref() peels under foreign same-leaf. Non-WeakRef / missing target fail closed.
func jsWeakRefTargetType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsWeakRefTargetTypeEx(n, content, typedLocals, factories, nil, nil)
}

func jsWeakRefTargetTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if n == nil || n.Type() != "new_expression" {
		return ""
	}
	ctor := ingest.ChildByField(n, "constructor")
	if ctor == nil || ctor.Type() != "identifier" || ingest.NodeText(ctor, content) != "WeakRef" {
		return ""
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch != nil && ch.Type() == "arguments" {
				args = ch
				break
			}
		}
	}
	if args == nil {
		return ""
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
	if first == nil {
		return ""
	}
	return jsExprLeafType(first, content, typedLocals, factories, classFields, methodReturns)
}

// jsWeakRefDerefType recovers T from new WeakRef(x).deref() when x peels to T
// (new T() / typed local / factory / method-return), or wa.deref() after const wa = new WeakRef(...)
// (referent stored under genLocals["@weakref."+name]). Zero-arg deref only.
// Enables new WeakRef(new A()).deref().run() / new WeakRef(new BoxA().get()).deref().run() /
// wa.deref().run() under foreign same-leaf methods.
func jsWeakRefDerefType(n *grammar.Node, content []byte, typedLocals, factories, genLocals map[string]string) string {
	return jsWeakRefDerefTypeEx(n, content, typedLocals, factories, genLocals, nil, nil)
}

// jsWeakRefDerefTypeEx peels WeakRef.deref with method-return targets.
func jsWeakRefDerefTypeEx(n *grammar.Node, content []byte, typedLocals, factories, genLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "deref" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
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
	// wa.deref() after const wa = new WeakRef(...) — referent via @weakref map.
	if obj.Type() == "identifier" {
		if genLocals == nil {
			return ""
		}
		return genLocals["@weakref."+ingest.NodeText(obj, content)]
	}
	// new WeakRef(x).deref() — peel target inline (method-return via Ex).
	return jsWeakRefTargetTypeEx(obj, content, typedLocals, factories, classFields, methodReturns)
}

// jsReflectGetType recovers T from Reflect.get(obj, key) when obj peels to a
// uniform property-value type T: object literal {k: new T()}, objValueLocal, or
// Object.assign/fromEntries sources with uniform values. Key ignored (all values
// agree). Enables Reflect.get({k: new A()}, "k").run() / Reflect.get(oa, "k").run()
// under foreign same-leaf. Mixed property values / non-Reflect callees fail closed.
// Method-return object values use jsReflectGetTypeEx.
func jsReflectGetType(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string) string {
	return jsReflectGetTypeEx(n, content, typedLocals, factories, objValueLocals, nil, nil)
}

// jsReflectGetTypeEx peels Reflect.get({k: new BoxA().get()}, "k") under foreign
// same-leaf (method-return property values via jsObjectLiteralValueTypeEx).
func jsReflectGetTypeEx(n *grammar.Node, content []byte, typedLocals, factories, objValueLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	objN := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if objN == nil || prop == nil ||
		objN.Type() != "identifier" || ingest.NodeText(objN, content) != "Reflect" ||
		ingest.NodeText(prop, content) != "get" {
		return ""
	}
	var first *grammar.Node
	pos := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		pos++
		if pos == 1 {
			first = ch
		}
	}
	if pos < 2 || first == nil {
		// Reflect.get requires target + key; receiver-only fails closed.
		return ""
	}
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
	if first == nil {
		return ""
	}
	extra := jsExtraLocals{classFields: classFields, methodReturns: methodReturns}
	if first.Type() == "object" {
		// Class peels via concrete path; method-return when methodReturns set.
		return jsObjectLiteralValueTypeEx(first, content, typedLocals, factories, extra)
	}
	if first.Type() == "identifier" && objValueLocals != nil {
		return objValueLocals[ingest.NodeText(first, content)]
	}
	// Object.assign / Object.fromEntries sources with uniform values.
	if t := jsObjectAssignValueTypeEx(first, content, typedLocals, factories, objValueLocals, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsObjectFromEntriesValueTypeEx(first, content, typedLocals, factories, nil, nil, classFields, methodReturns); t != "" {
		return t
	}
	return ""
}

// jsIteratorFromYieldType recovers T from Iterator.from(iterable) when the first
// positional arg peels as an array source of T. Enables
// Iterator.from([new A()]).next().value.run() /
// Iterator.from([new BoxA().get()]).toArray()[0].run() under foreign same-leaf.
// Non-Iterator callees / missing iterable fail closed.
// Method-return iterable elements need classFields/methodReturns (may be nil).
func jsIteratorFromYieldType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories, mapLocals, entryArrayLocals, setLocals map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != "Iterator" ||
		ingest.NodeText(prop, content) != "from" {
		return ""
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
	if first == nil {
		return ""
	}
	extra := jsExtraLocals{
		mapLocals: mapLocals, setLocals: setLocals, entryArrayLocals: entryArrayLocals,
		classFields: classFields, methodReturns: methodReturns,
	}
	return jsArraySourceElemType(first, content, arrayLocals, typedLocals, factories, extra)
}

// jsArrayIsNullishOnly reports [null] / [undefined] / [null, undefined, …]
// (only null/undefined elements). Empty arrays fail closed (use jsIsEmptyArrayLiteral).
// Enables [null].with(0, new A()) / [null].toSpliced(0,1,new A()) overwrite peels
// under foreign same-leaf.
func jsArrayIsNullishOnly(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
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
		return false
	}
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		el := n.Child(i)
		if el == nil || el.Type() == "[" || el.Type() == "]" || el.Type() == "," {
			continue
		}
		saw = true
		if !jsIsNullUndefinedLiteral(el, content) {
			return false
		}
	}
	return saw
}

// jsProxyTargetType recovers T from new Proxy(target[, handler]) when target
// peels to T (new T() / typed local / factory). Enables new Proxy(a, {}).run() /
// const pa = new Proxy(a, {}); pa.run() under foreign same-leaf. Missing target
// / non-Proxy constructors fail closed. Handler ignored.
func jsProxyTargetType(n *grammar.Node, content []byte, typedLocals, factories map[string]string) string {
	return jsProxyTargetTypeEx(n, content, typedLocals, factories, nil, nil)
}

// jsProxyTargetTypeEx peels new Proxy(new BoxA().get(), {}) method-return targets.
func jsProxyTargetTypeEx(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
	if n == nil || n.Type() != "new_expression" {
		return ""
	}
	ctor := ingest.ChildByField(n, "constructor")
	if ctor == nil || ctor.Type() != "identifier" || ingest.NodeText(ctor, content) != "Proxy" {
		return ""
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		// Some grammars attach args as sibling children after constructor.
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch != nil && ch.Type() == "arguments" {
				args = ch
				break
			}
		}
	}
	if args == nil {
		return ""
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
	if first == nil {
		return ""
	}
	return jsExprLeafType(first, content, typedLocals, factories, classFields, methodReturns)
}

// jsPrototypeClassType recovers Class from Class.prototype member access.
// Used by Object.create(A.prototype) identity peels. Other members fail closed.
func jsPrototypeClassType(n *grammar.Node, content []byte) string {
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
	switch n.Type() {
	case "member_expression", "member_expression_optional", "optional_chain":
		prop := ingest.ChildByField(n, "property")
		obj := ingest.ChildByField(n, "object")
		if prop == nil || obj == nil || ingest.NodeText(prop, content) != "prototype" {
			return ""
		}
		if obj.Type() == "identifier" {
			return ingest.NodeText(obj, content)
		}
	}
	return ""
}

// jsIsIdentityCallback reports whether cb is an identity of its first formal
// parameter: x => x / (x) => x / (x) => { return x } / function(x){ return x }.
// Parenthesized expression bodies and single-return statement blocks only.
func jsIsIdentityCallback(cb *grammar.Node, content []byte) bool {
	return jsCallbackReturnedParamIndex(cb, content) == 0
}

// jsIsNestedIdentityMapCallback reports whether cb is xs => xs.map(x => x) /
// (xs) => xs.map(x => x) / (xs) => { return xs.map(x => x) } /
// function(xs){ return xs.map(function(x){ return x }) } — identity map of the
// first formal param. Sole return must be firstParam.map(identityCallback).
// Enables aa.flatMap(xs => xs.map(x => x))[0].run() under foreign same-leaf
// (same leaf as aa.flatMap(xs => xs) / aa.flat()). Non-identity map / chained
// methods / non-param receiver fail closed.
func jsIsNestedIdentityMapCallback(cb *grammar.Node, content []byte) bool {
	params := jsCallbackParamNames(cb, content)
	if len(params) == 0 {
		return false
	}
	first := params[0]
	ret := jsCallbackSoleReturnExpr(cb, content)
	if ret == nil {
		return false
	}
	for ret != nil && ret.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < ret.ChildCount(); i++ {
			ch := ret.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		ret = inner
	}
	if ret == nil || ret.Type() != "call_expression" {
		return false
	}
	fn := ingest.ChildByField(ret, "function")
	args := ingest.ChildByField(ret, "arguments")
	if fn == nil || args == nil {
		return false
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return false
	}
	obj := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if obj == nil || prop == nil ||
		obj.Type() != "identifier" || ingest.NodeText(obj, content) != first ||
		ingest.NodeText(prop, content) != "map" {
		return false
	}
	var mapCb *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if mapCb == nil {
			mapCb = ch
		}
	}
	// Exactly one arg: the identity mapper. thisArg / multi-arg fail closed.
	if count != 1 || mapCb == nil || !jsIsIdentityCallback(mapCb, content) {
		return false
	}
	return true
}

// jsIsIdentityArrayCallback reports whether cb is an identity-array of its first
// formal parameter: x => [x] / (x) => [x] / (x) => { return [x] } /
// function(x){ return [x] }. Sole return must be a one-element array whose only
// element is the first formal param. Multi-element / non-ident / nested fail closed.
func jsIsIdentityArrayCallback(cb *grammar.Node, content []byte) bool {
	params := jsCallbackParamNames(cb, content)
	if len(params) == 0 {
		return false
	}
	first := params[0]
	ret := jsCallbackSoleReturnExpr(cb, content)
	if ret == nil {
		return false
	}
	for ret != nil && ret.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < ret.ChildCount(); i++ {
			ch := ret.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		ret = inner
	}
	if ret == nil || ret.Type() != "array" {
		return false
	}
	var only *grammar.Node
	count := 0
	for i := uint32(0); i < ret.ChildCount(); i++ {
		ch := ret.Child(i)
		if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
			continue
		}
		count++
		only = ch
	}
	if count != 1 || only == nil || only.Type() != "identifier" {
		return false
	}
	return ingest.NodeText(only, content) == first
}

// jsFlatMapSoleArrayReturnElemType recovers T from a flatMap callback whose sole
// return is an array of uniform concrete T: () => [new A()] / (_v) => [a0] /
// () => { return [new A()] } / () => [new BoxA().get()] method-return.
// One-level flatten yields T (receiver ignored — same leaf as
// Array.from({length}, () => new A())). Empty / mixed / non-array sole returns
// fail closed. methodReturns peels method-return inserts under foreign same-leaf.
func jsFlatMapSoleArrayReturnElemType(cb *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	ret := jsCallbackSoleReturnExpr(cb, content)
	if ret == nil {
		return ""
	}
	for ret != nil && ret.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < ret.ChildCount(); i++ {
			ch := ret.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		ret = inner
	}
	if ret == nil || ret.Type() != "array" {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < ret.ChildCount(); i++ {
		ch := ret.Child(i)
		if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "," {
			continue
		}
		// Class/typed-local via concrete path; method-return when methodReturns set.
		t := jsExprLeafType(ch, content, typedLocals, factories, classFields, methodReturns)
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

// jsCallbackReturnedParamIndex returns the 0-based index of the formal param
// that is the sole return of cb, or -1 if cb is not a pure param-return
// identity (x => x / (a,b)=>a / (a,b)=>{return b}). Nested functions ignored.
func jsCallbackReturnedParamIndex(cb *grammar.Node, content []byte) int {
	if cb == nil {
		return -1
	}
	params := jsCallbackParamNames(cb, content)
	if len(params) == 0 {
		return -1
	}
	ret := jsCallbackSoleReturnIdent(cb, content)
	if ret == "" {
		return -1
	}
	for i, p := range params {
		if p == ret {
			return i
		}
	}
	return -1
}

// jsCallbackParamNames returns ordered simple identifier formal param names.
// Rest / defaults / patterns fail closed (empty for that slot stops collection
// only for non-identifier slots — any non-simple param fails the whole list).
func jsCallbackParamNames(cb *grammar.Node, content []byte) []string {
	if cb == nil {
		return nil
	}
	var names []string
	add := func(p *grammar.Node) bool {
		if p == nil {
			return false
		}
		// Peel TS wrappers.
		if p.Type() == "required_parameter" || p.Type() == "optional_parameter" || p.Type() == "assignment_pattern" {
			if n := ingest.ChildByField(p, "pattern"); n != nil {
				p = n
			} else if n := ingest.ChildByField(p, "name"); n != nil {
				p = n
			} else if n := ingest.ChildByType(p, "identifier"); n != nil {
				p = n
			} else {
				return false
			}
		}
		if p.Type() != "identifier" {
			return false
		}
		name := ingest.NodeText(p, content)
		if name == "" || name == "_" {
			return false
		}
		names = append(names, name)
		return true
	}
	switch cb.Type() {
	case "arrow_function":
		if p := ingest.ChildByField(cb, "parameter"); p != nil {
			if !add(p) {
				return nil
			}
			return names
		}
		params := ingest.ChildByField(cb, "parameters")
		if params == nil {
			for i := uint32(0); i < cb.ChildCount(); i++ {
				ch := cb.Child(i)
				if ch.Type() == "formal_parameters" {
					params = ch
					break
				}
			}
		}
		if params == nil {
			return nil
		}
		for i := uint32(0); i < params.ChildCount(); i++ {
			ch := params.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			if !add(ch) {
				return nil
			}
		}
		return names
	case "function_expression", "function_declaration":
		params := ingest.ChildByField(cb, "parameters")
		if params == nil {
			return nil
		}
		for i := uint32(0); i < params.ChildCount(); i++ {
			ch := params.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			if !add(ch) {
				return nil
			}
		}
		return names
	}
	return nil
}

// jsCallbackSoleReturnExpr recovers the sole return expression of cb when the
// body is a pure expression body or a single `return <expr>` statement block.
// Nested functions / multi-statement / empty fail closed. Parentheses peeled
// once around expression bodies only (callers may peel further).
func jsCallbackSoleReturnExpr(cb *grammar.Node, content []byte) *grammar.Node {
	if cb == nil {
		return nil
	}
	body := ingest.ChildByField(cb, "body")
	if body == nil {
		return nil
	}
	// Peel outer parentheses: (x) / ([x])
	for body != nil && body.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < body.ChildCount(); i++ {
			ch := body.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		body = inner
	}
	if body == nil {
		return nil
	}
	// Expression body: x => x / x => [x]
	if body.Type() != "statement_block" {
		return body
	}
	// Statement block: (x) => { return x; } / function(x){ return [x] }
	var retExpr *grammar.Node
	stmts := 0
	for i := uint32(0); i < body.ChildCount(); i++ {
		ch := body.Child(i)
		if ch == nil || ch.Type() == "{" || ch.Type() == "}" {
			continue
		}
		// Skip empty statements.
		if ch.Type() == ";" {
			continue
		}
		stmts++
		if ch.Type() != "return_statement" {
			return nil
		}
		var expr *grammar.Node
		for j := uint32(0); j < ch.ChildCount(); j++ {
			c := ch.Child(j)
			if c == nil || c.Type() == "return" || c.Type() == ";" {
				continue
			}
			expr = c
			break
		}
		retExpr = expr
	}
	if stmts != 1 || retExpr == nil {
		return nil
	}
	// Peel parentheses around return expr.
	for retExpr != nil && retExpr.Type() == "parenthesized_expression" {
		var inner *grammar.Node
		for i := uint32(0); i < retExpr.ChildCount(); i++ {
			ch := retExpr.Child(i)
			if ch.Type() == "(" || ch.Type() == ")" {
				continue
			}
			inner = ch
			break
		}
		retExpr = inner
	}
	return retExpr
}

// jsCallbackSoleReturnIdent recovers the identifier name returned by cb when
// the body is a pure identity return: expression body `x` / `(x)`, or a
// statement block with a single `return x` (no other statements). Nested
// functions fail closed. Empty / multi-statement / non-ident returns fail closed.
func jsCallbackSoleReturnIdent(cb *grammar.Node, content []byte) string {
	retExpr := jsCallbackSoleReturnExpr(cb, content)
	if retExpr == nil || retExpr.Type() != "identifier" {
		return ""
	}
	return ingest.NodeText(retExpr, content)
}

// jsArrayReduceElemType recovers T from arr.reduce(fn[, init]) /
// arr.reduceRight(fn[, init]) when:
//   - arr peels to uniform element type T
//   - fn is a pure param-return identity: (a,b)=>a / (a,b)=>b / (a)=>a
//     (returns first or second formal param unchanged)
//   - optional init peels to T when present
//
// Enables [new A()].reduce((a,b)=>a).run() / as.reduce((a,b)=>b, new A()).run() /
// [new A()].reduceRight((a,b)=>a).run() under foreign same-leaf methods.
// Non-identity reducers / mixed init fail closed.
func jsArrayReduceElemType(n *grammar.Node, content []byte, arrayLocals, typedLocals, factories map[string]string, extra jsExtraLocals) string {
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
	prop := ingest.ChildByField(fn, "property")
	if prop == nil {
		return ""
	}
	name := ingest.NodeText(prop, content)
	if name != "reduce" && name != "reduceRight" {
		return ""
	}
	var cb, init *grammar.Node
	pos := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		pos++
		if pos == 1 {
			cb = ch
		} else if pos == 2 {
			init = ch
		}
	}
	if pos < 1 || pos > 2 || cb == nil {
		return ""
	}
	// Identity of first or second formal only (accumulator or current value).
	idx := jsCallbackReturnedParamIndex(cb, content)
	if idx != 0 && idx != 1 {
		return ""
	}
	t := jsArraySourceElemType(ingest.ChildByField(fn, "object"), content, arrayLocals, typedLocals, factories, extra)
	if t == "" {
		// Empty/unknown array: peel from init when callback returns accumulator
		// (param 0). Enables [].reduce((a,b)=>a, new A()).run() /
		// [].reduce((a,b)=>a, new BoxA().get()).run() under dual-class.
		if init != nil && idx == 0 {
			if it := jsExprLeafType(init, content, typedLocals, factories, extra.classFields, extra.methodReturns); it != "" {
				return it
			}
		}
		return ""
	}
	// init must agree with element T when present (Class or method-return peels).
	if init != nil && jsExprLeafType(init, content, typedLocals, factories, extra.classFields, extra.methodReturns) != t {
		return ""
	}
	return t
}

// jsPromiseFinallyType recovers T from p.finally(fn) / await p.finally(fn) when
// p peels to a Promise of value T. finally is identity for the resolved value
// (callback ignored). Enables (await Promise.resolve(new A()).finally(()=>{})).run()
// and (await Promise.resolve(new BoxA().get()).finally(()=>{})).run() under foreign
// same-leaf methods (method-return via methodReturns). Unknown receivers fail closed.
func jsPromiseFinallyType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
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
		return jsPromiseFinallyType(arg, content, typedLocals, factories, classFields, methodReturns)
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
	if fn == nil {
		return ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "finally" {
		return ""
	}
	// Require ≥1 arg (callback present); body ignored (not type-changing).
	count := 0
	if args != nil {
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			count++
		}
	}
	if count < 1 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	// Class peels via resolve/race; method-return via Ex (methodReturns).
	if t := jsPromiseResolveArgTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseRaceValueTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseThenIdentityType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseCatchType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseFinallyType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	return ""
}

// jsPromiseCatchType recovers T from p.catch(fn) / await p.catch(fn).
// When p peels to a Promise of value T, catch is identity for the fulfilled
// value (rejection handler ignored for typing). When p is Promise.reject(...)
// (no fulfilled value), recover T from the catch handler body:
// catch(() => new A()) / catch(() => new BoxA().get()).
// Enables (await Promise.resolve(new A()).catch(() => null)).run() /
// (await Promise.reject(null).catch(() => new A())).run() /
// (await Promise.reject(null).catch(() => new BoxA().get())).run() under foreign
// same-leaf methods (method-return via methodReturns). Unknown receivers fail closed.
func jsPromiseCatchType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if n == nil {
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
		return jsPromiseCatchType(arg, content, typedLocals, factories, classFields, methodReturns)
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
	if fn == nil {
		return ""
	}
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "catch" {
		return ""
	}
	// Require ≥1 arg (handler present).
	var firstArg *grammar.Node
	count := 0
	if args != nil {
		for i := uint32(0); i < args.ChildCount(); i++ {
			ch := args.Child(i)
			if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
				continue
			}
			count++
			if firstArg == nil {
				firstArg = ch
			}
		}
	}
	if count < 1 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	// Class peels via resolve/race; method-return via Ex (methodReturns).
	if t := jsPromiseResolveArgTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseRaceValueTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseThenIdentityType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseFinallyType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseCatchType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	// Promise.reject(...).catch(() => new A() / new BoxA().get()) — no fulfilled
	// value; recover T from expression-bodied handler under foreign same-leaf.
	if jsIsPromiseRejectCall(obj, content) {
		if t := jsPromiseCatchHandlerReturnType(firstArg, content, typedLocals, factories, classFields, methodReturns); t != "" {
			return t
		}
	}
	return ""
}

// jsIsPromiseRejectCall reports Promise.reject(...).
func jsIsPromiseRejectCall(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
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
		return false
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || (fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain") {
		return false
	}
	base := ingest.ChildByField(fn, "object")
	prop := ingest.ChildByField(fn, "property")
	if base == nil || prop == nil {
		return false
	}
	if ingest.NodeText(prop, content) != "reject" {
		return false
	}
	return base.Type() == "identifier" && ingest.NodeText(base, content) == "Promise"
}

// jsPromiseCatchHandlerReturnType recovers T from an expression-bodied catch
// handler: () => new A() / () => new BoxA().get() / (e) => x. Blocks fail closed.
func jsPromiseCatchHandlerReturnType(handler *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
	if handler == nil {
		return ""
	}
	// Arrow expression body: () => new A()
	if handler.Type() == "arrow_function" {
		body := ingest.ChildByField(handler, "body")
		if body == nil || body.Type() == "statement_block" {
			return ""
		}
		return jsExprLeafType(body, content, typedLocals, factories, classFields, methodReturns)
	}
	return ""
}

// jsPromiseThenIdentityType recovers T from p.then(x => x) / await p.then(x => x)
// when p peels to a Promise of value T via Promise.resolve / race / any or a
// further identity then chain. Enables
// Promise.resolve(new A()).then(x => x).then(a => a.run()) /
// (await Promise.resolve(new A()).then(x => x)).run() and the method-return
// forms Promise.resolve(new BoxA().get()).then(x => x) under foreign same-leaf
// (method-return via methodReturns). Non-identity then callbacks / unknown
// receivers fail closed.
func jsPromiseThenIdentityType(n *grammar.Node, content []byte, typedLocals, factories map[string]string, classFields, methodReturns map[string]map[string]string) string {
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
		return jsPromiseThenIdentityType(arg, content, typedLocals, factories, classFields, methodReturns)
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
	if fn.Type() != "member_expression" && fn.Type() != "member_expression_optional" && fn.Type() != "optional_chain" {
		return ""
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, content) != "then" {
		return ""
	}
	// First positional arg must be identity callback.
	var cb *grammar.Node
	count := 0
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch == nil || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		count++
		if cb == nil {
			cb = ch
		}
	}
	// then(onFulfilled[, onRejected]) — require ≥1 arg; only first must be identity.
	if count < 1 || cb == nil || !jsIsIdentityCallback(cb, content) {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	// Peel Promise.resolve / race / any / further identity then / finally / catch.
	// Class peels via resolve/race; method-return via Ex (methodReturns).
	if t := jsPromiseResolveArgTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseRaceValueTypeEx(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseFinallyType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseCatchType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	if t := jsPromiseThenIdentityType(obj, content, typedLocals, factories, classFields, methodReturns); t != "" {
		return t
	}
	return ""
}
