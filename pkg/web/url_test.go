package web

import (
	"strings"
	"testing"
)

func TestEncodeDecodeCodeURL_RoundTrip(t *testing.T) {
	ref := "path:./main.go"
	u := EncodeCodeURL(ref)
	if !strings.HasPrefix(u, CodePathPrefix) {
		t.Fatalf("expected prefix, got %q", u)
	}
	got, ok := DecodeCodePath(u)
	if !ok {
		t.Fatal("decode failed")
	}
	if got != ref {
		t.Fatalf("round-trip: want %q got %q", ref, got)
	}
}

func TestEncodeCodeURL_SymbolAddsAnchor(t *testing.T) {
	u := EncodeCodeURL("path:./helper.go::helper")
	if !strings.Contains(u, "#sym-") {
		t.Fatalf("expected anchor fragment, got %q", u)
	}
	if !strings.Contains(u, "path") {
		t.Fatalf("expected encoded path ref in URL, got %q", u)
	}
}
