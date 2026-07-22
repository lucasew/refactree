package nix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
)

func TestReferenceProvider_ListScopeChildren(t *testing.T) {
	root := t.TempDir()
	nixpkgsRoot := filepath.Join(root, "nixpkgs")
	libDir := filepath.Join(nixpkgsRoot, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nixpkgsRoot, "default.nix"), []byte("import ./lib/default.nix\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nixpkgsRoot, "release.nix"), []byte("{ }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "default.nix"), []byte("{\n  id = x: x;\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	old := os.Getenv("NIX_PATH")
	t.Cleanup(func() { _ = os.Setenv("NIX_PATH", old) })
	if err := os.Setenv("NIX_PATH", "nixpkgs="+nixpkgsRoot); err != nil {
		t.Fatal(err)
	}

	children, ok, err := (referenceProvider{}).ListScopeChildren(ingest.ParseReference("nix:nixpkgs"), "", false)
	if err != nil {
		t.Fatalf("list scope children failed: %v", err)
	}
	if !ok {
		t.Fatal("expected nix provider child listing")
	}

	if !hasScopeChild(children, refpkg.ScopeChild{Ref: ingest.ParseReference("nix:nixpkgs/lib"), Kind: refpkg.ScopeChildDir}) {
		t.Fatalf("expected lib directory child, got %+v", children)
	}
	if !hasScopeChild(children, refpkg.ScopeChild{Ref: ingest.ParseReference("nix:nixpkgs/release.nix"), Kind: refpkg.ScopeChildFile}) {
		t.Fatalf("expected release.nix file child, got %+v", children)
	}
}

func TestWalkSymbols_NixProviderScope(t *testing.T) {
	root := t.TempDir()
	nixpkgsRoot := filepath.Join(root, "nixpkgs")
	libDir := filepath.Join(nixpkgsRoot, "lib")
	mustWriteFile(t, filepath.Join(libDir, "default.nix"), "{\n  id = x: x;\n}\n")

	old := os.Getenv("NIX_PATH")
	t.Cleanup(func() { _ = os.Setenv("NIX_PATH", old) })
	if err := os.Setenv("NIX_PATH", "nixpkgs="+nixpkgsRoot); err != nil {
		t.Fatal(err)
	}

	out := []string{}
	err := ingest.WalkAtoms(".", "nix:nixpkgs/lib", ingest.ListOptions{IncludeHidden: true}, func(sym ingest.AtomInfo) bool {
		out = append(out, sym.Atom.Reference)
		return true
	})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if len(out) != 1 || out[0] != "nix:nixpkgs/lib::id" {
		t.Fatalf("unexpected refs: %v", out)
	}
}

func hasScopeChild(children []refpkg.ScopeChild, want refpkg.ScopeChild) bool {
	for _, child := range children {
		if child.Kind == want.Kind && child.Ref == want.Ref {
			return true
		}
	}
	return false
}

func mustWriteFile(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
