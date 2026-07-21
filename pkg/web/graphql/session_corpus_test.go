package graphql

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestSessionCorpus_AbsorbsFileOnce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package example\n\nfunc A() { B() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package example\n\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewSessionCorpus(dir)
	if err := c.AbsorbSeed(filepath.Join(dir, "a.go")); err != nil {
		t.Fatal(err)
	}
	n1 := c.Len()
	if n1 < 1 {
		t.Fatalf("len=%d", n1)
	}
	// Second absorb of same seed must not grow (paths already present).
	if err := c.AbsorbSeed(filepath.Join(dir, "a.go")); err != nil {
		t.Fatal(err)
	}
	if c.Len() != n1 {
		t.Fatalf("re-absorb grew corpus %d -> %d", n1, c.Len())
	}

	var edges int
	if err := c.StreamVisit(context.Background(), "path:./a.go::A", func(ev StreamEvent) bool {
		if ev.Type == "edge" {
			edges++
		}
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if edges < 1 {
		t.Fatalf("edges=%d", edges)
	}
	// Second visit uses same corpus; still works.
	if err := c.StreamVisit(context.Background(), "path:./b.go::B", func(ev StreamEvent) bool {
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if c.Len() < n1 {
		t.Fatal("corpus shrank")
	}
}
