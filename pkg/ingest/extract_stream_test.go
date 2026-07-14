package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestWalkExtracts_DirStreamsThenMaterialize(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package p\n\nfunc A() {}\n")
	mustWrite(t, filepath.Join(dir, "sub", "b.go"), "package p\n\nfunc B() {}\n")

	var n int
	err := ingest.WalkExtracts(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      dir,
		Recursive: true,
	}, func(fe *ingest.FileExtract) bool {
		n++
		if fe == nil || fe.Language != "go" {
			t.Fatalf("bad extract: %+v", fe)
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("expected >=2 extracts, got %d", n)
	}

	res, err := ingest.MaterializeSource(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      dir,
		Recursive: true,
	}, ingest.MaterializeOptions{ExpandImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entities) < 2 {
		t.Fatalf("expected entities, got %+v", res.Entities)
	}
}

func TestWalkExtracts_HopSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "only.go")
	mustWrite(t, path, "package p\n\nfunc Only() {}\n")
	mustWrite(t, filepath.Join(dir, "other.go"), "package p\n\nfunc Other() {}\n")

	var paths []string
	err := ingest.WalkExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractHop,
		Root:  dir,
		Paths: []string{path},
	}, func(fe *ingest.FileExtract) bool {
		paths = append(paths, fe.Path)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "only.go" {
		t.Fatalf("hop should parse one file, got %v", paths)
	}
}

func TestWalkExtracts_StopEarly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package p\n\nfunc A() {}\n")
	mustWrite(t, filepath.Join(dir, "b.go"), "package p\n\nfunc B() {}\n")

	var n int
	err := ingest.WalkExtracts(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      dir,
		Recursive: true,
	}, func(*ingest.FileExtract) bool {
		n++
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("stop early: got %d yields", n)
	}
}

func TestIngest_UsesSpine(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	got, err := ingest.Ingest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entities) == 0 {
		t.Fatal("expected entities from Ingest wrapper")
	}
	_ = os.ErrNotExist
}
