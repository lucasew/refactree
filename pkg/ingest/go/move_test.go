package ingestgo

import "testing"

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
