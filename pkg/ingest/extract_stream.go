// Discovery and graph materialization for package ingest.
//
// Call sites should use:
//   - WalkExtracts / WalkSymbols for lazy listing (no graph)
//   - ProjectResult / DirResult / SeedResult or MaterializeSource for a *Result
//
// Do not reintroduce a second WalkDir+parse path beside WalkExtracts.

package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/projectfs"
)

// ExtractKind selects how paths enter WalkExtracts.
type ExtractKind int

const (
	// ExtractDir walks a directory tree (skip rules + recursive policy).
	ExtractDir ExtractKind = iota
	// ExtractSeed BFS-expands from seed file paths (neighbors + import probes).
	// Used for serve annotate and canonicalize file hops.
	ExtractSeed
	// ExtractHop parses only the given path(s) with no neighbor BFS.
	// Used for single-file list scope.
	ExtractHop
)

// ExtractSource is the input to WalkExtracts: which paths to parse under Root.
type ExtractSource struct {
	Kind ExtractKind
	// Root is the ingest root for path relativity, the extract cache, and Resolve.
	// Empty means ".".
	Root string

	// Dir is the directory to walk when Kind is ExtractDir.
	// Empty means Root. Paths in FileExtract are relative to Root.
	Dir string
	// Recursive controls directory descent for ExtractDir.
	Recursive bool

	// Paths are seed/hop files (absolute or relative to Root).
	// Used by ExtractSeed and ExtractHop.
	Paths []string

	// FS is optional project content (nil = disk). LSP overlays pass a projectfs.Overlay.
	FS projectfs.FS
}

// MaterializeOptions controls graph construction over a closed extract set.
type MaterializeOptions struct {
	// ExpandImports one-hop parses import/reexport targets under Root
	// (project-complete Dir loads only; off for Seed/Hop).
	ExpandImports bool
	// FS is optional project content for ExpandImports reads (nil = disk).
	FS projectfs.FS
}

// WalkExtracts pulls FileExtract values for src, invoking yield for each
// non-nil extract. Returning false from yield stops early (lazy list / fzf).
// It never resolves the graph and never one-hop expands imports (Seed may BFS).
func WalkExtracts(src ExtractSource, yield func(*FileExtract) bool) error {
	if yield == nil {
		return fmt.Errorf("WalkExtracts: nil yield")
	}
	rootAbs, err := absRoot(src.Root)
	if err != nil {
		return err
	}

	fsys := src.FS
	if fsys == nil {
		fsys = projectfs.OS{}
	}

	switch src.Kind {
	case ExtractDir:
		dirAbs := rootAbs
		if src.Dir != "" {
			if filepath.IsAbs(src.Dir) {
				dirAbs, err = filepath.Abs(src.Dir)
			} else {
				dirAbs, err = filepath.Abs(filepath.Join(rootAbs, src.Dir))
			}
			if err != nil {
				return err
			}
		}
		return walkExtractsDir(rootAbs, dirAbs, src.Recursive, fsys, yield)

	case ExtractSeed:
		paths, err := normalizeSourcePaths(rootAbs, src.Paths)
		if err != nil {
			return err
		}
		return walkExtractsSeed(rootAbs, paths, fsys, yield)

	case ExtractHop:
		paths, err := normalizeSourcePaths(rootAbs, src.Paths)
		if err != nil {
			return err
		}
		return walkExtractsHop(rootAbs, paths, fsys, yield)

	default:
		return fmt.Errorf("WalkExtracts: unknown kind %d", src.Kind)
	}
}

// CollectExtracts drains WalkExtracts into a slice (complete materialize input).
func CollectExtracts(src ExtractSource) ([]*FileExtract, error) {
	var out []*FileExtract
	err := WalkExtracts(src, func(fe *FileExtract) bool {
		out = append(out, fe)
		return true
	})
	return out, err
}

// Materialize builds a *Result from a closed extract set under root.
// ExpandImports runs only when opts.ExpandImports is true.
func Materialize(root string, extracts []*FileExtract, opts MaterializeOptions) *Result {
	rootAbs, err := absRoot(root)
	if err != nil {
		rootAbs = root
		if rootAbs == "" {
			rootAbs = "."
		}
	}
	if opts.ExpandImports {
		seen := map[string]bool{}
		for _, fe := range extracts {
			if fe == nil {
				continue
			}
			abs := filepath.Join(rootAbs, filepath.FromSlash(strings.TrimPrefix(fe.Path, "./")))
			seen[abs] = true
		}
		fsys := opts.FS
		if fsys == nil {
			fsys = projectfs.OS{}
		}
		extracts = appendImportTargetExtracts(rootAbs, extracts, seen, fsys)
	}
	return resolve(rootAbs, extracts)
}

// MaterializeSource collects extracts via WalkExtracts then Materialize.
func MaterializeSource(src ExtractSource, opts MaterializeOptions) (*Result, error) {
	extracts, err := CollectExtracts(src)
	if err != nil {
		return nil, err
	}
	root := src.Root
	if root == "" {
		root = "."
	}
	if opts.FS == nil {
		opts.FS = src.FS
	}
	return Materialize(root, extracts, opts), nil
}

// ProjectResult is full-Dir Materialize with ExpandImports (mv, fuzzy, fixtures).
func ProjectResult(root string) (*Result, error) {
	return MaterializeSource(ExtractSource{
		Kind:      ExtractDir,
		Root:      root,
		Recursive: true,
	}, MaterializeOptions{ExpandImports: true})
}

// DirResult is Dir Materialize with ExpandImports (optional recursion).
func DirResult(root string, recursive bool) (*Result, error) {
	return MaterializeSource(ExtractSource{
		Kind:      ExtractDir,
		Root:      root,
		Recursive: recursive,
	}, MaterializeOptions{ExpandImports: true})
}

// SeedResult is Seed BFS Materialize without ExpandImports (serve, file hops).
func SeedResult(root, seedPath string) (*Result, error) {
	return MaterializeSource(ExtractSource{
		Kind:  ExtractSeed,
		Root:  root,
		Paths: []string{seedPath},
	}, MaterializeOptions{ExpandImports: false})
}

func absRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}
	return filepath.Abs(root)
}

func normalizeSourcePaths(rootAbs string, paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		var abs string
		if filepath.IsAbs(p) {
			abs = p
		} else {
			// Prefer cwd-absolute when the path already resolves under root
			// (e.g. join(relRoot, file) from WalkSymbols). Else join to rootAbs.
			if cand, err := filepath.Abs(p); err == nil {
				if rel, err := filepath.Rel(rootAbs, cand); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					abs = cand
				}
			}
			if abs == "" {
				abs = filepath.Join(rootAbs, filepath.FromSlash(strings.TrimPrefix(p, "./")))
			}
		}
		abs, err := filepath.Abs(abs)
		if err != nil {
			return nil, err
		}
		out = append(out, abs)
	}
	return out, nil
}

func walkExtractsDir(parseRoot, dirAbs string, recursive bool, fsys projectfs.FS, yield func(*FileExtract) bool) error {
	return filepath.WalkDir(dirAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != dirAbs && isSkippedDirName(d.Name()) {
				return filepath.SkipDir
			}
			if !recursive && path != dirAbs {
				return filepath.SkipDir
			}
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		fe, parseErr := parseFileCached(parseRoot, path, info, fsys)
		if parseErr != nil {
			return parseErr
		}
		if fe == nil {
			return nil
		}
		if !yield(fe) {
			return filepath.SkipAll
		}
		return nil
	})
}

func walkExtractsHop(rootAbs string, paths []string, fsys projectfs.FS, yield func(*FileExtract) bool) error {
	for _, abs := range paths {
		fe, err := parseFileCached(rootAbs, abs, nil, fsys)
		if err != nil {
			return err
		}
		if fe == nil {
			continue
		}
		if !yield(fe) {
			return nil
		}
	}
	return nil
}

func walkExtractsSeed(rootAbs string, seeds []string, fsys projectfs.FS, yield func(*FileExtract) bool) error {
	seen := map[string]bool{}
	queue := append([]string(nil), seeds...)

	for len(queue) > 0 {
		absPath := queue[0]
		queue = queue[1:]
		if seen[absPath] {
			continue
		}
		seen[absPath] = true

		if rel, err := filepath.Rel(rootAbs, absPath); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}

		fe, err := parseFileCached(rootAbs, absPath, nil, fsys)
		if err != nil {
			return err
		}
		if fe == nil {
			continue
		}
		if !yield(fe) {
			return nil
		}

		if isVendoredPath(rootAbs, absPath) {
			continue
		}
		for _, neigh := range bfsNeighbors(rootAbs, fe) {
			if !seen[neigh] {
				queue = append(queue, neigh)
			}
		}
	}
	return nil
}
