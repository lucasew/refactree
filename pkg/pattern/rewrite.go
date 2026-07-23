package pattern

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lucasew/refactree/pkg/ingest"
)

// Instantiate builds replacement text from a template node or call-shaped emit IR.
// source is the file content that produced m (for Span.Text).
func Instantiate(repl Node, m Match, source []byte) (string, error) {
	switch repl.Kind {
	case "template":
		return expandTemplate(repl.Text, m, source)
	case "call":
		return instantiateCall(repl, m, source)
	case "token", "type_token", "lit":
		return repl.Text, nil
	case "capture", "ref":
		return captureText(m, repl.As, source), nil
	case "string":
		if repl.FromCapture != "" {
			v := captureText(m, repl.FromCapture, source)
			// String IR holes always emit a string literal. If the capture is
			// already quoted source, keep it; otherwise quote the raw span text
			// (A1: CaptureGroup binds the unquoted interior).
			if _, ok := unquoteLiteral(v); ok {
				return v, nil
			}
			return quoteString(v), nil
		}
		if repl.Equals != "" {
			return quoteString(repl.Equals), nil
		}
		return "", fmt.Errorf("string replacement needs from_capture or equals")
	default:
		// Fallback: if Text looks like a template
		if repl.Text != "" {
			return expandTemplate(repl.Text, m, source)
		}
		return "", fmt.Errorf("replacement: unsupported kind %q", repl.Kind)
	}
}

func captureText(m Match, name string, source []byte) string {
	if name == "" || m.Captures == nil {
		return ""
	}
	return m.Captures[name].Text(source)
}

func expandTemplate(tmpl string, m Match, source []byte) (string, error) {
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
			b.WriteString(captureText(m, name, source))
			continue
		}
		b.WriteByte(tmpl[i])
		i++
	}
	return b.String(), nil
}

func instantiateCall(n Node, m Match, source []byte) (string, error) {
	if n.Callee == nil {
		return "", fmt.Errorf("call replacement missing callee")
	}
	fn, err := Instantiate(*n.Callee, m, source)
	if err != nil {
		return "", err
	}
	// If callee is capture/ref, Instantiate handles it
	if n.Callee.Kind == "capture" || n.Callee.Kind == "ref" {
		fn = captureText(m, n.Callee.As, source)
	}
	var parts []string
	for _, a := range n.Args {
		if a.Kind == "rest" {
			if v := captureText(m, a.As, source); v != "" {
				parts = append(parts, v)
			}
			continue
		}
		p, err := Instantiate(a, m, source)
		if err != nil {
			return "", err
		}
		parts = append(parts, p)
	}
	return fn + "(" + strings.Join(parts, ", ") + ")", nil
}

// EditsForMatches turns matches + replacement into ingest.Edit values.
// source is the file content for all matches (one file per call).
// If setCapture is non-empty, each edit targets that capture's Span; matches
// missing the capture are skipped. Otherwise the whole match root is replaced.
func EditsForMatches(matches []Match, repl Node, source []byte, setCapture string) ([]ingest.Edit, error) {
	var edits []ingest.Edit
	for _, m := range matches {
		text, err := Instantiate(repl, m, source)
		if err != nil {
			return nil, err
		}
		sp := m.Span
		if setCapture != "" {
			c, ok := m.Captures[setCapture]
			if !ok {
				continue
			}
			sp = c
		}
		edits = append(edits, ingest.Edit{
			File:      m.File,
			StartByte: sp.StartByte,
			EndByte:   sp.EndByte,
			NewText:   text,
		})
	}
	return edits, nil
}
