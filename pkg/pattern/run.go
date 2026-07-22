package pattern

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
	// Return false to stop the walk early.
	OnMatch func(Match) bool

	// OnFile is invoked after a file is matched (and, for rewrite, after its edits
	// are computed). fileEdits is nil/empty for pure grep. Return false to stop.
	OnFile func(rel string, matches []Match, fileEdits []ingest.Edit) bool
}

// OpFromCLI builds an Op from grep/rewrite argv strings.
func OpFromCLI(mode, lang, patternStr, replacementStr string) (Op, error) {
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

// Run loads sources under root, matches op.PatternIR, and for rewrite builds edits.
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

// Stream processes one file at a time (map): hop-materialize → match → callbacks.
func Stream(root string, op Op, opts StreamOptions) error {
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

	return forEachSourceFile(rootAbs, op.Lang, opts.Paths, func(abs, lang string) error {
		rel, err := filepath.Rel(rootAbs, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

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
		pf, err := ingest.ParseSourceFile(abs, lang)
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
				return nil
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

// Apply runs a rewrite op and writes edits under root, file-by-file.
func Apply(root string, op Op) (RunResult, error) {
	return ApplyWithOptions(root, op, RunOptions{})
}

// ApplyWithOptions is Apply with a path filter.
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

func forEachSourceFile(rootAbs, langFilter string, paths []string, fn func(abs, lang string) error) error {
	if len(paths) == 0 {
		return walkSourceFiles(rootAbs, langFilter, fn)
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
			if err := walkSourceFiles(abs, langFilter, func(f, lang string) error {
				if seen[f] {
					return nil
				}
				seen[f] = true
				return fn(f, lang)
			}); err != nil {
				return err
			}
			continue
		}
		lang, ok := ingest.LanguageForFile(abs)
		if !ok {
			continue
		}
		if langFilter != "" && lang != langFilter {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if err := fn(abs, lang); err != nil {
			return err
		}
	}
	return nil
}

func walkSourceFiles(root, langFilter string, fn func(abs, lang string) error) error {
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
		lang, ok := ingest.LanguageForFile(path)
		if !ok {
			return nil
		}
		if langFilter != "" && lang != langFilter {
			return nil
		}
		return fn(path, lang)
	})
}
