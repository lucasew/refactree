package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/lucasew/refactree/pkg/projectfs"
)

// appendImportTargetExtracts parses files referenced by imports/reexports when
// the language driver resolves them to a path under rootAbs (including a single
// node_modules entry). Does not recurse into dependency trees.
func appendImportTargetExtracts(rootAbs string, extracts []*FileExtract, seen map[string]bool, fsys projectfs.FS) []*FileExtract {
	if seen == nil {
		seen = map[string]bool{}
	}
	if fsys == nil {
		fsys = projectfs.OS{}
	}
	known := map[string]bool{}
	for _, fe := range extracts {
		if fe == nil {
			continue
		}
		p := strings.TrimPrefix(filepath.ToSlash(fe.Path), "./")
		known[p] = true
	}
	n := len(extracts)
	for i := 0; i < n; i++ {
		fe := extracts[i]
		if fe == nil {
			continue
		}
		driver, ok := languageDriverForName(fe.Language)
		if !ok {
			continue
		}
		var specs []string
		for _, im := range fe.Imports {
			specs = append(specs, im.SourcePath)
		}
		for _, re := range fe.Reexports {
			specs = append(specs, re.SourcePath)
		}
		ctx := ImportResolveContext{
			RootDir:      rootAbs,
			ImporterPath: fe.Path,
			KnownFiles:   known,
		}
		for _, spec := range specs {
			refStr := driver.ResolveImport(spec, ctx)
			r := ParseReference(refStr)
			if r.Provider != "path" || r.Path == "" {
				continue
			}
			rel := strings.TrimPrefix(filepath.ToSlash(r.Path), "./")
			abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
			if seen[abs] {
				continue
			}
			seen[abs] = true
			extra, err := parseFileCached(rootAbs, abs, nil, fsys)
			if err != nil || extra == nil {
				continue
			}
			extracts = append(extracts, extra)
			known[strings.TrimPrefix(filepath.ToSlash(extra.Path), "./")] = true
		}
	}
	return extracts
}

// bfsNeighbors returns absolute paths worth extracting next for inspection BFS:
// other source files in the same directory (same-package / co-located), and
// local files/dirs suggested by import specs (filesystem probe under root).
func bfsNeighbors(rootAbs string, fe *FileExtract) []string {
	var out []string
	absDir := filepath.Join(rootAbs, filepath.FromSlash(filepath.Dir(fe.Path)))
	if fe.Path == "." || filepath.Dir(fe.Path) == "." {
		absDir = rootAbs
	}
	// Co-located sources (Go package peers, sibling modules, etc.).
	if !isVendoredPath(rootAbs, absDir) {
		out = append(out, listSourceFilesInDir(absDir)...)
	}

	importerDir := filepath.ToSlash(filepath.Dir(fe.Path))
	if importerDir == "." {
		importerDir = ""
	}
	for _, imp := range fe.Imports {
		out = append(out, probeImportTargets(rootAbs, importerDir, imp.SourcePath)...)
	}
	for _, re := range fe.Reexports {
		out = append(out, probeImportTargets(rootAbs, importerDir, re.SourcePath)...)
	}
	return out
}

func listSourceFilesInDir(absDir string) []string {
	if isSkippedDirName(filepath.Base(absDir)) {
		return nil
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := languageDriverForFile(name); !ok {
			continue
		}
		out = append(out, filepath.Join(absDir, name))
	}
	return out
}

// isSkippedDirName reports dependency/build trees that must not be walked for
// inspection BFS or full ingest (still parse a file if it is an explicit seed).
func isSkippedDirName(name string) bool {
	switch strings.ToLower(name) {
	case "node_modules", ".git", "vendor", "dist", "build", "out", "coverage",
		".svelte-kit", ".next", ".nuxt", ".venv", "venv", "__pycache__",
		"target", ".turbo", ".cache", ".parcel-cache":
		return true
	default:
		return false
	}
}

// isVendoredPath reports whether abs is under a skipped dependency/build dir
// relative to rootAbs (or absolute path containing those segments).
func isVendoredPath(rootAbs, abs string) bool {
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		rel = abs
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if isSkippedDirName(part) {
			return true
		}
	}
	return false
}

// probeImportTargets maps an import spec to local files/dirs under rootAbs
// without requiring a prior full walk (knownFiles may be incomplete during BFS).
func probeImportTargets(rootAbs, importerDirRel, sourcePath string) []string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil
	}

	var candidates []string

	// Relative path (JS/Python relative, or ./foo).
	if strings.HasPrefix(sourcePath, ".") || strings.HasPrefix(sourcePath, "/") {
		base := importerDirRel
		if strings.HasPrefix(sourcePath, "/") {
			base = ""
		}
		joined := filepath.ToSlash(filepath.Clean(filepath.Join(base, sourcePath)))
		joined = strings.TrimPrefix(joined, "./")
		candidates = append(candidates, joined)
	}

	// Dotted / slashed module path (Python absolute, some JS).
	mod := strings.ReplaceAll(sourcePath, ".", "/")
	candidates = append(candidates, mod, sourcePath)

	// Go-style first segment as local dir (when not a full module URL).
	if !strings.Contains(sourcePath, ".") || strings.Count(sourcePath, "/") > 0 {
		candidates = append(candidates, sourcePath)
	}
	if i := strings.Index(sourcePath, "/"); i > 0 {
		candidates = append(candidates, sourcePath[:i])
	}

	seen := map[string]bool{}
	var out []string
	add := func(abs string) {
		if abs == "" || seen[abs] {
			return
		}
		st, err := os.Stat(abs)
		if err != nil {
			return
		}
		seen[abs] = true
		if st.IsDir() {
			// Never expand node_modules / vendor package roots as flat file lists.
			if isSkippedDirName(filepath.Base(abs)) {
				return
			}
			if isVendoredPath(rootAbs, abs) {
				// Barrel resolve only: index entry, not every file in the package.
				for _, idx := range []string{"index.js", "index.mjs", "index.cjs", "index.ts", "index.tsx"} {
					candidate := filepath.Join(abs, idx)
					if st2, err := os.Stat(candidate); err == nil && !st2.IsDir() {
						if _, ok := languageDriverForFile(candidate); ok {
							out = append(out, candidate)
						}
					}
				}
				return
			}
			out = append(out, listSourceFilesInDir(abs)...)
			return
		}
		if _, ok := languageDriverForFile(abs); ok {
			out = append(out, abs)
		}
	}

	for _, c := range candidates {
		c = strings.Trim(strings.TrimPrefix(filepath.ToSlash(c), "./"), "/")
		if c == "" || c == ".." || strings.HasPrefix(c, "../") {
			continue
		}
		// Bare package names (no relative path) that only resolve under node_modules
		// are left symbolic — BFS must not enter dependency trees from project code.
		// Relative hops (./foo) under an already-vendored seed remain allowed via abs paths.
		base := filepath.Join(rootAbs, filepath.FromSlash(c))
		if !strings.HasPrefix(sourcePath, ".") && isVendoredPath(rootAbs, base) {
			continue
		}
		add(base)
		add(base + ".py")
		add(base + ".go")
		add(base + ".js")
		add(filepath.Join(base, "__init__.py"))
		add(filepath.Join(base, "index.js"))
	}
	return out
}

type extractCacheKey struct {
	rootAbs string
	absPath string
}

type extractCacheEntry struct {
	modTime int64
	extract *FileExtract
}

var extractCache sync.Map // extractCacheKey -> extractCacheEntry

// parseFileCached returns a cached FileExtract when the file's mtime matches a
// previous parse for the same ingest root; otherwise parses and stores.
// fsys nil means disk.
func parseFileCached(rootAbs, absPath string, info os.FileInfo, fsys projectfs.FS) (*FileExtract, error) {
	if fsys == nil {
		fsys = projectfs.OS{}
	}
	if info == nil {
		var err error
		info, err = fsys.Stat(absPath)
		if err != nil {
			return nil, err
		}
	}

	key := extractCacheKey{rootAbs: rootAbs, absPath: absPath}
	modTime := info.ModTime().UnixNano()

	if v, ok := extractCache.Load(key); ok {
		ent := v.(extractCacheEntry)
		if ent.modTime == modTime {
			return ent.extract, nil
		}
	}

	fe, err := parseFile(rootAbs, absPath, fsys)
	if err != nil || fe == nil {
		return fe, err
	}

	extractCache.Store(key, extractCacheEntry{
		modTime: modTime,
		extract: fe,
	})
	return fe, nil
}

// ClearExtractCache drops all cached per-file extracts. Intended for tests.
func ClearExtractCache() {
	extractCache.Range(func(key, _ any) bool {
		extractCache.Delete(key)
		return true
	})
}

// parseFile parses a single source file and returns its FileExtract.
// Returns nil (no error) for unsupported file types.
//
// grammar.Parser serializes its own native handle; callers need no external
// parse lock (prefer one Parser per goroutine under parallel load).
//
// Some pure-Go tree-sitter grammars can SIGSEGV on valid source (observed on
// Go slice/variadic patterns). SetPanicOnFault turns that into a recoverable
// error so HTTP serve and ingest walks stay up.
func parseFile(dir, absPath string, fsys projectfs.FS) (*FileExtract, error) {
	if fsys == nil {
		fsys = projectfs.OS{}
	}
	driver, ok := languageDriverForFile(absPath)
	if !ok {
		return nil, nil
	}
	lang, ok := driver.TreeSitterGrammar(absPath)
	if !ok {
		return nil, nil
	}

	relPath, err := filepath.Rel(dir, absPath)
	if err != nil {
		return nil, err
	}

	source, err := fsys.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", relPath, err)
	}

	prevFault := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(prevFault)

	var fe *FileExtract
	var parseErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				parseErr = fmt.Errorf("tree-sitter fault parsing %s: %v", relPath, r)
			}
		}()

		pf, err := ParseSourceLanguage(source, lang, relPath)
		if err != nil {
			parseErr = err
			return
		}
		defer pf.Close()

		fe = driver.Extract(pf.Root, source, relPath)
	}()
	return fe, parseErr
}
