package graphql

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestBuildProjectGraph_HasImportEdges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package example\n\nimport \"fmt\"\n\nfunc A() { fmt.Println() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nb, err := BuildProjectGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(nb.Nodes) < 2 {
		t.Fatalf("nodes=%d", len(nb.Nodes))
	}
	var sawExternal, sawEdge bool
	for _, n := range nb.Nodes {
		if n.External {
			sawExternal = true
		}
	}
	if len(nb.Edges) > 0 {
		sawEdge = true
	}
	if !sawEdge {
		t.Fatalf("expected import edges, nodes=%d edges=%d", len(nb.Nodes), len(nb.Edges))
	}
	if !sawExternal {
		t.Fatal("expected external go:fmt-style stub")
	}
}
