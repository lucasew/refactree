package main

import (
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestNormalizeRefForIngestDir_PathInsideDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "doc.go")

	ref := ingest.ParseReference(file + "::newDocCmd")
	norm := normalizeRefForIngestDir(dir, ref)

	if norm.Provider != "path" || norm.Path != "./doc.go" || norm.Symbol != "newDocCmd" {
		t.Fatalf("unexpected normalized ref: %+v", norm)
	}
}

func TestNormalizeRefForIngestDir_PathOutsideDir(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(t.TempDir(), "doc.go")

	ref := ingest.ParseReference(other + "::newDocCmd")
	norm := normalizeRefForIngestDir(dir, ref)

	if norm.Path != ref.Path {
		t.Fatalf("expected unchanged path for outside ref, got %q want %q", norm.Path, ref.Path)
	}
}
