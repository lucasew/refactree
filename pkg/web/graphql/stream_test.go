package graphql

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestStreamNeighborhood_EmitsFocusNodesEdgesDone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package example\n\nfunc A() { B() }\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var types []string
	var edges int
	err := StreamNeighborhood(context.Background(), dir, "path:./a.go::A", func(ev StreamEvent) bool {
		types = append(types, ev.Type)
		if ev.Type == "edge" {
			edges++
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if types[0] != "focus" {
		t.Fatalf("first=%v", types[0])
	}
	if types[len(types)-1] != "done" {
		t.Fatalf("last=%v", types[len(types)-1])
	}
	if edges < 1 {
		t.Fatalf("expected edges streamed, got %d events=%v", edges, types)
	}
}
