package ingest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
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
		return planCrossFileMove(dir, result, src, dst, sourceEntity, driver)
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
	// Spans store the identifier leaf (e.g. "toJson"), while references may be
	// qualified ("Gson.toJson"). Always rewrite source text with the leaf.
	newText := symbolNameLeaf(destSymbol)

	// 1. Rename the entity definition.
	for _, ent := range result.Entities {
		if ent.Reference == sourceRef {
			ref := ParseReference(ent.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
				NewText:   newText,
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
				NewText:   newText,
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
				NewText:   newText,
			})
		}
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("no entity found for reference %s", sourceRef)
	}

	return edits, nil
}

// symbolNameLeaf returns the identifier text written at a definition/use span.
// Qualified symbols use "." separators; Go pointer receivers may prefix "*".
func symbolNameLeaf(symbol string) string {
	leaf := symbol
	if i := strings.LastIndex(leaf, "."); i >= 0 {
		leaf = leaf[i+1:]
	}
	return strings.TrimPrefix(leaf, "*")
}

// planCrossFileMove orchestrates a cross-file move using the MoveDriver interface.
// It extracts the declaration, inserts it at the destination, and if the source
// and destination are in different directories, rewrites imports in consumer files.
func planCrossFileMove(dir string, result *Result, src, dst Reference, sourceEntity Entity, driver MoveDriver) ([]Edit, error) {
	srcRel := strings.TrimPrefix(src.Path, "./")
	dstRel := strings.TrimPrefix(dst.Path, "./")

	decl, err := driver.ExtractDecl(filepath.Join(dir, srcRel), sourceEntity)
	if err != nil {
		return nil, err
	}

	dstPath := filepath.Join(dir, dstRel)
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			dstContent = nil
		} else {
			return nil, fmt.Errorf("reading %s: %w", dstRel, err)
		}
	}

	insertEdit := driver.InsertDecl(dstRel, dstContent, decl)

	edits := []Edit{
		{
			File:      srcRel,
			StartByte: decl.RemoveStart,
			EndByte:   decl.RemoveEnd,
			NewText:   "",
		},
		insertEdit,
	}

	// Rewrite imports in consumer files that reference the moved symbol.
	// For same-directory moves, only rewrite if the source file itself
	// appears in import targets (e.g. Python relative imports, JS path imports).
	if srcRel != dstRel {
		importEdits, err := planCrossFileMoveImportRewrites(dir, result, src, dst, driver)
		if err != nil {
			return nil, err
		}
		edits = append(edits, importEdits...)
	}

	return edits, nil
}

// planCrossFileMoveImportRewrites finds all consumer files that import the
// source module/package and rewrites them to point to the destination.
func planCrossFileMoveImportRewrites(dir string, result *Result, src, dst Reference, driver MoveDriver) ([]Edit, error) {
	// Build the set of reference strings that all point at the source entity/file.
	// Different languages resolve imports to different provider namespaces
	// (e.g. path:./utils.py vs python:fastapi.utils), so we need to match
	// against every form the source might appear as in alias/relation targets.
	srcTargets := buildSourceTargetSet(result, src)

	// Find files that have aliases targeting the source symbol or source module.
	consumerFiles := map[string]bool{}
	for _, alias := range result.Aliases {
		if srcTargets[alias.Target] {
			ref := ParseReference(alias.Reference)
			consumerFile := strings.TrimPrefix(ref.Path, "./")
			consumerFiles[consumerFile] = true
		}
	}
	// Also check relations targeting the source.
	for _, rel := range result.Relations {
		if srcTargets[rel.Target] {
			ref := ParseReference(rel.Reference)
			consumerFile := strings.TrimPrefix(ref.Path, "./")
			consumerFiles[consumerFile] = true
		}
	}

	var edits []Edit
	for consumerFile := range consumerFiles {
		// Don't rewrite the source or destination files themselves.
		srcRel := strings.TrimPrefix(src.Path, "./")
		dstRel := strings.TrimPrefix(dst.Path, "./")
		if consumerFile == srcRel || consumerFile == dstRel {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, consumerFile))
		if err != nil {
			continue
		}

		occs := driver.RewriteImports(consumerFile, content, result, src, dst)
		edits = append(edits, occs...)
	}

	return edits, nil
}

// buildSourceTargetSet builds a set of reference strings that might appear as
// alias or relation targets for the given source reference. Because different
// languages resolve imports into different provider namespaces (e.g.
// "path:./utils.py::X" vs "python:fastapi.utils::X"), we scan the existing
// result for any target that points at the same file, with or without the
// symbol qualifier.
func buildSourceTargetSet(result *Result, src Reference) map[string]bool {
	srcRef := src.String()
	srcFileRef := FileRef(src.Path)
	srcDirRef := FileRef("./" + path.Dir(strings.TrimPrefix(src.Path, "./")))
	srcRel := strings.TrimPrefix(src.Path, "./")

	targets := map[string]bool{
		srcRef:     true,
		srcFileRef: true,
		srcDirRef:  true,
	}

	// Scan all aliases and relations to find targets that reference the same
	// file (by any provider namespace). We match on file path suffix to
	// catch e.g. "python:fastapi.utils" mapping to file "utils.py".
	for _, alias := range result.Aliases {
		ref := ParseReference(alias.Target)
		if ref.Provider == "path" {
			continue // already covered by the direct matches above
		}
		if aliasTargetMatchesFile(result, alias.Target, ref, srcRel, src.Symbol) {
			targets[alias.Target] = true
		}
	}
	for _, rel := range result.Relations {
		ref := ParseReference(rel.Target)
		if ref.Provider == "path" {
			continue
		}
		if aliasTargetMatchesFile(result, rel.Target, ref, srcRel, src.Symbol) {
			targets[rel.Target] = true
		}
	}

	return targets
}

// aliasTargetMatchesFile checks if a non-path target reference actually points
// at the given source file. It does this by checking if the target entity
// exists among the entities declared in the source file.
func aliasTargetMatchesFile(result *Result, targetStr string, targetRef Reference, srcFileRel, srcSymbol string) bool {
	if srcSymbol != "" {
		// For symbol-level matches, check if there's an entity in the source
		// file with matching symbol name.
		wantEntRef := SymbolRef("./"+srcFileRel, srcSymbol)
		for _, ent := range result.Entities {
			if ent.Reference == wantEntRef {
				// Now check: does targetRef reference the same symbol?
				if targetRef.Symbol == srcSymbol {
					return true
				}
			}
		}
		return false
	}

	// For file/dir level matches, check if any alias or entity from the
	// source file has a target that resolves to this reference.
	srcFilePath := "./" + srcFileRel
	for _, alias := range result.Aliases {
		ref := ParseReference(alias.Reference)
		if ref.Path == srcFilePath && alias.Target == targetStr {
			return true
		}
	}
	return false
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

	// Update import references in all consumer files (files not under srcDir).
	// Dispatches to the per-language MoveDriver.RewriteImports when available;
	// falls back to global leaf-based string replacement for languages without
	// a registered move driver.
	for _, f := range result.Files {
		if isUnderDir(f.Path, srcDir) {
			continue
		}
		fcontent, err := os.ReadFile(filepath.Join(dir, f.Path))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.Path, err)
		}

		if driver, ok := moveDriverForLanguage(f.Language); ok {
			occs := driver.RewriteImports(f.Path, fcontent, result, src, dst)
			edits = append(edits, occs...)
		} else {
			// Fallback: whole-word leaf-based path token replace.
			pathOld := oldDirBase
			pathNew := newDirBase
			if cp := CommonPathPrefix(srcDir, dstDir); cp != "" {
				if rel := strings.Trim(strings.TrimPrefix(dstDir, cp), "/"); rel != "" {
					pathNew = rel
				}
			}
			if pathOld != pathNew {
				occs := FindAllWholeWordOccurrences(f.Path, fcontent, pathOld, pathNew)
				edits = append(edits, occs...)
			}
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

// FindAllWholeWordOccurrences finds all whole-word occurrences of oldBase in
// content and returns edits to replace them with newBase. A "whole word" match
// means the match is not preceded or followed by a letter, digit or underscore.
func FindAllWholeWordOccurrences(file string, content []byte, oldBase, newBase string) []Edit {
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
		endPos := pos + len(oldBase)

		// Check word boundaries.
		if pos > 0 && isWordChar(text[pos-1]) {
			off = endPos
			continue
		}
		if endPos < len(text) && isWordChar(text[endPos]) {
			off = endPos
			continue
		}

		edits = append(edits, Edit{
			File:      file,
			StartByte: uint32(pos),
			EndByte:   uint32(endPos),
			NewText:   newBase,
		})
		off = endPos
	}
	return edits
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// FindAllOccurrences finds all occurrences of oldBase in content and returns
// edits to replace them with newBase.
func FindAllOccurrences(file string, content []byte, oldBase, newBase string) []Edit {
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

// FindAllOccurrencesInStrings is like findAllOccurrences but only produces
// edits for matches that occur inside Go string literals ("..." or `...`).
// Used for updating import path strings (the location part) using directory
// names without touching bare identifiers (which may be qualifiers whose
// name comes from the package directive).
func FindAllOccurrencesInStrings(file string, content []byte, oldBase, newBase string) []Edit {
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

// CommonPathPrefix returns the common directory prefix of two paths.
func CommonPathPrefix(a, b string) string {
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
