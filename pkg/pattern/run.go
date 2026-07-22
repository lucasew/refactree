package pattern

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// RunResult is the outcome of applying an op over a project root (usually a copy of input/).
type RunResult struct {
	Matches []Match
	Edits   []ingest.Edit
}

// Run loads sources under root, matches op.PatternIR, and for rewrite builds edits
// from op.ReplacementIR. It does not write the filesystem.
func Run(root string, op Op) (RunResult, error) {
	if op.Lang != "" && op.Lang != "go" {
		return RunResult{}, fmt.Errorf("pattern: lang %q not supported yet (only go)", op.Lang)
	}

	result, err := ingest.MaterializeSource(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      root,
		Recursive: true,
	}, ingest.MaterializeOptions{ExpandImports: true})
	if err != nil {
		return RunResult{}, fmt.Errorf("materialize: %w", err)
	}

	var all []Match
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pf, err := ingest.ParseSourceFile(path, "go")
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		defer pf.Close()

		ms, err := MatchFile(root, rel, source, pf.Root, op.PatternIR, result)
		if err != nil {
			return fmt.Errorf("match %s: %w", rel, err)
		}
		all = append(all, ms...)
		return nil
	})
	if err != nil {
		return RunResult{}, err
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
	res, err := Run(root, op)
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
