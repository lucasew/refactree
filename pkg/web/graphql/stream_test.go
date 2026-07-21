package graphql

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestStreamNeighborhood_EmitsFocusEdgesDone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package example\n\nfunc A() { B() }\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var types []string
	var edges, nodes int
	err := StreamNeighborhood(context.Background(), dir, "path:./a.go::A", func(ev StreamEvent) bool {
		types = append(types, ev.Type)
		switch ev.Type {
		case "edge":
			edges++
		case "node":
			nodes++
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
	// nodes should not be bulk-streamed (only focus carries a node)
	if nodes > 0 {
		t.Fatalf("expected no standalone node events, got %d", nodes)
	}
}

func TestLookupNode_Cheap(t *testing.T) {
	n := LookupNode("/tmp", "path:./pkg/web/server.go::New")
	if n == nil || n.Kind != NodeKindAtom || n.Label != "New" {
		t.Fatalf("%+v", n)
	}
	if n.Language != "go" {
		t.Fatalf("language=%q want go", n.Language)
	}
	ext := LookupNode("/tmp", "go:fmt")
	if ext == nil || !ext.External || ext.Language != "go" {
		t.Fatalf("%+v", ext)
	}
}
