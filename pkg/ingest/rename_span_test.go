package ingest

import "testing"

func TestMaskNonNewlinesInPlace(t *testing.T) {
	buf := []byte("ab\ncd\nef")
	MaskNonNewlinesInPlace(buf, 0, 5)
	if got := string(buf); got != "  \n  \nef" {
		t.Fatalf("mask mid: got %q", got)
	}
	// clamp past end and negative start
	buf = []byte("xy")
	MaskNonNewlinesInPlace(buf, -1, 99)
	if got := string(buf); got != "  " {
		t.Fatalf("mask clamp: got %q", got)
	}
	// empty range
	buf = []byte("keep")
	MaskNonNewlinesInPlace(buf, 2, 2)
	if got := string(buf); got != "keep" {
		t.Fatalf("empty range: got %q", got)
	}
}

func TestIdentUsed(t *testing.T) {
	if IdentUsed("foo bar", "foo", IsIdentChar) != true {
		t.Fatal("expected foo hit")
	}
	if IdentUsed("foobar", "foo", IsIdentChar) {
		t.Fatal("prefix must not match")
	}
	if IdentUsed("xfoo", "foo", IsIdentChar) {
		t.Fatal("suffix must not match")
	}
	if IdentUsed("", "foo", IsIdentChar) {
		t.Fatal("empty text")
	}
	if IdentUsed("foo", "", IsIdentChar) {
		t.Fatal("empty ident")
	}
	if IdentUsed("foo", "foo", nil) {
		t.Fatal("nil isIdent")
	}
	// $ is an ident char for Java/JS class
	if !IdentUsed("$foo bar", "$foo", IsIdentCharJava) {
		t.Fatal("expected $foo hit")
	}
	if IdentUsed("a$foo", "$foo", IsIdentCharJava) {
		t.Fatal("$ mid-ident must not match as start")
	}
}
