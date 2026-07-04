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
	edits := findPathSegmentOccurrencesInStrings("f.go", content, "pkg/api", "pkg/api_fuzz")
	if len(edits) != 1 || edits[0].NewText != "pkg/api_fuzz" {
		t.Fatalf("full path edits: %+v", edits)
	}
	edits = findPathSegmentOccurrencesInStringsWithParent("f.go", content, "api", "api_fuzz", "pkg")
	if len(edits) != 1 {
		t.Fatalf("parent-scoped leaf edits: %+v", edits)
	}
	edits = findPathSegmentOccurrencesInStrings("f.go", content, "cas", "cas_fuzz")
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
	write("pkg/b/b.go", "package b\n\ntype Unrelated struct{}\n\nfunc (Unrelated) WriteImage() {}\n\nfunc WriteImage() {}\n\nconst msg = \"a.WriteImage\"\n\nfunc Other() {\n\tUnrelated{}.WriteImage()\n\tWriteImage()\n}\n")
	write("cmd/app/main.go", "package main\n\nimport (\n\t\"example/pkg/a\"\n\t\"example/pkg/b\"\n)\n\nfunc main() {\n\tvar d a.Driver\n\td.WriteImage()\n\tb.Unrelated{}.WriteImage()\n\tb.WriteImage()\n\t_ = \"pkg.a.WriteImage\"\n}\n")
	edits, err := ingest.Rename(dir, "path:./pkg/a/a.go::*impl.WriteImage", "path:./pkg/a/a.go::*impl.Renamed")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatal(err)
	}
	bGot, err := os.ReadFile(filepath.Join(dir, "pkg/b/b.go"))
	if err != nil {
		t.Fatal(err)
	}
	mainGot, err := os.ReadFile(filepath.Join(dir, "cmd/app/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []struct{ name, body string }{
		{"pkg/b/b.go", string(bGot)},
		{"cmd/app/main.go", string(mainGot)},
	} {
		if strings.Contains(got.body, "Renamed") {
			t.Fatalf("%s wrongly renamed unrelated selectors:\n%s", got.name, got.body)
		}
		if !strings.Contains(got.body, "Unrelated{}.WriteImage()") {
			t.Fatalf("%s missing Unrelated{}.WriteImage():\n%s", got.name, got.body)
		}
	}
	if !strings.Contains(string(bGot), `"a.WriteImage"`) {
		t.Fatalf("string literal corrupted in pkg/b:\n%s", bGot)
	}
	if !strings.Contains(string(mainGot), `"pkg.a.WriteImage"`) {
		t.Fatalf("string literal corrupted in main:\n%s", mainGot)
	}
	if !strings.Contains(string(mainGot), "b.WriteImage()") {
		t.Fatalf("unrelated package call corrupted in main:\n%s", mainGot)
	}
}

func TestSelectorFindsVarCallInFacade(t *testing.T) {
	content := []byte("package wallpaper\n\nfunc SetStatic(path string) error {\n\tvar d Driver\n\treturn d.SetStatic(path)\n}\n\nfunc Wrap(ctx context.Context, path string) error {\n\td, err := driver.Get[Driver](ctx)\n\tif err != nil { return err }\n\treturn d.SetStatic(ctx, path)\n}\n")
	edits := findSelectorLeafEdits("facade.go", content, "SetStatic", "Fuzz", nil)
	if len(edits) != 2 {
		t.Fatalf("expected 2 selector edits, got %+v", edits)
	}
}

func TestSelectorIgnoresCommentApostrophes(t *testing.T) {
	content := []byte("package wallpaper\n\nfunc SetStatic(path string) error {\n\t// Ignore errors if service doesn't exist\n\td, err := get()\n\tif err != nil { return err }\n\treturn d.SetStatic(ctx, path)\n}\n")
	edits := findSelectorLeafEdits("facade.go", content, "SetStatic", "Fuzz", nil)
	if len(edits) != 1 {
		t.Fatalf("expected 1 selector edit despite comment apostrophe, got %+v", edits)
	}
}

func TestSelectorOnRealFacadeFile(t *testing.T) {
	content, err := os.ReadFile("/tmp/grok-goal-064fb14fad6a/implementer/work/ws-sel1/runs/workspaced/42/pkg/driver/wallpaper/facade.go")
	if err != nil { t.Fatal(err) }
	edits := findSelectorLeafEdits("pkg/driver/wallpaper/facade.go", content, "SetStatic", "Fuzz", nil)
	var hits []ingest.Edit
	for _, e := range edits {
		if e.StartByte == 1305 {
			hits = append(hits, e)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("expected selector at 1305, edits=%+v", edits)
	}
}
