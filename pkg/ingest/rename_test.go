package ingest_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestRename_MissingEntity(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ingest.Rename(dir, "path:./main.go::doesNotExist", "path:./main.go::renamed")
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
	if !strings.Contains(err.Error(), "no entity found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRename_RenamesDefinitionAndCallsite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.py"), []byte("def helper():\n    pass\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("from helper import helper\n\n\ndef main():\n    helper()\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "path:./helper.py::helper", "path:./helper.py::renamed")
	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if len(edits) != 3 {
		t.Fatalf("expected 3 edits (definition + import + callsite), got %d", len(edits))
	}

	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatalf("apply edits failed: %v", err)
	}

	app, err := os.ReadFile(filepath.Join(dir, "app.py"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join(dir, "helper.py"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(helper), "def renamed():") {
		t.Fatalf("expected helper definition renamed, got:\n%s", helper)
	}
	if !strings.Contains(string(app), "from helper import renamed") {
		t.Fatalf("expected imported symbol renamed, got:\n%s", app)
	}
	if !strings.Contains(string(app), "renamed()") {
		t.Fatalf("expected callsite renamed, got:\n%s", app)
	}
}

func TestRename_ShorthandPathReferences(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.py"), []byte("def helper():\n    pass\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("from helper import helper\n\n\ndef main():\n    helper()\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "helper.py::helper", "helper.py::renamed")
	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if len(edits) != 3 {
		t.Fatalf("expected 3 edits (definition + import + callsite), got %d", len(edits))
	}
}

func TestRename_PythonAliasedImport_RenamesImportedMemberOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.py"), []byte("def helper():\n    pass\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("from helper import helper as h\n\n\ndef main():\n    h()\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "path:./helper.py::helper", "path:./helper.py::renamed")
	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (definition + imported member), got %d", len(edits))
	}

	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatalf("apply edits failed: %v", err)
	}

	app, err := os.ReadFile(filepath.Join(dir, "app.py"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join(dir, "helper.py"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(helper), "def renamed():") {
		t.Fatalf("expected helper definition renamed, got:\n%s", helper)
	}
	if !strings.Contains(string(app), "from helper import renamed as h") {
		t.Fatalf("expected imported member renamed, got:\n%s", app)
	}
	if !strings.Contains(string(app), "h()") {
		t.Fatalf("expected aliased callsite to stay as h(), got:\n%s", app)
	}
	if strings.Contains(string(app), "renamed()") {
		t.Fatalf("did not expect aliased callsite renamed, got:\n%s", app)
	}
}

func TestRename_JSAliasedImport_RenamesImportedMemberOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.js"), []byte("export function helper() {\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte("import { helper as h } from \"./helper.js\";\n\nfunction main() {\n  h();\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "path:./helper.js::helper", "path:./helper.js::doHelp")
	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (definition + imported member), got %d", len(edits))
	}

	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatalf("apply edits failed: %v", err)
	}

	main, err := os.ReadFile(filepath.Join(dir, "main.js"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join(dir, "helper.js"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(helper), "function doHelp()") {
		t.Fatalf("expected helper definition renamed, got:\n%s", helper)
	}
	if !strings.Contains(string(main), "import { doHelp as h }") {
		t.Fatalf("expected imported member renamed, got:\n%s", main)
	}
	if !strings.Contains(string(main), "h();") {
		t.Fatalf("expected aliased callsite to stay as h(), got:\n%s", main)
	}
	if strings.Contains(string(main), "doHelp();") {
		t.Fatalf("did not expect aliased callsite renamed, got:\n%s", main)
	}
}

func TestRename_ModuleAliasMemberCallsite_RenamesMemberAccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helpers.py"), []byte("def helper():\n    pass\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("import helpers as h\n\n\ndef main():\n    h.helper()\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "path:./helpers.py::helper", "path:./helpers.py::renamed")
	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (definition + member callsite), got %d", len(edits))
	}

	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatalf("apply edits failed: %v", err)
	}

	app, err := os.ReadFile(filepath.Join(dir, "app.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(app), "h.renamed()") {
		t.Fatalf("expected member access renamed, got:\n%s", app)
	}
}

func TestPackageMove_OnlyGraphConsumersRewritten(t *testing.T) {
	// Moving top-level ./pkg must not rewrite files under nested …/pkg that
	// share only the leaf name and have no import/use edge to the real package.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "lib.go"), []byte("package pkg\n\nfunc Hello() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"./pkg\"\n\nfunc main() { pkg.Hello() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Unrelated tree sharing the leaf name "pkg", with path text that a naive
	// segment rewrite would change, but no graph edge to top-level ./pkg.
	nested := filepath.Join(dir, "testdata", "fixture", "input", "pkg")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "x.go"), []byte("package pkg\n\nfunc Local() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(dir, "testdata", "fixture", "input", "other.go")
	// Local import to nested pkg only (resolves under testdata/…/input/pkg).
	if err := os.WriteFile(unrelated, []byte("package input\n\nimport nest \"./pkg\"\n\nfunc F() { nest.Local() }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "path:./pkg", "path:./pkga")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	for _, e := range edits {
		if strings.Contains(e.File, "testdata") {
			t.Fatalf("package move must not edit tree without use of moved package: %+v", e)
		}
	}
	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pkga", "lib.go")); err != nil {
		t.Fatalf("expected pkga/lib.go after move: %v", err)
	}
	// Nested fixture tree unchanged.
	got, err := os.ReadFile(unrelated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "pkga") {
		t.Fatalf("unrelated file rewritten:\n%s", got)
	}
	if !strings.Contains(string(got), `"./pkg"`) {
		t.Fatalf("local fixture import lost:\n%s", got)
	}
}

func TestPackageMove_ModuleImportPathConsumers(t *testing.T) {
	// Real monorepo shape: go.mod + import module/pkg/sub — must rewrite to module/pkga/sub.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/mod\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "sub", "lib.go"), []byte("package sub\n\nfunc Hello() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"example.com/mod/pkg/sub\"\n\nfunc main() { sub.Hello() }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "path:./pkg", "path:./pkga")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	var sawMain bool
	for _, e := range edits {
		if e.File == "main.go" {
			sawMain = true
			break
		}
	}
	if !sawMain {
		t.Fatalf("expected consumer rewrite on main.go; edits=%d", len(edits))
	}
	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(main), "example.com/mod/pkga/sub") {
		t.Fatalf("module import not rewritten:\n%s", main)
	}
	if strings.Contains(string(main), "example.com/mod/pkg/sub") {
		t.Fatalf("old module import still present:\n%s", main)
	}
	if _, err := os.Stat(filepath.Join(dir, "pkga", "sub", "lib.go")); err != nil {
		t.Fatalf("package not relocated: %v", err)
	}
}

func TestMove_GoCrossFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.go"), []byte("package main\n\nfunc helper() {\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ls.go"), []byte("package main\n\nfunc other() {\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n\thelper()\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	edits, err := ingest.Rename(dir, "doc.go::helper", "ls.go::helper")
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (remove+insert), got %d", len(edits))
	}

	if err := ingest.ApplyEdits(dir, edits); err != nil {
		t.Fatalf("apply edits failed: %v", err)
	}

	doc, err := os.ReadFile(filepath.Join(dir, "doc.go"))
	if err != nil {
		t.Fatal(err)
	}
	ls, err := os.ReadFile(filepath.Join(dir, "ls.go"))
	if err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(doc), "func helper()") {
		t.Fatalf("expected helper removed from doc.go, got:\n%s", doc)
	}
	if !strings.Contains(string(ls), "func helper()") {
		t.Fatalf("expected helper inserted into ls.go, got:\n%s", ls)
	}
	if !strings.Contains(string(main), "helper()") {
		t.Fatalf("expected callsite unchanged, got:\n%s", main)
	}
}

func TestApplyEdits_AppliesDescendingOffsets(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("alpha beta gamma\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := ingest.ApplyEdits(dir, []ingest.Edit{
		{File: "main.go", Span: ingest.Span{StartByte: 11, EndByte: 16}, NewText: "G"}, // gamma
		{File: "main.go", Span: ingest.Span{StartByte: 0, EndByte: 5}, NewText: "A"},   // alpha
	})
	if err != nil {
		t.Fatalf("apply edits failed: %v", err)
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "A beta G\n" {
		t.Fatalf("unexpected output: %q", string(got))
	}
}

func TestApplyEdits_OutOfBounds(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("abc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := ingest.ApplyEdits(dir, []ingest.Edit{{
		File:      "main.go",
		Span: ingest.Span{StartByte: 0, EndByte: 999},
		NewText:   "x",
	}})
	if err == nil {
		t.Fatal("expected out-of-bounds error")
	}
	if !strings.Contains(err.Error(), "out of bounds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyEdits_MissingFile(t *testing.T) {
	dir := t.TempDir()
	err := ingest.ApplyEdits(dir, []ingest.Edit{{
		File:      "missing.go",
		Span: ingest.Span{StartByte: 0, EndByte: 1},
		NewText:   "x",
	}})
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSymbolLeaf(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"helper", "helper"},
		{"A.Run", "Run"},
		{"*A.Run", "Run"},
		{"*A", "A"},
		{"Outer.Inner.field", "field"},
		// TS/JS string property keys (astro content types: Render.'.md')
		{"Render.'.md'", "'.md'"},
		{`Render.".md"`, `".md"`},
		{`Foo."a.b.c"`, `"a.b.c"`},
		{"Type.'.astro'", "'.astro'"},
		{"Outer.Inner.'.md'", "'.md'"},
	}
	for _, tc := range cases {
		if got := ingest.AtomName(tc.in); got != tc.want {
			t.Errorf("AtomName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
