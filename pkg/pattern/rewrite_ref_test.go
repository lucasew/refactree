package pattern

import (
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestRefEmitText(t *testing.T) {
	cases := []struct {
		ref  string
		want string
	}{
		{"go:context::Background", "context.Background"},
		{"@go:context::Background", "context.Background"},
		{"go:net/http::ListenAndServe", "http.ListenAndServe"},
		{"go:fmt::Errorf", "fmt.Errorf"},
		{"go:testing::T", "testing.T"},
	}
	for _, tc := range cases {
		got, err := refEmitText(tc.ref)
		if err != nil {
			t.Fatalf("refEmitText(%q): %v", tc.ref, err)
		}
		if got != tc.want {
			t.Errorf("refEmitText(%q)=%q want %q", tc.ref, got, tc.want)
		}
	}
}

func TestExpandTemplateRef(t *testing.T) {
	got, err := expandTemplate(`@go:context::Background`, Match{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "context.Background" {
		t.Fatalf("got %q", got)
	}
	got, err = expandTemplate(`c=@go:net/http::ListenAndServe`, Match{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// full template with literal prefix (set form is split before expand)
	if got != "c=http.ListenAndServe" {
		t.Fatalf("got %q", got)
	}
	got, err = expandTemplate(`@go:fmt::Errorf($MSG)`, Match{
		Captures: map[string][]ingest.Span{"MSG": {{StartByte: 0, EndByte: 3}}},
	}, []byte("err"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "fmt.Errorf(err)" {
		t.Fatalf("got %q", got)
	}
}

func TestSetCaptureWithRefEmit(t *testing.T) {
	pat, err := ParsePattern(`$c:@go:context::Background`)
	if err != nil {
		t.Fatal(err)
	}
	name, tmpl := splitCaptureSet(pat, `c=@go:testing::T`)
	// testing.T is not a declared capture as left side of = — wait, c is declared
	if name != "c" {
		t.Fatalf("name=%q", name)
	}
	got, err := expandTemplate(tmpl, Match{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// User wrote c=@go:testing::T meaning set c to testing.T emit
	// But split gives tmpl=@go:testing::T → testing.T
	if got != "testing.T" {
		t.Fatalf("tmpl expand=%q want testing.T", got)
	}
}
