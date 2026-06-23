package goref

import (
	"os"
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
	target, ok, err := ResolveSymbolTarget("fmt", "Printf", "")
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

func TestResolvePackageDir_Stdlib(t *testing.T) {
	dir, err := ResolvePackageDir("fmt", "")
	if err != nil {
		t.Fatalf("resolve package dir failed: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(dir), filepath.ToSlash(filepath.Join("src", "fmt"))) {
		t.Fatalf("unexpected package dir: %q", dir)
	}
}

func TestResolvePackageDir_LocalModuleWithWorkDir(t *testing.T) {
	dir := t.TempDir()
	modName := "example.com/localmod"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+modName+"\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolvePackageDir(modName, dir)
	if err != nil {
		t.Fatalf("expected local module via workDir: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(dir) {
		t.Fatalf("got %q want %q", got, dir)
	}
}

func TestResolveModuleCachePackageDir_Subpackage(t *testing.T) {
	modCache := t.TempDir()
	pkg := filepath.Join(modCache, "github.com", "example", "lib@v1.2.3", "sub", "pkg")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Getenv("GOMODCACHE")
	t.Cleanup(func() {
		_ = os.Setenv("GOMODCACHE", old)
		_ = os.Chdir(cwd)
	})
	if err := os.Setenv("GOMODCACHE", modCache); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveModuleCachePackageDir("github.com/example/lib/sub/pkg")
	if !ok {
		t.Fatal("expected module-cache resolution")
	}
	if filepath.Clean(got) != filepath.Clean(pkg) {
		t.Fatalf("unexpected package dir: %q", got)
	}
}
