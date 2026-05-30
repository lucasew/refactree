package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/ingest"
)

func TestCoerceLocalPathRef_Directory(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("cmd", 0755); err != nil {
		t.Fatal(err)
	}

	ref := ingest.ParseReference("cmd")
	got := coerceLocalPathRef(ref)
	if got.Provider != "path" || got.Path != "./cmd" {
		t.Fatalf("unexpected coerced ref: %+v", got)
	}
}

func TestCoerceLocalPathRef_File(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("x.go", []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ref := ingest.ParseReference("x.go")
	got := coerceLocalPathRef(ref)
	if got.Provider != "path" || got.Path != "./x.go" {
		t.Fatalf("unexpected coerced ref: %+v", got)
	}
}

func TestCoerceLocalPathRef_MissingPath(t *testing.T) {
	ref := ingest.ParseReference(filepath.Join("no", "such", "path"))
	got := coerceLocalPathRef(ref)
	if got.Provider != ref.Provider || got.Path != ref.Path {
		t.Fatalf("expected unchanged ref, got %+v", got)
	}
}
