package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile_GoProviderScope(t *testing.T) {
	l, err := NewLoader(".")
	if err != nil {
		t.Fatal(err)
	}

	v := l.LoadFile("go:fmt")
	if v.Error != "" {
		t.Fatalf("go:fmt error: %s", v.Error)
	}
	if v.Provider != "go" {
		t.Fatalf("provider=%q", v.Provider)
	}
	if len(v.Siblings) == 0 {
		t.Fatal("expected siblings in fmt package")
	}

	// Symbol deep-link should open the file that defines Println.
	v2 := l.LoadFile("go:fmt::Println")
	if v2.Error != "" {
		t.Fatalf("go:fmt::Println error: %s", v2.Error)
	}
	if len(v2.Segments) == 0 {
		t.Fatal("expected source segments for Println file")
	}
	if v2.FocusID == "" {
		t.Fatal("expected focus id for symbol")
	}

	var sawDef bool
	for _, seg := range v2.Segments {
		if seg.IsDef && strings.Contains(seg.Reference, "go:fmt") && seg.Text == "Println" {
			sawDef = true
			break
		}
	}
	if !sawDef {
		// Println might be split across segments; accept any go:fmt def with Println text nearby by scanning all defs
		for _, seg := range v2.Segments {
			if seg.IsDef && seg.Reference == "go:fmt::Println" {
				sawDef = true
				break
			}
		}
	}
	if !sawDef {
		t.Fatal("expected definition segment remapped to go:fmt::Println")
	}
}

func TestLoadFile_PathStillWorks(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := NewLoader(dir)
	if err != nil {
		t.Fatal(err)
	}
	v := l.LoadFile("path:./main.go")
	if v.Error != "" {
		t.Fatal(v.Error)
	}
	if len(v.Segments) == 0 {
		t.Fatal("expected segments")
	}
}

func TestEncodeProviderFileURL(t *testing.T) {
	u := EncodeProviderFileURL(FileReferenceForView("go:fmt"), "print.go")
	if !strings.Contains(u, "file=print.go") {
		t.Fatalf("missing file query: %s", u)
	}
	if !strings.Contains(u, CodePathPrefix) {
		t.Fatalf("missing code prefix: %s", u)
	}
}
