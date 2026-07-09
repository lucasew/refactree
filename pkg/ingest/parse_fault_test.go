package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFile_TreeSitterFaultIsError(t *testing.T) {
	// Known pure-Go grammar fault on this file (slice/variadic near out[1:]...).
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// When tests run from pkg/ingest, ../.. is module root.
	wd, _ := os.Getwd()
	if strings.HasSuffix(filepath.ToSlash(wd), "/pkg/ingest") {
		root, _ = filepath.Abs(filepath.Join(wd, "../.."))
	}
	abs := filepath.Join(root, "internal/fuzzy/catalog.go")
	if _, err := os.Stat(abs); err != nil {
		t.Skip(err)
	}
	// Should not SIGSEGV the process.
	fe, err := parseFile(root, abs)
	if err == nil && fe != nil {
		// Grammar may be fixed upstream; success is fine.
		t.Log("parse succeeded; no fault on this grammar version")
		return
	}
	if err == nil {
		t.Fatal("expected extract or fault error, got nil,nil")
	}
	if !strings.Contains(err.Error(), "tree-sitter fault") {
		t.Fatalf("want tree-sitter fault error, got %v", err)
	}
}
