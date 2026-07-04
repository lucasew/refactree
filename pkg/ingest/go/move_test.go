package ingestgo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestCrossPackageMoveRejectsInitAndTests(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkga"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkgb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkga", "a.go"), []byte("package pkga\n\nfunc init() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkgb", "b.go"), []byte("package pkgb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ingest.Rename(dir, "path:./pkga/a.go::init", "path:./pkgb/b.go::init")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported init move, got %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "pkga", "a_test.go"), []byte("package pkga\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ingest.Rename(dir, "path:./pkga/a_test.go::TestX", "path:./pkgb/b.go::TestX")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported test move, got %v", err)
	}
}

func TestPathSegmentImportRewrite(t *testing.T) {
	content := []byte(`package p
import (
	"example/pkg/api"
	"example/pkg/palette/api"
)
const s = "case lucas"
`)
	edits := ingest.FindAllPathSegmentOccurrencesInStrings("f.go", content, "pkg/api", "pkg/api_fuzz")
	if len(edits) != 1 || edits[0].NewText != "pkg/api_fuzz" {
		t.Fatalf("full path edits: %+v", edits)
	}
	edits = ingest.FindAllPathSegmentOccurrencesInStringsWithParent("f.go", content, "api", "api_fuzz", "pkg")
	if len(edits) != 1 {
		t.Fatalf("parent-scoped leaf edits: %+v", edits)
	}
	edits = ingest.FindAllPathSegmentOccurrencesInStrings("f.go", content, "cas", "cas_fuzz")
	if len(edits) != 0 {
		t.Fatalf("expected no cas substring hits, got %+v", edits)
	}
}

func TestCrossPackageMoveRejectsUnexported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkga"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkgb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkga", "a.go"), []byte("package pkga\n\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkgb", "b.go"), []byte("package pkgb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ingest.Rename(dir, "path:./pkga/a.go::helper", "path:./pkgb/b.go::helper")
	if err == nil || !strings.Contains(err.Error(), "unexported") {
		t.Fatalf("expected unsupported unexported move, got %v", err)
	}
}
