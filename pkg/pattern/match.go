package pattern

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// Match is one successful pattern match in a file.
type Match struct {
	File      string
	StartByte uint32
	EndByte   uint32
	Captures  map[string]string
	// emitQuoted: when true and no emitOverride, re-quote Captures[name] as string lit.
	emitQuoted map[string]bool
	// emitOverride: rewrite text for a capture (e.g. strip via regex group; full token still in Captures).
	emitOverride map[string]string
}

type span struct{ start, end uint32 }

type tok struct {
	start, end uint32
	text       string
	target     string
}

type linkIndex map[span]string

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
		idx[span{use.StartByte, use.EndByte}] = use.Target
	}
	return idx
}

// MatchFile finds matches using a token stream + pattern IR.
func MatchFile(root, fileRel string, source []byte, rootNode *grammar.Node, pat Node, result *ingest.Result) ([]Match, error) {
	links := buildLinkIndex(result, fileRel)
	tokens := buildTokens(rootNode, source, links)
	seq := flattenPattern(pat)
	if len(seq) == 0 {
		return nil, nil
	}

	var out []Match
	for i := 0; i < len(tokens); {
		m, next, ok, err := matchSeq(tokens, i, seq, source, rootNode)
		if err != nil {
			return nil, err
		}
		if !ok {
			i++
			continue
		}
		m.File = fileRel
		out = append(out, m)
		if next <= i {
			i++
		} else {
			i = next
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

func matchSeq(tokens []tok, startIdx int, seq []Node, source []byte, root *grammar.Node) (Match, int, bool, error) {
	caps := map[string]string{}
	quoted := map[string]bool{}
	override := map[string]string{}
	pos := startIdx
	if pos >= len(tokens) || len(seq) == 0 {
		return Match{}, startIdx, false, nil
	}

	matchStart := tokens[pos].start
	matchEnd := tokens[pos].end

	for si := 0; si < len(seq); si++ {
		step := seq[si]

		if step.Kind == "rest" {
			after := seq[si+1:]
			j, next, restText, ok, err := findRest(tokens, pos, after, caps, quoted, override, source, root)
			if err != nil || !ok {
				return Match{}, startIdx, false, err
			}
			if step.As != "" && step.As != "_" {
				caps[step.As] = restText
			}
			if j > pos {
				matchEnd = tokens[j-1].end
			}
			pos = next
			// after already matched and filled caps
			break
		}

		for pos < len(tokens) && isSpaceText(tokens[pos].text) {
			pos++
		}
		if pos >= len(tokens) {
			return Match{}, startIdx, false, nil
		}

		ok, advance, err := matchStep(tokens, pos, step, caps, quoted, override, source, root)
		if err != nil || !ok {
			return Match{}, startIdx, false, err
		}

		if si == 0 && step.Kind == "ref" {
			matchStart = selectorStart(tokens, pos)
		} else if si == 0 {
			matchStart = tokens[pos].start
		}
		matchEnd = tokens[pos+advance-1].end
		pos += advance
	}

	// Call-like: use covering AST node for the whole match span (args + callee).
	if root != nil && looksLikeCallSeq(seq) {
		if n := coveringNode(root, matchStart, matchEnd); n != nil {
			matchStart, matchEnd = n.StartByte(), n.EndByte()
		}
	}

	return Match{
		StartByte:    matchStart,
		EndByte:      matchEnd,
		Captures:     caps,
		emitQuoted:   quoted,
		emitOverride: override,
	}, pos, true, nil
}

func findRest(tokens []tok, pos int, after []Node, caps map[string]string, quoted map[string]bool, override map[string]string, source []byte, root *grammar.Node) (restEndIdx, next int, restText string, ok bool, err error) {
	if len(after) == 0 {
		if pos < len(tokens) {
			return len(tokens), len(tokens), string(source[tokens[pos].start:tokens[len(tokens)-1].end]), true, nil
		}
		return pos, pos, "", true, nil
	}
	for j := pos; j <= len(tokens); j++ {
		// Save and try
		c2 := cloneStr(caps)
		q2 := cloneBool(quoted)
		o2 := cloneStr(override)
		_, next, ok, err := matchSeqFrom(tokens, j, after, c2, q2, o2, source, root)
		if err != nil {
			return 0, 0, "", false, err
		}
		if !ok {
			continue
		}
		// commit
		for k, v := range c2 {
			caps[k] = v
		}
		for k, v := range q2 {
			quoted[k] = v
		}
		for k, v := range o2 {
			override[k] = v
		}
		rt := ""
		if j > pos {
			rt = string(source[tokens[pos].start:tokens[j-1].end])
		}
		return j, next, rt, true, nil
	}
	return 0, 0, "", false, nil
}

func matchSeqFrom(tokens []tok, startIdx int, seq []Node, caps map[string]string, quoted map[string]bool, override map[string]string, source []byte, root *grammar.Node) (Match, int, bool, error) {
	pos := startIdx
	matchStart, matchEnd := uint32(0), uint32(0)
	if pos < len(tokens) {
		matchStart, matchEnd = tokens[pos].start, tokens[pos].end
	}
	for _, step := range seq {
		if step.Kind == "rest" {
			return Match{}, startIdx, false, fmt.Errorf("nested rest")
		}
		for pos < len(tokens) && isSpaceText(tokens[pos].text) {
			pos++
		}
		if pos >= len(tokens) {
			return Match{}, startIdx, false, nil
		}
		ok, advance, err := matchStep(tokens, pos, step, caps, quoted, override, source, root)
		if err != nil || !ok {
			return Match{}, startIdx, false, err
		}
		matchEnd = tokens[pos+advance-1].end
		pos += advance
	}
	return Match{StartByte: matchStart, EndByte: matchEnd}, pos, true, nil
}

func looksLikeCallSeq(seq []Node) bool {
	for _, s := range seq {
		if s.Kind == "lit" && s.Text == "(" {
			return true
		}
	}
	return false
}

func matchStep(tokens []tok, pos int, step Node, caps map[string]string, quoted map[string]bool, override map[string]string, source []byte, root *grammar.Node) (bool, int, error) {
	if pos >= len(tokens) {
		return false, 0, nil
	}
	t := tokens[pos]

	switch step.Kind {
	case "group":
		inner := flattenPattern(*step.Callee)
		// match inner sequence starting at pos
		c2 := cloneStr(caps)
		q2 := cloneBool(quoted)
		o2 := cloneStr(override)
		m, next, ok, err := matchSeqFrom(tokens, pos, inner, c2, q2, o2, source, root)
		if err != nil || !ok {
			return false, 0, err
		}
		for k, v := range c2 {
			caps[k] = v
		}
		for k, v := range q2 {
			quoted[k] = v
		}
		for k, v := range o2 {
			override[k] = v
		}
		start, end := m.StartByte, m.EndByte
		// fix: matchSeqFrom doesn't set start well — use tokens
		start = tokens[pos].start
		if next > pos {
			end = tokens[next-1].end
		}
		if n := coveringNode(root, start, end); n != nil {
			start, end = n.StartByte(), n.EndByte()
		}
		if step.As != "" {
			caps[step.As] = string(source[start:end])
		}
		return true, next - pos, nil

	case "lit", "token", "type_token":
		if t.text != step.Text {
			return false, 0, nil
		}
		if step.As != "" {
			caps[step.As] = t.text
		}
		return true, 1, nil

	case "ref":
		if t.target != step.Ref {
			return false, 0, nil
		}
		if step.As != "" {
			caps[step.As] = selectorText(tokens, pos)
		}
		return true, 1, nil

	case "capture":
		if step.As != "" {
			caps[step.As] = t.text
		}
		return true, 1, nil

	case "string":
		content, uqOK := unquoteLiteral(t.text)
		if step.Equals != "" {
			if !uqOK || content != step.Equals {
				return false, 0, nil
			}
		}
		if step.Regex != "" {
			if !uqOK {
				return false, 0, nil
			}
			re, err := regexp.Compile(step.Regex)
			if err != nil {
				return false, 0, err
			}
			m := re.FindStringSubmatch(content)
			if m == nil {
				return false, 0, nil
			}
			if step.As != "" {
				// Full token in Captures (lock A)
				caps[step.As] = t.text
				quoted[step.As] = true
				// Fixture stretch: group 1 as emit override (already quoted content)
				if step.CaptureGroup > 0 && step.CaptureGroup < len(m) {
					override[step.As] = quoteString(m[step.CaptureGroup])
				}
			}
			return true, 1, nil
		}
		if step.As != "" {
			caps[step.As] = t.text
			if uqOK {
				quoted[step.As] = true
			}
		}
		return true, 1, nil

	default:
		return false, 0, fmt.Errorf("unsupported step kind %q", step.Kind)
	}
}

func quoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteByte(s[i])
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(s[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}

func selectorText(tokens []tok, pos int) string {
	start := pos
	for start >= 2 && tokens[start-1].text == "." {
		start -= 2
	}
	var b strings.Builder
	for i := start; i <= pos; i++ {
		b.WriteString(tokens[i].text)
	}
	return b.String()
}

func selectorStart(tokens []tok, pos int) uint32 {
	start := pos
	for start >= 2 && tokens[start-1].text == "." {
		start -= 2
	}
	return tokens[start].start
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
		if t, ok := links[span{raw[i].start, raw[i].end}]; ok {
			raw[i].target = t
		}
	}
	for s, target := range links {
		found := false
		for i := range raw {
			if raw[i].start == s.start && raw[i].end == s.end {
				raw[i].target = target
				found = true
				break
			}
		}
		if !found && int(s.end) <= len(source) {
			raw = append(raw, tok{start: s.start, end: s.end, text: string(source[s.start:s.end]), target: target})
		}
	}
	// Prefer innermost: drop tokens that strictly contain another
	var candidates []tok
	for _, a := range raw {
		if a.text == "" || isSpaceText(a.text) {
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
			if b.start >= a.start && b.end <= a.end && (b.start > a.start || b.end < a.end) {
				inner = true
				break
			}
		}
		if !inner {
			leaves = append(leaves, a)
		}
	}
	sort.SliceStable(leaves, func(i, j int) bool {
		if leaves[i].start != leaves[j].start {
			return leaves[i].start < leaves[j].start
		}
		return leaves[i].end < leaves[j].end
	})
	// Dedupe identical spans
	var out []tok
	for _, t := range leaves {
		if len(out) > 0 && out[len(out)-1].start == t.start && out[len(out)-1].end == t.end {
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
	text := ingest.NodeText(n, source)
	typ := n.Type()

	// Keep string literals as one token (not quote/content/quote pieces).
	switch typ {
	case "interpreted_string_literal", "raw_string_literal", "string_literal", "string", "template_string":
		if text != "" {
			*out = append(*out, tok{start: n.StartByte(), end: n.EndByte(), text: text})
		}
		return
	}

	// Composite grammar tokens (e.g. empty interface{}) — whole span, no children.
	if text == "interface{}" || (len(text) > 2 && strings.HasSuffix(text, "{}") && !strings.Contains(text, " ") && !strings.Contains(text, "\t") && !strings.Contains(text, "\n")) {
		*out = append(*out, tok{start: n.StartByte(), end: n.EndByte(), text: text})
		return
	}
	if n.ChildCount() == 0 {
		if text != "" {
			*out = append(*out, tok{start: n.StartByte(), end: n.EndByte(), text: text})
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		collectLeaves(n.Child(i), source, out)
	}
}

func isSpaceText(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return s != ""
}

func unquoteLiteral(raw string) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	q := raw[0]
	if (q != '"' && q != '\'' && q != '`') || raw[len(raw)-1] != q {
		return "", false
	}
	inner := raw[1 : len(raw)-1]
	if q == '`' {
		return inner, true
	}
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) {
			i++
			switch inner[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\', '"', '\'':
				b.WriteByte(inner[i])
			default:
				b.WriteByte('\\')
				b.WriteByte(inner[i])
			}
			continue
		}
		b.WriteByte(inner[i])
	}
	return b.String(), true
}

func cloneStr(m map[string]string) map[string]string {
	o := make(map[string]string, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}
func cloneBool(m map[string]bool) map[string]bool {
	o := make(map[string]bool, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}
