package pattern

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ParsePattern parses a CLI pattern string into match IR.
// Supported subset (v1): type tokens, @ref holes, calls, string/lit/capture/rest args.
func ParsePattern(s string) (Node, error) {
	p := &parser{s: s}
	p.skipSpace()
	n, err := p.parseExpr(true)
	if err != nil {
		return Node{}, err
	}
	p.skipSpace()
	if !p.done() {
		return Node{}, p.errf("trailing input %q", p.rest())
	}
	return n, nil
}

// ParseReplacement parses a CLI replacement template into replacement IR.
func ParseReplacement(s string) (Node, error) {
	// Same surface as patterns; @ref on holes is allowed but usually plain $Name.
	return ParsePattern(s)
}

type parser struct {
	s string
	i int
}

func (p *parser) done() bool { return p.i >= len(p.s) }

func (p *parser) rest() string {
	if p.done() {
		return ""
	}
	return p.s[p.i:]
}

func (p *parser) peek() byte {
	if p.done() {
		return 0
	}
	return p.s[p.i]
}

func (p *parser) peekRune() (rune, int) {
	if p.done() {
		return 0, 0
	}
	return utf8.DecodeRuneInString(p.s[p.i:])
}

func (p *parser) skipSpace() {
	for p.i < len(p.s) {
		r, w := utf8.DecodeRuneInString(p.s[p.i:])
		if !unicode.IsSpace(r) {
			return
		}
		p.i += w
	}
}

func (p *parser) errf(format string, args ...any) error {
	return fmt.Errorf("pattern:%d: %s", p.i, fmt.Sprintf(format, args...))
}

func (p *parser) parseExpr(allowCall bool) (Node, error) {
	p.skipSpace()
	if p.done() {
		return Node{}, p.errf("unexpected end")
	}

	// type_token: interface{}
	if strings.HasPrefix(p.s[p.i:], "interface{}") {
		p.i += len("interface{}")
		n := Node{Kind: "type_token", Text: "interface{}", As: "ROOT"}
		return n, nil
	}

	// Bare word type_token like any (only when not followed by '(' or '.' as package)
	if r, _ := p.peekRune(); unicode.IsLetter(r) || r == '_' {
		start := p.i
		ident := p.scanIdent()
		p.skipSpace()
		if allowCall && p.peek() == '(' {
			// treat as callee name without capture — unusual; wrap as capture-less ref-less
			// For replacement/call like fmt.Errorf(...) we need callee as text.
			// Represent as capture with fixed text via kind "lit" for callee? Better: kind "text".
			// Matcher uses capture/ref for callee. Instantiator for call uses instantiateNode.
			// Add kind "text" for raw emission — or type_token for simple idents.
			// For match, plain identifier call is rare in our fixtures.
			// Use kind "lit" for callee text in replacement of form Foo(bar) without $.
			callee := Node{Kind: "lit", Text: ident}
			return p.parseCall(callee)
		}
		// Standalone bare ident → type_token (any) or lit
		if ident == "any" || ident == "interface{}" {
			return Node{Kind: "type_token", Text: ident, As: "ROOT"}, nil
		}
		// Reset not needed — only type_token/any for now
		_ = start
		return Node{Kind: "type_token", Text: ident, As: "ROOT"}, nil
	}

	// $$$rest or $hole
	if p.peek() == '$' {
		n, err := p.parseHole()
		if err != nil {
			return Node{}, err
		}
		p.skipSpace()
		if allowCall && p.peek() == '(' {
			return p.parseCall(n)
		}
		return n, nil
	}

	// string literal
	if p.peek() == '"' || p.peek() == '`' {
		return p.parseStringNode()
	}

	// number lit
	if p.peek() >= '0' && p.peek() <= '9' {
		return p.parseNumberLit()
	}

	return Node{}, p.errf("unexpected %q", p.rest())
}

func (p *parser) parseCall(callee Node) (Node, error) {
	if p.peek() != '(' {
		return Node{}, p.errf("expected '('")
	}
	p.i++ // (
	var args []Node
	p.skipSpace()
	if p.peek() != ')' {
		for {
			p.skipSpace()
			arg, err := p.parseExpr(true)
			if err != nil {
				return Node{}, err
			}
			args = append(args, arg)
			p.skipSpace()
			if p.peek() == ',' {
				p.i++
				continue
			}
			break
		}
	}
	p.skipSpace()
	if p.peek() != ')' {
		return Node{}, p.errf("expected ')'")
	}
	p.i++
	return Node{
		Kind:   "call",
		As:     "ROOT",
		Callee: &callee,
		Args:   args,
	}, nil
}

func (p *parser) parseHole() (Node, error) {
	if p.peek() != '$' {
		return Node{}, p.errf("expected '$'")
	}
	p.i++
	// $$$
	if strings.HasPrefix(p.s[p.i:], "$$") {
		p.i += 2
		name := p.scanIdent()
		if name == "" {
			return Node{}, p.errf("expected rest capture name after $$$")
		}
		return Node{Kind: "rest", As: name}, nil
	}
	name := p.scanIdent()
	if name == "" {
		return Node{}, p.errf("expected capture name after $")
	}

	// optional :constraint
	if p.peek() == ':' {
		p.i++
		return p.parseConstrainedHole(name)
	}
	return Node{Kind: "capture", As: name}, nil
}

func (p *parser) parseConstrainedHole(name string) (Node, error) {
	p.skipSpace()
	switch p.peek() {
	case '@':
		p.i++
		ref, err := p.scanRef()
		if err != nil {
			return Node{}, err
		}
		return Node{Kind: "ref", As: name, Ref: ref}, nil
	case '/':
		re, err := p.scanRegex()
		if err != nil {
			return Node{}, err
		}
		n := Node{Kind: "string", As: name, Regex: re}
		// If the regex has a capturing group, rebind $name to group 1 (fixture convention).
		if strings.Contains(re, "(") && !strings.Contains(re, "(?") {
			n.CaptureGroup = 1
		} else if strings.Contains(re, "(") {
			// Has some group; if there's a numbered or plain capture for suffix, use 1
			// when pattern contains "(.*" or similar. Count capturing groups roughly.
			if capturingGroupCount(re) >= 1 {
				n.CaptureGroup = 1
			}
		}
		return n, nil
	case '"', '`':
		strNode, err := p.parseStringNode()
		if err != nil {
			return Node{}, err
		}
		strNode.As = name
		return strNode, nil
	default:
		return Node{}, p.errf("expected @ref, /regex/, or string after %s:", name)
	}
}

func capturingGroupCount(re string) int {
	// Rough: count '(' not followed by '?' and not escaped.
	n := 0
	for i := 0; i < len(re); i++ {
		if re[i] == '\\' {
			i++
			continue
		}
		if re[i] == '(' {
			if i+1 < len(re) && re[i+1] == '?' {
				continue
			}
			n++
		}
	}
	return n
}

func (p *parser) scanRef() (string, error) {
	// provider:path::symbol or provider:path
	// e.g. go:fmt::Errorf, go:net/http::ListenAndServe, go:context.Context
	start := p.i
	if p.done() {
		return "", p.errf("expected ref after @")
	}
	// read until delimiter for call/arg: ( ) , whitespace end
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '(' || c == ')' || c == ',' || unicode.IsSpace(rune(c)) {
			break
		}
		p.i++
	}
	ref := p.s[start:p.i]
	if ref == "" || !strings.Contains(ref, ":") {
		return "", p.errf("invalid ref %q (want provider:path[::symbol])", ref)
	}
	return ref, nil
}

func (p *parser) scanRegex() (string, error) {
	if p.peek() != '/' {
		return "", p.errf("expected '/'")
	}
	p.i++
	start := p.i
	for p.i < len(p.s) {
		if p.s[p.i] == '\\' && p.i+1 < len(p.s) {
			p.i += 2
			continue
		}
		if p.s[p.i] == '/' {
			re := p.s[start:p.i]
			p.i++
			return re, nil
		}
		p.i++
	}
	return "", p.errf("unterminated regex")
}

func (p *parser) parseStringNode() (Node, error) {
	if p.done() {
		return Node{}, p.errf("expected string")
	}
	quote := p.s[p.i]
	if quote != '"' && quote != '`' {
		return Node{}, p.errf("expected string quote")
	}
	p.i++
	start := p.i
	if quote == '`' {
		for p.i < len(p.s) && p.s[p.i] != '`' {
			p.i++
		}
		if p.done() {
			return Node{}, p.errf("unterminated raw string")
		}
		content := p.s[start:p.i]
		p.i++
		return Node{Kind: "string", Equals: content}, nil
	}
	// interpreted "
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '"' {
			p.i++
			return Node{Kind: "string", Equals: b.String()}, nil
		}
		if c == '\\' && p.i+1 < len(p.s) {
			p.i++
			esc := p.s[p.i]
			p.i++
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\', '"':
				b.WriteByte(esc)
			default:
				b.WriteByte('\\')
				b.WriteByte(esc)
			}
			continue
		}
		b.WriteByte(c)
		p.i++
	}
	return Node{}, p.errf("unterminated string")
}

func (p *parser) parseNumberLit() (Node, error) {
	start := p.i
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
	}
	if start == p.i {
		return Node{}, p.errf("expected number")
	}
	return Node{Kind: "lit", Text: p.s[start:p.i]}, nil
}

func (p *parser) scanIdent() string {
	if p.done() {
		return ""
	}
	r, w := p.peekRune()
	if r != '_' && !unicode.IsLetter(r) {
		return ""
	}
	start := p.i
	p.i += w
	for p.i < len(p.s) {
		r, w = utf8.DecodeRuneInString(p.s[p.i:])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		p.i += w
	}
	return p.s[start:p.i]
}
