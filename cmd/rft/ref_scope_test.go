package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestNormalizeRefForCommandScope_PathDirectory(t *testing.T) {
	tmp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("cmd", 0755); err != nil {
		t.Fatal(err)
	}

	ref := coerceLocalPathRef(ingest.ParseReference("cmd"))
	dir, norm := normalizeRefForCommandScope(ref)

	if dir != "cmd" {
		t.Fatalf("unexpected dir: %q", dir)
	}
	if norm.Provider != "path" || norm.Path != "./" {
		t.Fatalf("unexpected normalized ref: %+v", norm)
	}
}

func TestResolvePathRefForBrowse_PathDirectory(t *testing.T) {
	tmp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("cmd", 0755); err != nil {
		t.Fatal(err)
	}

	ref := coerceLocalPathRef(ingest.ParseReference("cmd"))
	dir, norm := normalizeRefForCommandScope(ref)
	got := resolvePathRefForBrowse(dir, norm)

	wantAbs, err := filepath.Abs("cmd")
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "path" || got.Path != wantAbs {
		t.Fatalf("unexpected browse ref: %+v", got)
	}
}

func TestResolvePathRefForBrowse_PathFile(t *testing.T) {
	tmp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("cmd", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("cmd", "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ref := coerceLocalPathRef(ingest.ParseReference("cmd/main.go"))
	dir, norm := normalizeRefForCommandScope(ref)
	got := resolvePathRefForBrowse(dir, norm)

	wantAbs, err := filepath.Abs(filepath.Join("cmd", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "path" || got.Path != wantAbs || got.Symbol != "" {
		t.Fatalf("unexpected browse ref: %+v", got)
	}
}
