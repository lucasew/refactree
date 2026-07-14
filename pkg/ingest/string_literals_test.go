package ingest

import (
	"reflect"
	"testing"
)

func TestForEachStringLiteral(t *testing.T) {
	src := []byte(`x = "hello"; y = ` + "`raw`")
	var got []struct {
		lit   string
		start int
	}
	ForEachStringLiteral(src, func(lit string, start int) bool {
		got = append(got, struct {
			lit   string
			start int
		}{lit, start})
		return true
	})
	want := []struct {
		lit   string
		start int
	}{
		{`"hello"`, 4},
		{"`raw`", 17},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestForEachStringLiteralEscapes(t *testing.T) {
	src := []byte(`"a\"b"`)
	var lits []string
	ForEachStringLiteral(src, func(lit string, start int) bool {
		lits = append(lits, lit)
		return true
	})
	if len(lits) != 1 || lits[0] != `"a\"b"` {
		t.Fatalf("escaped quote: got %#v", lits)
	}
}

func TestFindAllOccurrencesInStringsUsesWalker(t *testing.T) {
	content := []byte(`import "old/pkg"; // old/pkg outside; var s = "old/pkg"`)
	edits := FindAllOccurrencesInStrings("f.go", content, "old/pkg", "new/pkg")
	if len(edits) != 2 {
		t.Fatalf("want 2 string matches, got %d: %#v", len(edits), edits)
	}
	for _, e := range edits {
		if e.NewText != "new/pkg" {
			t.Fatalf("bad edit: %#v", e)
		}
	}
}
