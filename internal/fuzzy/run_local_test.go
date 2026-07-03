package fuzzy_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestRunLocalIngestAndMv(t *testing.T) {
	moduleRoot := fuzzy.ModuleRoot()
	srcFixture := filepath.Join(moduleRoot, "testdata", "mv", "go_move_cross_file", "input")
	local := t.TempDir()
	copyTree(t, srcFixture, local)

	catalog := filepath.Join(t.TempDir(), "projects.toml")
	data := fmt.Sprintf(`
[projects.local_go]
language = "go"
local_path = %q
root = "."
setup_task = "-"
ingest_roots = ["."]

[projects.local_go.mv]
enabled = true
ops = ["rename", "cross_file"]

[projects.local_go.isolate]
engine = "auto"

[projects.local_go.mise.tasks.test]
run = "true"
`, local)
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

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
