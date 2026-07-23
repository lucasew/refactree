package graphql

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	err := StreamNeighborhood(t.Context(), dir, "path:./a.go::A", func(ev StreamEvent) bool {
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
	// cwd is pkg/web/graphql; module root is ../../..
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// File path normalizes to module path; two-line label.
	n := LookupNode(root, "path:./pkg/web/server.go::New")
	if n == nil || n.Kind != NodeKindAtom {
		t.Fatalf("%+v", n)
	}
	if n.ID != "path:./pkg/web::New" {
		t.Fatalf("id=%q", n.ID)
	}
	if !strings.Contains(n.Label, "pkg/web") || !strings.Contains(n.Label, "New") {
		t.Fatalf("label=%q", n.Label)
	}
	if n.Language != "go" {
		t.Fatalf("language=%q want go", n.Language)
	}
	ext := LookupNode(root, "go:fmt")
	if ext == nil || !ext.External || ext.Language != "go" {
		t.Fatalf("%+v", ext)
	}
}
