package python_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/python"
)

func TestPythonResidualSameFileImport(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "helpers.py"), "def helper():\n    return 1\n\ndef other():\n    return helper()\n")
	mustWrite(t, filepath.Join(dir, "utils.py"), "def existing():\n    pass\n")

	plan, err := ingest.Rename(dir, "path:./helpers.py::helper", "path:./utils.py::helper")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ApplyPlan(dir, plan); err != nil {
		t.Fatal(err)
	}
	helpers := mustRead(t, filepath.Join(dir, "helpers.py"))
	utils := mustRead(t, filepath.Join(dir, "utils.py"))
	if !strings.Contains(helpers, "from utils import helper") {
		t.Fatalf("helpers missing residual import:\n%s", helpers)
	}
	if !strings.Contains(helpers, "return helper()") {
		t.Fatalf("helpers lost residual use:\n%s", helpers)
	}
	if !strings.Contains(utils, "def helper():") {
		t.Fatalf("utils missing moved helper:\n%s", utils)
	}
}

// Package-root residual import (ingest_roots = package dir, e.g. boltons):
// after moving a name between sibling modules, the source file must import with
// a same-package relative form so "python -m" / installed package loads work.
func TestPythonResidualPackageRootImport(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "__init__.py"), "")
	mustWrite(t, filepath.Join(dir, "helpers.py"), "def helper():\n    return 1\n\ndef other():\n    return helper()\n")
	mustWrite(t, filepath.Join(dir, "utils.py"), "def existing():\n    pass\n")

	plan, err := ingest.Rename(dir, "path:./helpers.py::helper", "path:./utils.py::helper")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ApplyPlan(dir, plan); err != nil {
		t.Fatal(err)
	}
	helpers := mustRead(t, filepath.Join(dir, "helpers.py"))
	utils := mustRead(t, filepath.Join(dir, "utils.py"))
	if !strings.Contains(helpers, "from .utils import helper") {
		t.Fatalf("helpers missing package-relative residual import:\n%s", helpers)
	}
	if strings.Contains(helpers, "from utils import helper") {
		t.Fatalf("helpers used bare top-level residual import:\n%s", helpers)
	}
	if !strings.Contains(helpers, "return helper()") {
		t.Fatalf("helpers lost residual use:\n%s", helpers)
	}
	if !strings.Contains(utils, "def helper():") {
		t.Fatalf("utils missing moved helper:\n%s", utils)
	}
}

func TestPythonLocalDepImportNewModule(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "models.py"), "class Config:\n    pass\n\ndef create_config(name):\n    return Config(name)\n")

	plan, err := ingest.Rename(dir, "path:./models.py::create_config", "path:./factory.py::create_config")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ApplyPlan(dir, plan); err != nil {
		t.Fatal(err)
	}
	factory := mustRead(t, filepath.Join(dir, "factory.py"))
	models := mustRead(t, filepath.Join(dir, "models.py"))
	if !strings.Contains(factory, "from models import Config") {
		t.Fatalf("factory missing local-dep import:\n%s", factory)
	}
	if !strings.Contains(factory, "def create_config") {
		t.Fatalf("factory missing moved func:\n%s", factory)
	}
	if strings.Contains(models, "def create_config") {
		t.Fatalf("models still has create_config:\n%s", models)
	}
}

func TestPythonLocalDepImportPackageRoot(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "__init__.py"), "")
	mustWrite(t, filepath.Join(dir, "models.py"), "class Config:\n    pass\n\ndef create_config(name):\n    return Config(name)\n")

	plan, err := ingest.Rename(dir, "path:./models.py::create_config", "path:./factory.py::create_config")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ApplyPlan(dir, plan); err != nil {
		t.Fatal(err)
	}
	factory := mustRead(t, filepath.Join(dir, "factory.py"))
	if !strings.Contains(factory, "from .models import Config") {
		t.Fatalf("factory missing package-relative local-dep import:\n%s", factory)
	}
	if strings.Contains(factory, "from models import Config") {
		t.Fatalf("factory used bare top-level local-dep import:\n%s", factory)
	}
}

func TestPythonClassCrossFileResidualImport(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "models.py"), "class Config:\n    def __init__(self, name):\n        self.name = name\n\ndef create_config(name):\n    return Config(name)\n")
	mustWrite(t, filepath.Join(dir, "types.py"), "pass\n")
	mustWrite(t, filepath.Join(dir, "app.py"), "from models import Config\n\nc = Config(\"test\")\n")

	plan, err := ingest.Rename(dir, "path:./models.py::Config", "path:./types.py::Config")
	if err != nil {
		t.Fatal(err)
	}
	if err := ingest.ApplyPlan(dir, plan); err != nil {
		t.Fatal(err)
	}
	models := mustRead(t, filepath.Join(dir, "models.py"))
	app := mustRead(t, filepath.Join(dir, "app.py"))
	if !strings.Contains(models, "from types import Config") {
		t.Fatalf("models missing residual import:\n%s", models)
	}
	if !strings.Contains(app, "from types import Config") {
		t.Fatalf("app import not rewritten:\n%s", app)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
