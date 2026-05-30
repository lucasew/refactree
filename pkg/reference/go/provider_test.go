package goref

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveImport_LocalPackageDir(t *testing.T) {
	ref := ResolveImport("helperpkg", map[string]bool{"helperpkg": true})
	if ref != "path:./helperpkg" {
		t.Fatalf("unexpected reference: %q", ref)
	}
}

func TestResolveImport_StdlibPathKeepsSlashes(t *testing.T) {
	ref := ResolveImport("net/http", map[string]bool{})
	if ref != "go:net/http" {
		t.Fatalf("unexpected reference: %q", ref)
	}
}

func TestResolveSymbolTarget_StdlibPackage(t *testing.T) {
	target, ok, err := ResolveSymbolTarget("fmt", "Printf")
	if err != nil {
		t.Fatalf("resolve symbol target failed: %v", err)
	}
	if !ok {
		t.Fatal("expected symbol target to resolve")
	}
	if target.Symbol != "Printf" {
		t.Fatalf("unexpected symbol: %q", target.Symbol)
	}

	suffix := filepath.ToSlash(filepath.Join("src", "fmt"))
	if !strings.HasSuffix(filepath.ToSlash(target.Dir), suffix) {
		t.Fatalf("unexpected target dir: %q", target.Dir)
	}
}
