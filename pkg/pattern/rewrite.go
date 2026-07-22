package pattern

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// Instantiate builds the replacement text for a match from replacement_ir.
func Instantiate(repl Node, m Match) (string, error) {
	return instantiateNode(repl, m)
}

func instantiateNode(n Node, m Match) (string, error) {
	switch n.Kind {
	case "call":
		return instantiateCall(n, m)
	case "type_token":
		if n.Text == "" {
			return "", fmt.Errorf("type_token replacement missing text")
		}
		return n.Text, nil
	case "string":
		if n.FromCapture != "" {
			v, ok := m.Captures[n.FromCapture]
			if !ok {
				return "", fmt.Errorf("string from_capture %q not bound", n.FromCapture)
			}
			// Always emit as a double-quoted Go string for format args.
			return strconv.Quote(v), nil
		}
		if n.Equals != "" {
			return strconv.Quote(n.Equals), nil
		}
		if n.Text != "" {
			return strconv.Quote(n.Text), nil
		}
		return "", fmt.Errorf("string replacement needs from_capture, equals, or text")
	case "capture":
		v, ok := m.Captures[n.As]
		if !ok {
			return "", fmt.Errorf("capture %q not bound", n.As)
		}
		if m.emitQuoted[n.As] {
			return strconv.Quote(v), nil
		}
		return v, nil
	case "lit":
		return n.Text, nil
	case "ref":
		// Unusual in replacement; emit bound capture if As set, else ref string.
		if n.As != "" {
			if v, ok := m.Captures[n.As]; ok {
				return v, nil
			}
		}
		return n.Ref, nil
	default:
		return "", fmt.Errorf("replacement: unsupported kind %q", n.Kind)
	}
}

func instantiateCall(n Node, m Match) (string, error) {
	if n.Callee == nil {
		return "", fmt.Errorf("call replacement missing callee")
	}
	fn, err := instantiateNode(*n.Callee, m)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, a := range n.Args {
		if a.Kind == "rest" {
			v := m.Captures[a.As]
			if v != "" {
				parts = append(parts, v)
			}
			continue
		}
		p, err := instantiateNode(a, m)
		if err != nil {
			return "", err
		}
		parts = append(parts, p)
	}
	return fn + "(" + strings.Join(parts, ", ") + ")", nil
}

// EditsForMatches turns matches + replacement_ir into ingest.Edit values.
func EditsForMatches(matches []Match, repl Node) ([]ingest.Edit, error) {
	var edits []ingest.Edit
	for _, m := range matches {
		text, err := Instantiate(repl, m)
		if err != nil {
			return nil, err
		}
		edits = append(edits, ingest.Edit{
			File:      m.File,
			StartByte: m.StartByte,
			EndByte:   m.EndByte,
			NewText:   text,
		})
	}
	return edits, nil
}
