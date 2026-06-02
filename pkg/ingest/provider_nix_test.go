package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestWalkSymbols_NixProviderScope(t *testing.T) {
	root := t.TempDir()
	nixpkgsRoot := filepath.Join(root, "nixpkgs")
	libDir := filepath.Join(nixpkgsRoot, "lib")
	mustWrite(t, filepath.Join(libDir, "default.nix"), "{\n  id = x: x;\n}\n")

	old := os.Getenv("NIX_PATH")
	t.Cleanup(func() { _ = os.Setenv("NIX_PATH", old) })
	if err := os.Setenv("NIX_PATH", "nixpkgs="+nixpkgsRoot); err != nil {
		t.Fatal(err)
	}

	refs, err := collectRefs(".", "nix:nixpkgs/lib", ingest.ListOptions{IncludeHidden: true})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if len(refs) != 1 || refs[0] != "nix:nixpkgs/lib::id" {
		t.Fatalf("unexpected refs: %v", refs)
	}
}
