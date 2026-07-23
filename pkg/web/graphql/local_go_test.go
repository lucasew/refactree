package graphql

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStreamProject_LocalGoPackagesArePathNotExternal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg", "lib"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "lib", "lib.go"), []byte("package lib\n\nfunc L() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"example.com/app/pkg/lib\"\n\nfunc main() { lib.L() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c := NewSessionCorpus(dir)
	var ids []string
	err := c.StreamProject(t.Context(), func(ev StreamEvent) bool {
		if ev.Type == "edge" && ev.Edge != nil {
			ids = append(ids, ev.Edge.From, ev.Edge.To)
			t.Logf("edge %s %s → %s", ev.Edge.Kind, ev.Edge.From, ev.Edge.To)
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("expected edges from crawl")
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "go:example.com/app") {
			t.Fatalf("local package still go: id %q", id)
		}
		n := LookupNode(dir, id)
		if n == nil {
			t.Fatalf("nil node %s", id)
		}
		if n.External {
			t.Fatalf("in-repo node marked external: %s", id)
		}
	}
}

func TestLocalizeProviderToPath_GoModule(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.22\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "pkg", "lib"), 0755)
	os.WriteFile(filepath.Join(dir, "pkg", "lib", "lib.go"), []byte("package lib\n"), 0644)
	got := graphRefIDString(dir, "go:example.com/app/pkg/lib")
	if got != "path:./pkg/lib" {
		t.Fatalf("got %q want path:./pkg/lib", got)
	}
	// real external stays go:
	ext := graphRefIDString(dir, "go:fmt")
	if !strings.HasPrefix(ext, "go:") {
		t.Fatalf("fmt should stay go:, got %q", ext)
	}
}
