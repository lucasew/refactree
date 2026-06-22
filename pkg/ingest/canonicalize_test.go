package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalizeReference_ReexportChain(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("barrel/index.js", "export * from \"./inner.js\";\n")
	write("barrel/inner.js", "export { real } from \"./impl.js\";\n")
	write("barrel/impl.js", "export function real() {}\n")

	got := CanonicalizeReference(root, ParseReference("path:./barrel/index.js::real"))
	want := "path:./barrel/impl.js::real"
	if got.String() != want {
		t.Fatalf("got %q want %q", got.String(), want)
	}
}

func TestCanonicalizeInResult_UsesOnlyGraph(t *testing.T) {
	// Minimal hand-built Result — no filesystem.
	result := &Result{
		Entities: []Entity{
			{Reference: "path:./impl.js::real"},
		},
		Aliases: []Alias{
			{Reference: "path:./barrel.js", Target: "path:./inner.js"},
			{Reference: "path:./inner.js", Target: "path:./impl.js::real"},
		},
	}
	got := CanonicalizeInResult(result, ParseReference("path:./barrel.js::real"))
	if got.String() != "path:./impl.js::real" {
		t.Fatalf("got %q", got.String())
	}
}

func TestCanonicalizeReference_DefaultExportSoleEntity(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "mod.js")
	if err := os.WriteFile(p, []byte("export default function Thing() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := CanonicalizeReference(root, ParseReference("path:./mod.js"))
	if got.String() != "path:./mod.js::Thing" {
		t.Fatalf("got %q want path:./mod.js::Thing", got.String())
	}
}

func TestCanonicalizeReference_PesquisarrParaglide(t *testing.T) {
	root := "/home/lucasew/WORKSPACE/OPENSOURCE-own/pesquisarr"
	if _, err := os.Stat(root); err != nil {
		t.Skip("pesquisarr missing")
	}
	got := CanonicalizeReference(root, ParseReference(
		"path:./node_modules/@inlang/paraglide-js/dist/index.js::paraglideVitePlugin",
	))
	if got.String() != "path:./node_modules/@inlang/paraglide-js/dist/bundler-plugins/vite.js::paraglideVitePlugin" {
		t.Fatalf("got %q", got.String())
	}
}

func TestCanonicalizePathReference_StillDirectoryOnly(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "mod.js")
	if err := os.WriteFile(p, []byte("export default function Thing() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := CanonicalizePathReference(root, ParseReference("path:./mod.js"))
	if got.Symbol != "" {
		t.Fatalf("path-only canonicalize should not set symbol, got %q", got.String())
	}
}
