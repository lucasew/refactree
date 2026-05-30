package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// coerceLocalPathRef converts provider-less references into path references
// when they point to an existing local filesystem entry.
func coerceLocalPathRef(ref ingest.Reference) ingest.Reference {
	if ref.Provider != "" || ref.Path == "" {
		return ref
	}

	if _, err := os.Stat(ref.Path); err != nil {
		return ref
	}

	ref.Provider = "path"
	if filepath.IsAbs(ref.Path) || strings.HasPrefix(ref.Path, "./") || strings.HasPrefix(ref.Path, "../") {
		return ref
	}
	ref.Path = "./" + ref.Path
	return ref
}
