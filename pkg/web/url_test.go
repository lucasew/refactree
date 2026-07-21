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

func TestDecodeCodePath_BareFolderSegments(t *testing.T) {
	// Unencoded /code/cmd and /code/cmd/rft must gain path:./ (folder rail clicks).
	cases := []struct {
		path string
		want string
	}{
		{"/code/cmd", "path:./cmd"},
		{"/code/cmd/rft", "path:./cmd/rft"},
		{"/code/path%3A.%2Fcmd", "path:./cmd"},
		{"/code/path%3A.%2Fcmd%2Frft", "path:./cmd/rft"},
		{"/code/path:.%2Fcmd", "path:./cmd"},
	}
	for _, tc := range cases {
		got, ok := DecodeCodePath(tc.path)
		if !ok {
			t.Fatalf("DecodeCodePath(%q) failed", tc.path)
		}
		if got != tc.want {
			t.Fatalf("DecodeCodePath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestCanonicalFSRef_BareNames(t *testing.T) {
	if got := canonicalFSRef("cmd"); got != "path:./cmd" {
		t.Fatalf("got %q", got)
	}
	if got := canonicalFSRef("cmd/rft"); got != "path:./cmd/rft" {
		t.Fatalf("got %q", got)
	}
	if got := canonicalFSRef("path:./cmd"); got != "path:./cmd" {
		t.Fatalf("got %q", got)
	}
	if got := canonicalFSRef("go:fmt"); got != "go:fmt" {
		t.Fatalf("got %q", got)
	}
}
