package fuzzy_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestRunLocalIngestAndMv(t *testing.T) {
	catalog, _ := localGoCatalog(t, nil)

	res, err := fuzzy.Run(context.Background(), fuzzy.Options{
		CatalogPath: catalog,
		Mode:        fuzzy.ModeRun,
		Seed:        42,
		Iterations:  2,
		WorkRoot:    filepath.Join(t.TempDir(), "work"),
		ReportDir:   filepath.Join(t.TempDir(), "reports"),
		Allow:       true,
		NoIsolate:   true,
		StrictRefs:  false,
		FailFast:    true,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		t.Fatalf("run: %v (report %#v)", err, res)
	}
	if res.BugCount != 0 {
		t.Fatalf("bugs: %d report %s", res.BugCount, res.ReportDir)
	}
}

func TestPrefetchThenOfflineIngest(t *testing.T) {
	catalog, local := localGoCatalog(t, []string{".deps"})
	workRoot := filepath.Join(t.TempDir(), "work")
	seedDeps(t, local)

	pre := runPrefetch(t, catalog, workRoot)
	if pre.Passed != 1 {
		t.Fatalf("prefetch passed=%d", pre.Passed)
	}
	snap := filepath.Join(workRoot, "preserve", "local_go", ".deps", "ok")
	if _, err := os.Stat(snap); err != nil {
		t.Fatalf("missing preserve snapshot: %v", err)
	}
	worktree := filepath.Join(workRoot, "runs", "local_go", fuzzy.PrefetchRunID)
	if st, err := os.Stat(worktree); err != nil || !st.IsDir() {
		t.Fatalf("missing prefetch worktree: %v", err)
	}

	_, err := fuzzy.Run(context.Background(), fuzzy.Options{
		CatalogPath: catalog,
		Mode:        fuzzy.ModePrefetch,
		WorkRoot:    workRoot,
		Offline:     true,
		Allow:       true,
		NoIsolate:   true,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err == nil {
		t.Fatal("expected prefetch --offline to fail")
	}

	res := runIngest(t, catalog, workRoot, true)
	if res.BugCount != 0 || res.Passed != 1 {
		t.Fatalf("offline ingest bugs=%d passed=%d", res.BugCount, res.Passed)
	}
	ingestTree := filepath.Join(workRoot, "runs", "local_go", fuzzy.IngestRunID)
	if _, err := os.Stat(ingestTree); err != nil {
		t.Fatalf("missing ingest worktree: %v", err)
	}
}

func TestPrefetchIdempotent(t *testing.T) {
	catalog, local := localGoCatalog(t, []string{".deps"})
	workRoot := filepath.Join(t.TempDir(), "work")
	seedDeps(t, local)

	worktree := filepath.Join(workRoot, "runs", "local_go", fuzzy.PrefetchRunID)
	first := runPrefetch(t, catalog, workRoot)
	st1, err := os.Stat(worktree)
	if err != nil {
		t.Fatal(err)
	}
	snapBefore, err := os.ReadFile(filepath.Join(workRoot, "preserve", "local_go", ".deps", "ok"))
	if err != nil {
		t.Fatal(err)
	}

	second := runPrefetch(t, catalog, workRoot)
	if first.Passed != 1 || second.Passed != 1 {
		t.Fatalf("passed first=%d second=%d", first.Passed, second.Passed)
	}
	st2, err := os.Stat(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(st1, st2) {
		t.Fatal("prefetch rerun replaced worktree directory instead of reusing it")
	}
	snapAfter, err := os.ReadFile(filepath.Join(workRoot, "preserve", "local_go", ".deps", "ok"))
	if err != nil {
		t.Fatal(err)
	}
	if string(snapAfter) != string(snapBefore) {
		t.Fatalf("snapshot changed on rerun: %q -> %q", snapBefore, snapAfter)
	}
	for _, leak := range []string{
		filepath.Join(workRoot, "preserve", "local_go.tmp"),
		filepath.Join(workRoot, "preserve", "local_go.old"),
	} {
		if _, err := os.Stat(leak); !os.IsNotExist(err) {
			t.Fatalf("leaked snapshot path %s: %v", leak, err)
		}
	}
}

func TestIngestIdempotent(t *testing.T) {
	catalog, _ := localGoCatalog(t, nil)
	workRoot := filepath.Join(t.TempDir(), "work")
	worktree := filepath.Join(workRoot, "runs", "local_go", fuzzy.IngestRunID)

	first := runIngest(t, catalog, workRoot, false)
	st1, err := os.Stat(worktree)
	if err != nil {
		t.Fatal(err)
	}
	second := runIngest(t, catalog, workRoot, false)
	if first.Passed != 1 || second.Passed != 1 {
		t.Fatalf("passed first=%d second=%d", first.Passed, second.Passed)
	}
	st2, err := os.Stat(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(st1, st2) {
		t.Fatal("ingest rerun replaced worktree directory instead of reusing it")
	}
}

func runPrefetch(t *testing.T, catalog, workRoot string) *fuzzy.Result {
	t.Helper()
	res, err := fuzzy.Run(context.Background(), fuzzy.Options{
		CatalogPath: catalog,
		Mode:        fuzzy.ModePrefetch,
		WorkRoot:    workRoot,
		ReportDir:   filepath.Join(t.TempDir(), "reports"),
		Allow:       true,
		NoIsolate:   true,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		t.Fatalf("prefetch: %v (report %#v)", err, res)
	}
	return res
}

func runIngest(t *testing.T, catalog, workRoot string, offline bool) *fuzzy.Result {
	t.Helper()
	res, err := fuzzy.Run(context.Background(), fuzzy.Options{
		CatalogPath: catalog,
		Mode:        fuzzy.ModeIngest,
		WorkRoot:    workRoot,
		ReportDir:   filepath.Join(t.TempDir(), "reports"),
		Allow:       true,
		NoIsolate:   true,
		Offline:     offline,
		FailFast:    true,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		t.Fatalf("ingest: %v (report %#v)", err, res)
	}
	return res
}

func seedDeps(t *testing.T, local string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(local, ".deps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, ".deps", "ok"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func localGoCatalog(t *testing.T, preserve []string) (catalog string, local string) {
	t.Helper()
	moduleRoot := fuzzy.ModuleRoot()
	srcFixture := filepath.Join(moduleRoot, "testdata", "mv", "go_move_cross_file", "input")
	local = t.TempDir()
	copyTree(t, srcFixture, local)

	preserveLine := "preserve_globs = []"
	if len(preserve) > 0 {
		quoted := make([]string, len(preserve))
		for i, g := range preserve {
			quoted[i] = fmt.Sprintf("%q", g)
		}
		preserveLine = "preserve_globs = [" + strings.Join(quoted, ", ") + "]"
	}

	catalog = filepath.Join(t.TempDir(), "projects.toml")
	data := fmt.Sprintf(`
[projects.local_go]
language = "go"
local_path = %q
root = "."
setup_task = "-"
ingest_roots = ["."]
%s

[projects.local_go.mv]
enabled = true
ops = ["rename", "cross_file"]

[projects.local_go.isolate]
setup_network = true
check_network = false

[projects.local_go.mise.tasks.test]
run = "true"
`, local, preserveLine)
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return catalog, local
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
