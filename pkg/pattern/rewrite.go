package pattern

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lucasew/refactree/pkg/ingest"
)

// Instantiate builds replacement text from a template node or call-shaped emit IR.
func Instantiate(repl Node, m Match) (string, error) {
	switch repl.Kind {
	case "template":
		return expandTemplate(repl.Text, m)
	case "call":
		return instantiateCall(repl, m)
	case "token", "type_token", "lit":
		return repl.Text, nil
	case "capture", "ref":
		return captureText(m, repl.As), nil
	case "string":
		if repl.FromCapture != "" {
			return captureText(m, repl.FromCapture), nil
		}
		if repl.Equals != "" {
			return quoteString(repl.Equals), nil
		}
		return "", fmt.Errorf("string replacement needs from_capture or equals")
	default:
		// Fallback: if Text looks like a template
		if repl.Text != "" {
			return expandTemplate(repl.Text, m)
		}
		return "", fmt.Errorf("replacement: unsupported kind %q", repl.Kind)
	}
}

func captureText(m Match, name string) string {
	if name == "" {
		return ""
	}
	if m.emitOverride != nil {
		if v, ok := m.emitOverride[name]; ok {
			return v
		}
	}
	v := m.Captures[name]
	// Captures for strings already include quotes when from string holes.
	return v
}

func expandTemplate(tmpl string, m Match) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(tmpl) {
		if tmpl[i] == '$' {
			i++
			start := i
			for i < len(tmpl) {
				r, w := utf8.DecodeRuneInString(tmpl[i:])
				if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					break
				}
				i += w
			}
			name := tmpl[start:i]
			if name == "" {
				b.WriteByte('$')
				continue
			}
			b.WriteString(captureText(m, name))
			continue
		}
		b.WriteByte(tmpl[i])
		i++
	}
	return b.String(), nil
}

func instantiateCall(n Node, m Match) (string, error) {
	if n.Callee == nil {
		return "", fmt.Errorf("call replacement missing callee")
	}
	fn, err := Instantiate(*n.Callee, m)
	if err != nil {
		return "", err
	}
	// If callee is capture/ref, Instantiate handles it
	if n.Callee.Kind == "capture" || n.Callee.Kind == "ref" {
		fn = captureText(m, n.Callee.As)
	}
	var parts []string
	for _, a := range n.Args {
		if a.Kind == "rest" {
			if v := m.Captures[a.As]; v != "" {
				parts = append(parts, v)
			}
			continue
		}
		p, err := Instantiate(a, m)
		if err != nil {
			return "", err
		}
		parts = append(parts, p)
	}
	return fn + "(" + strings.Join(parts, ", ") + ")", nil
}

// EditsForMatches turns matches + replacement into ingest.Edit values.
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
