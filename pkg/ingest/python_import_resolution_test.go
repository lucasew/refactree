package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestIngest_PythonRelativeImportResolvesToLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}

	app := "from .localmod import helper\n\n\ndef main():\n    return helper()\n"
	local := "def helper():\n    return 1\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg", "app.py"), []byte(app), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "localmod.py"), []byte(local), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ingest.Ingest(dir)
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	hasAlias := false
	hasRelation := false
	for _, a := range result.Aliases {
		if a.Reference == "path:./pkg/app.py" && a.Target == "path:./pkg/localmod.py::helper" {
			hasAlias = true
		}
	}
	for _, rel := range result.Relations {
		if rel.Reference == "path:./pkg/app.py::main" && rel.Target == "path:./pkg/localmod.py::helper" {
			hasRelation = true
		}
	}

	if !hasAlias {
		t.Fatalf("expected relative import alias target path:./pkg/localmod.py::helper, got aliases: %+v", result.Aliases)
	}
	if !hasRelation {
		t.Fatalf("expected helper callsite target path:./pkg/localmod.py::helper, got relations: %+v", result.Relations)
	}
}

func TestIngest_PythonAbsoluteDottedImportResolvesToLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}

	app := "from pkg.sub import helper\n\n\ndef main():\n    return helper()\n"
	sub := "def helper():\n    return 1\n"
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(app), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "sub.py"), []byte(sub), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ingest.Ingest(dir)
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	hasAlias := false
	hasRelation := false
	for _, a := range result.Aliases {
		if a.Reference == "path:./app.py" && a.Target == "path:./pkg/sub.py::helper" {
			hasAlias = true
		}
	}
	for _, rel := range result.Relations {
		if rel.Reference == "path:./app.py::main" && rel.Target == "path:./pkg/sub.py::helper" {
			hasRelation = true
		}
	}

	if !hasAlias {
		t.Fatalf("expected absolute dotted import alias target path:./pkg/sub.py::helper, got aliases: %+v", result.Aliases)
	}
	if !hasRelation {
		t.Fatalf("expected helper callsite target path:./pkg/sub.py::helper, got relations: %+v", result.Relations)
	}
}
