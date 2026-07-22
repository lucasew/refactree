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
	if len(res.Atoms) < 2 {
		t.Fatalf("expected entities, got %+v", res.Atoms)
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

func TestProjectResult_UsesSpine(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	got, err := ingest.ProjectResult(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Atoms) == 0 {
		t.Fatal("expected entities from ProjectResult")
	}
	_ = os.ErrNotExist
}

func TestSeedResult_BFSNeighbors(t *testing.T) {
	dir := t.TempDir()
	// Two co-located Go files: seed one, BFS should pull the sibling.
	mustWrite(t, filepath.Join(dir, "a.go"), "package p\n\nfunc A() {}\n")
	mustWrite(t, filepath.Join(dir, "b.go"), "package p\n\nfunc B() {}\n")
	res, err := ingest.SeedResult(dir, filepath.Join(dir, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	refs := map[string]bool{}
	for _, e := range res.Atoms {
		refs[e.Reference] = true
	}
	if !refs["path:./a.go::A"] {
		t.Fatalf("missing A: %+v", res.Atoms)
	}
	if !refs["path:./b.go::B"] {
		t.Fatalf("seed BFS should include sibling B: %+v", res.Atoms)
	}
}

func TestDirResult_NonRecursive(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "root.go"), "package p\n\nfunc Root() {}\n")
	mustWrite(t, filepath.Join(dir, "sub", "nested.go"), "package p\n\nfunc Nested() {}\n")
	res, err := ingest.DirResult(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Atoms {
		if e.Reference == "path:./sub/nested.go::Nested" {
			t.Fatalf("non-recursive should omit nested: %+v", res.Atoms)
		}
	}
	found := false
	for _, e := range res.Atoms {
		if e.Reference == "path:./root.go::Root" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected root entity: %+v", res.Atoms)
	}
}
