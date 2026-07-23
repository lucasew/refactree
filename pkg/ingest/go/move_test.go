package ingestgo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestMethodReceiverType_StripsGenericArgs(t *testing.T) {
	cases := []struct {
		symbol string
		want   string
	}{
		{"*Session.Close", "Session"},
		{"Session.Group", "Session"},
		{"*Set[T].Add", "Set"},
		{"Set[T].Len", "Set"},
		{"*Map[K,V].Get", "Map"},
		{"Add", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := methodReceiverType(tc.symbol); got != tc.want {
			t.Errorf("methodReceiverType(%q)=%q want %q", tc.symbol, got, tc.want)
		}
	}
}

func TestGoIdentUsed_SkipsCommentsAndStrings(t *testing.T) {
	if goIdentUsed(`// Helper used elsewhere`+"\n"+`func F() {}`, "Helper") {
		t.Fatal("line comment should not count as use")
	}
	if goIdentUsed(`/* Helper */`+"\n"+`func F() {}`, "Helper") {
		t.Fatal("block comment should not count as use")
	}
	if goIdentUsed(`func F() string { return "Helper" }`, "Helper") {
		t.Fatal("string literal should not count as use")
	}
	if goIdentUsed("func F() string { return `Helper` }", "Helper") {
		t.Fatal("raw string should not count as use")
	}
	if !goIdentUsed(`func F() { Helper() }`, "Helper") {
		t.Fatal("real call should count as use")
	}
	if goIdentUsed(`func Helpers() {}`, "Helper") {
		t.Fatal("identifier prefix should not match")
	}
}

func TestFindSelectorLeafEdits_SkipsCommentApostrophe(t *testing.T) {
	content := []byte("package wallpaper\n\nfunc Wrap() error {\n\t// Ignore errors if service doesn't exist\n\treturn d.SetStatic(ctx, path)\n}\n")
	edits := findSelectorLeafEdits("facade.go", content, "SetStatic", "Fuzz", nil)
	if len(edits) != 1 {
		t.Fatalf("expected 1 selector edit, got %+v", edits)
	}
}

func TestFindPathSegmentOccurrencesInStrings(t *testing.T) {
	content := []byte("package p\nimport (\n\t\"example/pkg/api\"\n\t\"example/pkg/palette/api\"\n)\nconst s = \"case lucas\"\n")
	edits := findPathSegmentOccurrencesInStrings("f.go", content, "pkg/api", "pkg/api_fuzz")
	if len(edits) != 1 || edits[0].NewText != "pkg/api_fuzz" {
		t.Fatalf("full path edits: %+v", edits)
	}
	edits = findPathSegmentOccurrencesInStringsWithParent("f.go", content, "api", "api_fuzz", "pkg")
	if len(edits) != 1 {
		t.Fatalf("parent-scoped leaf edits: %+v", edits)
	}
	edits = findPathSegmentOccurrencesInStrings("f.go", content, "cas", "cas_fuzz")
	if len(edits) != 0 {
		t.Fatalf("expected no cas substring hits, got %+v", edits)
	}
}

func TestExtractDeclVarConstFromGroup(t *testing.T) {
	dir := t.TempDir()
	src := "package apps\n\nimport \"errors\"\n\nvar (\n\tErrNoVersions   = errors.New(\"no versions found\")\n\tErrNoGoVersions = ErrNoVersions\n)\n"
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	off := uint32(strings.Index(src, "ErrNoGoVersions"))
	decl, err := moveDriver{}.ExtractDecl(path, ingest.Atom{
		Reference: "path:./src.go::ErrNoGoVersions",
		StartByte: off,
		EndByte:   off + uint32(len("ErrNoGoVersions")),
	})
	if err != nil {
		t.Fatalf("var group: %v", err)
	}
	if decl.DeclText != "var ErrNoGoVersions = ErrNoVersions" {
		t.Fatalf("var group decl: %q", decl.DeclText)
	}

	src2 := "package apps\n\nconst (\n\tA = 1\n\tB = 2\n)\n"
	path2 := filepath.Join(dir, "const.go")
	if err := os.WriteFile(path2, []byte(src2), 0o644); err != nil {
		t.Fatal(err)
	}
	off2 := uint32(strings.Index(src2, "B"))
	decl2, err := moveDriver{}.ExtractDecl(path2, ingest.Atom{StartByte: off2, EndByte: off2 + 1})
	if err != nil {
		t.Fatalf("const group: %v", err)
	}
	if decl2.DeclText != "const B = 2" {
		t.Fatalf("const group decl: %q", decl2.DeclText)
	}

	src3 := "package apps\n\nvar Single = 3\n"
	path3 := filepath.Join(dir, "single.go")
	if err := os.WriteFile(path3, []byte(src3), 0o644); err != nil {
		t.Fatal(err)
	}
	off3 := uint32(strings.Index(src3, "Single"))
	decl3, err := moveDriver{}.ExtractDecl(path3, ingest.Atom{StartByte: off3, EndByte: off3 + 6})
	if err != nil {
		t.Fatalf("single var: %v", err)
	}
	if decl3.DeclText != "var Single = 3" {
		t.Fatalf("single var decl: %q", decl3.DeclText)
	}
}

// Shared parent leaf (…/driver/wallpaper under cmd vs pkg) must not both rewrite
// when only cmd/…/driver/wallpaper is moved — parentDir is the full prefix.
func TestFindPathSegmentWithFullParentPath(t *testing.T) {
	content := []byte(`package p
import (
	"example/cmd/app/driver/wallpaper"
	"example/pkg/driver/wallpaper"
	_ "example/pkg/driver/wallpaper/feh"
)
`)
	edits := findPathSegmentOccurrencesInStringsWithParent(
		"f.go", content, "wallpaper", "wallpaper_fuzz", "cmd/app/driver",
	)
	if len(edits) != 1 {
		t.Fatalf("want 1 edit for cmd tree only, got %+v", edits)
	}
}

func TestExtractDecl_IotaConstGroupUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := "package tg\n\ntype PoolKind int\n\nconst (\n\tControl PoolKind = iota\n\tIO\n\tCPU\n)\n"
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	off := uint32(strings.Index(src, "Control"))
	_, err := moveDriver{}.ExtractDecl(path, ingest.Atom{
		Reference: "path:./a.go::Control",
		StartByte: off,
		EndByte:   off + uint32(len("Control")),
	})
	if err == nil {
		t.Fatal("expected error for iota const group extract")
	}
	if !strings.Contains(err.Error(), "iota") || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("got %v", err)
	}
}

func TestExtractDecl_StructFieldUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := "package types\n\ntype SudoCommand struct {\n\tSlug string\n\tCommand string\n}\n"
	path := filepath.Join(dir, "types.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	off := uint32(strings.Index(src, "Slug"))
	_, err := moveDriver{}.ExtractDecl(path, ingest.Atom{
		Reference: "path:./types.go::SudoCommand.Slug",
		StartByte: off,
		EndByte:   off + 4,
	})
	if err == nil {
		t.Fatal("expected error for struct field extract")
	}
	if !strings.Contains(err.Error(), "struct field is not supported") {
		t.Fatalf("got %v", err)
	}
}
