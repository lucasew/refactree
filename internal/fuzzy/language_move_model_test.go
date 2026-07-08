package fuzzy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoMoveModelGrainsAndSameModule(t *testing.T) {
	t.Parallel()
	m, err := moveModelForLanguage("go")
	if err != nil {
		t.Fatal(err)
	}
	grains := m.Grains()
	if len(grains) != 2 || grains[0] != GrainDeclaration || grains[1] != GrainPackage {
		t.Fatalf("go grains: %v", grains)
	}
	if !m.SameModule("./pkg/a.go", "./pkg/b.go") {
		t.Fatal("same package should be same module")
	}
	if m.SameModule("./pkg/a.go", "./other/a.go") {
		t.Fatal("different packages should differ")
	}
}

func TestJavascriptMoveModelFileIsModule(t *testing.T) {
	t.Parallel()
	m, err := moveModelForLanguage("javascript")
	if err != nil {
		t.Fatal(err)
	}
	if m.SameModule("./a.js", "./b.js") {
		t.Fatal("different files are different modules in javascript")
	}
	if !m.SameModule("./a.js", "./a.js") {
		t.Fatal("same file should be same module")
	}
	for _, g := range m.Grains() {
		if g == GrainPackage {
			t.Fatal("javascript should not list package grain by default")
		}
	}
}

func TestListDeclarationAndPackageNodes(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "pkg", "a.go"), []byte("package pkg\nfunc Helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "pkg", "b.go"), []byte("package pkg\nfunc Other() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, fails, err := RunIngestOnRoot(work, InvariantOptions{})
	if err != nil || len(fails) > 0 {
		t.Fatalf("ingest: %v %v", err, fails)
	}
	m, _ := moveModelForLanguage("go")
	decls := m.ListNodes(result, GrainDeclaration, "go")
	if len(decls) < 2 {
		t.Fatalf("expected declaration nodes, got %d", len(decls))
	}
	pkgs := m.ListNodes(result, GrainPackage, "go")
	if len(pkgs) != 1 || pkgs[0].Path != "./pkg" {
		t.Fatalf("package nodes: %+v", pkgs)
	}
	same := filesInModule(result, m, "go", m.ModuleKey("./pkg/a.go"))
	if len(same) != 2 {
		t.Fatalf("files in module: %v", same)
	}
}
