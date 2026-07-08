package ingest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"

	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/go"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/java"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/python"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
	_ "github.com/lucasew/refactree/pkg/ingest/java"
	_ "github.com/lucasew/refactree/pkg/ingest/js"
	_ "github.com/lucasew/refactree/pkg/ingest/nix"
	_ "github.com/lucasew/refactree/pkg/ingest/python"
)

func TestIngest(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "ingest")
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

			expectedBytes, err := os.ReadFile(filepath.Join(dir, "expected.json"))
			if err != nil {
				t.Fatalf("reading expected.json: %v", err)
			}
			var expected ingest.Result
			if err := json.Unmarshal(expectedBytes, &expected); err != nil {
				t.Fatalf("parsing expected.json: %v", err)
			}

			got, err := ingest.Ingest(dir)
			if err != nil {
				t.Fatalf("ingest: %v", err)
			}

			ingest.SortResult(&expected)
			ingest.SortResult(got)

			expJSON, _ := json.MarshalIndent(&expected, "", "  ")
			gotJSON, _ := json.MarshalIndent(got, "", "  ")

			if string(gotJSON) != string(expJSON) {
				t.Errorf("mismatch\n--- expected ---\n%s\n--- got ---\n%s", expJSON, gotJSON)
			}
		})
	}
}
