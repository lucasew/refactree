package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestStageEditsAndValidate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.go")
	src := "package main\n\nfunc Hello() {}\n\nfunc main() { Hello() }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := ingest.Rename(dir, "path:./main.go::Hello", "path:./main.go::Hi")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Edits) == 0 {
		t.Fatal("expected edits")
	}
	ov, err := ingest.StageEdits(dir, nil, plan.Edits)
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ValidateStagedProject(dir, ov); err != nil {
		t.Fatalf("validate: %v", err)
	}
	// disk unchanged
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Fatalf("disk changed: %q", got)
	}
}
