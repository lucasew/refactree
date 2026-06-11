package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveModuleTarget_FromPYTHONPATH(t *testing.T) {
	tmp := t.TempDir()

	moduleName := "fixturemod"
	moduleFile := filepath.Join(tmp, moduleName+".py")
	if err := os.WriteFile(moduleFile, []byte("def hello():\n    return 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	packageName := "fixturepkg"
	packageDir := filepath.Join(tmp, packageName)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "__init__.py"), []byte("def hi():\n    return 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PYTHONPATH")
	joined := tmp
	if oldPath != "" {
		joined = tmp + string(os.PathListSeparator) + oldPath
	}
	if err := os.Setenv("PYTHONPATH", joined); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PYTHONPATH", oldPath)
	})

	modTarget, err := ResolveModuleTarget(moduleName)
	if err != nil {
		if strings.Contains(err.Error(), "python executable not found") {
			t.Skip(err.Error())
		}
		t.Fatalf("resolve module target failed: %v", err)
	}
	if filepath.Clean(modTarget.Dir) != filepath.Clean(tmp) {
		t.Fatalf("unexpected module dir: got %q want %q", modTarget.Dir, tmp)
	}
	if modTarget.File != moduleName+".py" {
		t.Fatalf("unexpected module file: got %q want %q", modTarget.File, moduleName+".py")
	}

	pkgTarget, err := ResolveModuleTarget(packageName)
	if err != nil {
		t.Fatalf("resolve package target failed: %v", err)
	}
	if filepath.Clean(pkgTarget.Dir) != filepath.Clean(packageDir) {
		t.Fatalf("unexpected package dir: got %q want %q", pkgTarget.Dir, packageDir)
	}
	if pkgTarget.File != "__init__.py" {
		t.Fatalf("unexpected package file: got %q want %q", pkgTarget.File, "__init__.py")
	}
}

func TestResolveSymbolTarget(t *testing.T) {
	target, ok, err := ResolveSymbolTarget("os", "path")
	if err != nil {
		if strings.Contains(err.Error(), "python executable not found") {
			t.Skip(err.Error())
		}
		t.Fatalf("resolve symbol target failed: %v", err)
	}
	if !ok {
		t.Fatal("expected symbol target to resolve")
	}
	if target.Symbol != "path" {
		t.Fatalf("unexpected symbol: %q", target.Symbol)
	}
	if target.Dir == "" {
		t.Fatal("expected non-empty target dir")
	}
}
