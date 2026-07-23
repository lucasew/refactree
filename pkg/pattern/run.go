package pattern

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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
	// Empty means walk the whole Root via ingest.WalkExtracts (ExtractDir).
	Paths []string
}

// StreamOptions is RunOptions plus per-match / per-file callbacks for map-style streaming.
type StreamOptions struct {
	Paths []string

	// OnMatch is invoked for each match as soon as its file is processed.
	// source is that file's bytes (for Span.Text). Return false to stop early.
	OnMatch func(m Match, source []byte) bool

	// OnFile is invoked after a file is matched (and, for rewrite, after its edits
	// are computed). source is that file's bytes. Return false to stop.
	OnFile func(rel string, matches []Match, fileEdits []ingest.Edit, source []byte) bool
}

// OpFromCLI builds an Op from grep/rewrite argv strings.
// For rewrite, replacement may be "name=template" to rewrite only capture $name
// (when name is declared by the pattern); otherwise the whole match is replaced.
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
		setName, tmpl := splitCaptureSet(pat, replacementStr)
		repl, err := ParseReplacement(tmpl)
		if err != nil {
			return Op{}, fmt.Errorf("replacement: %w", err)
		}
		op.Replacement = &replacementStr
		op.ReplacementIR = &repl
		op.SetCapture = setName
	}
	return op, nil
}

// splitCaptureSet parses optional "name=template" rewrite emit.
// If name is a capture declared by pat, returns (name, template); else ("", repl).
func splitCaptureSet(pat Node, repl string) (name, template string) {
	i := strings.IndexByte(repl, '=')
	if i <= 0 {
		return "", repl
	}
	key := repl[:i]
	if !isCaptureIdent(key) {
		return "", repl
	}
	for _, n := range CaptureNames(pat) {
		if n == key {
			return key, repl[i+1:]
		}
	}
	return "", repl
}

func isCaptureIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
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
		OnMatch: func(m Match, _ []byte) bool {
			out.Matches = append(out.Matches, m)
			return true
		},
		OnFile: func(rel string, matches []Match, fileEdits []ingest.Edit, _ []byte) bool {
			if len(fileEdits) > 0 {
				out.Edits = append(out.Edits, fileEdits...)
			}
			return true
		},
	})
	return out, err
}

// Stream processes files via ingest.WalkExtracts (same skip rules / walk policy as
// ls, mv, serve). Per file: optional materialize for @ref, parse AST, match.
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

	needLinks := PatternNeedsLinks(op.PatternIR)
	stop := false

	return walkExtractSources(rootAbs, opts.Paths, func(fe *ingest.FileExtract) error {
		if stop {
			return nil
		}
		if fe == nil {
			return nil
		}
		if op.Lang != "" && fe.Language != op.Lang {
			return nil
		}

		rel := strings.TrimPrefix(filepath.ToSlash(fe.Path), "./")
		abs := filepath.Join(rootAbs, filepath.FromSlash(rel))

		// Soft size guard on walk-discovered files only (not explicit single-file hops
		// when caller listed that path as a file — walkExtractSources marks that).
		// Here we only know extracts from WalkExtracts; size-check every walk path.
		if st, err := os.Stat(abs); err == nil && st.Size() > maxGrepFileBytes {
			// Explicit hop of a single huge file: still process if Paths named it.
			if !isExplicitFilePath(rootAbs, opts.Paths, abs) {
				slog.Debug("pattern skip large file", "path", rel, "size", st.Size())
				return nil
			}
		}

		slog.Debug("pattern visit", "path", rel, "lang", fe.Language)

		source, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		pf, err := ingest.ParseSourceFile(abs, fe.Language)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}

		var fileResult *ingest.Result
		if needLinks {
			// Reuse the extract we already have from WalkExtracts (no second hop walk).
			fileResult = ingest.Materialize(rootAbs, []*ingest.FileExtract{fe}, ingest.MaterializeOptions{
				ExpandImports: false,
			})
		}

		ms, err := MatchFile(rootAbs, rel, source, pf.Root, op.PatternIR, fileResult)
		pf.Close()
		if err != nil {
			return fmt.Errorf("match %s: %w", rel, err)
		}

		for _, m := range ms {
			if opts.OnMatch != nil && !opts.OnMatch(m, source) {
				stop = true
				return nil
			}
		}

		var fileEdits []ingest.Edit
		if op.Mode == "rewrite" && len(ms) > 0 {
			fileEdits, err = EditsForMatches(ms, *op.ReplacementIR, source, op.SetCapture)
			if err != nil {
				return err
			}
		}
		if opts.OnFile != nil && !opts.OnFile(rel, ms, fileEdits, source) {
			stop = true
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
		OnMatch: func(m Match, _ []byte) bool {
			out.Matches = append(out.Matches, m)
			return true
		},
		OnFile: func(rel string, matches []Match, fileEdits []ingest.Edit, _ []byte) bool {
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

// maxGrepFileBytes soft-skips very large sources during recursive walks.
// Explicit file path arguments still process those files.
const maxGrepFileBytes = 256 << 10 // 256 KiB

// walkExtractSources streams FileExtract values using ingest.WalkExtracts —
// same directory skip rules and parse path as the rest of the system.
func walkExtractSources(rootAbs string, paths []string, fn func(*ingest.FileExtract) error) error {
	if len(paths) == 0 {
		var walkErr error
		err := ingest.WalkExtracts(ingest.ExtractSource{
			Kind:      ingest.ExtractDir,
			Root:      rootAbs,
			Recursive: true,
		}, func(fe *ingest.FileExtract) bool {
			if walkErr != nil {
				return false
			}
			if err := fn(fe); err != nil {
				walkErr = err
				return false
			}
			return true
		})
		if err != nil {
			return err
		}
		return walkErr
	}

	// Partition paths into files vs directories; each uses Hop or Dir on WalkExtracts.
	var filePaths []string
	var dirPaths []string
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(rootAbs, p)
		}
		abs, err := filepath.Abs(abs)
		if err != nil {
			return err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return err
		}
		if st.IsDir() {
			dirPaths = append(dirPaths, abs)
		} else {
			filePaths = append(filePaths, abs)
		}
	}

	var walkErr error
	yield := func(fe *ingest.FileExtract) bool {
		if walkErr != nil {
			return false
		}
		if err := fn(fe); err != nil {
			walkErr = err
			return false
		}
		return true
	}

	if len(filePaths) > 0 {
		if err := ingest.WalkExtracts(ingest.ExtractSource{
			Kind:  ingest.ExtractHop,
			Root:  rootAbs,
			Paths: filePaths,
		}, yield); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
	}
	for _, dirAbs := range dirPaths {
		// Dir may be outside rootAbs; Root stays project root for path relativity.
		// WalkExtracts ExtractDir uses Root for parse cache and Dir for walk start.
		relDir := dirAbs
		if r, err := filepath.Rel(rootAbs, dirAbs); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			relDir = r
		}
		if err := ingest.WalkExtracts(ingest.ExtractSource{
			Kind:      ingest.ExtractDir,
			Root:      rootAbs,
			Dir:       relDir,
			Recursive: true,
		}, yield); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
	}
	return walkErr
}

func isExplicitFilePath(rootAbs string, paths []string, abs string) bool {
	for _, p := range paths {
		cand := p
		if !filepath.IsAbs(cand) {
			cand = filepath.Join(rootAbs, p)
		}
		cand, err := filepath.Abs(cand)
		if err != nil {
			continue
		}
		if cand == abs {
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return true
			}
		}
	}
	return false
}
