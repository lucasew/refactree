package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeProvider_ResolveFromNodeModules(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "react"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "react", "index.js"), []byte("export function createElement() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "react", "package.json"), []byte(`{"main":"index.js"}`), 0644); err != nil {
		t.Fatal(err)
	}

	provider, ok := referenceProviderForName("node")
	if !ok {
		t.Fatal("expected node provider to be registered")
	}
	ref, ok := provider.Resolve("react", ImportResolveContext{
		RootDir:      root,
		ImporterPath: "src/main.js",
	})
	if !ok {
		t.Fatal("expected provider to resolve")
	}
	if ref != "path:./node_modules/react/index.js" {
		t.Fatalf("unexpected reference: %q", ref)
	}
}

func TestNodeProvider_ResolveFromNODE_PATH(t *testing.T) {
	root := t.TempDir()
	nodePath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(nodePath, "react"), 0755); err != nil {
		t.Fatal(err)
	}
	moduleFile := filepath.Join(nodePath, "react", "index.js")
	if err := os.WriteFile(moduleFile, []byte("export function createElement() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	old := os.Getenv("NODE_PATH")
	t.Cleanup(func() { _ = os.Setenv("NODE_PATH", old) })
	if err := os.Setenv("NODE_PATH", nodePath); err != nil {
		t.Fatal(err)
	}

	provider, ok := referenceProviderForName("node")
	if !ok {
		t.Fatal("expected node provider to be registered")
	}
	ref, ok := provider.Resolve("react", ImportResolveContext{
		RootDir:      root,
		ImporterPath: "src/main.js",
	})
	if !ok {
		t.Fatal("expected provider to resolve")
	}

	expectedSuffix := filepath.ToSlash(moduleFile)
	if !strings.HasPrefix(ref, "path:") || !strings.HasSuffix(ref, expectedSuffix) {
		t.Fatalf("expected absolute path reference ending with %q, got %q", expectedSuffix, ref)
	}
}

func TestNodeProvider_FallbackSymbolic(t *testing.T) {
	root := t.TempDir()
	provider, ok := referenceProviderForName("node")
	if !ok {
		t.Fatal("expected node provider to be registered")
	}
	ref, ok := provider.Resolve("totally-missing-pkg", ImportResolveContext{
		RootDir:      root,
		ImporterPath: "main.js",
	})
	if !ok {
		t.Fatal("expected provider result")
	}
	if ref != "node:totally-missing-pkg" {
		t.Fatalf("unexpected fallback ref: %q", ref)
	}
}
