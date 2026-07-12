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
	if !strings.Contains(u, "helper") {
		t.Fatalf("expected symbol in URL path, got %q", u)
	}
	base := strings.Split(u, "#")[0]
	got, ok := DecodeCodePath(base)
	if !ok || got != "path:./helper.go::helper" {
		t.Fatalf("decode: ok=%v got=%q", ok, got)
	}
}

func TestEncodeCodeURLInRoot_MatchesEncodeCodeURL(t *testing.T) {
	ref := "path:./pkg/web/url.go"
	if got, want := EncodeCodeURLInRoot("/any/root", ref), EncodeCodeURL(ref); got != want {
		t.Fatalf("EncodeCodeURLInRoot = %q, EncodeCodeURL = %q", got, want)
	}
}
