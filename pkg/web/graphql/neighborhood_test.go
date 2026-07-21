package graphql

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildNeighborhood_AtomEgo(t *testing.T) {
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

	nb, err := BuildNeighborhood(dir, "path:./a.go::A")
	if err != nil {
		t.Fatal(err)
	}
	if nb.Focus == nil || nb.Focus.ID == "" {
		t.Fatal("expected focus")
	}
	if nb.Focus.Kind != NodeKindAtom {
		t.Fatalf("kind=%v", nb.Focus.Kind)
	}
	if !nb.Incomplete {
		t.Fatal("lazy neighborhood should be incomplete")
	}
	if len(nb.Nodes) < 1 {
		t.Fatal("expected nodes")
	}
	if len(nb.Edges) < 1 {
		t.Fatalf("expected use edges from A, got %d nodes=%d", len(nb.Edges), len(nb.Nodes))
	}
}

func TestBuildNeighborhood_FileHasAtomUseEdges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package example\n\nfunc A() { B() }\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nb, err := BuildNeighborhood(dir, "path:./a.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(nb.Nodes) < 2 {
		t.Fatalf("expected file + atoms, got %d", len(nb.Nodes))
	}
	if len(nb.Edges) < 1 {
		t.Fatalf("expected use edges among file atoms, got %d", len(nb.Edges))
	}
}
