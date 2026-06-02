package nixref

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTarget_DirectoryUsesDefaultNixAsBackingFile(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "default.nix"), []byte("{\n  id = x: x;\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "extra.nix"), []byte("{ }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	old := os.Getenv("NIX_PATH")
	t.Cleanup(func() { _ = os.Setenv("NIX_PATH", old) })
	if err := os.Setenv("NIX_PATH", "nixpkgs="+root); err != nil {
		t.Fatal(err)
	}

	target, err := ResolveTarget("nixpkgs/lib")
	if err != nil {
		t.Fatalf("resolve target failed: %v", err)
	}
	if filepath.Clean(target.Dir) != filepath.Clean(libDir) {
		t.Fatalf("unexpected dir: got %q want %q", target.Dir, libDir)
	}
	if target.File != "default.nix" {
		t.Fatalf("unexpected backing file: got %q want %q", target.File, "default.nix")
	}
	if !target.IsDir {
		t.Fatal("expected directory target")
	}

	if !MatchesEntityPath(target, "default.nix") {
		t.Fatal("expected default.nix to match directory-backed nix target")
	}
	if MatchesEntityPath(target, "extra.nix") {
		t.Fatal("did not expect sibling nix file to match directory-backed nix target")
	}
}
