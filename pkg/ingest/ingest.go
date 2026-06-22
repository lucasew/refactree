package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

// Ingest discovers source files under dir, parses them with tree-sitter,
// and resolves cross-file references.
//
// Per-file extraction is cached by absolute path + ingest root + mtime so
// repeated walks (e.g. web serve) skip tree-sitter when sources are unchanged.
// resolve() still runs over the full extract set because relations depend on
// other files' entities and imports.
func Ingest(dir string) (*Result, error) {
	return ingestDir(dir, true)
}

// IngestWithRecursion discovers source files under dir and optionally descends
// into nested directories.
func IngestWithRecursion(dir string, recursive bool) (*Result, error) {
	return ingestDir(dir, recursive)
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
	lang, ok := grammar.GetByExtension(absPath)
	if !ok {
		return nil, nil
	}
	driver, ok := languageDriverForFile(absPath)
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
