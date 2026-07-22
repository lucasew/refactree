package pattern

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// RunResult is the collected outcome of a run (non-streaming convenience).
type RunResult struct {
	Matches []Match
	Edits   []ingest.Edit
}

// RunOptions controls which files are scanned under Root.
type RunOptions struct {
	// Paths are optional file or directory paths (absolute or relative to Root).
	// Empty means walk the whole Root.
	Paths []string
}

// StreamOptions is RunOptions plus per-match / per-file callbacks for map-style streaming.
type StreamOptions struct {
	Paths []string

	// OnMatch is invoked for each match as soon as its file is processed.
	// Return false to stop the walk early. Nil means collect-only via return value helpers.
	OnMatch func(Match) bool

	// OnFile is invoked after a file is matched (and, for rewrite, after its edits
	// are computed). fileEdits is nil for grep. Return false to stop.
	OnFile func(rel string, matches []Match, fileEdits []ingest.Edit) bool
}

// OpFromCLI builds an Op from grep/rewrite argv strings.
func OpFromCLI(mode, lang, patternStr, replacementStr string) (Op, error) {
	if lang == "" {
		lang = "go"
	}
	pat, err := ParsePattern(patternStr)
	if err != nil {
		return Op{}, fmt.Errorf("pattern: %w", err)
	}
	op := Op{
		Mode:      mode,
		Lang:      lang,
		Pattern:   patternStr,
		PatternIR: pat,
	}
	if mode == "rewrite" {
		repl, err := ParseReplacement(replacementStr)
		if err != nil {
			return Op{}, fmt.Errorf("replacement: %w", err)
		}
		op.Replacement = &replacementStr
		op.ReplacementIR = &repl
	}
	return op, nil
}

// Run loads sources under root, matches op.PatternIR, and for rewrite builds edits
// from op.ReplacementIR. It does not write the filesystem.
// Processing is per-file (map); results are collected (reduce) for the return value.
func Run(root string, op Op) (RunResult, error) {
	return RunWithOptions(root, op, RunOptions{})
}

// RunWithOptions is Run with an optional path filter.
func RunWithOptions(root string, op Op, opts RunOptions) (RunResult, error) {
	var out RunResult
	err := Stream(root, op, StreamOptions{
		Paths: opts.Paths,
		OnMatch: func(m Match) bool {
			out.Matches = append(out.Matches, m)
			return true
		},
		OnFile: func(rel string, matches []Match, fileEdits []ingest.Edit) bool {
			if len(fileEdits) > 0 {
				out.Edits = append(out.Edits, fileEdits...)
			}
			return true
		},
	})
	return out, err
}

// Stream processes one file at a time (map): hop-materialize → match → optional
// rewrite edits for that file. Callbacks fire as each file finishes so CLI can
// print matches without waiting for the whole tree (reduce is the consumer).
func Stream(root string, op Op, opts StreamOptions) error {
	if op.Lang != "" && op.Lang != "go" {
		return fmt.Errorf("pattern: lang %q not supported yet (only go)", op.Lang)
	}
	if op.PatternIR.Kind == "" {
		return fmt.Errorf("pattern: empty pattern_ir")
	}
	if op.Mode == "rewrite" && op.ReplacementIR == nil {
		return fmt.Errorf("rewrite missing replacement_ir")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	return forEachGoFile(rootAbs, opts.Paths, func(abs string) error {
		rel, err := filepath.Rel(rootAbs, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Per-file hop: enough for import-resolved stdlib/module refs in this file.
		// Avoids a full-tree Materialize before any output (map, not global shuffle).
		fileResult, err := ingest.MaterializeSource(ingest.ExtractSource{
			Kind:  ingest.ExtractHop,
			Root:  rootAbs,
			Paths: []string{abs},
		}, ingest.MaterializeOptions{ExpandImports: false})
		if err != nil {
			return fmt.Errorf("materialize %s: %w", rel, err)
		}

		source, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		pf, err := ingest.ParseSourceFile(abs, "go")
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		ms, err := MatchFile(rootAbs, rel, source, pf.Root, op.PatternIR, fileResult)
		pf.Close()
		if err != nil {
			return fmt.Errorf("match %s: %w", rel, err)
		}

		for _, m := range ms {
			if opts.OnMatch != nil && !opts.OnMatch(m) {
				return nil // early stop
			}
		}

		var fileEdits []ingest.Edit
		if op.Mode == "rewrite" && len(ms) > 0 {
			fileEdits, err = EditsForMatches(ms, *op.ReplacementIR)
			if err != nil {
				return err
			}
		}
		if opts.OnFile != nil && !opts.OnFile(rel, ms, fileEdits) {
			return nil
		}
		return nil
	})
}

// Apply runs a rewrite op and writes edits under root, file-by-file as matches are found.
func Apply(root string, op Op) (RunResult, error) {
	return ApplyWithOptions(root, op, RunOptions{})
}

// ApplyWithOptions is Apply with a path filter. Each file is rewritten as soon as
// it is processed (no global edit barrier).
func ApplyWithOptions(root string, op Op, opts RunOptions) (RunResult, error) {
	if op.Mode != "rewrite" {
		return RunResult{}, fmt.Errorf("Apply: mode is %q, want rewrite", op.Mode)
	}
	var out RunResult
	var applyErr error
	err := Stream(root, op, StreamOptions{
		Paths: opts.Paths,
		OnMatch: func(m Match) bool {
			out.Matches = append(out.Matches, m)
			return true
		},
		OnFile: func(rel string, matches []Match, fileEdits []ingest.Edit) bool {
			if len(fileEdits) == 0 {
				return true
			}
			out.Edits = append(out.Edits, fileEdits...)
			if err := ingest.ApplyEdits(root, fileEdits); err != nil {
				applyErr = err
				return false
			}
			return true
		},
	})
	if err != nil {
		return out, err
	}
	return out, applyErr
}

// forEachGoFile walks paths (or root) and calls fn for each .go file in order.
// Files are discovered incrementally for directory walks (no full path list first
// when Paths is empty or a single dir — still walks, yielding as found).
func forEachGoFile(rootAbs string, paths []string, fn func(abs string) error) error {
	if len(paths) == 0 {
		return walkGoFilesFn(rootAbs, fn)
	}
	seen := map[string]bool{}
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(rootAbs, p)
		}
		var err error
		abs, err = filepath.Abs(abs)
		if err != nil {
			return err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return err
		}
		if st.IsDir() {
			if err := walkGoFilesFn(abs, func(f string) error {
				if seen[f] {
					return nil
				}
				seen[f] = true
				return fn(f)
			}); err != nil {
				return err
			}
			continue
		}
		if !strings.HasSuffix(abs, ".go") {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if err := fn(abs); err != nil {
			return err
		}
	}
	return nil
}

func walkGoFilesFn(root string, fn func(abs string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		return fn(path)
	})
}
