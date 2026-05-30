package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestBrowseScopeFromReference_Directory(t *testing.T) {
	dir := t.TempDir()
	ref := ingest.ParseReference(dir)

	root, rel, err := browseScopeFromReference(ref)
	if err != nil {
		t.Fatalf("browse scope: %v", err)
	}
	if root != dir {
		t.Fatalf("unexpected root: got %q want %q", root, dir)
	}
	if rel != "." {
		t.Fatalf("unexpected rel: got %q want %q", rel, ".")
	}
}

func TestBrowseScopeFromReference_File(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	ref := ingest.ParseReference(file + "::main")
	root, rel, err := browseScopeFromReference(ref)
	if err != nil {
		t.Fatalf("browse scope: %v", err)
	}
	if root != dir {
		t.Fatalf("unexpected root: got %q want %q", root, dir)
	}
	if rel != "main.go" {
		t.Fatalf("unexpected rel: got %q want %q", rel, "main.go")
	}
}

func TestBrowseSetCurrentRel_RejectsOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	if err := model.setCurrentRel("../outside"); err == nil {
		t.Fatal("expected outside-root path error")
	}
}

func TestParentProviderPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"fmt", ""},
		{"net/http", "net"},
		{"github.com/lucasew/refactree/cmd/rft", "github.com/lucasew/refactree/cmd"},
		{"", ""},
	}
	for _, tc := range cases {
		got := parentProviderPath(tc.in)
		if got != tc.want {
			t.Fatalf("parentProviderPath(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewBrowseModelFromReference_GoProvider(t *testing.T) {
	model, err := newBrowseModelFromReference(ingest.ParseReference("go:fmt"), false)
	if err != nil {
		t.Fatalf("new browse model from go reference: %v", err)
	}
	if model.mode != "provider" {
		t.Fatalf("unexpected mode: %q", model.mode)
	}
	if model.providerRef.Provider != "go" || model.providerRef.Path != "fmt" {
		t.Fatalf("unexpected provider ref: %+v", model.providerRef)
	}
	if model.providerDir == "" {
		t.Fatal("expected providerDir to be resolved")
	}
}

func TestDocToMarkdown(t *testing.T) {
	doc := &ingest.DocResult{
		Name:      "Printf",
		Signature: "func Printf(format string, a ...any) (n int, err error)",
		DocString: "Printf formats according to a format specifier.",
	}

	got := docToMarkdown(doc)
	if !strings.Contains(got, "# Printf") {
		t.Fatalf("missing title in markdown: %q", got)
	}
	if !strings.Contains(got, "```") || !strings.Contains(got, "func Printf") {
		t.Fatalf("missing fenced signature in markdown: %q", got)
	}
	if !strings.Contains(got, "Printf formats according to a format specifier.") {
		t.Fatalf("missing doc string in markdown: %q", got)
	}
}
