package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTargetMatchesPackageSymbol_ModulePathExact(t *testing.T) {
	mod := "example.com/mod"
	// Nested package matches full import path only.
	if !targetMatchesPackageSymbol(ParseReference("go:example.com/mod/pkg/db::Open"), "pkg/db", mod) {
		t.Fatal("expected go:example.com/mod/pkg/db to match pkg/db under module")
	}
	// Suffix collision: package dir "db" must not match .../pkg/db.
	if targetMatchesPackageSymbol(ParseReference("go:example.com/mod/pkg/db::Open"), "db", mod) {
		t.Fatal("pkg/db must not match short pkgDir db")
	}
	// Correct short package at module root segment.
	if !targetMatchesPackageSymbol(ParseReference("go:example.com/mod/db::Open"), "db", mod) {
		t.Fatal("expected go:example.com/mod/db to match pkgDir db")
	}
	// Empty pkgDir is module root only — not fmt/os single-segment stdlib.
	if !targetMatchesPackageSymbol(ParseReference("go:example.com/mod::Printf"), "", mod) {
		t.Fatal("root package should match module path exactly")
	}
	if targetMatchesPackageSymbol(ParseReference("go:fmt::Printf"), "", mod) {
		t.Fatal("empty pkgDir must not match go:fmt")
	}
	if targetMatchesPackageSymbol(ParseReference("go:os::Open"), "", mod) {
		t.Fatal("empty pkgDir must not match go:os")
	}
	// path: still matches by file directory.
	if !targetMatchesPackageSymbol(ParseReference("path:./pkg/db/db.go::Open"), "pkg/db", mod) {
		t.Fatal("path entity dir should match")
	}
}

func TestTargetMatchesPackageSymbol_NoModulePath(t *testing.T) {
	// Without module path: no empty-root match, no suffix match.
	if targetMatchesPackageSymbol(ParseReference("go:fmt::Printf"), "", "") {
		t.Fatal("empty pkgDir without module must not match single-segment")
	}
	if targetMatchesPackageSymbol(ParseReference("go:example.com/mod/pkg/db::Open"), "db", "") {
		t.Fatal("suffix match must not apply without module path")
	}
	if !targetMatchesPackageSymbol(ParseReference("go:pkg/db::Open"), "pkg/db", "") {
		t.Fatal("exact pkgDir equality should still match")
	}
}

func TestExpandRenameSourceSet_UsesModuleImportPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result := &Result{
		Relations: []Relation{
			{Target: "go:example/pkg/db::FromContext"},
			{Target: "go:example/other/db::FromContext"}, // wrong package, same leaf
			{Target: "go:fmt::FromContext"},              // single-segment false friend
		},
	}
	set := expandRenameSourceSet(dir, result, []string{"path:./pkg/db/db.go::FromContext"})
	if !set["go:example/pkg/db::FromContext"] {
		t.Fatalf("expected go:example/pkg/db::FromContext in set, got %v", set)
	}
	if set["go:example/other/db::FromContext"] {
		t.Fatal("must not expand other/db via trailing /db suffix")
	}
	if set["go:fmt::FromContext"] {
		t.Fatal("must not expand stdlib single-segment")
	}
}
