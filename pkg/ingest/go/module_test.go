package ingestgo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
)

func TestReferenceProvider_ListScopeChildren_FiltersNonPackages(t *testing.T) {
	modCache := t.TempDir()
	libDir := filepath.Join(modCache, "github.com", "example", "lib@v1.2.3")
	if err := os.MkdirAll(filepath.Join(libDir, "scan"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(libDir, "doc"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "print.go"), []byte("package lib\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "scan", "scan.go"), []byte("package scan\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "doc", "README.txt"), []byte("not go\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldModCache := os.Getenv("GOMODCACHE")
	t.Cleanup(func() { _ = os.Setenv("GOMODCACHE", oldModCache) })
	if err := os.Setenv("GOMODCACHE", modCache); err != nil {
		t.Fatal(err)
	}

	children, ok, err := (referenceProvider{}).ListScopeChildren(ingest.ParseReference("go:github.com/example/lib"), "", false)
	if err != nil {
		t.Fatalf("list scope children failed: %v", err)
	}
	if !ok {
		t.Fatal("expected go provider child listing")
	}

	if !hasScopeChild(children, refpkg.ScopeChild{Ref: ingest.ParseReference("go:github.com/example/lib/scan"), Kind: refpkg.ScopeChildDir}) {
		t.Fatalf("expected scan child package, got %+v", children)
	}
	if hasScopeChild(children, refpkg.ScopeChild{Ref: ingest.ParseReference("go:github.com/example/lib/doc"), Kind: refpkg.ScopeChildDir}) {
		t.Fatalf("did not expect non-go directory child, got %+v", children)
	}
}

func hasScopeChild(children []refpkg.ScopeChild, want refpkg.ScopeChild) bool {
	for _, child := range children {
		if child.Kind == want.Kind && child.Ref == want.Ref {
			return true
		}
	}
	return false
}
