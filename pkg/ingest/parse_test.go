package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSourceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pf, err := ParseSourceFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer pf.Close()
	if string(pf.Source) != "package p\n" {
		t.Fatalf("source: %q", pf.Source)
	}
	if pf.Root == nil {
		t.Fatal("nil root")
	}
	pf.Close() // double-close must be safe
}

func TestParseSourceFileUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.unknownlang")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSourceFile(path, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSource(t *testing.T) {
	content := []byte("package p\n")
	pf, err := ParseSource(content, "x.go", "")
	if err != nil {
		t.Fatal(err)
	}
	defer pf.Close()
	if pf.Root == nil || string(pf.Source) != string(content) {
		t.Fatal("bad parse")
	}
}
