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

// CanonicalizePathReference rewrites path refs that point at a directory to the
// language-specific backing file (via DirectoryModuleResolver on language drivers).
// baseDir is the project/serve root used to stat relative paths.
func CanonicalizePathReference(baseDir string, ref Reference) Reference {
	ref = normalizePathReference(ref)
	if ref.Provider != "path" || ref.Path == "" || ref.Path == "./" {
		return ref
	}

	if baseDir == "" {
		baseDir = "."
	}
	rel := strings.TrimPrefix(ref.Path, "./")
	abs := ref.Path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(baseDir, filepath.FromSlash(rel))
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return ref
	}

	entry, ok := ResolveDirectoryModuleFile(abs)
	if !ok {
		return ref
	}
	entry = filepath.ToSlash(entry)
	ref.Path = "./" + pathJoinSlash(rel, entry)
	return ref
}

func pathJoinSlash(dir, base string) string {
	dir = strings.TrimSuffix(strings.TrimPrefix(dir, "./"), "/")
	if dir == "" || dir == "." {
		return base
	}
	return dir + "/" + base
}

// ResolveInputReferenceScope parses input, coerces implicit local paths, and
// resolves scope + normalized reference for ingest operations.
func ResolveInputReferenceScope(baseDir, input string) ReferenceScope {
	ref := ParseReference(input)
	ref = CoerceLocalPathReference(baseDir, ref)
	ref = CanonicalizePathReference(baseDir, ref)
	return ResolveReferenceScope(baseDir, ref)
}
