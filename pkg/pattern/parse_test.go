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
		name    string
		pattern string
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
				if r.Kind != op.ReplacementIR.Kind {
					t.Fatalf("repl kind got %s want %s\n%s", r.Kind, op.ReplacementIR.Kind, mustJSON(r))
				}
			}
		})
	}
}

func mustJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
