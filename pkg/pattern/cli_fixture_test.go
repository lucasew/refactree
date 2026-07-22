package pattern_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
	"github.com/lucasew/refactree/pkg/pattern"
)

// Ensures CLI string forms (not only hand-authored IR) drive the engine.
func TestCLIPatternsOnFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "pattern")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			dir := filepath.Join(fixtureDir, entry.Name())
			fileOp, err := pattern.LoadOp(filepath.Join(dir, "op.json"))
			if err != nil {
				t.Fatal(err)
			}
			repl := ""
			if fileOp.Replacement != nil {
				repl = *fileOp.Replacement
			}
			op, err := pattern.OpFromCLI(fileOp.Mode, fileOp.Lang, fileOp.Pattern, repl)
			if err != nil {
				t.Fatalf("OpFromCLI: %v", err)
			}
			tmp := t.TempDir()
			copyDir(t, filepath.Join(dir, "input"), tmp)
			switch op.Mode {
			case "grep":
				res, err := pattern.Run(tmp, op)
				if err != nil {
					t.Fatal(err)
				}
				if fileOp.ExpectMatchCount != nil && len(res.Matches) != *fileOp.ExpectMatchCount {
					t.Fatalf("matches %d want %d", len(res.Matches), *fileOp.ExpectMatchCount)
				}
			case "rewrite":
				if _, err := pattern.Apply(tmp, op); err != nil {
					t.Fatal(err)
				}
				compareDir(t, filepath.Join(dir, "expected"), tmp)
			}
		})
	}
}
