package ingest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestDocFor_PythonFunction(t *testing.T) {
	dir := t.TempDir()
	content := "def helper(x):\n    \"\"\"does help\"\"\"\n    return x\n"
	if err := os.WriteFile(filepath.Join(dir, "helper.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ingest.DocFor(dir, "Path:./helper.py::helper")
	if err != nil {
		t.Fatalf("doc lookup failed: %v", err)
	}

	if doc.Name != "helper" {
		t.Fatalf("unexpected Name: %q", doc.Name)
	}
	if !strings.Contains(doc.Signature, "def helper(x)") {
		t.Fatalf("unexpected signature: %q", doc.Signature)
	}
	if doc.DocString != "does help" {
		t.Fatalf("unexpected docstring: %q", doc.DocString)
	}
}

func TestDocFor_PythonClass(t *testing.T) {
	dir := t.TempDir()
	content := "class Greeter:\n    \"\"\"Greeter docs\"\"\"\n\n    def hi(self):\n        return 'hi'\n"
	if err := os.WriteFile(filepath.Join(dir, "helper.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ingest.DocFor(dir, "Path:./helper.py::Greeter")
	if err != nil {
		t.Fatalf("doc lookup failed: %v", err)
	}

	if doc.Name != "Greeter" {
		t.Fatalf("unexpected Name: %q", doc.Name)
	}
	if !strings.Contains(doc.Signature, "class Greeter") {
		t.Fatalf("unexpected signature: %q", doc.Signature)
	}
	if doc.DocString != "Greeter docs" {
		t.Fatalf("unexpected docstring: %q", doc.DocString)
	}
}

func TestDocFor_DirectoryReference_PythonInit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	content := "def helper():\n    \"\"\"from init\"\"\"\n    pass\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg", "__init__.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ingest.DocFor(dir, "Path:./Package::helper")
	if err != nil {
		t.Fatalf("doc lookup failed: %v", err)
	}
	if doc.DocString != "from init" {
		t.Fatalf("unexpected docstring: %q", doc.DocString)
	}
}

func TestDocFor_DirectoryReference_JSIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	content := "// helper docs\nfunction helper() {\n}\nexport { helper };\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg", "index.js"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ingest.DocFor(dir, "Path:./Package::helper")
	if err != nil {
		t.Fatalf("doc lookup failed: %v", err)
	}
	if doc.Name != "helper" {
		t.Fatalf("unexpected Name: %q", doc.Name)
	}
	if !strings.Contains(doc.DocString, "helper docs") {
		t.Fatalf("unexpected docstring: %q", doc.DocString)
	}
}

func TestDocFor_DirectoryReference_GoFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "a.go"), []byte("package pkg\n\n// helper docs\nfunc helper() {\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ingest.DocFor(dir, "Path:./Package::helper")
	if err != nil {
		t.Fatalf("doc lookup failed: %v", err)
	}
	if doc.Name != "helper" {
		t.Fatalf("unexpected Name: %q", doc.Name)
	}
	if !strings.Contains(doc.DocString, "helper docs") {
		t.Fatalf("unexpected docstring: %q", doc.DocString)
	}
}

func TestDocFor_GoProviderStdlibFunction(t *testing.T) {
	doc, err := ingest.DocFor(".", "go:fmt::Printf")
	if err != nil {
		t.Fatalf("doc lookup failed: %v", err)
	}

	if doc.Name != "Printf" {
		t.Fatalf("unexpected Name: %q", doc.Name)
	}
	if !strings.Contains(doc.Signature, "func Printf(") {
		t.Fatalf("unexpected signature: %q", doc.Signature)
	}
	if !strings.Contains(doc.DocString, "Printf formats according") {
		t.Fatalf("unexpected docstring: %q", doc.DocString)
	}
}
