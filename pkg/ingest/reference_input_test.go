package ingest_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolveMoveArgs_CrossDirKeepsProjectRelativePaths(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "pkg", "pattern", "match.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package pattern\n\ntype Span struct{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg", "ingest"), 0755); err != nil {
		t.Fatal(err)
	}

	root, src, dst := ingest.ResolveMoveArgs(dir,
		"path:./pkg/pattern/match.go::Span",
		"path:./pkg/ingest/span.go::Span",
	)
	if root != dir {
		t.Fatalf("root=%q want project root %q", root, dir)
	}
	srcRef := ingest.ParseReference(src)
	dstRef := ingest.ParseReference(dst)
	if srcRef.Path != "./pkg/pattern/match.go" || srcRef.Name != "Span" {
		t.Fatalf("source ref=%+v want path ./pkg/pattern/match.go::Span", srcRef)
	}
	if dstRef.Path != "./pkg/ingest/span.go" || dstRef.Name != "Span" {
		t.Fatalf("destination ref=%+v want path ./pkg/ingest/span.go::Span", dstRef)
	}
	// Regression: old CLI used source parent as root and rebased dest under it.
	if strings.Contains(dstRef.Path, "pattern/pkg") {
		t.Fatalf("destination rebased under source package: %q", dstRef.Path)
	}
}

func TestResolveMoveArgs_BarePackageDirs(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"cmd/codegen", "cmd/gen"} {
		if err := os.MkdirAll(filepath.Join(dir, p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Minimal Go file so coerce/canonicalize have something under codegen.
	if err := os.WriteFile(filepath.Join(dir, "cmd/codegen/main.go"), []byte("package codegen\n"), 0644); err != nil {
		t.Fatal(err)
	}

	root, src, dst := ingest.ResolveMoveArgs(dir, "cmd/codegen", "cmd/gen")
	if root != dir {
		t.Fatalf("root=%q want %q", root, dir)
	}
	srcRef := ingest.ParseReference(src)
	dstRef := ingest.ParseReference(dst)
	if srcRef.Provider != "path" || !strings.Contains(srcRef.Path, "codegen") {
		t.Fatalf("source ref=%+v", srcRef)
	}
	if dstRef.Provider != "path" || !strings.Contains(dstRef.Path, "gen") {
		t.Fatalf("destination ref=%+v", dstRef)
	}
	if srcRef.Name != "" || dstRef.Name != "" {
		t.Fatalf("package move should have empty symbols: src=%+v dst=%+v", srcRef, dstRef)
	}
}
