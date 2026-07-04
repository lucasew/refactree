package ingestgo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestCrossPackageMoveRejectsUnexportedAndTests(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkga"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkgb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkga", "a.go"), []byte("package pkga\n\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkgb", "b.go"), []byte("package pkgb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ingest.Rename(dir, "path:./pkga/a.go::helper", "path:./pkgb/b.go::helper")
	if err == nil || !strings.Contains(err.Error(), "unexported symbol") {
		t.Fatalf("expected unsupported unexported move, got %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "pkga", "a_test.go"), []byte("package pkga\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ingest.Rename(dir, "path:./pkga/a_test.go::TestX", "path:./pkgb/b.go::TestX")
	if err == nil || !strings.Contains(err.Error(), "non-test file") {
		t.Fatalf("expected unsupported test move, got %v", err)
	}
}

func TestPathSegmentImportRewrite(t *testing.T) {
	content := []byte(`package p
import (
	"example/pkg/api"
	"example/pkg/palette/api"
)
const s = "case lucas"
`)
	edits := ingest.FindAllPathSegmentOccurrencesInStrings("f.go", content, "pkg/api", "pkg/api_fuzz")
	if len(edits) != 1 || edits[0].NewText != "pkg/api_fuzz" {
		t.Fatalf("full path edits: %+v", edits)
	}
	edits = ingest.FindAllPathSegmentOccurrencesInStringsWithParent("f.go", content, "api", "api_fuzz", "pkg")
	if len(edits) != 1 {
		t.Fatalf("parent-scoped leaf edits: %+v", edits)
	}
	edits = ingest.FindAllPathSegmentOccurrencesInStrings("f.go", content, "cas", "cas_fuzz")
	if len(edits) != 0 {
		t.Fatalf("expected no cas substring hits, got %+v", edits)
	}
}

func TestScaffoldDerivedRejects(t *testing.T) {
	// Minimized from workspaced fuzzy scaffolds iter 3 and 9.
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example\n\ngo 1.22\n")
	write("pkg/taskgroup/taskgroup_test.go", "package taskgroup\n\nimport \"testing\"\n\nfunc TestMap_Empty(t *testing.T) {}\n")
	write("pkg/deployer/planner.go", "package deployer\n")
	_, err := ingest.Rename(dir,
		"path:./pkg/taskgroup/taskgroup_test.go::TestMap_Empty",
		"path:./pkg/deployer/planner.go::TestMap_Empty")
	if err == nil || !strings.Contains(err.Error(), "non-test file") {
		t.Fatalf("iter3 scaffold: expected non-test reject, got %v", err)
	}

	write("cmd/selfupdate/root.go", "package selfupdate\n\nfunc createWorkspacedShim() {}\n\nfunc Run() { createWorkspacedShim() }\n")
	write("pkg/terminal/kitty/driver.go", "package kitty\n")
	_, err = ingest.Rename(dir,
		"path:./cmd/selfupdate/root.go::createWorkspacedShim",
		"path:./pkg/terminal/kitty/driver.go::createWorkspacedShim")
	if err == nil || !strings.Contains(err.Error(), "unexported symbol") {
		t.Fatalf("iter9 scaffold: expected unexported reject, got %v", err)
	}
}

func TestRenameMethodDoesNotTouchUnrelatedLeaf(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example\n\ngo 1.22\n")
	write("pkg/a/a.go", "package a\n\ntype Driver interface {\n\tWriteImage()\n}\n\ntype impl struct{}\n\nfunc (d *impl) WriteImage() {}\n")
	write("pkg/b/b.go", "package b\n\nfunc WriteImage() {}\n\nconst msg = \"WriteImage\"\n\nfunc Other() { WriteImage() }\n")
	edits, err := ingest.Rename(dir, "path:./pkg/a/a.go::*impl.WriteImage", "path:./pkg/a/a.go::*impl.Renamed")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "pkg/b/b.go"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`func WriteImage()`, `const msg = "WriteImage"`, `WriteImage()`} {
		if !strings.Contains(got, want) {
			t.Fatalf("unrelated leaf corrupted, missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Renamed") {
		t.Fatalf("unrelated package was renamed:\n%s", got)
	}
}
