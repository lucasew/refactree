package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestCoerceLocalPathReference_Directory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0755); err != nil {
		t.Fatal(err)
	}

	ref := ingest.ParseReference("cmd")
	got := ingest.CoerceLocalPathReference(dir, ref)
	if got.Provider != "path" || got.Path != "./cmd" {
		t.Fatalf("unexpected coerced ref: %+v", got)
	}
}

func TestCoerceLocalPathReference_File(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ref := ingest.ParseReference("x.go")
	got := ingest.CoerceLocalPathReference(dir, ref)
	if got.Provider != "path" || got.Path != "./x.go" {
		t.Fatalf("unexpected coerced ref: %+v", got)
	}
}

func TestCoerceLocalPathReference_MissingPath(t *testing.T) {
	dir := t.TempDir()
	ref := ingest.ParseReference(filepath.Join("no", "such", "path"))
	got := ingest.CoerceLocalPathReference(dir, ref)
	if got.Provider != ref.Provider || got.Path != ref.Path {
		t.Fatalf("expected unchanged ref, got %+v", got)
	}
}

func TestResolveInputReferenceScope(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0755); err != nil {
		t.Fatal(err)
	}

	scope := ingest.ResolveInputReferenceScope(dir, "cmd")
	if scope.Dir != filepath.Join(dir, "cmd") {
		t.Fatalf("unexpected scope dir: %q", scope.Dir)
	}
	if scope.Reference.Provider != "path" || scope.Reference.Path != "./" {
		t.Fatalf("unexpected scope ref: %+v", scope.Reference)
	}
}
