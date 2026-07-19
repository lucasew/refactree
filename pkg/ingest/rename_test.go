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
		{File: "main.go", StartByte: 11, EndByte: 16, NewText: "G"}, // gamma
		{File: "main.go", StartByte: 0, EndByte: 5, NewText: "A"},   // alpha
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
		StartByte: 0,
		EndByte:   999,
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
		StartByte: 0,
		EndByte:   1,
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
		if got := ingest.SymbolLeaf(tc.in); got != tc.want {
			t.Errorf("SymbolLeaf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

