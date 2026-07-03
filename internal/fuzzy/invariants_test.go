package fuzzy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestInvariantsOnIngestFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "ingest")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(fixtureDir, name)
			_, fails, err := fuzzy.RunIngestOnRoot(dir, fuzzy.IngestRunOptions{StrictRefs: false})
			if err != nil {
				t.Fatalf("ingest: %v", err)
			}
			if len(fails) > 0 {
				t.Fatalf("invariants: %v", fails)
			}
		})
	}
}
