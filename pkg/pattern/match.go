package pattern

import (
	"bytes"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// Span is a half-open UTF-8 byte range [StartByte, EndByte) in a source buffer.
// Offsets match tree-sitter StartByte/EndByte (not rune indexes). Used for
// tokens, match roots, pattern captures, and hyperlink leaves. Text is always
// derived from the original file []byte — never stored on the span.
type Span struct {
	StartByte uint32
	EndByte   uint32
}

// Text returns src[StartByte:EndByte].
func (s Span) Text(src []byte) string {
	b := s.Bytes(src)
	if b == nil {
		return ""
	}
	return string(b)
}

// Bytes returns src[StartByte:EndByte], or nil if out of range / empty invalid.
func (s Span) Bytes(src []byte) []byte {
	if src == nil || s.EndByte > uint32(len(src)) || s.StartByte > s.EndByte {
		return nil
	}
	return src[s.StartByte:s.EndByte]
}

// Empty reports whether the span has zero length (StartByte == EndByte).
func (s Span) Empty() bool { return s.StartByte >= s.EndByte }

// Eq reports whether s.Text(src) equals want without an intermediate string
// when lengths differ.
func (s Span) Eq(src []byte, want string) bool {
	b := s.Bytes(src)
	return len(b) == len(want) && string(b) == want
}

// Match is one successful pattern match in a file.
// It does not retain file contents; pass the file []byte at the call site
// (Stream OnMatch/OnFile, Span.Text, PublicCaptures, Instantiate).
type Match struct {
	File string
	Span // root match range
	// Captures maps pattern $names to one or more source spans.
	// Single holes have len 1; Multi (*) holes list every site in the gap.
	Captures map[string][]Span
}

// CaptureFirst returns the first span for name, or a zero Span.
func (m Match) CaptureFirst(name string) (Span, bool) {
	ss := m.Captures[name]
	if len(ss) == 0 {
		return Span{}, false
	}
	return ss[0], true
}

// tok is a significant leaf: a Span plus optional hyperlink target.
type tok struct {
	Span
	target string
}

type linkIndex map[Span]string

func buildLinkIndex(result *ingest.Result, fileRel string) linkIndex {
	idx := linkIndex{}
	if result == nil {
		return idx
	}
	norm := strings.TrimPrefix(filepath.ToSlash(fileRel), "./")
	for _, use := range result.Uses {
		ref := ingest.ParseReference(use.Reference)
		p := strings.TrimPrefix(filepath.ToSlash(ref.Path), "./")
		if p != norm || use.Target == "" || use.StartByte >= use.EndByte {
			continue
		}
		idx[Span{use.StartByte, use.EndByte}] = use.Target
	}
	return idx
}

// MatchFile finds matches using a token stream + pattern NFA.
func MatchFile(root, fileRel string, source []byte, rootNode *grammar.Node, pat Node, result *ingest.Result) ([]Match, error) {
	_ = root
	links := buildLinkIndex(result, fileRel)
	tokens := buildTokens(rootNode, source, links)
	if len(tokens) == 0 {
		return nil, nil
	}
	out, err := matchPattern(pat, tokens, source, rootNode)
	if err != nil {
		return nil, err
	}
	callish := looksLikeCallSeq(flattenPattern(pat))
	for i := range out {
		out[i].File = fileRel
		if callish && rootNode != nil {
			if n := coveringNode(rootNode, out[i].StartByte, out[i].EndByte); n != nil {
				out[i].Span = Span{StartByte: n.StartByte(), EndByte: n.EndByte()}
			}
		}
	}
	return out, nil
}

func flattenPattern(pat Node) []Node {
	switch pat.Kind {
	case "call":
		seq := []Node{}
		if pat.Callee != nil {
			seq = append(seq, *pat.Callee)
		}
		seq = append(seq, Node{Kind: "lit", Text: "("})
		for i, a := range pat.Args {
			if a.Kind == "rest" {
				seq = append(seq, a)
				continue
			}
			if i > 0 {
				seq = append(seq, Node{Kind: "lit", Text: ","})
			}
			seq = append(seq, flattenPattern(a)...)
		}
		seq = append(seq, Node{Kind: "lit", Text: ")"})
		return seq
	case "seq":
		var seq []Node
		for _, a := range pat.Args {
			seq = append(seq, flattenPattern(a)...)
		}
		return seq
	default:
		return []Node{pat}
	}
}


func looksLikeCallSeq(seq []Node) bool {
	for _, s := range seq {
		if s.Kind == "lit" && s.Text == "(" {
			return true
		}
	}
	return false
}


func selectorStart(tokens []tok, pos int, source []byte) uint32 {
	return selectorSpan(tokens, pos, source).StartByte
}

func selectorSpan(tokens []tok, pos int, source []byte) Span {
	i := pos
	for i >= 2 && tokens[i-1].Eq(source, ".") {
		i -= 2
	}
	return Span{StartByte: tokens[i].StartByte, EndByte: tokens[pos].EndByte}
}

func coveringNode(root *grammar.Node, start, end uint32) *grammar.Node {
	if root == nil {
		return nil
	}
	var best *grammar.Node
	var walk func(*grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.StartByte() <= start && n.EndByte() >= end {
			if best == nil || (n.EndByte()-n.StartByte()) < (best.EndByte()-best.StartByte()) {
				best = n
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return best
}

func buildTokens(root *grammar.Node, source []byte, links linkIndex) []tok {
	var raw []tok
	collectLeaves(root, source, &raw)
	for i := range raw {
		if t, ok := links[raw[i].Span]; ok {
			raw[i].target = t
		}
	}
	for s, target := range links {
		found := false
		for i := range raw {
			if raw[i].StartByte == s.StartByte && raw[i].EndByte == s.EndByte {
				raw[i].target = target
				found = true
				break
			}
		}
		if !found && int(s.EndByte) <= len(source) && s.StartByte < s.EndByte {
			raw = append(raw, tok{Span: s, target: target})
		}
	}
	// Prefer innermost: drop tokens that strictly contain another
	var candidates []tok
	for _, a := range raw {
		if a.StartByte >= a.EndByte || isSpaceSpan(source, a) {
			continue
		}
		candidates = append(candidates, a)
	}
	var leaves []tok
	for i, a := range candidates {
		inner := false
		for j, b := range candidates {
			if i == j {
				continue
			}
			if b.StartByte >= a.StartByte && b.EndByte <= a.EndByte && (b.StartByte > a.StartByte || b.EndByte < a.EndByte) {
				inner = true
				break
			}
		}
		if !inner {
			leaves = append(leaves, a)
		}
	}
	sort.SliceStable(leaves, func(i, j int) bool {
		if leaves[i].StartByte != leaves[j].StartByte {
			return leaves[i].StartByte < leaves[j].StartByte
		}
		return leaves[i].EndByte < leaves[j].EndByte
	})
	// Dedupe identical spans
	var out []tok
	for _, t := range leaves {
		if len(out) > 0 && out[len(out)-1].StartByte == t.StartByte && out[len(out)-1].EndByte == t.EndByte {
			if out[len(out)-1].target == "" {
				out[len(out)-1].target = t.target
			}
			continue
		}
		out = append(out, t)
	}
	return out
}

func collectLeaves(n *grammar.Node, source []byte, out *[]tok) {
	if n == nil || n.IsNull() {
		return
	}
	start, end := n.StartByte(), n.EndByte()
	typ := n.Type()

	// Keep string literals as one token (not quote/content/quote pieces).
	switch typ {
	case "interpreted_string_literal", "raw_string_literal", "string_literal", "string", "template_string":
		if start < end {
			*out = append(*out, tok{Span: Span{StartByte: start, EndByte: end}})
		}
		return
	}

	// Composite grammar tokens (e.g. empty interface{}) — whole span, no children.
	if isCompositeTokenSpan(source, start, end) {
		*out = append(*out, tok{Span: Span{StartByte: start, EndByte: end}})
		return
	}
	if n.ChildCount() == 0 {
		if start < end {
			*out = append(*out, tok{Span: Span{StartByte: start, EndByte: end}})
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		collectLeaves(n.Child(i), source, out)
	}
}

// isCompositeTokenSpan reports grammar spans like interface{} kept as one token.
func isCompositeTokenSpan(src []byte, start, end uint32) bool {
	if start >= end || int(end) > len(src) {
		return false
	}
	b := src[start:end]
	if string(b) == "interface{}" {
		return true
	}
	if len(b) > 2 && b[len(b)-2] == '{' && b[len(b)-1] == '}' {
		for _, c := range b[:len(b)-2] {
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				return false
			}
		}
		return true
	}
	return false
}

func isSpaceSpan(src []byte, t tok) bool {
	b := t.Bytes(src)
	return len(b) > 0 && len(bytes.TrimSpace(b)) == 0
}

// tokenContentMap derives the regex/equals match surface from a token span in
// source. For string lits, content is unquoted; for other tokens, content is
// the raw source slice. srcOf maps each content byte to an offset within the
// token (0-based); closeOff is the exclusive end of content within the token.
// Absolute source offset = t.StartByte + srcOf[i].
func tokenContentMap(src []byte, t tok) (content string, srcOf []int, closeOff int, quoted bool) {
	raw := t.Bytes(src)
	if content, srcOf, closeOff, ok := unquoteLiteralMap(raw); ok {
		return content, srcOf, closeOff, true
	}
	srcOf = make([]int, len(raw))
	for i := range raw {
		srcOf[i] = i
	}
	return string(raw), srcOf, len(raw), false
}

// unquoteLiteralMap unquotes a string token and records, for each content byte,
// the starting offset within the raw token of the source bytes that produced it.
// closeOff is the raw index of the closing quote.
func unquoteLiteralMap(raw []byte) (content string, srcOf []int, closeOff int, ok bool) {
	if len(raw) < 2 {
		return "", nil, 0, false
	}
	q := raw[0]
	if (q != '"' && q != '\'' && q != '`') || raw[len(raw)-1] != q {
		return "", nil, 0, false
	}
	closeOff = len(raw) - 1
	if q == '`' {
		inner := raw[1:closeOff]
		srcOf = make([]int, len(inner))
		for i := range inner {
			srcOf[i] = 1 + i
		}
		return string(inner), srcOf, closeOff, true
	}
	var b strings.Builder
	i := 1
	for i < closeOff {
		if raw[i] == '\\' && i+1 < closeOff {
			escStart := i
			i++
			switch raw[i] {
			case 'n':
				b.WriteByte('\n')
				srcOf = append(srcOf, escStart)
			case 't':
				b.WriteByte('\t')
				srcOf = append(srcOf, escStart)
			case '\\', '"', '\'':
				b.WriteByte(raw[i])
				srcOf = append(srcOf, escStart)
			default:
				// Preserve unknown escapes as two content bytes, both mapped.
				b.WriteByte('\\')
				srcOf = append(srcOf, escStart)
				b.WriteByte(raw[i])
				srcOf = append(srcOf, i)
			}
			i++
			continue
		}
		srcOf = append(srcOf, i)
		b.WriteByte(raw[i])
		i++
	}
	return b.String(), srcOf, closeOff, true
}

// unquoteLiteral is the content-only helper (no offset map).
// Prefers strconv.Unquote for std Go string/raw/char lits; falls back to the
// offset-map unquoter for odd single-quoted spans tree-sitter may emit.
func unquoteLiteral(raw string) (string, bool) {
	if s, err := strconv.Unquote(raw); err == nil {
		return s, true
	}
	content, _, _, ok := unquoteLiteralMap([]byte(raw))
	return content, ok
}

// contentSpanToSource maps a half-open range in unquoted/raw content into an
// absolute source Span using srcOf (content byte → raw token offset) and
// tokenStart. closeOff is the exclusive raw end of content (closing quote or
// len(token)). ok is false when the content range is out of bounds.
func contentSpanToSource(tokenStart uint32, srcOf []int, closeOff, cs, ce int) (Span, bool) {
	if cs < 0 || ce < cs || ce > len(srcOf) {
		return Span{}, false
	}
	if cs == ce {
		// Empty match: zero-width at cs (or at content end before closeOff).
		var s uint32
		if cs < len(srcOf) {
			s = tokenStart + uint32(srcOf[cs])
		} else {
			s = tokenStart + uint32(closeOff)
		}
		return Span{StartByte: s, EndByte: s}, true
	}
	start := tokenStart + uint32(srcOf[cs])
	var end uint32
	if ce < len(srcOf) {
		// Exclusive end is the start of the next content byte's source.
		end = tokenStart + uint32(srcOf[ce])
	} else {
		end = tokenStart + uint32(closeOff)
	}
	if end < start {
		return Span{}, false
	}
	return Span{StartByte: start, EndByte: end}, true
}
