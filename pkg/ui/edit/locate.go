package edit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// locateDefinition resolves a symbol reference to an editor Location.
// ref should already be canonicalized (barrels / aliases followed).
func locateDefinition(baseDir string, ref ingest.Reference) (Location, error) {
	if ref.Name == "" {
		return Location{}, fmt.Errorf("no definition for %s", ref.String())
	}

	scope := ingest.ResolveReferenceScope(baseDir, ref)
	var found ingest.Atom
	var ok bool
	err := ingest.WalkAtoms(scope.Dir, scope.Reference.String(), ingest.ListOptions{
		IncludeHidden: true,
		Recursive:     true,
	}, func(sym ingest.AtomInfo) bool {
		found = sym.Atom
		ok = true
		return false
	})
	if err != nil {
		return Location{}, err
	}
	if !ok {
		return Location{}, fmt.Errorf("cannot resolve definition for %s", ref.String())
	}

	entRef := ingest.ParseReference(found.Reference)
	// Atom paths from WalkAtoms are relative to scope.Dir, not baseDir.
	path, err := absoluteRefPath(scope.Dir, entRef)
	if err != nil {
		return Location{}, err
	}
	return locationAtByte(path, found.StartByte)
}

func locationAtByte(path string, startByte uint32) (Location, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Location{}, err
	}
	li := grammar.NewLineIndexBytes(src)
	line, col0 := li.LineColumnAtU32(startByte)
	return Location{
		Path:   path,
		Line:   line,
		Column: col0 + 1, // editor columns are 1-based
	}, nil
}

func locationFileStart(path string) Location {
	return Location{Path: path, Line: 1, Column: 1}
}

func absoluteRefPath(baseDir string, ref ingest.Reference) (string, error) {
	if ref.Path == "" {
		return "", fmt.Errorf("empty path in reference %s", ref.String())
	}
	if filepath.IsAbs(ref.Path) {
		return filepath.Clean(ref.Path), nil
	}
	rel := strings.TrimPrefix(ref.Path, "./")
	if baseDir == "" {
		baseDir = "."
	}
	abs, err := filepath.Abs(filepath.Join(baseDir, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	return abs, nil
}

// isDirRef reports whether the path of ref is an existing directory under baseDir.
func isDirRef(baseDir string, ref ingest.Reference) (abs string, isDir bool, err error) {
	abs, err = absoluteRefPath(baseDir, ref)
	if err != nil {
		return "", false, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return abs, false, err
	}
	return abs, st.IsDir(), nil
}
