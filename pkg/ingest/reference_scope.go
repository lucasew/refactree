package ingest

import (
	"os"
	"path/filepath"
	"strings"
)

// ReferenceScope is a normalized listing/doc scope tuple.
type ReferenceScope struct {
	Dir       string
	Reference Reference
}

// ResolveReferenceScope computes an ingest scope directory and normalizes
// path references to that scope.
func ResolveReferenceScope(baseDir string, ref Reference) ReferenceScope {
	if baseDir == "" {
		baseDir = "."
	}

	scopeDir := baseDir
	if ref.Provider == "path" {
		p := strings.TrimPrefix(ref.Path, "./")
		targetAbs := pathAbsFromBase(baseDir, ref.Path)
		if st, err := os.Stat(targetAbs); err == nil && st.IsDir() {
			switch {
			case filepath.IsAbs(ref.Path):
				scopeDir = ref.Path
			case p == "" || p == ".":
				scopeDir = baseDir
			default:
				scopeDir = filepath.Join(baseDir, p)
			}
		} else if p != "" {
			if filepath.IsAbs(ref.Path) {
				scopeDir = filepath.Dir(ref.Path)
			} else {
				scopeDir = filepath.Join(baseDir, filepath.Dir(p))
			}
		}
	}

	return ReferenceScope{
		Dir:       scopeDir,
		Reference: NormalizeReferenceForScope(baseDir, scopeDir, ref),
	}
}

// NormalizeReferenceForScope rewrites path references to be relative to
// scopeDir when they point inside scopeDir.
func NormalizeReferenceForScope(baseDir, scopeDir string, ref Reference) Reference {
	if ref.Provider != "path" || ref.Path == "" {
		return ref
	}

	scopeAbs, err := filepath.Abs(scopeDir)
	if err != nil {
		return ref
	}

	targetAbs := pathAbsFromBase(baseDir, ref.Path)
	rel, err := filepath.Rel(scopeAbs, targetAbs)
	if err != nil {
		return ref
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ref
	}

	rel = filepath.ToSlash(rel)
	if rel == "." {
		ref.Path = "./"
		return ref
	}
	ref.Path = "./" + rel
	return ref
}

// AbsolutePathReferenceForScope converts scoped path references into absolute
// path targets under scope.Dir.
func AbsolutePathReferenceForScope(scope ReferenceScope) Reference {
	ref := scope.Reference
	if ref.Provider != "path" || ref.Path == "" || filepath.IsAbs(ref.Path) {
		return ref
	}

	rel := strings.TrimPrefix(ref.Path, "./")
	target := scope.Dir
	if rel != "" && rel != "." {
		target = filepath.Join(scope.Dir, filepath.FromSlash(rel))
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return ref
	}
	ref.Path = abs
	return ref
}

func pathAbsFromBase(baseDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	trimmed := strings.TrimPrefix(p, "./")
	joined := filepath.Join(baseDir, trimmed)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return joined
	}
	return abs
}
