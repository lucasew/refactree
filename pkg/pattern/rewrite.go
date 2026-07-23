package pattern

import (
	"fmt"
	"strconv"
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
	case "capture":
		return captureText(m, repl.As, source), nil
	case "ref":
		// Bound capture ($F:@ref matched) uses As; bare @ref IR uses Ref.
		if repl.As != "" {
			return captureText(m, repl.As, source), nil
		}
		return refEmitText(repl.Ref)
	case "string":
		if repl.FromCapture != "" {
			v := captureText(m, repl.FromCapture, source)
			// String IR holes always emit a string literal. If the capture is
			// already quoted source, keep it; otherwise quote the raw span text
			// (A1: CaptureGroup binds the unquoted interior).
			if _, err := strconv.Unquote(v); err == nil {
				return v, nil
			}
			return strconv.Quote(v), nil
		}
		if repl.Equals != "" {
			return strconv.Quote(repl.Equals), nil
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
	sp, ok := m.CaptureFirst(name)
	if !ok {
		return ""
	}
	return sp.Text(source)
}

// expandTemplate fills $name captures and @provider:path::Symbol ref emits.
// Ref emit is a best-effort source selector (e.g. go:net/http::Get → http.Get);
// it does not rewrite imports.
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
		if tmpl[i] == '@' {
			ref, end, ok := scanTemplateRef(tmpl, i)
			if !ok {
				b.WriteByte('@')
				i++
				continue
			}
			emit, err := refEmitText(ref)
			if err != nil {
				return "", err
			}
			b.WriteString(emit)
			i = end
			continue
		}
		b.WriteByte(tmpl[i])
		i++
	}
	return b.String(), nil
}

// scanTemplateRef reads @provider:path::Symbol starting at @. Returns the ref
// without the leading @, and the index after the ref.
func scanTemplateRef(s string, at int) (ref string, end int, ok bool) {
	if at >= len(s) || s[at] != '@' {
		return "", at, false
	}
	i := at + 1
	start := i
	for i < len(s) {
		c := s[i]
		if c == '(' || c == ')' || c == '{' || c == '}' || c == ',' || c == '*' ||
			c == '$' || c == '@' || c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		i++
	}
	if i == start || !strings.Contains(s[start:i], ":") {
		return "", at, false
	}
	return s[start:i], i, true
}

// refEmitText turns a product ref into source-like selector text for templates.
// go:context::Background → context.Background
// go:net/http::ListenAndServe → http.ListenAndServe
func refEmitText(ref string) (string, error) {
	ref = strings.TrimPrefix(ref, "@")
	r := ingest.ParseReference(ref)
	if r.Name == "" {
		if r.Path == "" {
			return "", fmt.Errorf("replacement ref %q: empty symbol", ref)
		}
		return r.Path, nil
	}
	pkg := r.Path
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		pkg = pkg[i+1:]
	}
	if pkg == "" {
		return r.Name, nil
	}
	return pkg + "." + r.Name, nil
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
// If setCapture is non-empty, one edit is emitted per span of that capture
// (Multi * holes may have several). Matches with no such spans are skipped.
// Otherwise the whole match root is replaced once.
func EditsForMatches(matches []Match, repl Node, source []byte, setCapture string) ([]ingest.Edit, error) {
	var edits []ingest.Edit
	for _, m := range matches {
		text, err := Instantiate(repl, m, source)
		if err != nil {
			return nil, err
		}
		if setCapture != "" {
			sites := m.Captures[setCapture]
			if len(sites) == 0 {
				continue
			}
			for _, sp := range sites {
				edits = append(edits, ingest.Edit{
					File:      m.File,
					StartByte: sp.StartByte,
					EndByte:   sp.EndByte,
					NewText:   text,
				})
			}
			continue
		}
		edits = append(edits, ingest.Edit{
			File:      m.File,
			StartByte: m.Span.StartByte,
			EndByte:   m.Span.EndByte,
			NewText:   text,
		})
	}
	return edits, nil
}
