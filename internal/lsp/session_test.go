package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasGitAcceptsDirAndFile(t *testing.T) {
	root := t.TempDir()
	// No .git yet.
	if hasGit(root) {
		t.Fatal("expected no git marker")
	}

	// Normal repo: .git is a directory.
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !hasGit(root) {
		t.Fatal("expected hasGit for .git directory")
	}

	// Linked worktree / submodule: .git is a plain file.
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /tmp/fake.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasGit(wt) {
		t.Fatal("expected hasGit for .git file (worktree)")
	}
}

func TestDiscoverRootFindsWorktreeFromNestedPath(t *testing.T) {
	// Layout:
	//   outer/          (no .git)
	//   outer/wt/       (.git file — linked worktree root)
	//   outer/wt/pkg/x  (start path)
	// Without accepting .git files, walk would miss wt and fall back to pkg/x.
	outer := t.TempDir()
	wt := filepath.Join(outer, "wt")
	nested := filepath.Join(wt, "pkg", "x")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /tmp/fake.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := discoverRoot(nested)
	if !ok {
		t.Fatal("discoverRoot returned !ok")
	}
	want, err := filepath.Abs(wt)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("discoverRoot=%q want worktree root %q", got, want)
	}
}

func TestDiscoverRootFindsNormalRepo(t *testing.T) {
	outer := t.TempDir()
	repo := filepath.Join(outer, "repo")
	nested := filepath.Join(repo, "cmd", "rft")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := discoverRoot(nested)
	if !ok {
		t.Fatal("discoverRoot returned !ok")
	}
	want, err := filepath.Abs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("discoverRoot=%q want %q", got, want)
	}
}
