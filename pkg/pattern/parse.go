package pattern

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ParsePattern parses a CLI pattern string into match IR.
//
// Dialect (see testdata/pattern/README.md):
//
//	literal tokens, /regex/, @ref, $name, $name:@ref, $name:/re/, $name:{…}, $$$_
func ParsePattern(s string) (Node, error) {
	p := &parser{s: s}
	p.skipSpace()
	// Top-level: sequence of atoms, sugar as call if looks like callee(
	n, err := p.parseSequenceAsNode()
	if err != nil {
		return Node{}, err
	}
	p.skipSpace()
	if !p.done() {
		return Node{}, p.errf("trailing input %q", p.rest())
	}
	return n, nil
}

// ParseReplacement parses a rewrite template (literals + $captures), not a match pattern.
func ParseReplacement(s string) (Node, error) {
	// Template IR: single "template" kind with raw string, or parse $Name holes.
	// For fixtures we also accept call-shaped templates via the same surface as patterns
	// but emit-only. Simplest: store as template string node.
	return Node{Kind: "template", Text: s}, nil
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

// parseSequenceAsNode reads atoms until end.
// If single atom, return it; if callee + ( args ), return call sugar; else seq.
func (p *parser) parseSequenceAsNode() (Node, error) {
	atoms, err := p.parseAtomList(0)
	if err != nil {
		return Node{}, err
	}
	return sequenceToNode(atoms), nil
}

func sequenceToNode(atoms []Node) Node {
	if len(atoms) == 0 {
		return Node{Kind: "seq", Args: nil}
	}
	if len(atoms) == 1 {
		return atoms[0]
	}
	// Sugar: REF/CAPTURE ( ... )  => call
	if len(atoms) >= 2 && atoms[1].Kind == "lit" && atoms[1].Text == "(" {
		// find matching close at end
		if atoms[len(atoms)-1].Kind == "lit" && atoms[len(atoms)-1].Text == ")" {
			callee := atoms[0]
			inner := atoms[2 : len(atoms)-1]
			args := splitArgs(inner)
			return Node{Kind: "call", As: "ROOT", Callee: &callee, Args: args}
		}
	}
	return Node{Kind: "seq", Args: atoms}
}

func splitArgs(atoms []Node) []Node {
	var args []Node
	var cur []Node
	depth := 0
	for _, a := range atoms {
		if a.Kind == "lit" && a.Text == "(" {
			depth++
		}
		if a.Kind == "lit" && a.Text == ")" {
			depth--
		}
		if depth == 0 && a.Kind == "lit" && a.Text == "," {
			if len(cur) == 1 {
				args = append(args, cur[0])
			} else if len(cur) > 1 {
				args = append(args, Node{Kind: "seq", Args: cur})
			}
			cur = nil
			continue
		}
		cur = append(cur, a)
	}
	if len(cur) == 1 {
		args = append(args, cur[0])
	} else if len(cur) > 1 {
		args = append(args, Node{Kind: "seq", Args: cur})
	}
	return args
}

// parseAtomList reads atoms until EOF or the stop byte (')' or '}') is next.
// stop==0 means only EOF stops (top-level sequences).
// Bare ')' and '}' are normal lit tokens when they are not the active stop;
// inside $name:{…}, nested '{'…'}' pairs are consumed as lits (brace depth).
func (p *parser) parseAtomList(stop byte) ([]Node, error) {
	var atoms []Node
	braceDepth := 0
	for {
		p.skipSpace()
		if p.done() {
			break
		}
		c := p.peek()
		if stop != 0 && c == stop {
			// Nested braces inside a group: treat '}' as a lit while depth > 0.
			if !(stop == '}' && braceDepth > 0) {
				break
			}
		}
		atom, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		if atom.Kind == "lit" {
			switch atom.Text {
			case "{":
				braceDepth++
			case "}":
				braceDepth--
			}
		}
		atoms = append(atoms, atom)

		// After an atom, if '(' follows, include call argument list as tokens.
		p.skipSpace()
		if p.peek() == '(' {
			p.i++
			atoms = append(atoms, Node{Kind: "lit", Text: "("})
			inner, err := p.parseAtomList(')')
			if err != nil {
				return nil, err
			}
			atoms = append(atoms, inner...)
			p.skipSpace()
			if p.peek() != ')' {
				return nil, p.errf("expected ')'")
			}
			p.i++
			atoms = append(atoms, Node{Kind: "lit", Text: ")"})
			continue
		}
	}
	return atoms, nil
}

func (p *parser) parseAtom() (Node, error) {
	p.skipSpace()
	if p.done() {
		return Node{}, p.errf("unexpected end")
	}

	// $$$_ or $$$name
	if strings.HasPrefix(p.s[p.i:], "$$$") {
		p.i += 3
		name := p.scanIdent()
		if name == "" {
			name = "_"
		}
		return Node{Kind: "rest", As: name}, nil
	}

	// $name or $name:constraint or $name:{...}
	if p.peek() == '$' {
		return p.parseCapture()
	}

	// @ref (unbound)
	if p.peek() == '@' {
		p.i++
		ref, err := p.scanRef()
		if err != nil {
			return Node{}, err
		}
		return Node{Kind: "ref", Ref: ref}, nil
	}

	// /regex/
	if p.peek() == '/' {
		re, err := p.scanRegex()
		if err != nil {
			return Node{}, err
		}
		return Node{Kind: "string", Regex: re, CaptureGroup: defaultCaptureGroup(re)}, nil
	}

	// quoted string literal token
	if p.peek() == '"' || p.peek() == '`' {
		return p.parseStringLit()
	}

	// punctuation single-char tokens
	if isPatternPunct(p.peek()) {
		c := p.s[p.i]
		p.i++
		return Node{Kind: "lit", Text: string(c)}, nil
	}

	// number lit
	if p.peek() >= '0' && p.peek() <= '9' {
		start := p.i
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
		return Node{Kind: "lit", Text: p.s[start:p.i]}, nil
	}

	// word token, optionally with {}
	if r, _ := p.peekRune(); unicode.IsLetter(r) || r == '_' {
		ident := p.scanIdent()
		if strings.HasPrefix(p.s[p.i:], "{}") {
			ident += "{}"
			p.i += 2
		}
		return Node{Kind: "token", Text: ident, As: "ROOT"}, nil
	}

	// interface{} already handled via ident+{}

	return Node{}, p.errf("unexpected %q", p.rest())
}

func (p *parser) parseCapture() (Node, error) {
	if p.peek() != '$' {
		return Node{}, p.errf("expected '$'")
	}
	p.i++
	name := p.scanIdent()
	if name == "" {
		return Node{}, p.errf("expected capture name")
	}

	if p.peek() != ':' {
		return Node{Kind: "capture", As: name}, nil
	}
	p.i++ // :

	// $name:{ ... } or $name:{ ... }*
	if p.peek() == '{' {
		p.i++
		inner, err := p.parseAtomList('}')
		if err != nil {
			return Node{}, err
		}
		p.skipSpace()
		if p.peek() != '}' {
			return Node{}, p.errf("expected '}'")
		}
		p.i++
		innerNode := sequenceToNode(inner)
		n := Node{Kind: "group", As: name, Callee: &innerNode}
		n.Multi = p.scanMultiStar()
		return n, nil
	}

	// $name:@ref or $name:@ref*
	if p.peek() == '@' {
		p.i++
		ref, err := p.scanRef()
		if err != nil {
			return Node{}, err
		}
		n := Node{Kind: "ref", As: name, Ref: ref}
		n.Multi = p.scanMultiStar()
		return n, nil
	}

	// $name:/regex/ or $name:/regex/*
	if p.peek() == '/' {
		re, err := p.scanRegex()
		if err != nil {
			return Node{}, err
		}
		n := Node{Kind: "string", As: name, Regex: re, CaptureGroup: defaultCaptureGroup(re)}
		n.Multi = p.scanMultiStar()
		return n, nil
	}

	return Node{}, p.errf("expected @ref, /regex/, or {…} after $%s:", name)
}

// scanMultiStar consumes an optional trailing * (zero-or-more sites in the gap).
func (p *parser) scanMultiStar() bool {
	p.skipSpace()
	if p.peek() == '*' {
		p.i++
		return true
	}
	return false
}

func defaultCaptureGroup(re string) int {
	// Stretch for fixtures: if there is a capturing group, rebind emit value (not span).
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
	if n >= 1 {
		return 1
	}
	return 0
}

func (p *parser) scanRef() (string, error) {
	start := p.i
	// Allow / in package paths (go:net/http::ListenAndServe).
	// Stop before * so $name:@ref* can set Multi.
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '(' || c == ')' || c == '{' || c == '}' || c == ',' || c == '*' || unicode.IsSpace(rune(c)) {
			break
		}
		// Bare /regex/ is only after $name: — not inside @ref
		p.i++
	}
	ref := p.s[start:p.i]
	if ref == "" || !strings.Contains(ref, ":") {
		return "", p.errf("invalid ref %q", ref)
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

func (p *parser) parseStringLit() (Node, error) {
	q := p.s[p.i]
	p.i++
	start := p.i
	if q == '`' {
		for p.i < len(p.s) && p.s[p.i] != '`' {
			p.i++
		}
		if p.done() {
			return Node{}, p.errf("unterminated string")
		}
		content := p.s[start:p.i]
		p.i++
		return Node{Kind: "string", Equals: content}, nil
	}
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

func isPatternPunct(c byte) bool {
	switch c {
	case '(', ')', '{', '}', '[', ']', ',', '.', '*', '+', '-', '=', '!', '?', ':', ';', '<', '>', '|', '&', '^', '%', '#', '~':
		return true
	default:
		return false
	}
}
