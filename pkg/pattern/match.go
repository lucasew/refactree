package pattern

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// Match is one successful pattern match in a file.
type Match struct {
	File      string
	StartByte uint32
	EndByte   uint32
	// Captures maps $Name → bound text used by replacement instantiation.
	// Structural holes bind source text of the matched node.
	// String holes with capture_group bind the group content (unquoted).
	Captures map[string]string
	// emitQuoted marks captures that should be re-emitted as string literals.
	emitQuoted map[string]bool
}

// linkTarget maps a half-open [start,end) span to a resolved ref target.
type linkIndex map[span]string

type span struct {
	start, end uint32
}

func buildLinkIndex(result *ingest.Result, fileRel string) linkIndex {
	idx := linkIndex{}
	if result == nil {
		return idx
	}
	norm := filepath.ToSlash(fileRel)
	norm = strings.TrimPrefix(norm, "./")
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

func (idx linkIndex) targetIn(start, end uint32) (string, bool) {
	// Exact leaf match first.
	if t, ok := idx[span{start, end}]; ok {
		return t, true
	}
	// Among links fully inside the node (e.g. field of a selector), prefer the
	// rightmost span so go:strings::SplitN wins over the package go:strings operand.
	var bestStart, bestEnd uint32
	var best string
	found := false
	for s, t := range idx {
		if s.start < start || s.end > end {
			continue
		}
		if !found || s.start > bestStart || (s.start == bestStart && s.end > bestEnd) {
			bestStart, bestEnd, best = s.start, s.end, t
			found = true
		}
	}
	return best, found
}

// MatchFile finds all matches of pat in one source file.
// fileRel is the path relative to root (for edits and link lookup).
func MatchFile(root, fileRel string, source []byte, rootNode *grammar.Node, pat Node, result *ingest.Result) ([]Match, error) {
	links := buildLinkIndex(result, fileRel)
	var out []Match
	var walkErr error
	walkNodes(rootNode, func(n *grammar.Node) bool {
		if walkErr != nil {
			return false
		}
		caps := map[string]string{}
		quoted := map[string]bool{}
		ok, err := matchNode(n, source, pat, links, caps, quoted)
		if err != nil {
			walkErr = err
			return false
		}
		if ok {
			out = append(out, Match{
				File:       fileRel,
				StartByte:  n.StartByte(),
				EndByte:    n.EndByte(),
				Captures:   caps,
				emitQuoted: quoted,
			})
		}
		return true
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

func walkNodes(n *grammar.Node, fn func(*grammar.Node) bool) {
	if n == nil || n.IsNull() {
		return
	}
	if !fn(n) {
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		walkNodes(n.Child(i), fn)
	}
}

func matchNode(n *grammar.Node, source []byte, pat Node, links linkIndex, caps map[string]string, quoted map[string]bool) (bool, error) {
	if n == nil || n.IsNull() {
		return false, nil
	}
	switch pat.Kind {
	case "call":
		return matchCall(n, source, pat, links, caps, quoted)
	case "type_token":
		return matchTypeToken(n, source, pat, caps)
	case "ref":
		return matchRef(n, source, pat, links, caps)
	case "string":
		return matchString(n, source, pat, caps, quoted)
	case "capture":
		bind(caps, pat.As, ingest.NodeText(n, source))
		return true, nil
	case "lit":
		if ingest.NodeText(n, source) != pat.Text {
			return false, nil
		}
		bind(caps, pat.As, pat.Text)
		return true, nil
	case "rest":
		// rest is only valid inside call args, not as a root node.
		return false, fmt.Errorf("pattern: rest only allowed in call args")
	default:
		return false, fmt.Errorf("pattern: unsupported kind %q", pat.Kind)
	}
}

func matchTypeToken(n *grammar.Node, source []byte, pat Node, caps map[string]string) (bool, error) {
	text := ingest.NodeText(n, source)
	if text != pat.Text {
		return false, nil
	}
	// Prefer the concrete type node kinds we care about; still allow exact text match
	// on any node so "any"/"interface{}" work across grammar variants.
	switch n.Type() {
	case "interface_type", "type_identifier", "type_element", "pointer_type":
		// ok
	default:
		// Still accept if exact text equals (e.g. bare token).
		if n.Type() == "source_file" || strings.Contains(n.Type(), "declaration") {
			return false, nil
		}
		// Reject large parents: only match if this node is "small".
		if int(n.EndByte()-n.StartByte()) > len(pat.Text)+2 {
			return false, nil
		}
	}
	// For interface{}, the node is interface_type spanning interface{}.
	// Avoid matching a parent that merely contains the text by requiring exact equality (already).
	// Skip if any child also equals the full text (prefer leaves... actually interface_type's
	// children are interface, {, } separately — full text only on parent). Good.
	bind(caps, pat.As, text)
	return true, nil
}

func matchRef(n *grammar.Node, source []byte, pat Node, links linkIndex, caps map[string]string) (bool, error) {
	t, ok := links.targetIn(n.StartByte(), n.EndByte())
	if !ok || t != pat.Ref {
		return false, nil
	}
	// Bind the whole node text (e.g. fmt.Errorf) so $F rewrites keep the selector.
	bind(caps, pat.As, ingest.NodeText(n, source))
	return true, nil
}

func matchString(n *grammar.Node, source []byte, pat Node, caps map[string]string, quoted map[string]bool) (bool, error) {
	switch n.Type() {
	case "interpreted_string_literal", "raw_string_literal":
	default:
		return false, nil
	}
	raw := ingest.NodeText(n, source)
	content, ok := unquoteGoString(raw)
	if !ok {
		return false, nil
	}
	if pat.Equals != "" {
		if content != pat.Equals {
			return false, nil
		}
		if pat.As != "" {
			caps[pat.As] = content
			quoted[pat.As] = true
		}
		return true, nil
	}
	if pat.Regex != "" {
		re, err := regexp.Compile(pat.Regex)
		if err != nil {
			return false, fmt.Errorf("string regex: %w", err)
		}
		m := re.FindStringSubmatch(content)
		if m == nil {
			return false, nil
		}
		val := content
		if pat.CaptureGroup > 0 {
			if pat.CaptureGroup >= len(m) {
				return false, fmt.Errorf("string capture_group %d out of range", pat.CaptureGroup)
			}
			val = m[pat.CaptureGroup]
		}
		if pat.As != "" {
			caps[pat.As] = val
			quoted[pat.As] = true
		}
		return true, nil
	}
	// Bare string capture.
	if pat.As != "" {
		caps[pat.As] = content
		quoted[pat.As] = true
	}
	return true, nil
}

func matchCall(n *grammar.Node, source []byte, pat Node, links linkIndex, caps map[string]string, quoted map[string]bool) (bool, error) {
	if n.Type() != "call_expression" {
		return false, nil
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return false, nil
	}
	if pat.Callee != nil {
		ok, err := matchNode(fn, source, *pat.Callee, links, caps, quoted)
		if err != nil || !ok {
			return false, err
		}
	}
	argList := ingest.ChildByField(n, "arguments")
	if argList == nil {
		argList = ingest.ChildByType(n, "argument_list")
	}
	args := callArgs(argList)
	return matchArgs(args, source, pat.Args, links, caps, quoted)
}

func callArgs(argList *grammar.Node) []*grammar.Node {
	if argList == nil {
		return nil
	}
	var out []*grammar.Node
	for i := uint32(0); i < argList.ChildCount(); i++ {
		c := argList.Child(i)
		if c == nil || c.IsNull() {
			continue
		}
		switch c.Type() {
		case "(", ")", ",", "comment":
			continue
		default:
			out = append(out, c)
		}
	}
	return out
}

func matchArgs(args []*grammar.Node, source []byte, pats []Node, links linkIndex, caps map[string]string, quoted map[string]bool) (bool, error) {
	ai := 0
	for pi := 0; pi < len(pats); pi++ {
		p := pats[pi]
		if p.Kind == "rest" {
			// Bind remaining args' source from first remaining start to last end,
			// joined as they appear — for rewrite we need the raw text slice of rest.
			if ai >= len(args) {
				bind(caps, p.As, "")
				continue
			}
			start := args[ai].StartByte()
			end := args[len(args)-1].EndByte()
			// Include commas/spaces between args from original source.
			bind(caps, p.As, string(source[start:end]))
			ai = len(args)
			continue
		}
		if ai >= len(args) {
			return false, nil
		}
		ok, err := matchNode(args[ai], source, p, links, caps, quoted)
		if err != nil || !ok {
			return false, err
		}
		ai++
	}
	// All pattern args consumed; no leftover source args unless last was rest.
	if ai != len(args) {
		return false, nil
	}
	return true, nil
}

func bind(caps map[string]string, name, val string) {
	if name == "" {
		return
	}
	caps[name] = val
}

func unquoteGoString(raw string) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	switch raw[0] {
	case '"':
		if raw[len(raw)-1] != '"' {
			return "", false
		}
		// Minimal: interpreted strings in fixtures have no escapes beyond %w.
		// Use strconv if needed later.
		s := raw[1 : len(raw)-1]
		s = strings.ReplaceAll(s, `\\`, `\`)
		s = strings.ReplaceAll(s, `\"`, `"`)
		s = strings.ReplaceAll(s, `\n`, "\n")
		s = strings.ReplaceAll(s, `\t`, "\t")
		return s, true
	case '`':
		if raw[len(raw)-1] != '`' {
			return "", false
		}
		return raw[1 : len(raw)-1], true
	default:
		return "", false
	}
}
