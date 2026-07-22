package pattern

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// Match is one successful pattern match in a file.
type Match struct {
	File      string
	StartByte uint32
	EndByte   uint32
	Captures  map[string]string
	// emitQuoted marks captures that should be re-emitted as string literals.
	emitQuoted map[string]bool
}

type span struct {
	start, end uint32
}

// tok is one grammar-facing token: a leaf-ish tree node with optional ref target.
type tok struct {
	start, end uint32
	text       string
	target     string // resolved hyperlink target, if any
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
		if p != norm {
			continue
		}
		if use.Target == "" || use.StartByte >= use.EndByte {
			continue
		}
		idx[span{use.StartByte, use.EndByte}] = use.Target
	}
	return idx
}

// MatchFile finds all matches of pat in one source file.
//
// Matching is token-sequence based (not semantic call AST shapes):
// the parse tree is flattened to ordered tokens; @ref holes match tokens that
// carry a hyperlink target; literals match token text; "calls" are just
// sequences like $F@ref · ( · args · ).
func MatchFile(root, fileRel string, source []byte, rootNode *grammar.Node, pat Node, result *ingest.Result) ([]Match, error) {
	links := buildLinkIndex(result, fileRel)
	tokens := buildTokens(rootNode, source, links)
	seq, err := patternToSeq(pat)
	if err != nil {
		return nil, err
	}
	if len(seq) == 0 {
		return nil, nil
	}

	var out []Match
	for i := 0; i < len(tokens); i++ {
		m, next, ok, err := matchSeq(tokens, i, seq, source)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		m.File = fileRel
		out = append(out, m)
		// Non-overlapping: continue after this match (greedy map-style).
		if next > i {
			i = next - 1
		}
	}
	return out, nil
}

// patternToSeq lowers nested pattern IR into a flat token-pattern sequence.
func patternToSeq(pat Node) ([]Node, error) {
	switch pat.Kind {
	case "call":
		var seq []Node
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
			seq = append(seq, a)
		}
		seq = append(seq, Node{Kind: "lit", Text: ")"})
		return seq, nil
	case "token", "type_token", "ref", "string", "capture", "lit":
		return []Node{pat}, nil
	case "rest":
		return nil, fmt.Errorf("pattern: rest only allowed inside calls")
	default:
		return nil, fmt.Errorf("pattern: unsupported kind %q", pat.Kind)
	}
}

// matchSeq tries to match seq starting at tokens[i].
// Returns the match, index after last consumed token, ok, err.
func matchSeq(tokens []tok, i int, seq []Node, source []byte) (Match, int, bool, error) {
	caps := map[string]string{}
	quoted := map[string]bool{}
	if i >= len(tokens) {
		return Match{}, i, false, nil
	}
	start := tokens[i].start
	pos := i
	end := tokens[i].end

	for si, step := range seq {
		if step.Kind == "rest" {
			// Consume until we can match the remainder of seq (usually ")").
			restEnd, nextPos, ok, err := matchRest(tokens, pos, seq[si+1:], caps, quoted, source)
			if err != nil || !ok {
				return Match{}, i, false, err
			}
			if step.As != "" {
				if restEnd > tokens[pos].start {
					// text from current pos through last rest token
					// matchRest returns end byte and next index after rest
				}
				// bind in matchRest
			}
			_ = restEnd
			pos = nextPos
			if pos > 0 && pos <= len(tokens) {
				end = tokens[pos-1].end
			}
			// rest consumes the rest of seq too
			return Match{
				StartByte:  start,
				EndByte:    end,
				Captures:   caps,
				emitQuoted: quoted,
			}, pos, true, nil
		}

		if pos >= len(tokens) {
			return Match{}, i, false, nil
		}

		// Optional: skip pure whitespace tokens if any slipped in
		for pos < len(tokens) && isSpaceText(tokens[pos].text) {
			pos++
		}
		if pos >= len(tokens) {
			return Match{}, i, false, nil
		}

		ok, advance, err := matchOne(tokens, pos, step, caps, quoted, source)
		if err != nil || !ok {
			return Match{}, i, false, err
		}
		end = tokens[pos+advance-1].end
		// Selector sugar: $F:@ref on Errorf also binds fmt.Errorf when preceded by ident "."
		if step.Kind == "ref" && step.As != "" {
			caps[step.As] = selectorText(tokens, pos)
		}
		pos += advance
	}

	// Extend start left if first step was ref and we expanded selector text —
	// match span should cover full selector for rewrite roots.
	if len(seq) > 0 && seq[0].Kind == "ref" {
		start = selectorStart(tokens, i)
	}

	return Match{
		StartByte:  start,
		EndByte:    end,
		Captures:   caps,
		emitQuoted: quoted,
	}, pos, true, nil
}

func matchRest(tokens []tok, pos int, after []Node, caps map[string]string, quoted map[string]bool, source []byte) (endByte uint32, nextPos int, ok bool, err error) {
	// Find the earliest position >= pos where `after` matches; rest is tokens[pos:that).
	if len(after) == 0 {
		// rest to end of... shouldn't happen for calls
		if pos < len(tokens) {
			return tokens[len(tokens)-1].end, len(tokens), true, nil
		}
		return 0, pos, true, nil
	}
	// Name of rest capture is not in after — caller bound rest step separately.
	// We need the rest step's As from the parent; pass via caps convention: look for empty.
	// Actually matchSeq handles rest As — fix matchSeq to bind.

	for j := pos; j <= len(tokens); j++ {
		// try match after at j
		if len(after) == 0 {
			return 0, j, true, nil
		}
		// trial match without committing caps — use copies
		trialCaps := map[string]string{}
		for k, v := range caps {
			trialCaps[k] = v
		}
		trialQ := map[string]bool{}
		for k, v := range quoted {
			trialQ[k] = v
		}
		m, next, ok, err := matchSeq(tokens, j, after, source)
		_ = m
		if err != nil {
			return 0, pos, false, err
		}
		if !ok {
			continue
		}
		// success: rest is [pos, j)
		var restText string
		if j > pos {
			restText = string(source[tokens[pos].start:tokens[j-1].end])
			endByte = tokens[j-1].end
		} else {
			endByte = tokens[pos].start
		}
		// merge trial caps from after into caps — re-run to fill caps properly
		_, next, ok, err = matchSeqFill(tokens, j, after, caps, quoted, source)
		if err != nil || !ok {
			return 0, pos, false, err
		}
		// bind rest — As unknown here; matchSeq must set it
		_ = restText
		return endByte, next, true, nil
	}
	return 0, pos, false, nil
}

// matchSeqFill like matchSeq but starts mid-stream and only matches the sequence (no rest recursion issues).
func matchSeqFill(tokens []tok, i int, seq []Node, caps map[string]string, quoted map[string]bool, source []byte) (Match, int, bool, error) {
	if i >= len(tokens) && len(seq) > 0 {
		return Match{}, i, false, nil
	}
	start := uint32(0)
	end := uint32(0)
	if i < len(tokens) {
		start = tokens[i].start
		end = tokens[i].end
	}
	pos := i
	for _, step := range seq {
		if step.Kind == "rest" {
			return Match{}, i, false, fmt.Errorf("nested rest not supported")
		}
		for pos < len(tokens) && isSpaceText(tokens[pos].text) {
			pos++
		}
		if pos >= len(tokens) {
			return Match{}, i, false, nil
		}
		ok, advance, err := matchOne(tokens, pos, step, caps, quoted, source)
		if err != nil || !ok {
			return Match{}, i, false, err
		}
		if step.Kind == "ref" && step.As != "" {
			caps[step.As] = selectorText(tokens, pos)
		}
		end = tokens[pos+advance-1].end
		pos += advance
	}
	return Match{StartByte: start, EndByte: end, Captures: caps, emitQuoted: quoted}, pos, true, nil
}

func matchOne(tokens []tok, pos int, step Node, caps map[string]string, quoted map[string]bool, source []byte) (ok bool, advance int, err error) {
	t := tokens[pos]
	switch step.Kind {
	case "lit":
		if t.text != step.Text {
			return false, 0, nil
		}
		bind(caps, step.As, t.text)
		return true, 1, nil
	case "token", "type_token":
		if t.text != step.Text {
			return false, 0, nil
		}
		bind(caps, step.As, t.text)
		return true, 1, nil
	case "ref":
		if t.target != step.Ref {
			return false, 0, nil
		}
		// text bound by caller via selectorText for As
		if step.As == "" {
			// nothing
		}
		return true, 1, nil
	case "capture":
		bind(caps, step.As, t.text)
		return true, 1, nil
	case "string":
		content, ok := unquoteLiteral(t.text)
		if !ok {
			return false, 0, nil
		}
		if step.Equals != "" && content != step.Equals {
			return false, 0, nil
		}
		if step.Regex != "" {
			re, err := regexp.Compile(step.Regex)
			if err != nil {
				return false, 0, fmt.Errorf("string regex: %w", err)
			}
			m := re.FindStringSubmatch(content)
			if m == nil {
				return false, 0, nil
			}
			val := content
			if step.CaptureGroup > 0 {
				if step.CaptureGroup >= len(m) {
					return false, 0, fmt.Errorf("string capture_group %d out of range", step.CaptureGroup)
				}
				val = m[step.CaptureGroup]
			}
			if step.As != "" {
				caps[step.As] = val
				quoted[step.As] = true
			}
			return true, 1, nil
		}
		if step.As != "" {
			caps[step.As] = content
			quoted[step.As] = true
		}
		return true, 1, nil
	default:
		return false, 0, fmt.Errorf("pattern: unsupported step kind %q", step.Kind)
	}
}

// selectorText returns "fmt.Errorf" when tokens[pos] is Errorf preceded by "." and "fmt".
func selectorText(tokens []tok, pos int) string {
	if pos >= len(tokens) {
		return ""
	}
	if pos >= 2 && tokens[pos-1].text == "." {
		return tokens[pos-2].text + "." + tokens[pos].text
	}
	return tokens[pos].text
}

func selectorStart(tokens []tok, pos int) uint32 {
	if pos >= 2 && tokens[pos-1].text == "." {
		return tokens[pos-2].start
	}
	return tokens[pos].start
}

func bind(caps map[string]string, name, val string) {
	if name == "" {
		return
	}
	caps[name] = val
}

func isSpaceText(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return s != ""
}

// buildTokens flattens the tree to ordered leaf-ish tokens and attaches ref targets.
func buildTokens(root *grammar.Node, source []byte, links linkIndex) []tok {
	var raw []tok
	collectLeaves(root, source, &raw)

	// Attach exact-span link targets.
	for i := range raw {
		if t, ok := links[span{raw[i].start, raw[i].end}]; ok {
			raw[i].target = t
		}
	}

	// Dedupe identical spans (keep first / with target).
	sort.SliceStable(raw, func(i, j int) bool {
		if raw[i].start != raw[j].start {
			return raw[i].start < raw[j].start
		}
		return raw[i].end < raw[j].end
	})
	var out []tok
	for _, t := range raw {
		if t.text == "" || isSpaceText(t.text) {
			continue
		}
		if len(out) > 0 {
			last := &out[len(out)-1]
			if last.start == t.start && last.end == t.end {
				if last.target == "" && t.target != "" {
					last.target = t.target
				}
				continue
			}
			// Prefer smaller tokens: if last fully contains t, replace last with t
			// when t is strictly inside (innermost).
			if t.start >= last.start && t.end <= last.end && (t.start > last.start || t.end < last.end) {
				// keep both if they are different roles? For stream we want leaves only.
				// collectLeaves should already be leaves — skip parent if present.
				if last.end-last.start > t.end-t.start {
					*last = t
					continue
				}
			}
		}
		out = append(out, t)
	}

	// Ensure every link span appears as a token (even if tree leaf text differed).
	for s, target := range links {
		found := false
		for i := range out {
			if out[i].start == s.start && out[i].end == s.end {
				out[i].target = target
				found = true
				break
			}
		}
		if !found && int(s.end) <= len(source) {
			out = append(out, tok{
				start:  s.start,
				end:    s.end,
				text:   string(source[s.start:s.end]),
				target: target,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].start != out[j].start {
			return out[i].start < out[j].start
		}
		return out[i].end < out[j].end
	})
	// Final dedupe
	var final []tok
	for _, t := range out {
		if len(final) > 0 && final[len(final)-1].start == t.start && final[len(final)-1].end == t.end {
			if final[len(final)-1].target == "" {
				final[len(final)-1].target = t.target
			}
			continue
		}
		final = append(final, t)
	}
	return final
}

func collectLeaves(n *grammar.Node, source []byte, out *[]tok) {
	if n == nil || n.IsNull() {
		return
	}
	// Named leaf or only anonymous children: emit this node's full text as a token
	// when it has no named children; otherwise recurse.
	namedKids := 0
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil || c.IsNull() {
			continue
		}
		// Treat nodes with a type that looks like punctuation as anonymous-ish.
		if isPunctType(c.Type()) || c.Type() == c.Type() && len(c.Type()) <= 2 {
			// still recurse into structure; punctuation emitted as own leaves when leaf
		}
		if !isPunctType(c.Type()) && c.ChildCount() > 0 || !isPunctType(c.Type()) {
			// count non-punctuation children as structure
			if !isPunctType(c.Type()) {
				namedKids++
			}
		}
	}

	// Simpler leaf rule: no children → token; only punct children → token for this node text
	// if the node's text is small; else recurse.
	if n.ChildCount() == 0 {
		text := ingest.NodeText(n, source)
		if text != "" {
			*out = append(*out, tok{start: n.StartByte(), end: n.EndByte(), text: text})
		}
		return
	}

	onlyPunct := true
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil || c.IsNull() {
			continue
		}
		if !isPunctType(c.Type()) && c.Type() != "comment" {
			// if child is a string literal node etc., not punct
			ct := c.Type()
			if ct != "\"" && ct != "'" && !isPunctType(ct) {
				onlyPunct = false
				break
			}
		}
	}

	// Always recurse into children to get fine-grained tokens (ident, ., (, strings…).
	for i := uint32(0); i < n.ChildCount(); i++ {
		collectLeaves(n.Child(i), source, out)
	}
	_ = namedKids
	_ = onlyPunct
}

func isPunctType(t string) bool {
	if t == "" {
		return false
	}
	// single-char punctuation node types in tree-sitter often are the char itself
	if utf8.RuneCountInString(t) == 1 {
		r, _ := utf8.DecodeRuneInString(t)
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}
	switch t {
	case "comment", "\n":
		return true
	default:
		return false
	}
}

func unquoteLiteral(raw string) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	switch raw[0] {
	case '"', '\'', '`':
		if raw[len(raw)-1] != raw[0] {
			return "", false
		}
		inner := raw[1 : len(raw)-1]
		if raw[0] == '`' {
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
	default:
		return "", false
	}
}
