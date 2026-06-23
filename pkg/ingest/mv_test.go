package ingest_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestMv(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "mv")
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

			// Read op.json.
			opBytes, err := os.ReadFile(filepath.Join(dir, "op.json"))
			if err != nil {
				t.Fatalf("reading op.json: %v", err)
			}
			var op struct {
				Source      string `json:"source"`
				Destination string `json:"destination"`
			}
			if err := json.Unmarshal(opBytes, &op); err != nil {
				t.Fatalf("parsing op.json: %v", err)
			}

			// Copy input/ to a temp directory.
			tmpDir := t.TempDir()
			copyDir(t, filepath.Join(dir, "input"), tmpDir)

			// Run rename.
			edits, err := ingest.Rename(tmpDir, op.Source, op.Destination)
			if err != nil {
				t.Fatalf("rename: %v", err)
			}
			if err := ingest.ApplyEdits(tmpDir, edits); err != nil {
				t.Fatalf("apply edits: %v", err)
			}

			// Compare every file in expected/ with the corresponding file in tmpDir.
			expectedDir := filepath.Join(dir, "expected")
			compareDir(t, expectedDir, tmpDir)
		})
	}
}

// copyDir recursively copies src into dst.
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
			if err := os.MkdirAll(dp, 0755); err != nil {
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

// compareDir asserts every file in expected/ matches the corresponding file in got/.
func compareDir(t *testing.T, expectedDir, gotDir string) {
	t.Helper()
	err := filepath.Walk(expectedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
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
