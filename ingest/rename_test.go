package ingest_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/ingest"
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

func TestRename_PythonAliasedImport_CurrentLimitation(t *testing.T) {
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

	// Current behavior: aliased imports are not linked to target entities,
	// so only the symbol definition itself is renamed.
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit with aliased import limitation, got %d", len(edits))
	}
	if edits[0].File != "helper.py" {
		t.Fatalf("expected edit in helper.py, got %+v", edits[0])
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
