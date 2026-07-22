package pattern_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/pattern"
	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestPatternFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "pattern")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("reading fixture dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			dir := filepath.Join(fixtureDir, entry.Name())
			op, err := pattern.LoadOp(filepath.Join(dir, "op.json"))
			if err != nil {
				t.Fatalf("load op: %v", err)
			}

			tmpDir := t.TempDir()
			copyDir(t, filepath.Join(dir, "input"), tmpDir)

			switch op.Mode {
			case "grep":
				res, err := pattern.Run(tmpDir, op)
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				if op.ExpectMatchCount != nil && len(res.Matches) != *op.ExpectMatchCount {
					t.Fatalf("match count: got %d want %d\nmatches=%+v", len(res.Matches), *op.ExpectMatchCount, res.Matches)
				}
			case "rewrite":
				if _, err := pattern.Apply(tmpDir, op); err != nil {
					t.Fatalf("apply: %v", err)
				}
				compareDir(t, filepath.Join(dir, "expected"), tmpDir)
			default:
				t.Fatalf("unknown mode %q", op.Mode)
			}
		})
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(dp, 0o755); err != nil {
				t.Fatal(err)
			}
			copyDir(t, sp, dp)
			continue
		}
		in, err := os.Open(sp)
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(dp)
		if err != nil {
			in.Close()
			t.Fatal(err)
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func compareDir(t *testing.T, expectedDir, gotDir string) {
	t.Helper()
	err := filepath.WalkDir(expectedDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(expectedDir, path)
		if err != nil {
			return err
		}
		expContent, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		gotPath := filepath.Join(gotDir, rel)
		gotContent, err := os.ReadFile(gotPath)
		if err != nil {
			t.Errorf("missing file %s: %v", rel, err)
			return nil
		}
		if string(expContent) != string(gotContent) {
			t.Errorf("file %s mismatch:\n--- expected ---\n%s\n--- got ---\n%s",
				rel, expContent, gotContent)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
