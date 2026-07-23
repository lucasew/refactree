// Package pattern implements structural match/rewrite for rft grep / rft rewrite
// (pattern IR, string dialect parser, and testdata/pattern fixtures).
package pattern

import (
	"encoding/json"
	"fmt"
	"os"
)

// Op is the fixture/CLI operation description (op.json).
type Op struct {
	Mode          string  `json:"mode"` // grep | rewrite
	Lang          string  `json:"lang"`
	Description   string  `json:"description,omitempty"`
	Pattern       string  `json:"pattern,omitempty"`
	Replacement   *string `json:"replacement"` // null for grep
	PatternIR     Node    `json:"pattern_ir"`
	ReplacementIR *Node   `json:"replacement_ir"`
	// SetCapture, when non-empty, limits rewrite edits to that capture's Span
	// instead of the whole match root. CLI form: replacement "name=template".
	SetCapture       string   `json:"set_capture,omitempty"`
	ExpectMatchCount *int     `json:"expect_match_count,omitempty"`
	Notes            []string `json:"notes,omitempty"`
}

// Node is one pattern/replacement IR node.
type Node struct {
	Kind string `json:"kind"`

	As  string `json:"as,omitempty"`
	Ref string `json:"ref,omitempty"`

	// token (grammar node text) / lit
	Text string `json:"text,omitempty"`

	// string
	Equals       string `json:"equals,omitempty"`
	Regex        string `json:"regex,omitempty"`
	CaptureGroup int    `json:"capture_group,omitempty"`
	FromCapture  string `json:"from_capture,omitempty"`

	// Multi (* suffix on $name:@ref or $name:/re/): collect every matching site
	// in the gap before the rest of the pattern (non-adjacent ok). Rewrite
	// name=… emits one edit per site.
	Multi bool `json:"multi,omitempty"`

	// call
	Callee *Node  `json:"callee,omitempty"`
	Args   []Node `json:"args,omitempty"`
}

// LoadOp reads and validates an op.json file.
func LoadOp(path string) (Op, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Op{}, err
	}
	var op Op
	if err := json.Unmarshal(b, &op); err != nil {
		return Op{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if op.Mode != "grep" && op.Mode != "rewrite" {
		return Op{}, fmt.Errorf("%s: mode must be grep or rewrite, got %q", path, op.Mode)
	}
	if op.PatternIR.Kind == "" {
		return Op{}, fmt.Errorf("%s: pattern_ir.kind required", path)
	}
	if op.Mode == "rewrite" {
		if op.ReplacementIR == nil {
			return Op{}, fmt.Errorf("%s: rewrite requires replacement_ir", path)
		}
	}
	return op, nil
}
