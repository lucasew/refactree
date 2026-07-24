package fuzzy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lucasew/refactree/pkg/ingest/python"
)

func TestRewriteExternalConsumers_Setutils(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "boltons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "boltons", "setutils_fuzz_3d1d.py"), []byte("x=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testPath := filepath.Join(dir, "tests", "test_setutils.py")
	if err := os.WriteFile(testPath, []byte("from boltons.setutils import IndexedSet\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := movePlan{
		Source:      "path:./setutils.py",
		Destination: "path:./setutils_fuzz_3d1d.py",
		Placement:   "new_module",
	}
	edits, err := rewriteExternalConsumers(dir, filepath.Join(dir, "boltons"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) == 0 {
		t.Fatal("expected external edits")
	}
	got, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "boltons.setutils_fuzz_3d1d") {
		t.Fatalf("test not rewritten:\n%s", got)
	}
}
