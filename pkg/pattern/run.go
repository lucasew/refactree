package pattern

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// RunResult is the outcome of applying an op over a project root.
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
func Run(root string, op Op) (RunResult, error) {
	return RunWithOptions(root, op, RunOptions{})
}

// RunWithOptions is Run with an optional path filter.
func RunWithOptions(root string, op Op, opts RunOptions) (RunResult, error) {
	if op.Lang != "" && op.Lang != "go" {
		return RunResult{}, fmt.Errorf("pattern: lang %q not supported yet (only go)", op.Lang)
	}
	if op.PatternIR.Kind == "" {
		return RunResult{}, fmt.Errorf("pattern: empty pattern_ir")
	}

	result, err := ingest.MaterializeSource(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      root,
		Recursive: true,
	}, ingest.MaterializeOptions{ExpandImports: true})
	if err != nil {
		return RunResult{}, fmt.Errorf("materialize: %w", err)
	}

	files, err := collectGoFiles(root, opts.Paths)
	if err != nil {
		return RunResult{}, err
	}

	var all []Match
	for _, abs := range files {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return RunResult{}, err
		}
		rel = filepath.ToSlash(rel)
		source, err := os.ReadFile(abs)
		if err != nil {
			return RunResult{}, err
		}
		pf, err := ingest.ParseSourceFile(abs, "go")
		if err != nil {
			return RunResult{}, fmt.Errorf("parse %s: %w", rel, err)
		}
		ms, err := MatchFile(root, rel, source, pf.Root, op.PatternIR, result)
		pf.Close()
		if err != nil {
			return RunResult{}, fmt.Errorf("match %s: %w", rel, err)
		}
		all = append(all, ms...)
	}

	out := RunResult{Matches: all}
	if op.Mode == "rewrite" {
		if op.ReplacementIR == nil {
			return out, fmt.Errorf("rewrite missing replacement_ir")
		}
		edits, err := EditsForMatches(all, *op.ReplacementIR)
		if err != nil {
			return out, err
		}
		out.Edits = edits
	}
	return out, nil
}

// Apply runs a rewrite op and writes edits under root.
func Apply(root string, op Op) (RunResult, error) {
	return ApplyWithOptions(root, op, RunOptions{})
}

// ApplyWithOptions is Apply with a path filter.
func ApplyWithOptions(root string, op Op, opts RunOptions) (RunResult, error) {
	res, err := RunWithOptions(root, op, opts)
	if err != nil {
		return res, err
	}
	if op.Mode != "rewrite" {
		return res, fmt.Errorf("Apply: mode is %q, want rewrite", op.Mode)
	}
	if err := ingest.ApplyEdits(root, res.Edits); err != nil {
		return res, err
	}
	return res, nil
}

func collectGoFiles(root string, paths []string) ([]string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return walkGoFiles(rootAbs)
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(rootAbs, p)
		}
		abs, err = filepath.Abs(abs)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if st.IsDir() {
			files, err := walkGoFiles(abs)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if !seen[f] {
					seen[f] = true
					out = append(out, f)
				}
			}
			continue
		}
		if !strings.HasSuffix(abs, ".go") {
			continue
		}
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out, nil
}

func walkGoFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		if strings.HasSuffix(path, ".go") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}
