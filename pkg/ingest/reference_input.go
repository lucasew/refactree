package ingest

import (
	"os"
	"path/filepath"
	"strings"
)

// CoerceLocalPathReference turns provider-less references into path references
// when they point to an existing local filesystem entry under baseDir.
func CoerceLocalPathReference(baseDir string, ref Reference) Reference {
	if baseDir == "" {
		baseDir = "."
	}
	if ref.Provider != "" || ref.Path == "" {
		return ref
	}

	statPath := ref.Path
	if !filepath.IsAbs(statPath) {
		statPath = filepath.Join(baseDir, statPath)
	}
	if _, err := os.Stat(statPath); err != nil {
		return ref
	}

	ref.Provider = "path"
	if filepath.IsAbs(ref.Path) || strings.HasPrefix(ref.Path, "./") || strings.HasPrefix(ref.Path, "../") {
		return ref
	}
	ref.Path = "./" + ref.Path
	return ref
}

// CleanRelDir strips a leading "./" and trailing "/" from a relative path.
// Used for directory keys that compare without those decorations.
func CleanRelDir(p string) string {
	return strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/")
}

func pathJoinSlash(dir, base string) string {
	dir = CleanRelDir(dir)
	if dir == "" || dir == "." {
		return base
	}
	return dir + "/" + base
}

// ResolveInputReferenceScope parses input, coerces implicit local paths, and
// canonicalizes the reference for ingest operations (directory module, definition hops).
func ResolveInputReferenceScope(baseDir, input string) ReferenceScope {
	ref := ParseReference(input)
	ref = CoerceLocalPathReference(baseDir, ref)
	ref = CanonicalizeReference(baseDir, ref)
	return ResolveReferenceScope(baseDir, ref)
}
