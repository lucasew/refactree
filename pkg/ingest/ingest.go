package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

// Ingest discovers source files under dir, parses them with tree-sitter,
// and resolves cross-file references. Walks the whole tree — appropriate for
// refactorings (mv/rename) and fixtures that need a complete graph.
//
// Per-file extraction is cached by absolute path + ingest root + mtime so
// repeated walks (e.g. web serve) skip tree-sitter when sources are unchanged.
// resolve() still runs over the full extract set because relations depend on
// other files' entities and imports.
func Ingest(dir string) (*Result, error) {
	return ingestDir(dir, true)
}

// IngestWithRecursion discovers source files under dir and optionally descends
// into nested directories. Still a walk of that dir (full or top-level only).
func IngestWithRecursion(dir string, recursive bool) (*Result, error) {
	return ingestDir(dir, recursive)
}

// IngestForFile builds a result focused on one source file by breadth-first
// collecting neighbors (same-directory peers and import targets) instead of
// walking the whole module. Intended for inspection (serve annotate, browse);
// refactorings should keep using Ingest.
//
// seedPath is absolute or relative to rootDir.
func IngestForFile(rootDir, seedPath string) (*Result, error) {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		rootAbs = rootDir
	}

	seedAbs := seedPath
	if !filepath.IsAbs(seedAbs) {
		seedAbs = filepath.Join(rootAbs, filepath.FromSlash(strings.TrimPrefix(seedPath, "./")))
	}
	seedAbs, err = filepath.Abs(seedAbs)
	if err != nil {
		return nil, err
	}
	if rel, err := filepath.Rel(rootAbs, seedAbs); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("seed file %q is outside root %q", seedAbs, rootAbs)
	}

	seen := map[string]bool{}
	var extracts []*FileExtract
	queue := []string{seedAbs}

	for len(queue) > 0 {
		absPath := queue[0]
		queue = queue[1:]
		if seen[absPath] {
			continue
		}
		seen[absPath] = true

		fe, err := parseFileCached(rootAbs, absPath, nil)
		if err != nil {
			return nil, err
		}
		if fe == nil {
			continue
		}
		extracts = append(extracts, fe)

		for _, neigh := range bfsNeighbors(rootAbs, fe) {
			if !seen[neigh] {
				queue = append(queue, neigh)
			}
		}
	}

	return resolve(rootAbs, extracts), nil
}

func ingestDir(dir string, recursive bool) (*Result, error) {
	rootAbs, err := filepath.Abs(dir)
	if err != nil {
		rootAbs = dir
	}

	var extracts []*FileExtract

	err = filepath.Walk(rootAbs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if !recursive && path != rootAbs {
				return filepath.SkipDir
			}
			return nil
		}
		fe, parseErr := parseFileCached(rootAbs, path, info)
		if parseErr != nil {
			return parseErr
		}
		if fe != nil {
			extracts = append(extracts, fe)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resolve(rootAbs, extracts), nil
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
	out = append(out, listSourceFilesInDir(absDir)...)

	importerDir := filepath.ToSlash(filepath.Dir(fe.Path))
	if importerDir == "." {
		importerDir = ""
	}
	for _, imp := range fe.Imports {
		out = append(out, probeImportTargets(rootAbs, importerDir, imp.SourcePath)...)
	}
	return out
}

func listSourceFilesInDir(absDir string) []string {
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
		base := filepath.Join(rootAbs, filepath.FromSlash(c))
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
func parseFileCached(rootAbs, absPath string, info os.FileInfo) (*FileExtract, error) {
	if info == nil {
		var err error
		info, err = os.Stat(absPath)
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

	fe, err := parseFile(rootAbs, absPath)
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
func parseFile(dir, absPath string) (*FileExtract, error) {
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

	source, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", relPath, err)
	}

	parser := grammar.NewParser()
	defer parser.Delete()

	if !parser.SetLanguage(lang) {
		return nil, fmt.Errorf("failed to set language for %s", relPath)
	}

	tree := parser.ParseString(string(source))
	defer tree.Delete()

	root := tree.RootNode()
	return driver.Extract(root, source, relPath), nil
}
