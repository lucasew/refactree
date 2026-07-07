package fuzzy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSetDefaultWorkRoot(t *testing.T) {
	prev := DefaultWorkRoot()
	t.Cleanup(func() { SetDefaultWorkRoot(prev) })

	want := filepath.Join(t.TempDir(), "custom-work-root")
	SetDefaultWorkRoot(want)
	if got := DefaultWorkRoot(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if st, err := os.Stat(want); err != nil || !st.IsDir() {
		t.Fatalf("work-root not created: %v", err)
	}
}

func TestSplitCommaIDs(t *testing.T) {
	got := splitCommaIDs(" a, b ,c,,")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("%v", got)
	}
}

func TestPrefetchNoOpWhenWarm(t *testing.T) {
	// Minimal local_path catalog that is "warm" after one Prefetch, second is no-op.
	moduleRoot := ModuleRoot()
	src := filepath.Join(moduleRoot, "testdata", "mv", "go_move_cross_file", "input")
	local := t.TempDir()
	copyDirForTest(t, src, local)
	if err := os.MkdirAll(filepath.Join(local, ".deps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, ".deps", "ok"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}

	workRoot := filepath.Join(t.TempDir(), "work")
	catalog := filepath.Join(t.TempDir(), "projects.toml")
	data := `
[projects.local_go]
language = "go"
local_path = "` + local + `"
root = "."
setup_task = "-"
ingest_roots = ["."]
preserve_globs = [".deps"]

[projects.local_go.mv]
enabled = false

[projects.local_go.isolate]
setup_network = false
check_network = false

[projects.local_go.mise.tasks.test]
run = "true"
`
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	root, err := Prefetch(ctx, PrefetchOptions{
		WorkRoot:    workRoot,
		CatalogPath: catalog,
		NoIsolate:   true,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		t.Fatalf("first prefetch: %v", err)
	}
	if root != workRoot {
		t.Fatalf("root %q", root)
	}

	// Mutate nothing; second must no-op (ValidateOfflineReady).
	root2, err := Prefetch(ctx, PrefetchOptions{
		WorkRoot:    workRoot,
		CatalogPath: catalog,
		NoIsolate:   true,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		t.Fatalf("second prefetch: %v", err)
	}
	if root2 != workRoot {
		t.Fatal(root2)
	}
}

func copyDirForTest(t *testing.T, src, dst string) {
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
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
