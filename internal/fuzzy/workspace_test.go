package fuzzy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestEnsureBareOfflineRequiresCache(t *testing.T) {
	ws, err := fuzzy.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := fuzzy.Project{ID: "demo", URL: "https://example.com/demo.git", Ref: "main", Family: "go"}
	_, _, err = ws.Prepare(p, "1", fuzzy.PrepareOptions{Offline: true})
	if err == nil {
		t.Fatal("expected offline prepare without cache to fail")
	}
}

func TestPrepareOfflineUsesBareCacheWithoutFetch(t *testing.T) {
	remote := initGitRepo(t)
	ref := gitOutput(t, remote, "rev-parse", "HEAD")

	workRoot := t.TempDir()
	ws, err := fuzzy.NewWorkspace(workRoot)
	if err != nil {
		t.Fatal(err)
	}
	p := fuzzy.Project{ID: "demo", URL: remote, Ref: ref, Family: "go", Root: "."}

	onlineDir, commit, err := ws.Prepare(p, "online", fuzzy.PrepareOptions{})
	if err != nil {
		t.Fatalf("online prepare: %v", err)
	}
	if commit == "" {
		t.Fatal("empty commit")
	}
	if err := os.WriteFile(filepath.Join(onlineDir, "marker.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Corrupt origin URL so any fetch would fail.
	cache := filepath.Join(workRoot, "cache", "demo.git")
	runGit(t, cache, "remote", "set-url", "origin", "https://invalid.example/nope.git")

	offlineDir, commit2, err := ws.Prepare(p, "offline", fuzzy.PrepareOptions{Offline: true})
	if err != nil {
		t.Fatalf("offline prepare: %v", err)
	}
	if commit2 != commit {
		t.Fatalf("commit mismatch: got %s want %s", commit2, commit)
	}
	if _, err := os.Stat(filepath.Join(offlineDir, "README")); err != nil {
		t.Fatalf("expected checked-out tree: %v", err)
	}
}

func TestPreserveSnapshotRoundTrip(t *testing.T) {
	ws, err := fuzzy.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "node_modules", "pkg", "index.js"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := fuzzy.Project{ID: "astro", PreserveGlobs: []string{"node_modules"}}
	if err := ws.SavePreserveSnapshot(p, work); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ws.RestorePreserveSnapshot(p, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "node_modules", "pkg", "index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "1" {
		t.Fatalf("got %q", data)
	}

	p2 := fuzzy.Project{ID: "missing", PreserveGlobs: []string{"node_modules"}}
	_, _, err = ws.Prepare(p2, "1", fuzzy.PrepareOptions{Offline: true})
	// local_path empty and no cache -> error before snapshot check
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPrepareReuseIsIdempotent(t *testing.T) {
	remote := initGitRepo(t)
	ref := gitOutput(t, remote, "rev-parse", "HEAD")
	ws, err := fuzzy.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := fuzzy.Project{
		ID:            "demo",
		URL:           remote,
		Ref:           ref,
		Family:        "go",
		Root:          ".",
		PreserveGlobs: []string{"vendor"},
	}
	first, commit, err := ws.Prepare(p, fuzzy.PrefetchRunID, fuzzy.PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(first, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "vendor", "lib"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ws.SavePreserveSnapshot(p, first); err != nil {
		t.Fatal(err)
	}
	st1, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}

	second, commit2, err := ws.Prepare(p, fuzzy.PrefetchRunID, fuzzy.PrepareOptions{Reuse: true})
	if err != nil {
		t.Fatal(err)
	}
	if second != first || commit2 != commit {
		t.Fatalf("reuse changed paths/commits: %s/%s %s/%s", first, second, commit, commit2)
	}
	st2, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(st1, st2) {
		t.Fatal("reuse replaced worktree directory")
	}
	data, err := os.ReadFile(filepath.Join(second, "vendor", "lib"))
	if err != nil || string(data) != "v1" {
		t.Fatalf("vendor restore: %v %q", err, data)
	}

	// Failed save must keep the previous snapshot.
	_ = os.RemoveAll(filepath.Join(first, "vendor"))
	if err := ws.SavePreserveSnapshot(p, first); err == nil {
		t.Fatal("expected save without globs to fail")
	}
	data, err = os.ReadFile(filepath.Join(ws.Root, "preserve", "demo", "vendor", "lib"))
	if err != nil || string(data) != "v1" {
		t.Fatalf("atomic save clobbered snapshot: %v %q", err, data)
	}
}

func TestPrepareOfflineRequiresPreserveSnapshot(t *testing.T) {
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := fuzzy.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := fuzzy.Project{
		ID:            "local",
		Family:        "go",
		LocalPath:     local,
		PreserveGlobs: []string{"node_modules"},
	}
	_, _, err = ws.Prepare(p, "1", fuzzy.PrepareOptions{Offline: true})
	if err == nil {
		t.Fatal("expected missing preserve snapshot error")
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}
