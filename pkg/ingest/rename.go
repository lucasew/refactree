package ingest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

// Edit describes a text replacement in a source file.
type Edit struct {
	File      string // path relative to the ingest root
	StartByte uint32
	EndByte   uint32
	NewText   string
}

// Rename computes the edits needed to rename or move a symbol from sourceRef to
// destRef. Same-file operations are treated as renames; cross-file operations
// are treated as moves.
func Rename(dir, sourceRef, destRef string) ([]Edit, error) {
	src := ParseReference(sourceRef)
	dst := ParseReference(destRef)
	if (src.Symbol == "") != (dst.Symbol == "") {
		return nil, fmt.Errorf("source and destination references must both include symbols or both omit them (for package moves)")
	}

	result, err := Ingest(dir)
	if err != nil {
		return nil, err
	}

	src, err = canonicalSourceReference(dir, result, src)
	if err != nil {
		return nil, err
	}
	dst, err = canonicalDestinationReference(dir, result, src, dst)
	if err != nil {
		return nil, err
	}

	sourceRef = src.String()
	destRef = dst.String()

	if src.Symbol == "" && dst.Symbol == "" {
		// package/dir move (no symbol); canonical* already short-circuit+normalize for Symbol==""
		return planPackageMove(dir, result, src, dst)
	}

	sourceEntity, ok := findEntityByReference(result, sourceRef)
	if !ok {
		return nil, fmt.Errorf("no entity found for reference %s", sourceRef)
	}

	if src.Path != dst.Path {
		if src.Symbol != dst.Symbol {
			return nil, fmt.Errorf("cross-file move with symbol rename is not supported yet")
		}
		srcLang := languageForRefPath(result, src.Path)
		driver, ok := moveDriverForLanguage(srcLang)
		if !ok {
			return nil, fmt.Errorf("cross-file move is not supported for language %q", srcLang)
		}
		return driver.PlanCrossFileMove(dir, result, src, dst, sourceEntity)
	}

	return planSymbolRename(result, sourceRef, dst.Symbol)
}

func findEntityByReference(result *Result, ref string) (Entity, bool) {
	for _, ent := range result.Entities {
		if ent.Reference == ref {
			return ent, true
		}
	}
	return Entity{}, false
}

func planSymbolRename(result *Result, sourceRef, destSymbol string) ([]Edit, error) {
	var edits []Edit

	// 1. Rename the entity definition.
	for _, ent := range result.Entities {
		if ent.Reference == sourceRef {
			ref := ParseReference(ent.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
				NewText:   destSymbol,
			})
		}
	}

	// 2. Rename at every call site that targets this entity.
	for _, rel := range result.Relations {
		if rel.Target == sourceRef {
			// Usage through explicit import alias bindings (`as`) should keep
			// the local alias name unchanged when renaming the imported symbol.
			if rel.ViaImportAlias {
				continue
			}
			ref := ParseReference(rel.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: rel.StartByte,
				EndByte:   rel.EndByte,
				NewText:   destSymbol,
			})
		}
	}

	// 3. Rename in import bindings that target this entity.
	for _, alias := range result.Aliases {
		if alias.Target == sourceRef {
			ref := ParseReference(alias.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: alias.StartByte,
				EndByte:   alias.EndByte,
				NewText:   destSymbol,
			})
		}
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("no entity found for reference %s", sourceRef)
	}

	return edits, nil
}

func planGoCrossFileMove(dir string, result *Result, src, dst Reference, sourceEntity Entity) ([]Edit, error) {
	srcRel := strings.TrimPrefix(src.Path, "./")
	dstRel := strings.TrimPrefix(dst.Path, "./")
	if path.Dir(srcRel) != path.Dir(dstRel) {
		return nil, fmt.Errorf("cross-directory Go move is not supported yet")
	}

	pkg, declText, removeStart, removeEnd, err := extractGoDeclFromEntity(filepath.Join(dir, srcRel), sourceEntity.StartByte)
	if err != nil {
		return nil, err
	}

	dstPath := filepath.Join(dir, dstRel)
	dstContent, err := os.ReadFile(dstPath)
	dstExists := true
	if err != nil {
		if os.IsNotExist(err) {
			dstExists = false
		} else {
			return nil, fmt.Errorf("reading %s: %w", dstRel, err)
		}
	}

	insertAt := uint32(0)
	insertText := ""
	if dstExists {
		insertAt = uint32(len(dstContent))
		if len(dstContent) > 0 && dstContent[len(dstContent)-1] != '\n' {
			insertText += "\n"
		}
		if len(dstContent) > 0 {
			insertText += "\n"
		}
		insertText += declText + "\n"
	} else {
		insertText = fmt.Sprintf("package %s\n\n%s\n", pkg, declText)
	}

	return []Edit{
		{
			File:      srcRel,
			StartByte: removeStart,
			EndByte:   removeEnd,
			NewText:   "",
		},
		{
			File:      dstRel,
			StartByte: insertAt,
			EndByte:   insertAt,
			NewText:   insertText,
		},
	}, nil
}

func extractGoDeclFromEntity(filePath string, nameStart uint32) (pkg, declText string, start, end uint32, err error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", 0, 0, err
	}

	lang, ok := grammar.GetByExtension(filePath)
	if !ok {
		return "", "", 0, 0, fmt.Errorf("unsupported language for %s", filePath)
	}

	parser := grammar.NewParser()
	defer parser.Delete()
	if !parser.SetLanguage(lang) {
		return "", "", 0, 0, fmt.Errorf("failed to set language for %s", filePath)
	}

	tree := parser.ParseString(string(source))
	defer tree.Delete()
	root := tree.RootNode()

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Type() == "package_clause" {
			if id := childByType(child, "package_identifier"); id != nil {
				pkg = nodeText(id, source)
			}
		}
	}

	declNode := findDeclContaining(root, nameStart, "go")
	if declNode == nil {
		return "", "", 0, 0, fmt.Errorf("declaration not found in %s", filePath)
	}

	start = declNode.StartByte()
	end = declNode.EndByte()
	declText = string(source[start:end])

	// Also remove up to two trailing newlines to avoid leaving extra gaps.
	removeEnd := end
	for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
		removeEnd++
		if removeEnd-end >= 2 {
			break
		}
	}

	return pkg, declText, start, removeEnd, nil
}

// ApplyEdits applies text replacements to files under dir.
// Edits are grouped by file and applied from last to first byte offset
// so that earlier offsets remain valid.
func ApplyEdits(dir string, edits []Edit) error {
	byFile := map[string][]Edit{}
	for _, e := range edits {
		byFile[e.File] = append(byFile[e.File], e)
	}

	for file, fileEdits := range byFile {
		target := filepath.Join(dir, file)
		content, err := os.ReadFile(target)
		if err != nil {
			if os.IsNotExist(err) {
				createAllowed := true
				for _, e := range fileEdits {
					if e.EndByte != 0 {
						createAllowed = false
						break
					}
				}
				if !createAllowed {
					return fmt.Errorf("reading %s: %w", file, err)
				}
				content = []byte{}
				// 0755 conventional for dirs created as side-effect of edit application
				// (no project-wide const used yet; see review Issue 8).
				if mkerr := os.MkdirAll(filepath.Dir(target), 0755); mkerr != nil {
					return fmt.Errorf("mkdir for %s: %w", file, mkerr)
				}
			} else {
				return fmt.Errorf("reading %s: %w", file, err)
			}
		}

		sort.Slice(fileEdits, func(i, j int) bool {
			return fileEdits[i].StartByte > fileEdits[j].StartByte
		})

		for _, e := range fileEdits {
			if int(e.EndByte) > len(content) {
				return fmt.Errorf("edit out of bounds in %s: end %d > len %d", file, e.EndByte, len(content))
			}
			content = append(content[:e.StartByte], append([]byte(e.NewText), content[e.EndByte:]...)...)
		}

		if err := os.WriteFile(target, content, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}

		// For package moves we emit full-file truncate edits (NewText:"") on every
		// source file under the old package dir. After the write, if the file is now
		// empty we remove it entirely instead of leaving a 0-byte file behind.
		// This keeps the "edits only" model for the returned plan while still
		// producing a clean result (no empty files from relocation).
		if len(content) == 0 {
			_ = os.Remove(target)
		}
	}

	return nil
}

// planPackageMove handles directory/package level moves (inter-package).
// It relocates all files under the source dir by clearing old locations and
// inserting full original contents at new locations, and performs textual
// replacement of the package base name in all other files (updating import
// spec strings, local bindings and qualifiers at call sites).
//
// Note on relocation: emits truncate (NewText:"") edits on old paths (edits-only
// model, no explicit file/dir removal in the plan). ApplyEdits removes any file
// that ends up 0 bytes after its edits (see the len(content)==0 Remove after
// WriteFile). This means old package files disappear entirely on disk instead of
// leaving 0-byte placeholders. Empty directories may remain (no dir pruning is
// performed, for simplicity). compareDir in tests only validates the expected/
// tree. This matches the "content deleted" intent from the original fixtures/task.
func planPackageMove(dir string, result *Result, src, dst Reference) ([]Edit, error) {
	srcDir := strings.TrimPrefix(src.Path, "./")
	dstDir := strings.TrimPrefix(dst.Path, "./")
	if srcDir == "" || dstDir == "" {
		return nil, fmt.Errorf("package move requires non-empty directory paths")
	}
	if srcDir == dstDir {
		return nil, nil // no-op; avoids pointless I/O + self-overwrite edits
	}
	oldDirBase := lastPathComponent(srcDir)
	newDirBase := lastPathComponent(dstDir)

	// Determine the name used as the "package symbol" brought into other packages
	// (the qualifier in foo.Bar(), the binding in import foo "...", etc.).
	// Harmonious with the rest of the system: this comes from the `package`
	// directive inside the moved Go files (see FileExtract.Package populated
	// by the Go language driver + extractGo), not from the directory basename.
	// We read one of the sources under srcDir (we're reading them for relocation
	// anyway) and extract the clause with a tiny scanner. Falls back to the
	// dir last segment (preserving old convention for non-Go and when no
	// explicit package clause is present).
	declaredName := oldDirBase
	for _, ff := range result.Files {
		if isUnderDir(ff.Path, srcDir) {
			if b, err := os.ReadFile(filepath.Join(dir, ff.Path)); err == nil {
				if n := extractPackageClauseName(b); n != "" {
					declaredName = n
					break
				}
			}
		}
	}
	// Because relocation copies the original file contents verbatim (including
	// the package clause), the declared name does not change just because the
	// directory moved. Therefore the qualifier / binding name that consumers
	// use for this package also stays the same.
	newDeclaredName := declaredName

	var edits []Edit

	// Relocate package contents: for every file listed under the source dir,
	// delete its content at old path, insert full original content at new path.
	for _, f := range result.Files {
		if !isUnderDir(f.Path, srcDir) {
			continue
		}
		srcFile := f.Path
		under := strings.TrimPrefix(srcFile, srcDir)
		under = strings.TrimPrefix(under, "/")
		dstFile := dstDir
		if under != "" {
			dstFile = path.Join(dstDir, under)
		}
		content, err := os.ReadFile(filepath.Join(dir, srcFile))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", srcFile, err)
		}
		// Clear entire old file.
		edits = append(edits, Edit{
			File:      srcFile,
			StartByte: 0,
			EndByte:   uint32(len(content)),
			NewText:   "",
		})
		// Insert full content at new location (create if needed handled by ApplyEdits).
		edits = append(edits, Edit{
			File:      dstFile,
			StartByte: 0,
			EndByte:   0,
			NewText:   string(content),
		})
	}

	// Update in all non-package (consumer) files.
	// Path updates (for import strings) use the directory last-segment tokens.
	//   - Go: the replace is scoped to string literals only. This updates the
	//     import path location without touching bare identifiers. The qualifier
	//     / binding name used by consumers is taken from the package directive
	//     (see declaredName computation + FileExtract.Package in the Go driver).
	//     This is the harmonious part requested: the "symbol brought up to the
	//     other packages" comes from `package foo`, not the dir basename.
	//   - JS/Python: the module name *is* the path last segment, so we do the
	//     previous global replace (updates the literal *and* the local bindings
	//     / uses like "import * as helpers", "helpers.xxx"). This keeps the
	//     py/js move_package fixtures working.
	//
	// For hierarchy moves the path-in-string replacement for Go substitutes the
	// old leaf with the new subpath (relative to common ancestor) so that a
	// tail ".../executil" becomes the correct new location inside the import.
	//
	// The declared-name pass (Go only) is usually a no-op on a pure directory
	// move (sources copied verbatim → package clause unchanged).
	for _, f := range result.Files {
		if isUnderDir(f.Path, srcDir) {
			continue
		}
		fcontent, err := os.ReadFile(filepath.Join(dir, f.Path))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.Path, err)
		}

		if f.Language == "go" {
			// For Go consumers, do full old subdir -> full new subdir replace inside import strings
			// when the consumer content contains the fuller path (e.g. "pkg/executil").
			// This inserts the "util/" level etc. instead of the replacement assuming/skip ping it.
			fullOld := srcDir
			fullNew := dstDir
			didFull := false
			if fullOld != fullNew {
				if strings.Contains(string(fcontent), fullOld) {
					occs := findAllOccurrencesInStrings(f.Path, fcontent, fullOld, fullNew)
					edits = append(edits, occs...)
					didFull = true
				}
			}

			// Leaf path (for bare leaf imports or as additional) inside strings for Go, guarded
			// by didFull to avoid overlap with the full subdir edit on the same file.
			pathOld := oldDirBase
			pathNew := newDirBase
			if cp := commonPathPrefix(srcDir, dstDir); cp != "" {
				if rel := strings.Trim(strings.TrimPrefix(dstDir, cp), "/"); rel != "" {
					pathNew = rel
				}
			}
			if pathOld != pathNew && !didFull {
				occs := findAllOccurrencesInStrings(f.Path, fcontent, pathOld, pathNew)
				edits = append(edits, occs...)
			}
		} else {
			// non-Go: always the global leaf-based path token replace. This updates both the
			// import literal *and* the bindings/uses (as before for py/js fixtures).
			pathOld := oldDirBase
			pathNew := newDirBase
			if cp := commonPathPrefix(srcDir, dstDir); cp != "" {
				if rel := strings.Trim(strings.TrimPrefix(dstDir, cp), "/"); rel != "" {
					pathNew = rel
				}
			}
			if pathOld != pathNew {
				occs := findAllOccurrences(f.Path, fcontent, pathOld, pathNew)
				edits = append(edits, occs...)
			}
		}

		// Go qualifier/binding name from the package directive (not dir last seg).
		if f.Language == "go" && declaredName != newDeclaredName {
			occs := findAllOccurrences(f.Path, fcontent, declaredName, newDeclaredName)
			edits = append(edits, occs...)
		}
	}

	return edits, nil
}

func isUnderDir(p, dir string) bool {
	d := strings.TrimSuffix(dir, "/")
	if d == "" {
		return false
	}
	if p == d || strings.HasPrefix(p, d+"/") {
		return true
	}
	return false
}

func findAllOccurrences(file string, content []byte, oldBase, newBase string) []Edit {
	if oldBase == "" || oldBase == newBase {
		return nil
	}
	text := string(content)
	var edits []Edit
	off := 0
	for {
		idx := strings.Index(text[off:], oldBase)
		if idx < 0 {
			break
		}
		pos := off + idx
		edits = append(edits, Edit{
			File:      file,
			StartByte: uint32(pos),
			EndByte:   uint32(pos + len(oldBase)),
			NewText:   newBase,
		})
		off = pos + len(oldBase)
	}
	return edits
}

// findAllOccurrencesInStrings is like findAllOccurrences but only produces
// edits for matches that occur inside Go string literals ("..." or `...`).
// Used for updating import path strings (the location part) using directory
// names without touching bare identifiers (which may be qualifiers whose
// name comes from the package directive).
func findAllOccurrencesInStrings(file string, content []byte, oldBase, newBase string) []Edit {
	if oldBase == "" || oldBase == newBase {
		return nil
	}
	text := string(content)
	var edits []Edit
	off := 0
	for off < len(text) {
		// find next string literal start
		dq := strings.IndexByte(text[off:], '"')
		rq := strings.IndexByte(text[off:], '`')
		start := -1
		isRaw := false
		if dq >= 0 && (rq < 0 || dq < rq) {
			start = off + dq
		} else if rq >= 0 {
			start = off + rq
			isRaw = true
		}
		if start < 0 {
			break
		}
		// find matching end
		end := -1
		if isRaw {
			e := strings.IndexByte(text[start+1:], '`')
			if e >= 0 {
				end = start + 1 + e
			}
		} else {
			// double-quoted, skip escapes
			i := start + 1
			for i < len(text) {
				if text[i] == '\\' && i+1 < len(text) {
					i += 2
					continue
				}
				if text[i] == '"' {
					end = i
					break
				}
				i++
			}
		}
		if end < 0 {
			break // unterminated; give up on this file
		}
		// search inside the literal [start:end+1]
		seg := text[start : end+1]
		sOff := 0
		for {
			idx := strings.Index(seg[sOff:], oldBase)
			if idx < 0 {
				break
			}
			pos := start + sOff + idx
			edits = append(edits, Edit{
				File:      file,
				StartByte: uint32(pos),
				EndByte:   uint32(pos + len(oldBase)),
				NewText:   newBase,
			})
			sOff += idx + len(oldBase)
		}
		off = end + 1
	}
	return edits
}

func commonPathPrefix(a, b string) string {
	aa := strings.Split(strings.Trim(a, "/"), "/")
	bb := strings.Split(strings.Trim(b, "/"), "/")
	n := len(aa)
	if len(bb) < n {
		n = len(bb)
	}
	var p []string
	for i := 0; i < n; i++ {
		if aa[i] != bb[i] {
			break
		}
		p = append(p, aa[i])
	}
	if len(p) == 0 {
		return ""
	}
	return strings.Join(p, "/") + "/"
}

func extractPackageClauseName(src []byte) string {
	s := string(src)
	i := strings.Index(s, "package ")
	if i == -1 {
		return ""
	}
	rest := s[i+len("package "):]
	for j, r := range rest {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '/' || r == ';' {
			if j > 0 {
				return rest[:j]
			}
			return ""
		}
	}
	return strings.TrimSpace(rest)
}
