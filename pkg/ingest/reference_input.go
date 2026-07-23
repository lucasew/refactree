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

// ResolveMoveArgs prepares Rename root and source/destination refs from CLI-style
// inputs under project baseDir (typically cwd).
//
// Unlike ResolveInputReferenceScope, this does not narrow the ingest root to the
// source file's parent directory. Both paths stay relative to baseDir so a
// destination like path:./pkg/ingest/span.go is not rebased under the source
// package (e.g. pkg/pattern/pkg/ingest/…).
//
// Source is canonicalized (barrels / definition hops). Destination is only
// coerced and path-normalized: the target file may not exist yet.
func ResolveMoveArgs(baseDir, source, destination string) (root, src, dst string) {
	if baseDir == "" {
		baseDir = "."
	}
	srcRef := ParseReference(source)
	srcRef = CoerceLocalPathReference(baseDir, srcRef)
	srcRef = CanonicalizeReference(baseDir, srcRef)
	srcRef = NormalizeReferenceForScope(baseDir, baseDir, srcRef)

	dstRef := ParseReference(destination)
	dstRef = CoerceLocalPathReference(baseDir, dstRef)
	dstRef = NormalizeReferenceForScope(baseDir, baseDir, dstRef)

	return baseDir, srcRef.String(), dstRef.String()
}
