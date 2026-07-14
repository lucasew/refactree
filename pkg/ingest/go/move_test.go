package ingestgo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

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
	decl, err := moveDriver{}.ExtractDecl(path, ingest.Entity{
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
	decl2, err := moveDriver{}.ExtractDecl(path2, ingest.Entity{StartByte: off2, EndByte: off2 + 1})
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
	decl3, err := moveDriver{}.ExtractDecl(path3, ingest.Entity{StartByte: off3, EndByte: off3 + 6})
	if err != nil {
		t.Fatalf("single var: %v", err)
	}
	if decl3.DeclText != "var Single = 3" {
		t.Fatalf("single var decl: %q", decl3.DeclText)
	}
}
