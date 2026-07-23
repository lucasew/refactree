package ingest

import (
	"cmp"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Edit describes a text replacement in a source file.
// Span is the half-open byte range [StartByte, EndByte) replaced by NewText.
type Edit struct {
	File string // path relative to the ingest root
	Span
	NewText string
}

// DirMove is a filesystem directory rename relative to the project root.
// Paths are slash-separated without a leading "./". Applied before Edits so
// package trees (including untracked/non-ingested co-located files) move as a unit.
type DirMove struct {
	From string
	To   string
}

// Plan is the full result of Rename: optional directory renames plus text edits.
// DirMoves run first (ApplyPlan); Edits target paths as they exist after those renames.
type Plan struct {
	DirMoves []DirMove
	Edits    []Edit
}

// Empty reports whether the plan has no directory moves and no text edits.
func (p Plan) Empty() bool {
	return len(p.DirMoves) == 0 && len(p.Edits) == 0
}

// Rename computes the plan needed to rename or move a symbol from sourceRef to
// destRef. Same-file operations are treated as renames; cross-file operations
// are treated as moves. Package moves may include DirMoves when a single
// source tree can be renamed wholesale (destination free).
func Rename(dir, sourceRef, destRef string) (Plan, error) {
	src := ParseReference(sourceRef)
	dst := ParseReference(destRef)
	if (src.Name == "") != (dst.Name == "") {
		return Plan{}, fmt.Errorf("source and destination references must both include symbols or both omit them (for package moves)")
	}

	// CLI may pass absolute path: refs; Result identity is ./rel under root.
	src = ProjectPathRef(dir, src)
	dst = ProjectPathRef(dir, dst)
	sourceRef = src.String()
	destRef = dst.String()

	slog.Debug("rename: materialize project", "root", dir, "source", sourceRef, "destination", destRef)
	result, err := ProjectResult(dir)
	if err != nil {
		return Plan{}, err
	}
	slog.Debug("rename: project loaded",
		"files", len(result.Files),
		"atoms", len(result.Atoms),
		"uses", len(result.Uses),
		"aliases", len(result.Aliases),
	)

	src, err = canonicalSourceReference(dir, result, src)
	if err != nil {
		return Plan{}, err
	}
	dst, err = canonicalDestinationReference(dir, result, src, dst)
	if err != nil {
		return Plan{}, err
	}
	// Keep project-relative path identity after canonicalize.
	src = ProjectPathRef(dir, src)
	dst = ProjectPathRef(dir, dst)
	slog.Debug("rename: canonical refs", "source", src.String(), "destination", dst.String())

	sourceRef = src.String()

	if src.Name == "" && dst.Name == "" {
		// package/dir move (no symbol); canonical* already short-circuit+normalize for Symbol==""
		slog.Debug("rename: package move")
		return planPackageMove(dir, result, src, dst)
	}

	sourceEntity, ok := findEntityByReference(result, sourceRef)
	if !ok {
		return Plan{}, fmt.Errorf("no entity found for reference %s", sourceRef)
	}

	if src.Path != dst.Path {
		if src.Name != dst.Name {
			return Plan{}, fmt.Errorf("cross-file move with symbol rename is not supported yet")
		}
		srcLang := languageForRefPath(result, src.Path)
		driver, ok := moveDriverForLanguage(srcLang)
		if !ok {
			return Plan{}, fmt.Errorf("cross-file move is not supported for language %q", srcLang)
		}
		slog.Debug("rename: cross-file move", "lang", srcLang, "from", src.Path, "to", dst.Path)
		edits, err := planCrossFileMove(dir, result, src, dst, sourceEntity, driver)
		if err != nil {
			return Plan{}, err
		}
		return Plan{Edits: edits}, nil
	}

	sourceRefs := []string{sourceRef}
	if srcLang := languageForRefPath(result, src.Path); srcLang != "" {
		if driver, ok := moveDriverForLanguage(srcLang); ok {
			if expander, ok := driver.(RenameExpander); ok {
				if extra := expander.ExpandRenameSources(dir, result, sourceRef); len(extra) > 0 {
					sourceRefs = uniqueStrings(append(sourceRefs, extra...))
				}
			}
		}
	}
	slog.Debug("rename: symbol rename",
		"path", src.Path,
		"from", AtomName(src.Name),
		"to", AtomName(dst.Name),
		"source_refs", len(sourceRefs),
	)
	edits, err := planSymbolRename(dir, result, sourceRefs, dst.Name)
	if err != nil {
		return Plan{}, err
	}
	return Plan{Edits: edits}, nil
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

func findEntityByReference(result *Result, ref string) (Atom, bool) {
	for _, ent := range result.Atoms {
		if ent.Reference == ref {
			return ent, true
		}
	}
	return Atom{}, false
}

func planSymbolRename(dir string, result *Result, sourceRefs []string, destSymbol string) ([]Edit, error) {
	if len(sourceRefs) == 0 {
		return nil, fmt.Errorf("no source references to rename")
	}
	// Uses often target language providers (go:mod/pkg::Sym) while entities
	// are path:./pkg/file.go::Sym. Expand so cross-package qualified calls rename.
	sourceSet := expandRenameSourceSet(dir, result, sourceRefs)
	var edits []Edit
	// Spans store the identifier leaf (e.g. "toJson"), while references may be
	// qualified ("Gson.toJson"). Always rewrite source text with the leaf.
	newText := AtomName(destSymbol)
	oldLeaf := AtomName(ParseReference(sourceRefs[0]).Name)
	slog.Debug("planSymbolRename", "source_set", len(sourceSet), "old_leaf", oldLeaf, "new_leaf", newText)

	// 1. Rename each related entity definition. Destination symbol qualifiers
	// stay aligned with each source entity's receiver prefix.
	nDef := 0
	for _, ent := range result.Atoms {
		if !sourceSet[ent.Reference] {
			continue
		}
		ref := ParseReference(ent.Reference)
		before := len(edits)
		edits = AppendReplaceSpan(edits, ref.Path, Span{StartByte: ent.StartByte, EndByte: ent.EndByte}, newText)
		if len(edits) > before {
			nDef++
		}
	}

	// 2. Rename at every call site that targets any expanded entity.
	// Prefer the registered site renamer (Rule / NFA backbone when pattern is
	// linked); otherwise walk result.Uses directly.
	useEdits := expandUseSiteRenames(dir, result, sourceSet, newText)
	edits = append(edits, useEdits...)

	// 3. Rename in import bindings that target any expanded entity.
	// Zero-span aliases (DefaultExport, re-exports) exist only for
	// CanonicalizeInResult — they are not textual sites and must not be rewritten
	// (rewriting [0:0] would insert the new name at the start of the file).
	nAlias := 0
	for _, alias := range result.Aliases {
		if !sourceSet[alias.Target] {
			continue
		}
		ref := ParseReference(alias.Reference)
		before := len(edits)
		edits = AppendReplaceSpan(edits, ref.Path, Span{StartByte: alias.StartByte, EndByte: alias.EndByte}, newText)
		if len(edits) > before {
			nAlias++
		}
	}
	slog.Debug("planSymbolRename: spans",
		"defs", nDef,
		"use_edits", len(useEdits),
		"aliases", nAlias,
		"total", len(edits),
	)

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
			edits = dedupeEdits(edits)
			if mover, ok := driver.(RenameFileMover); ok {
				if moves := mover.RenameFileMoves(result, sourceRefs, oldLeaf, newText); len(moves) > 0 {
					edits = applyRenameFileMoves(dir, edits, moves)
				}
			}
			return edits, nil
		}
	}

	return dedupeEdits(edits), nil
}

// applyRenameFileMoves folds per-span renames on files that must relocate into
// package-move style full-file edits (create new path, truncate old path).
func applyRenameFileMoves(rootDir string, edits []Edit, moves map[string]string) []Edit {
	if len(moves) == 0 {
		return edits
	}
	// Group span edits by file for files being relocated.
	byFile := map[string][]Edit{}
	var kept []Edit
	for _, e := range edits {
		file := strings.TrimPrefix(e.File, "./")
		if _, ok := moves[file]; ok {
			byFile[file] = append(byFile[file], e)
			continue
		}
		kept = append(kept, e)
	}
	for oldRel, newRel := range moves {
		if oldRel == "" || newRel == "" || oldRel == newRel {
			continue
		}
		content, err := os.ReadFile(filepath.Join(rootDir, oldRel))
		if err != nil {
			// Fall back to leaving span edits on the old path.
			kept = append(kept, byFile[oldRel]...)
			continue
		}
		newContent := string(ApplyEditsInMemory(content, byFile[oldRel]))
		kept = append(kept,
			Edit{File: oldRel, Span: Span{StartByte: 0, EndByte: uint32(len(content))}, NewText: ""},
			Edit{File: newRel, Span: Span{StartByte: 0, EndByte: 0}, NewText: newContent},
		)
	}
	return dedupeEdits(kept)
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
// rootDir is the ingest root (passed to language PackageImportMatchers).
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
		w := want{pkgDir: pkgDir, symbol: ref.Name}
		if ref.Name == "" || seenWant[w] {
			continue
		}
		seenWant[w] = true
		wants = append(wants, w)
	}
	if len(wants) == 0 || result == nil {
		return sourceSet
	}
	add := func(target string) {
		if target == "" || sourceSet[target] {
			return
		}
		t := ParseReference(target)
		if t.Name == "" {
			return
		}
		for _, w := range wants {
			if t.Name != w.symbol {
				continue
			}
			if targetMatchesPackageSymbol(rootDir, t, w.pkgDir) {
				sourceSet[target] = true
				return
			}
		}
	}
	for _, rel := range result.Uses {
		add(rel.Target)
	}
	for _, a := range result.Aliases {
		add(a.Target)
	}
	return sourceSet
}

// targetMatchesPackageSymbol reports whether ref names a symbol in package pkgDir
// (relative to the project root, e.g. "pkg/db").
// path: compares file directory. Other providers use PackageImportMatcher
// (language drivers; e.g. Go reads go.mod in ingest/go).
func targetMatchesPackageSymbol(rootDir string, ref Reference, pkgDir string) bool {
	if ref.Provider == "path" || ref.Provider == "" {
		dir := path.Dir(strings.TrimPrefix(ref.Path, "./"))
		if dir == "." {
			dir = ""
		}
		return dir == pkgDir
	}
	return packageImportIsPackage(rootDir, ref.Path, pkgDir)
}

// AtomName returns the identifier text written at a definition/use span.
// Qualified symbols use "." separators; pointer receivers may prefix "*".
//
// String-keyed members keep their quotes in the symbol path (e.g. Type.'.md'
// for a TS property named '.md'). A naive last-dot split would peel inside the
// quotes ("md'"); treat a trailing single- or double-quoted segment as the leaf.
func AtomName(symbol string) string {
	leaf := symbol
	if q := quotedSymbolLeaf(leaf); q != "" {
		return q
	}
	if i := strings.LastIndex(leaf, "."); i >= 0 {
		leaf = leaf[i+1:]
	}
	return strings.TrimPrefix(leaf, "*")
}

// quotedSymbolLeaf returns a trailing '…' or "…" leaf when it is a full path
// segment (start of symbol or immediately after "."). Empty string means fall back.
func quotedSymbolLeaf(symbol string) string {
	if len(symbol) < 2 {
		return ""
	}
	q := symbol[len(symbol)-1]
	if q != '\'' && q != '"' {
		return ""
	}
	// Scan left for the matching opener; content may contain "." ('.md').
	for i := len(symbol) - 2; i >= 0; i-- {
		if symbol[i] != q {
			continue
		}
		if i == 0 || symbol[i-1] == '.' {
			return symbol[i:]
		}
		// Quote mid-segment (unlikely); keep scanning for an earlier opener.
	}
	return ""
}

// planCrossFileMove orchestrates a cross-file move using the MoveDriver interface.
// It extracts the declaration, inserts it at the destination, and if the source
// and destination are in different directories, rewrites imports in consumer files.
func planCrossFileMove(dir string, result *Result, src, dst Reference, sourceEntity Atom, driver MoveDriver) ([]Edit, error) {
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
			Span: Span{StartByte: decl.RemoveStart, EndByte: decl.RemoveEnd},
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
	for _, rel := range result.Uses {
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
		if aliasTargetMatchesFile(result, alias.Target, ref, srcRel, src.Name) {
			targets[alias.Target] = true
		}
	}
	for _, rel := range result.Uses {
		ref := ParseReference(rel.Target)
		if ref.Provider == "path" {
			continue
		}
		if aliasTargetMatchesFile(result, rel.Target, ref, srcRel, src.Name) {
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
		wantEntRef := AtomRef("./"+srcFileRel, srcSymbol)
		for _, ent := range result.Atoms {
			if ent.Reference == wantEntRef {
				// Now check: does targetRef reference the same symbol?
				if targetRef.Name == srcSymbol {
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

// ApplyPlan applies directory renames then text edits under dir.
// DirMoves run first so package-tree renames (including untracked co-located
// files) complete before import/consumer rewrites target post-move paths.
func ApplyPlan(dir string, plan Plan) error {
	for _, m := range plan.DirMoves {
		if err := applyDirMove(dir, m); err != nil {
			return err
		}
	}
	return ApplyEdits(dir, plan.Edits)
}

// applyDirMove renames From → To under root (slash paths). Creates parent of To
// when missing. Fails if From is missing or To already exists.
func applyDirMove(root string, m DirMove) error {
	from := CleanRelDir(m.From)
	to := CleanRelDir(m.To)
	if from == "" || to == "" {
		return fmt.Errorf("dir move requires non-empty from/to")
	}
	if from == to {
		return nil
	}
	fromAbs := filepath.Join(root, filepath.FromSlash(from))
	toAbs := filepath.Join(root, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(toAbs), 0o755); err != nil {
		return fmt.Errorf("mkdir parent for %s: %w", to, err)
	}
	if err := os.Rename(fromAbs, toAbs); err != nil {
		return fmt.Errorf("rename dir %s → %s: %w", from, to, err)
	}
	slog.Debug("applyDirMove", "from", from, "to", to)
	return nil
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

		// File-level package relocate emits truncate (NewText:"") on old paths.
		// After the write, remove empty files instead of leaving 0-byte placeholders.
		if len(content) == 0 {
			_ = os.Remove(target)
		}
	}

	return nil
}

// canDirRename reports whether From can be os.Rename'd to To under root:
// From exists as a directory and To does not exist (no merge into an existing tree).
func canDirRename(root, from, to string) bool {
	from = CleanRelDir(from)
	to = CleanRelDir(to)
	if from == "" || to == "" || from == to {
		return false
	}
	fromAbs := filepath.Join(root, filepath.FromSlash(from))
	toAbs := filepath.Join(root, filepath.FromSlash(to))
	fi, err := os.Stat(fromAbs)
	if err != nil || !fi.IsDir() {
		return false
	}
	if _, err := os.Stat(toAbs); err == nil {
		return false
	} else if !os.IsNotExist(err) {
		return false
	}
	return true
}

// planPackageMove handles directory/package level moves (inter-package).
//
// Single pair + free destination: DirMove renames the tree on disk (pulls
// untracked/non-ingested co-located files), then Edits rewrite imports inside
// the moved tree (at new paths) and in consumer/support files.
//
// Multi-root (e.g. Java) or merge into an existing destination: file-level
// truncate/create relocation of ingested files only.
func planPackageMove(dir string, result *Result, src, dst Reference) (Plan, error) {
	srcDir := strings.TrimPrefix(src.Path, "./")
	dstDir := strings.TrimPrefix(dst.Path, "./")
	if srcDir == "" || dstDir == "" {
		return Plan{}, fmt.Errorf("package move requires non-empty directory paths")
	}
	if srcDir == dstDir {
		return Plan{}, nil // no-op; avoids pointless I/O + self-overwrite edits
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
	useDirMove := len(dirPairs) == 1 && canDirRename(dir, dirPairs[0][0], dirPairs[0][1])
	slog.Debug("planPackageMove", "srcDir", srcDir, "dstDir", dstDir, "dir_pairs", len(dirPairs), "dir_move", useDirMove)

	var plan Plan
	var edits []Edit
	movedFiles := map[string]bool{}
	nRelocated := 0

	if useDirMove {
		fromDir, toDir := dirPairs[0][0], dirPairs[0][1]
		plan.DirMoves = []DirMove{{From: fromDir, To: toDir}}
		pairSrc := Reference{Provider: "path", Path: "./" + fromDir}
		pairDst := Reference{Provider: "path", Path: "./" + toDir}
		// Mark ingested files as moved; plan import rewrites against post-rename paths.
		for _, f := range result.Files {
			rel := strings.TrimPrefix(f.Path, "./")
			if !isUnderDir(rel, fromDir) {
				continue
			}
			if movedFiles[rel] {
				continue
			}
			movedFiles[rel] = true
			nRelocated++
			under := strings.TrimPrefix(rel, fromDir)
			under = strings.TrimPrefix(under, "/")
			dstFile := toDir
			if under != "" {
				dstFile = path.Join(toDir, under)
			}
			content, err := os.ReadFile(filepath.Join(dir, rel))
			if err != nil {
				return Plan{}, fmt.Errorf("reading %s: %w", rel, err)
			}
			if driver, ok := moveDriverForLanguage(f.Language); ok {
				edits = append(edits, driver.RewriteImports(dstFile, content, result, pairSrc, pairDst)...)
			}
		}
		slog.Debug("planPackageMove: dir move relocate", "count", nRelocated, "from", fromDir, "to", toDir)
	} else {
		// File-level: truncate old paths + create new paths for ingested files only.
		// isUnderDir requires a path prefix (pkg/…), so nested leaves like
		// testdata/…/pkg are not relocated when moving top-level ./pkg.
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
				nRelocated++
				under := strings.TrimPrefix(srcFile, fromDir)
				under = strings.TrimPrefix(under, "/")
				dstFile := toDir
				if under != "" {
					dstFile = path.Join(toDir, under)
				}
				content, err := os.ReadFile(filepath.Join(dir, srcFile))
				if err != nil {
					return Plan{}, fmt.Errorf("reading %s: %w", srcFile, err)
				}
				newContent := string(content)
				if driver, ok := moveDriverForLanguage(f.Language); ok {
					newContent = string(ApplyEditsInMemory(content, driver.RewriteImports(dstFile, content, result, pairSrc, pairDst)))
				}
				edits = append(edits, Edit{
					File:    srcFile,
					Span:    Span{StartByte: 0, EndByte: uint32(len(content))},
					NewText: "",
				})
				edits = append(edits, Edit{
					File:    dstFile,
					Span:    Span{StartByte: 0, EndByte: 0},
					NewText: newContent,
				})
			}
		}
		slog.Debug("planPackageMove: file-level relocated", "count", nRelocated, "edits", len(edits))
	}

	// Rewrite imports only in files that the graph shows as consumers of
	// something defined under srcDir (path: or language import paths via
	// PackageImportMatcher). Not every file that textually contains the path leaf.
	consumers := packageMoveConsumerFiles(result, dir, srcDir, movedFiles)
	slog.Debug("planPackageMove: consumers", "count", len(consumers))
	langByFile := map[string]string{}
	for _, f := range result.Files {
		langByFile[strings.TrimPrefix(f.Path, "./")] = f.Language
	}
	nConsumerEdits := 0
	for consumerFile := range consumers {
		fcontent, err := os.ReadFile(filepath.Join(dir, consumerFile))
		if err != nil {
			return Plan{}, fmt.Errorf("reading %s: %w", consumerFile, err)
		}
		lang := langByFile[consumerFile]
		if driver, ok := moveDriverForLanguage(lang); ok {
			occs := driver.RewriteImports(consumerFile, fcontent, result, src, dst)
			nConsumerEdits += len(occs)
			edits = append(edits, occs...)
			continue
		}
		pathOld := oldDirBase
		pathNew := newDirBase
		if cp := CommonPathPrefix(srcDir, dstDir); cp != "" {
			if r := strings.Trim(strings.TrimPrefix(dstDir, cp), "/"); r != "" {
				pathNew = r
			}
		}
		if pathOld != pathNew {
			occs := FindAllWholeWordOccurrences(consumerFile, fcontent, pathOld, pathNew)
			nConsumerEdits += len(occs)
			edits = append(edits, occs...)
		}
	}
	slog.Debug("planPackageMove: consumer rewrites", "edits", nConsumerEdits, "total", len(edits))

	if planner != nil {
		extra, err := planner.RewriteSupportFiles(dir, result, movedFiles, srcDir, dstDir)
		if err != nil {
			return Plan{}, err
		}
		edits = append(edits, extra...)
	}

	plan.Edits = edits
	slog.Debug("planPackageMove: done", "dir_moves", len(plan.DirMoves), "edits", len(edits))
	return plan, nil
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

// packageMoveConsumerFiles returns project-relative files (not under the moved
// package tree) that have Uses or Aliases targeting something under srcDir.
// Matching is graph-based: path: refs under the tree, or language import paths
// via PackageImportMatcher. Linear in graph size.
func packageMoveConsumerFiles(result *Result, rootDir, srcDir string, movedFiles map[string]bool) map[string]bool {
	if result == nil {
		return nil
	}
	srcDir = CleanRelDir(srcDir)
	targets := targetsUnderPackageDir(result, rootDir, srcDir)
	if len(targets) == 0 {
		return nil
	}
	consumers := map[string]bool{}
	addConsumer := func(filePath string) {
		c := CleanRelDir(filePath)
		if c == "" || movedFiles[c] || isUnderDir(c, srcDir) {
			return
		}
		consumers[c] = true
	}
	for _, u := range result.Uses {
		if targets[u.Target] {
			addConsumer(ParseReference(u.Reference).Path)
		}
	}
	for _, a := range result.Aliases {
		if targets[a.Target] {
			addConsumer(ParseReference(a.Reference).Path)
		}
	}
	return consumers
}

// targetsUnderPackageDir collects reference strings that identify code under
// srcDir: path atoms/files and provider import paths for that package tree.
func targetsUnderPackageDir(result *Result, rootDir, srcDir string) map[string]bool {
	srcDir = CleanRelDir(srcDir)
	targets := map[string]bool{}
	if srcDir == "" || result == nil {
		return targets
	}
	consider := func(s string) {
		if s == "" || targets[s] {
			return
		}
		if targetRefersToPackageTree(rootDir, ParseReference(s), srcDir) {
			targets[s] = true
		}
	}
	for _, a := range result.Atoms {
		ref := ParseReference(a.Reference)
		p := CleanRelDir(ref.Path)
		if !isUnderDir(p, srcDir) {
			continue
		}
		targets[a.Reference] = true
		targets[FileRef("./"+p)] = true
	}
	for _, u := range result.Uses {
		consider(u.Target)
	}
	for _, al := range result.Aliases {
		consider(al.Target)
	}
	return targets
}

// targetRefersToPackageTree reports whether ref names a file/symbol under
// project-relative package directory srcDir (any depth), or a language-provider
// import path for that tree (via PackageImportMatcher — e.g. Go module paths).
func targetRefersToPackageTree(rootDir string, ref Reference, srcDir string) bool {
	srcDir = CleanRelDir(srcDir)
	if srcDir == "" {
		return false
	}
	if ref.Provider == "path" || ref.Provider == "" {
		return isUnderDir(CleanRelDir(ref.Path), srcDir)
	}
	return packageImportUnderTree(rootDir, ref.Path, srcDir)
}

// packageImportUnderTree asks registered PackageImportMatchers whether
// importPath refers to packageDir or a subpackage.
func packageImportUnderTree(rootDir, importPath, packageDir string) bool {
	moveDriversMu.RLock()
	defer moveDriversMu.RUnlock()
	for _, d := range moveDrivers {
		m, ok := d.(PackageImportMatcher)
		if !ok {
			continue
		}
		if m.ImportPathUnderPackageTree(rootDir, importPath, packageDir) {
			return true
		}
	}
	return false
}

// packageImportIsPackage asks registered PackageImportMatchers for an exact
// package import-path match.
func packageImportIsPackage(rootDir, importPath, packageDir string) bool {
	moveDriversMu.RLock()
	defer moveDriversMu.RUnlock()
	for _, d := range moveDrivers {
		m, ok := d.(PackageImportMatcher)
		if !ok {
			continue
		}
		if m.ImportPathIsPackage(rootDir, importPath, packageDir) {
			return true
		}
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
			Span: Span{StartByte: uint32(pos), EndByte: uint32(endPos)},
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
			Span: Span{StartByte: uint32(pos), EndByte: uint32(pos + len(oldBase))},
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
				Span: Span{StartByte: uint32(pos), EndByte: uint32(pos + len(oldBase))},
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
