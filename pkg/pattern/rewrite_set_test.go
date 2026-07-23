package pattern

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestSplitCaptureSet(t *testing.T) {
	pat, err := ParsePattern(`func $name:{/^Test/} { $$$_ $c:@go:context::Background $$$_ }`)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		repl     string
		wantName string
		wantTmpl string
	}{
		{"any", "", "any"},
		{"c=t.Context", "c", "t.Context"},
		{"name=Foo", "name", "Foo"},
		{"$F($MSG)", "", "$F($MSG)"},
		{"notacap=x", "", "notacap=x"}, // not declared → full template
		{"=x", "", "=x"},
		{"c=", "c", ""},
	}
	for _, tc := range cases {
		gotName, gotTmpl := splitCaptureSet(pat, tc.repl)
		if gotName != tc.wantName || gotTmpl != tc.wantTmpl {
			t.Errorf("splitCaptureSet(%q)=(%q,%q) want (%q,%q)",
				tc.repl, gotName, gotTmpl, tc.wantName, tc.wantTmpl)
		}
	}
}

func TestRewriteSetCaptureOnly(t *testing.T) {
	dir := t.TempDir()
	src := []byte(`package p

import (
	"context"
	"testing"
)

func TestFoo(t *testing.T) {
	_ = context.Background()
	_ = context.Background()
}

func Helper() {
	_ = context.Background()
}
`)
	path := filepath.Join(dir, "x_test.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	// Without *: only first Background in TestFoo.
	op, err := OpFromCLI("rewrite", "go",
		`func /Test.*/ (t *testing.T) { $$$_ $c:@go:context::Background $$$_ }`,
		`c=t.Context`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if op.SetCapture != "c" {
		t.Fatalf("SetCapture=%q want c", op.SetCapture)
	}
	res, err := Apply(dir, op)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edits) != 1 {
		t.Fatalf("edits=%d want 1 (first only); edits=%v", len(res.Edits), res.Edits)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Count(got, "t.Context()") != 1 {
		t.Fatalf("t.Context count=%d want 1\n%s", strings.Count(got, "t.Context()"), got)
	}
	if strings.Count(got, "context.Background") != 2 {
		t.Fatalf("context.Background count=%d want 2 (second Test + Helper)\n%s",
			strings.Count(got, "context.Background"), got)
	}
}

func TestRewriteSetCaptureMultiStar(t *testing.T) {
	dir := t.TempDir()
	src := []byte(`package p

import (
	"context"
	"testing"
)

func TestFoo(t *testing.T) {
	_ = context.Background()
	_ = context.Background()
}

func Helper() {
	_ = context.Background()
}
`)
	path := filepath.Join(dir, "x_test.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	// With *: every Background inside each Test* function.
	op, err := OpFromCLI("rewrite", "go",
		`func /Test.*/ (t *testing.T) { $$$_ $c:@go:context::Background* $$$_ }`,
		`c=t.Context`,
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(dir, op)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edits) != 2 {
		t.Fatalf("edits=%d want 2; edits=%v", len(res.Edits), res.Edits)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Count(got, "t.Context()") != 2 {
		t.Fatalf("t.Context count=%d want 2\n%s", strings.Count(got, "t.Context()"), got)
	}
	if strings.Count(got, "context.Background") != 1 {
		t.Fatalf("context.Background count=%d want 1 (Helper only)\n%s",
			strings.Count(got, "context.Background"), got)
	}
}

func TestParseMultiStar(t *testing.T) {
	n, err := ParsePattern(`$c:@go:context::Background*`)
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != "ref" || !n.Multi || n.As != "c" {
		t.Fatalf("got %+v", n)
	}
	n, err = ParsePattern(`$c:{@go:context::Background}*`)
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != "group" || !n.Multi {
		t.Fatalf("group multi got %+v", n)
	}
	n, err = ParsePattern(`$c:{t.Context}*`)
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != "group" || !n.Multi || n.As != "c" {
		t.Fatalf("t.Context multi group %+v", n)
	}
}

func TestRewriteMultiGroupTContext(t *testing.T) {
	dir := t.TempDir()
	src := []byte(`package p

import "testing"

func TestFoo(t *testing.T) {
	_ = t.Context()
	_ = t.Context()
}

func Helper(t *testing.T) {
	_ = t.Context()
}
`)
	path := filepath.Join(dir, "x_test.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	op, err := OpFromCLI("rewrite", "go",
		`func /Test.*/ (t *testing.T) { $$$_ $c:{t.Context}* $$$_ }`,
		`c=@go:context::Background`,
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(dir, op)
	if err != nil {
		t.Fatal(err)
	}
	// 2 site leaf rewrites + import "context" ensure for @go:context::Background
	if len(res.Edits) < 2 {
		t.Fatalf("edits=%d want >=2; %v", len(res.Edits), res.Edits)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Count(got, "context.Background()") != 2 {
		t.Fatalf("context.Background count=%d\n%s", strings.Count(got, "context.Background()"), got)
	}
	if strings.Count(got, "t.Context()") != 1 {
		t.Fatalf("Helper should keep one t.Context:\n%s", got)
	}
	if !strings.Contains(got, `"context"`) {
		t.Fatalf("expected import context:\n%s", got)
	}
}

func TestRewriteNestedMultiIteration(t *testing.T) {
	dir := t.TempDir()
	src := []byte(`package p

import "testing"

func TestFoo(t *testing.T) {
	_ = t.Context()
	_ = t.Context()
}
`)
	path := filepath.Join(dir, "x_test.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	op, err := OpFromCLI("rewrite", "go",
		`func /Test.*/ (t *testing.T) { $$$_ $i:{ $c:{t.Context} $$$_ }* }`,
		`c=@go:context::Background`,
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(dir, op)
	if err != nil {
		t.Fatal(err)
	}
	// 2 site rewrites + import ensure for @go:context::Background
	if len(res.Edits) < 2 {
		t.Fatalf("edits=%d want >=2; %v", len(res.Edits), res.Edits)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Count(got, "context.Background()") != 2 {
		t.Fatalf("context.Background count=%d\n%s", strings.Count(got, "context.Background()"), got)
	}
	if !strings.Contains(got, `"context"`) {
		t.Fatalf("expected import context:\n%s", got)
	}
}
