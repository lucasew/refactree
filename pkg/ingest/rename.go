package ingest

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
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

	result, err := ProjectResult(dir)
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

	sourceRefs := []string{sourceRef}
	if srcLang := languageForRefPath(result, src.Path); srcLang != "" {
		if driver, ok := moveDriverForLanguage(srcLang); ok {
			if expander, ok := driver.(RenameExpander); ok {
				if extra := expander.ExpandRenameSources(result, sourceRef); len(extra) > 0 {
					sourceRefs = uniqueStrings(append(sourceRefs, extra...))
				}
			}
		}
	}
	return planSymbolRename(dir, result, sourceRefs, dst.Symbol)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func findEntityByReference(result *Result, ref string) (Entity, bool) {
	for _, ent := range result.Entities {
		if ent.Reference == ref {
			return ent, true
		}
	}
	return Entity{}, false
}

func planSymbolRename(dir string, result *Result, sourceRefs []string, destSymbol string) ([]Edit, error) {
	if len(sourceRefs) == 0 {
		return nil, fmt.Errorf("no source references to rename")
	}
	// Relations often target language providers (go:mod/pkg::Sym) while entities
	// are path:./pkg/file.go::Sym. Expand so cross-package qualified calls rename.
	sourceSet := expandRenameSourceSet(dir, result, sourceRefs)
	var edits []Edit
	// Spans store the identifier leaf (e.g. "toJson"), while references may be
	// qualified ("Gson.toJson"). Always rewrite source text with the leaf.
	newText := SymbolLeaf(destSymbol)
	oldLeaf := SymbolLeaf(ParseReference(sourceRefs[0]).Symbol)

	// 1. Rename each related entity definition. Destination symbol qualifiers
	// stay aligned with each source entity's receiver prefix.
	for _, ent := range result.Entities {
		if !sourceSet[ent.Reference] {
			continue
		}
		ref := ParseReference(ent.Reference)
		edits = append(edits, Edit{
			File:      strings.TrimPrefix(ref.Path, "./"),
			StartByte: ent.StartByte,
			EndByte:   ent.EndByte,
			NewText:   newText,
		})
	}

	// 2. Rename at every call site that targets any expanded entity.
	for _, rel := range result.Relations {
		if !sourceSet[rel.Target] {
			continue
		}
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

	// 3. Rename in import bindings that target any expanded entity.
	for _, alias := range result.Aliases {
		if !sourceSet[alias.Target] {
			continue
		}
		ref := ParseReference(alias.Reference)
		edits = append(edits, Edit{
			File:      strings.TrimPrefix(ref.Path, "./"),
			StartByte: alias.StartByte,
			EndByte:   alias.EndByte,
			NewText:   newText,
		})
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("no entity found for reference %s", sourceRefs[0])
	}

	src0 := ParseReference(sourceRefs[0])
	if srcLang := languageForRefPath(result, src0.Path); srcLang != "" {
		if driver, ok := moveDriverForLanguage(srcLang); ok {
			if expander, ok := driver.(RenameSpanExpander); ok {
				extra := expander.ExtraRenameEdits(dir, result, sourceRefs, oldLeaf, newText)
				edits = append(edits, extra...)
			}
		}
	}

	return dedupeEdits(edits), nil
}

func dedupeEdits(edits []Edit) []Edit {
	type key struct {
		file       string
		start, end uint32
		text       string
	}
	seen := map[key]bool{}
	var out []Edit
	for _, e := range edits {
		k := key{e.File, e.StartByte, e.EndByte, e.NewText}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

// expandRenameSourceSet adds language-provider targets (e.g. go:mod/pkg::Sym)
// that refer to the same package-scoped symbols as the path: entity refs.
// rootDir is the ingest root (for reading go.mod module path when matching go: targets).
func expandRenameSourceSet(rootDir string, result *Result, sourceRefs []string) map[string]bool {
	sourceSet := map[string]bool{}
	type want struct {
		pkgDir string
		symbol string
	}
	var wants []want
	seenWant := map[want]bool{}
	for _, s := range sourceRefs {
		if s == "" {
			continue
		}
		sourceSet[s] = true
		ref := ParseReference(s)
		pkgDir := path.Dir(strings.TrimPrefix(ref.Path, "./"))
		if pkgDir == "." {
			pkgDir = ""
		}
		w := want{pkgDir: pkgDir, symbol: ref.Symbol}
		if ref.Symbol == "" || seenWant[w] {
			continue
		}
		seenWant[w] = true
		wants = append(wants, w)
	}
	if len(wants) == 0 || result == nil {
		return sourceSet
	}
	modulePath := readModulePathForRename(rootDir)
	add := func(target string) {
		if target == "" || sourceSet[target] {
			return
		}
		t := ParseReference(target)
		if t.Symbol == "" {
			return
		}
		for _, w := range wants {
			if t.Symbol != w.symbol {
				continue
			}
			if targetMatchesPackageSymbol(t, w.pkgDir, modulePath) {
				sourceSet[target] = true
				return
			}
		}
	}
	for _, rel := range result.Relations {
		add(rel.Target)
	}
	for _, a := range result.Aliases {
		add(a.Target)
	}
	return sourceSet
}

// targetMatchesPackageSymbol reports whether ref names a symbol in package pkgDir
// (relative to the module root, e.g. "pkg/db").
// For path: refs, compares file directory. For language providers (go:, …),
// matches the module import path modulePath+"/"+pkgDir exactly when modulePath
// is known — never bare trailing-segment suffix matches (db vs pkg/db) and never
// empty pkgDir against arbitrary single-segment imports (fmt, os, …).
func targetMatchesPackageSymbol(ref Reference, pkgDir, modulePath string) bool {
	if ref.Provider == "path" || ref.Provider == "" {
		dir := path.Dir(strings.TrimPrefix(ref.Path, "./"))
		if dir == "." {
			dir = ""
		}
		return dir == pkgDir
	}
	p := strings.Trim(ref.Path, "/")
	if modulePath != "" {
		want := modulePath
		if pkgDir != "" {
			want = modulePath + "/" + pkgDir
		}
		return p == want
	}
	// No module path available: only exact pkgDir equality (no suffix / empty root).
	if pkgDir == "" {
		return false
	}
	return p == pkgDir
}

// readModulePathForRename returns the go.mod module path for rootDir, or "".
func readModulePathForRename(rootDir string) string {
	if rootDir == "" {
		return ""
	}
	dir := rootDir
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == "" || parent == dir {
			return ""
		}
		dir = parent
	}
}

// SymbolLeaf returns the identifier text written at a definition/use span.
// Qualified symbols use "." separators; pointer receivers may prefix "*".
func SymbolLeaf(symbol string) string {
	leaf := symbol
	if i := strings.LastIndex(leaf, "."); i >= 0 {
		leaf = leaf[i+1:]
	}
	return strings.TrimPrefix(leaf, "*")
}

// applyEditsToString applies byte-offset edits to an in-memory buffer.
func applyEditsToString(content string, edits []Edit) string {
	if len(edits) == 0 {
		return content
	}
	sorted := append([]Edit(nil), edits...)
	slices.SortFunc(sorted, func(a, b Edit) int {
		return cmp.Compare(b.StartByte, a.StartByte)
	})
	buf := []byte(content)
	for _, e := range sorted {
		if int(e.EndByte) > len(buf) || e.StartByte > e.EndByte {
			continue
		}
		buf = append(buf[:e.StartByte], append([]byte(e.NewText), buf[e.EndByte:]...)...)
	}
	return string(buf)
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

	if finisher, ok := driver.(CrossFileMoveFinisher); ok {
		extra, err := finisher.FinishCrossFileMove(dir, result, src, dst, decl)
		if err != nil {
			return nil, err
		}
		edits = append(edits, extra...)
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
				// 0o755 conventional for dirs created as side-effect of edit application
				// (no project-wide const used yet; see review Issue 8).
				if mkerr := os.MkdirAll(filepath.Dir(target), 0o755); mkerr != nil {
					return fmt.Errorf("mkdir for %s: %w", file, mkerr)
				}
			} else {
				return fmt.Errorf("reading %s: %w", file, err)
			}
		}

		slices.SortFunc(fileEdits, func(a, b Edit) int {
			return cmp.Compare(b.StartByte, a.StartByte)
		})

		for _, e := range fileEdits {
			if int(e.EndByte) > len(content) {
				return fmt.Errorf("edit out of bounds in %s: end %d > len %d", file, e.EndByte, len(content))
			}
			content = append(content[:e.StartByte], append([]byte(e.NewText), content[e.EndByte:]...)...)
		}

		if err := os.WriteFile(target, content, 0o644); err != nil {
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
	oldDirBase := LastPathComponent(srcDir)
	newDirBase := LastPathComponent(dstDir)
	dirPairs := [][2]string{{srcDir, dstDir}}
	var planner PackageMovePlanner
	if p, ok := packageMovePlannerFor(result, srcDir); ok {
		planner = p
		if expanded := p.ExpandPackageDirs(result, srcDir, dstDir); len(expanded) > 0 {
			dirPairs = expanded
		}
	}

	var edits []Edit
	movedFiles := map[string]bool{}

	// Relocate package contents across every paired directory.
	for _, pair := range dirPairs {
		fromDir, toDir := pair[0], pair[1]
		pairSrc := Reference{Provider: "path", Path: "./" + fromDir}
		pairDst := Reference{Provider: "path", Path: "./" + toDir}
		for _, f := range result.Files {
			rel := strings.TrimPrefix(f.Path, "./")
			if !isUnderDir(rel, fromDir) {
				continue
			}
			srcFile := rel
			if movedFiles[srcFile] {
				continue
			}
			movedFiles[srcFile] = true
			under := strings.TrimPrefix(srcFile, fromDir)
			under = strings.TrimPrefix(under, "/")
			dstFile := toDir
			if under != "" {
				dstFile = path.Join(toDir, under)
			}
			content, err := os.ReadFile(filepath.Join(dir, srcFile))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", srcFile, err)
			}
			newContent := string(content)
			if driver, ok := moveDriverForLanguage(f.Language); ok {
				newContent = applyEditsToString(newContent, driver.RewriteImports(dstFile, content, result, pairSrc, pairDst))
			}
			edits = append(edits, Edit{
				File:      srcFile,
				StartByte: 0,
				EndByte:   uint32(len(content)),
				NewText:   "",
			})
			edits = append(edits, Edit{
				File:      dstFile,
				StartByte: 0,
				EndByte:   0,
				NewText:   newContent,
			})
		}
	}

	// Update references in all consumer files (files not relocated).
	for _, f := range result.Files {
		rel := strings.TrimPrefix(f.Path, "./")
		if movedFiles[rel] {
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

	if planner != nil {
		extra, err := planner.RewriteSupportFiles(dir, result, movedFiles, srcDir, dstDir)
		if err != nil {
			return nil, err
		}
		edits = append(edits, extra...)
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

// FindAllOccurrencesInStrings is like FindAllOccurrences but only produces
// edits for matches that occur inside double-quoted or raw string literals.
// Language packages use this for import-path text; language-specific boundary
// rules belong in those packages.
func FindAllOccurrencesInStrings(file string, content []byte, oldBase, newBase string) []Edit {
	if oldBase == "" || oldBase == newBase {
		return nil
	}
	var edits []Edit
	ForEachStringLiteral(content, func(seg string, start int) bool {
		sOff := 0
		for {
			idx := strings.Index(seg[sOff:], oldBase)
			if idx < 0 {
				break
			}
			posInSeg := sOff + idx
			pos := start + posInSeg
			edits = append(edits, Edit{
				File:      file,
				StartByte: uint32(pos),
				EndByte:   uint32(pos + len(oldBase)),
				NewText:   newBase,
			})
			sOff = posInSeg + len(oldBase)
		}
		return true
	})
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
