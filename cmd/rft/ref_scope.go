package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// normalizeRefForCommandScope derives ingest dir and normalized reference
// using the same path-scoping primitive used by ls/doc/mv/browse commands.
func normalizeRefForCommandScope(ref ingest.Reference) (string, ingest.Reference) {
	dir := "."
	if ref.Provider == "path" {
		p := strings.TrimPrefix(ref.Path, "./")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			dir = p
		} else if p != "" {
			dir = filepath.Dir(p)
		}
	}
	return dir, normalizeRefForIngestDir(dir, ref)
}

// resolvePathRefForBrowse turns command-scoped path refs into absolute paths
// so browse opens the same scope that ls/doc/mv ingest against.
func resolvePathRefForBrowse(dir string, ref ingest.Reference) ingest.Reference {
	if ref.Provider != "path" || ref.Path == "" || filepath.IsAbs(ref.Path) {
		return ref
	}

	rel := strings.TrimPrefix(ref.Path, "./")
	base := dir
	if rel != "" && rel != "." {
		base = filepath.Join(dir, filepath.FromSlash(rel))
	}

	abs, err := filepath.Abs(base)
	if err != nil {
		return ref
	}
	ref.Path = abs
	return ref
}
