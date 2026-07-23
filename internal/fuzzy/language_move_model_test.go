package fuzzy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestGoMoveModelGrainsAndSameModule(t *testing.T) {
	t.Parallel()
	m, err := moveModelForLanguage("go")
	if err != nil {
		t.Fatal(err)
	}
	grains := m.Grains()
	if len(grains) != 2 || grains[0] != GrainAtom || grains[1] != GrainPackage {
		t.Fatalf("go grains: %v", grains)
	}
	if !m.SameModule("./pkg/a.go", "./pkg/b.go") {
		t.Fatal("same package should be same module")
	}
	if m.SameModule("./pkg/a.go", "./other/a.go") {
		t.Fatal("different packages should differ")
	}
}

func TestJVMMoveModelFromJavaLanguage(t *testing.T) {
	t.Parallel()
	m, err := moveModelForLanguage("java")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.(jvmMoveModel); !ok {
		t.Fatalf("java should use jvmMoveModel, got %T", m)
	}
	if !m.SameModule("./com/foo/A.java", "./com/foo/B.java") {
		t.Fatal("same package dir is same module on jvm")
	}
}

// Catalog seed gson grain1 source0 picked path:./src/main/java because
// module-info.java lived at the source root. That is not a Java package.
func TestJVMPackageNodesSkipSourceRoots(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	root := filepath.Join(work, "src", "main", "java")
	pkg := filepath.Join(root, "com", "example")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "module-info.java"), []byte("module com.example {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "Box.java"), []byte("package com.example;\npublic class Box {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, fails, err := RunIngestOnRoot(work, InvariantOptions{})
	if err != nil || len(fails) > 0 {
		t.Fatalf("ingest: %v %v", err, fails)
	}
	m, err := moveModelForFamily(ingest.FamilyJVM)
	if err != nil {
		t.Fatal(err)
	}
	pkgs := m.ListNodes(result, GrainPackage, ingest.FamilyJVM)
	for _, n := range pkgs {
		if isJVMSourceRootDir(n.Path) {
			t.Fatalf("source root listed as package grain: %s (all=%+v)", n.Path, pkgs)
		}
		if strings.HasSuffix(strings.TrimPrefix(n.Path, "./"), "src/main/java") {
			t.Fatalf("src/main/java must not be a package node: %+v", pkgs)
		}
	}
	found := false
	for _, n := range pkgs {
		if strings.TrimPrefix(n.Path, "./") == "src/main/java/com/example" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected com.example package node, got %+v", pkgs)
	}
}

func TestIsJVMSourceRootDir(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"src/main/java":                 true,
		"./src/main/java":               true,
		"gson/src/main/java":            true,
		"src/main/java/com/google/gson": false,
		"com/google/gson":               false,
		"src/jmh/java":                  true,
		"pkg":                           false,
	}
	for dir, want := range cases {
		if got := isJVMSourceRootDir(dir); got != want {
			t.Errorf("isJVMSourceRootDir(%q)=%v want %v", dir, got, want)
		}
	}
}

func TestECMAMoveModelFileIsModule(t *testing.T) {
	t.Parallel()
	m, err := moveModelForLanguage("javascript")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.(ecmaMoveModel); !ok {
		t.Fatalf("javascript should use ecmaMoveModel, got %T", m)
	}
	if m.SameModule("./a.js", "./b.js") {
		t.Fatal("different files are different modules in ecma")
	}
	if !m.SameModule("./a.js", "./a.js") {
		t.Fatal("same file should be same module")
	}
	for _, g := range m.Grains() {
		if g == GrainPackage {
			t.Fatal("ecma should not list package grain by default")
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
	decls := m.ListNodes(result, GrainAtom, "go")
	if len(decls) < 2 {
		t.Fatalf("expected atom nodes, got %d", len(decls))
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
