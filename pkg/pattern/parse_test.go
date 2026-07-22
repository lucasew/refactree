package pattern

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePattern_Fixtures(t *testing.T) {
	// Round-trip: parse fixture pattern string, match same cases as pattern_ir intent.
	cases := []struct {
		name     string
		pattern  string
		wantKind string
	}{
		{"any", "interface{}", "token"},
		{"failed", `$F:@go:fmt::Errorf($MSG:/(?i)^failed to\s+(.*)/, $ERR)`, "call"},
		{"splitn", `$F:@go:strings::SplitN($S, $SEP, 2)`, "call"},
		{"listen", `$F:@go:net/http::ListenAndServe($ADDR, $HANDLER)`, "call"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := ParsePattern(tc.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if n.Kind != tc.wantKind {
				t.Fatalf("kind=%s want %s\n%s", n.Kind, tc.wantKind, mustJSON(n))
			}
		})
	}
}

func TestParsePattern_MatchesFixtureIR(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "pattern")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			op, err := LoadOp(filepath.Join(root, e.Name(), "op.json"))
			if err != nil {
				t.Fatal(err)
			}
			if op.Pattern == "" {
				t.Fatal("empty pattern")
			}
			got, err := ParsePattern(op.Pattern)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			// Structural checks rather than deep equal (IR may use As:ROOT etc.)
			if got.Kind != op.PatternIR.Kind {
				t.Fatalf("kind got %s want %s\ngot=%s\nwant=%s", got.Kind, op.PatternIR.Kind, mustJSON(got), mustJSON(op.PatternIR))
			}
			if op.Mode == "rewrite" && op.Replacement != nil && *op.Replacement != "" {
				r, err := ParseReplacement(*op.Replacement)
				if err != nil {
					t.Fatalf("parse replacement: %v", err)
				}
				// Replacement is a template string (locked dialect), not match IR.
				if r.Kind != "template" {
					t.Fatalf("repl kind got %s want template\n%s", r.Kind, mustJSON(r))
				}
				if r.Text != *op.Replacement {
					t.Fatalf("repl text got %q want %q", r.Text, *op.Replacement)
				}
			}
		})
	}
}

func mustJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func TestParsePattern_BareBracesAndDualRest(t *testing.T) {
	cases := []struct {
		name     string
		pat      string
		wantLit  string // if set, flatten must contain this lit
		wantRest int    // if >0, exact rest count in flatten
		wantKind string // if set, root kind
	}{
		{name: "close_brace", pat: `func Foo ( ) { }`, wantLit: "}"},
		{name: "rest_body", pat: `func Foo() { $$$_ }`, wantLit: "}", wantRest: 1},
		{name: "brace_rest", pat: `{ $$$_ }`, wantLit: "}", wantRest: 1},
		{name: "test_ctx", pat: `func /Test.*/ (t *testing.T) { $$$_ $c:@go:context::Background $$$_ }`, wantLit: "}", wantRest: 2},
		{name: "mid_brace", pat: `x } y`, wantLit: "}"},
		{name: "nested_group", pat: `$g:{ a { b } c }`, wantKind: "group"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := ParsePattern(tc.pat)
			if err != nil {
				t.Fatalf("ParsePattern(%q): %v", tc.pat, err)
			}
			if tc.wantKind != "" && n.Kind != tc.wantKind {
				t.Fatalf("kind=%s want %s", n.Kind, tc.wantKind)
			}
			seq := flattenPattern(n)
			if tc.wantLit != "" {
				found := false
				for _, s := range seq {
					if s.Kind == "lit" && s.Text == tc.wantLit {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected lit %q in flatten: %s", tc.wantLit, mustJSON(seq))
				}
			}
			if tc.wantRest > 0 {
				nrest := 0
				for _, s := range seq {
					if s.Kind == "rest" {
						nrest++
					}
				}
				if nrest != tc.wantRest {
					t.Fatalf("rest count=%d want %d; %s", nrest, tc.wantRest, mustJSON(seq))
				}
			}
		})
	}
}
