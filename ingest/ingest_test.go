package ingest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/lucasew/refactree/ingest"

	_ "github.com/lucasew/ccgo-tree-sitter/grammar/go"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/python"
)

func TestIngest(t *testing.T) {
	fixtureDir := filepath.Join("..", "testdata", "ingest")
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

			sortResult(&expected)
			sortResult(got)

			expJSON, _ := json.MarshalIndent(&expected, "", "  ")
			gotJSON, _ := json.MarshalIndent(got, "", "  ")

			if string(gotJSON) != string(expJSON) {
				t.Errorf("mismatch\n--- expected ---\n%s\n--- got ---\n%s", expJSON, gotJSON)
			}
		})
	}
}

func sortResult(r *ingest.Result) {
	sort.Slice(r.Files, func(i, j int) bool {
		return r.Files[i].Path < r.Files[j].Path
	})
	sort.Slice(r.Entities, func(i, j int) bool {
		if r.Entities[i].Reference != r.Entities[j].Reference {
			return r.Entities[i].Reference < r.Entities[j].Reference
		}
		return r.Entities[i].StartByte < r.Entities[j].StartByte
	})
	sort.Slice(r.Aliases, func(i, j int) bool {
		if r.Aliases[i].Reference != r.Aliases[j].Reference {
			return r.Aliases[i].Reference < r.Aliases[j].Reference
		}
		return r.Aliases[i].StartByte < r.Aliases[j].StartByte
	})
	sort.Slice(r.Relations, func(i, j int) bool {
		if r.Relations[i].Reference != r.Relations[j].Reference {
			return r.Relations[i].Reference < r.Relations[j].Reference
		}
		return r.Relations[i].StartByte < r.Relations[j].StartByte
	})
}
