package pattern

import (
	"fmt"
	"regexp"

	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// NFA over pattern AST. Alphabet = source tokens. ε for concat / empty rest /
// star. Capture open/close/append are side effects on edges.

type nfa struct {
	start, accept int
	states        []nfaState
}

type nfaState struct {
	eps   []epsEdge
	edges []nfaEdge
}

type epsEdge struct {
	to  int
	ops []capOp
}

type nfaEdge struct {
	to   int
	pred nfaPred
	ops  []capOp
}

type nfaPred struct {
	kind         predKind
	text         string
	ref          string
	regex        *regexp.Regexp
	equals       string
	captureGroup int
}

type predKind int

const (
	predAny           predKind = iota
	predAnyExceptFunc          // like any, but not the "func" keyword (multi stays in one function)
	predLit
	predToken
	predRef
	predRegex
	predEquals
	predCaptureAny
)

type capOpKind int

const (
	capOpen capOpKind = iota
	capClose
	capAppend
	capBindToken
	capBindRef
	capBindRegex
	capAppendToken
	capAppendRef
	capAppendRegex
)

type capOp struct {
	kind         capOpKind
	name         string
	captureGroup int
}

type nfaFrag struct{ start, accept int }

type nfaCompiler struct {
	n      nfa
	append bool // named binds append (inside multi body)
}

func (c *nfaCompiler) st() int {
	c.n.states = append(c.n.states, nfaState{})
	return len(c.n.states) - 1
}

func (c *nfaCompiler) eps(from, to int, ops ...capOp) {
	c.n.states[from].eps = append(c.n.states[from].eps, epsEdge{to: to, ops: ops})
}

func (c *nfaCompiler) edge(from int, e nfaEdge) {
	c.n.states[from].edges = append(c.n.states[from].edges, e)
}

func compilePattern(pat Node) (*nfa, error) {
	c := &nfaCompiler{}
	seq := flattenPattern(pat)
	f, err := c.compileSeq(seq)
	if err != nil {
		return nil, err
	}
	c.n.start = f.start
	c.n.accept = f.accept
	return &c.n, nil
}

func (c *nfaCompiler) compileSeq(seq []Node) (nfaFrag, error) {
	if len(seq) == 0 {
		s := c.st()
		return nfaFrag{s, s}, nil
	}
	frags := make([]nfaFrag, 0, len(seq))
	for _, step := range seq {
		f, err := c.compileNode(step)
		if err != nil {
			return nfaFrag{}, err
		}
		frags = append(frags, f)
	}
	for i := 0; i < len(frags)-1; i++ {
		c.eps(frags[i].accept, frags[i+1].start)
	}
	return nfaFrag{frags[0].start, frags[len(frags)-1].accept}, nil
}

func (c *nfaCompiler) compileNode(n Node) (nfaFrag, error) {
	if n.Multi {
		return c.compileStar(n)
	}
	switch n.Kind {
	case "rest":
		return c.compileRest(n.As), nil
	case "lit":
		return c.compileConsume(nfaPred{kind: predLit, text: n.Text}, bindOps(n.As, c.append, false, 0)), nil
	case "token", "type_token":
		return c.compileConsume(nfaPred{kind: predToken, text: n.Text}, bindOps(n.As, c.append, false, 0)), nil
	case "ref":
		return c.compileConsume(nfaPred{kind: predRef, ref: n.Ref}, bindOpsRef(n.As, c.append)), nil
	case "capture":
		return c.compileConsume(nfaPred{kind: predCaptureAny}, bindOps(n.As, c.append, false, 0)), nil
	case "string":
		p, ops, err := stringPredOps(n, c.append)
		if err != nil {
			return nfaFrag{}, err
		}
		return c.compileConsume(p, ops), nil
	case "group":
		return c.compileGroup(n)
	case "seq":
		return c.compileSeq(n.Args)
	case "call":
		return c.compileSeq(flattenPattern(n))
	default:
		s := c.st()
		return nfaFrag{s, s}, nil
	}
}

func bindOps(as string, appendMode, _ bool, _ int) []capOp {
	if as == "" || as == "_" || as == "ROOT" {
		return nil
	}
	if appendMode {
		return []capOp{{kind: capAppendToken, name: as}}
	}
	return []capOp{{kind: capBindToken, name: as}}
}

func bindOpsRef(as string, appendMode bool) []capOp {
	if as == "" || as == "_" || as == "ROOT" {
		return nil
	}
	if appendMode {
		return []capOp{{kind: capAppendRef, name: as}}
	}
	return []capOp{{kind: capBindRef, name: as}}
}

func stringPredOps(n Node, appendMode bool) (nfaPred, []capOp, error) {
	p := nfaPred{captureGroup: n.CaptureGroup}
	if n.Regex != "" {
		re, err := regexp.Compile(n.Regex)
		if err != nil {
			return p, nil, fmt.Errorf("pattern regex: %w", err)
		}
		p.kind, p.regex = predRegex, re
	} else if n.Equals != "" {
		p.kind, p.equals = predEquals, n.Equals
	} else {
		p.kind = predCaptureAny
	}
	as := n.As
	if as == "" || as == "_" || as == "ROOT" {
		return p, nil, nil
	}
	if p.kind == predRegex {
		if appendMode {
			return p, []capOp{{kind: capAppendRegex, name: as, captureGroup: n.CaptureGroup}}, nil
		}
		return p, []capOp{{kind: capBindRegex, name: as, captureGroup: n.CaptureGroup}}, nil
	}
	return p, bindOps(as, appendMode, false, 0), nil
}

func (c *nfaCompiler) compileConsume(p nfaPred, ops []capOp) nfaFrag {
	s, a := c.st(), c.st()
	c.edge(s, nfaEdge{to: a, pred: p, ops: ops})
	return nfaFrag{s, a}
}

func (c *nfaCompiler) compileRest(as string) nfaFrag {
	// loop with optional empty: s -ε-> mid -any*-> mid -ε-> a ; s -ε-> a
	s, mid, a := c.st(), c.st(), c.st()
	c.eps(s, mid)
	c.eps(s, a)
	if as != "" && as != "_" {
		// open sticky on first any; close when exiting mid -> a
		c.edge(mid, nfaEdge{
			to:   mid,
			pred: nfaPred{kind: predAny},
			ops:  []capOp{{kind: capOpen, name: as}},
		})
		c.eps(mid, a, capOp{kind: capClose, name: as})
	} else {
		c.edge(mid, nfaEdge{to: mid, pred: nfaPred{kind: predAny}})
		c.eps(mid, a)
	}
	return nfaFrag{s, a}
}

func (c *nfaCompiler) compileGroup(n Node) (nfaFrag, error) {
	if n.Callee == nil {
		s := c.st()
		return nfaFrag{s, s}, nil
	}
	inner := flattenPattern(*n.Callee)
	body, err := c.compileSeq(inner)
	if err != nil {
		return nfaFrag{}, err
	}
	if n.As == "" || n.As == "_" {
		return body, nil
	}
	s, a := c.st(), c.st()
	closeK := capClose
	if c.append {
		closeK = capAppend
	}
	c.eps(s, body.start, capOp{kind: capOpen, name: n.As})
	c.eps(body.accept, a, capOp{kind: closeK, name: n.As})
	return nfaFrag{s, a}, nil
}

// compileStar implements multi (*): within a gap, match R any number of times
// with arbitrary tokens between occurrences (not only consecutive R).
//
//	mid --any--> mid
//	mid --R/append--> mid
//	mid --ε--> accept
//
// Nested captures inside R use append mode so $i:{ $c:{t.Context} $$$_ }*
// accumulates every $c.
func (c *nfaCompiler) compileStar(n Node) (nfaFrag, error) {
	old := c.append
	c.append = true
	defer func() { c.append = old }()

	bodyNode := n
	bodyNode.Multi = false
	body, err := c.compileNode(bodyNode)
	if err != nil {
		return nfaFrag{}, err
	}
	s, mid, a := c.st(), c.st(), c.st()
	c.eps(s, mid)
	c.eps(mid, a) // zero or done
	// Skip non-R tokens (not across "func"); can also skip R — max score prefers matching R.
	c.edge(mid, nfaEdge{to: mid, pred: nfaPred{kind: predAnyExceptFunc}})
	// Match one R occurrence, then return to mid.
	c.eps(mid, body.start)
	c.eps(body.accept, mid)
	return nfaFrag{s, a}, nil
}

// --- runtime -----------------------------------------------------------------

type nfaThread struct {
	state int
	pos   int // next token index
	open  map[string]int
	caps  map[string][]Span
}

func cloneThread(t nfaThread) nfaThread {
	o := nfaThread{
		state: t.state,
		pos:   t.pos,
		open:  make(map[string]int, len(t.open)),
		caps:  make(map[string][]Span, len(t.caps)),
	}
	for k, v := range t.open {
		o.open[k] = v
	}
	for k, v := range t.caps {
		o.caps[k] = append([]Span(nil), v...)
	}
	return o
}

func applyOps(t *nfaThread, ops []capOp, tokens []tok, source []byte, lastTok int) {
	// lastTok = index of token just consumed, or -1 for pure ε
	for _, op := range ops {
		if op.name == "" {
			continue
		}
		switch op.kind {
		case capOpen:
			if _, ok := t.open[op.name]; !ok {
				if lastTok >= 0 {
					t.open[op.name] = lastTok
				} else {
					t.open[op.name] = t.pos // next token
				}
			}
		case capClose:
			start, ok := t.open[op.name]
			delete(t.open, op.name)
			if !ok {
				continue
			}
			end := t.pos - 1
			if end < start {
				end = start
			}
			if start >= 0 && start < len(tokens) && end >= 0 && end < len(tokens) {
				t.caps[op.name] = []Span{{
					StartByte: tokens[start].StartByte,
					EndByte:   tokens[end].EndByte,
				}}
			}
		case capAppend:
			start, ok := t.open[op.name]
			delete(t.open, op.name)
			if !ok {
				continue
			}
			end := t.pos - 1
			if end < start {
				end = start
			}
			if start >= 0 && start < len(tokens) && end >= 0 && end < len(tokens) {
				t.caps[op.name] = append(t.caps[op.name], Span{
					StartByte: tokens[start].StartByte,
					EndByte:   tokens[end].EndByte,
				})
			}
		case capBindToken, capAppendToken:
			if lastTok < 0 || lastTok >= len(tokens) {
				continue
			}
			sp := tokens[lastTok].Span
			if op.kind == capAppendToken {
				t.caps[op.name] = append(t.caps[op.name], sp)
			} else {
				t.caps[op.name] = []Span{sp}
			}
		case capBindRef, capAppendRef:
			if lastTok < 0 || lastTok >= len(tokens) {
				continue
			}
			sp := selectorSpan(tokens, lastTok, source)
			if op.kind == capAppendRef {
				t.caps[op.name] = append(t.caps[op.name], sp)
			} else {
				t.caps[op.name] = []Span{sp}
			}
		case capBindRegex, capAppendRegex:
			if lastTok < 0 || lastTok >= len(tokens) {
				continue
			}
			sp := bindRegexSpan(tokens[lastTok], source, op.captureGroup)
			// named groups filled in pred match path via side channel — keep simple
			if op.kind == capAppendRegex {
				t.caps[op.name] = append(t.caps[op.name], sp)
			} else {
				t.caps[op.name] = []Span{sp}
			}
		}
	}
}

func bindRegexSpan(t tok, source []byte, captureGroup int) Span {
	content, srcOf, closeOff, _ := tokenContentMap(source, t)
	if captureGroup <= 0 {
		return t.Span
	}
	// need re — span is full token if we can't re-run; caller should pass idx
	_ = content
	_ = srcOf
	_ = closeOff
	return t.Span
}

func predMatch(p nfaPred, tokens []tok, pos int, source []byte) (ok bool, named map[string]Span) {
	if pos < 0 || pos >= len(tokens) {
		return false, nil
	}
	t := tokens[pos]
	switch p.kind {
	case predAny, predCaptureAny:
		return true, nil
	case predAnyExceptFunc:
		return !t.Eq(source, "func"), nil
	case predLit, predToken:
		return t.Eq(source, p.text), nil
	case predRef:
		return t.target == p.ref, nil
	case predEquals:
		content, _, _, _ := tokenContentMap(source, t)
		return content == p.equals, nil
	case predRegex:
		content, srcOf, closeOff, _ := tokenContentMap(source, t)
		if p.equals != "" && content != p.equals {
			return false, nil
		}
		if p.regex == nil {
			return true, nil
		}
		idx := p.regex.FindStringSubmatchIndex(content)
		if idx == nil {
			return false, nil
		}
		named = map[string]Span{}
		names := p.regex.SubexpNames()
		for i, name := range names {
			if i == 0 || name == "" || 2*i+1 >= len(idx) || idx[2*i] < 0 {
				continue
			}
			sp, ok := contentSpanToSource(t.StartByte, srcOf, closeOff, idx[2*i], idx[2*i+1])
			if ok {
				named[name] = sp
			}
		}
		if p.captureGroup > 0 && 2*p.captureGroup+1 < len(idx) && idx[2*p.captureGroup] >= 0 {
			if sp, ok := contentSpanToSource(t.StartByte, srcOf, closeOff, idx[2*p.captureGroup], idx[2*p.captureGroup+1]); ok {
				named["\x00group"] = sp // internal
			}
		}
		return true, named
	default:
		return false, nil
	}
}

// matchNFA runs the NFA from token startIdx. Prefers accepting threads with more
// total capture sites, then shorter end position (stable for multi + rest).
func matchNFA(n *nfa, tokens []tok, startIdx int, source []byte, root *grammar.Node) (Match, int, bool, error) {
	if n == nil || len(n.states) == 0 {
		return Match{}, startIdx, false, nil
	}
	type key struct{ state, pos int }
	seen := map[key]int{} // best capture count seen

	var best *nfaThread
	bestScore, bestEnd := -1, 1<<30

	seed := nfaThread{
		state: n.start,
		pos:   startIdx,
		open:  map[string]int{},
		caps:  map[string][]Span{},
	}
	queue := []nfaThread{seed}

	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]

		// ε-closure with ops
		stack := []nfaThread{t}
		localSeen := map[int]bool{}
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if localSeen[cur.state] {
				continue
			}
			localSeen[cur.state] = true

			if cur.state == n.accept {
				score := captureScore(cur.caps)
				end := cur.pos
				if score > bestScore || (score == bestScore && end < bestEnd) {
					bestScore, bestEnd = score, end
					cp := cloneThread(cur)
					best = &cp
				}
			}
			for _, e := range n.states[cur.state].eps {
				nt := cloneThread(cur)
				nt.state = e.to
				applyOps(&nt, e.ops, tokens, source, -1)
				stack = append(stack, nt)
			}
		}

		// after ε-closure from t, try consuming (re-close from each ε-reachable)
		// Recompute ε-reachable set properly as list of threads
		ethreads := epsilonClose(n, t, tokens, source)
		for _, cur := range ethreads {
			if cur.state == n.accept {
				score := captureScore(cur.caps)
				end := cur.pos
				if score > bestScore || (score == bestScore && end < bestEnd) {
					bestScore, bestEnd = score, end
					cp := cloneThread(cur)
					best = &cp
				}
			}
			if cur.pos >= len(tokens) {
				continue
			}
			for _, e := range n.states[cur.state].edges {
				ok, named := predMatch(e.pred, tokens, cur.pos, source)
				if !ok {
					continue
				}
				nt := cloneThread(cur)
				last := nt.pos
				nt.pos++
				nt.state = e.to
				applyOps(&nt, e.ops, tokens, source, last)
				// regex named groups
				for name, sp := range named {
					if name == "\x00group" {
						// override last bind for captureGroup on primary name in ops
						for _, op := range e.ops {
							if op.kind == capBindRegex || op.kind == capAppendRegex {
								if op.captureGroup > 0 {
									if op.kind == capAppendRegex && len(nt.caps[op.name]) > 0 {
										nt.caps[op.name][len(nt.caps[op.name])-1] = sp
									} else {
										nt.caps[op.name] = []Span{sp}
									}
								}
							}
						}
						continue
					}
					nt.caps[name] = []Span{sp}
				}
				k := key{nt.state, nt.pos}
				sc := captureScore(nt.caps)
				if prev, ok := seen[k]; ok && prev >= sc {
					continue
				}
				seen[k] = sc
				queue = append(queue, nt)
			}
		}
	}

	if best == nil {
		return Match{}, startIdx, false, nil
	}
	// Match span: from startIdx token through last consumed.
	ms, me := tokens[startIdx].StartByte, tokens[startIdx].EndByte
	if best.pos > startIdx {
		me = tokens[best.pos-1].EndByte
	}
	_ = root
	return Match{
		Span:     Span{StartByte: ms, EndByte: me},
		Captures: best.caps,
	}, best.pos, true, nil
}

func epsilonClose(n *nfa, t nfaThread, tokens []tok, source []byte) []nfaThread {
	var out []nfaThread
	stack := []nfaThread{t}
	seen := map[int]bool{}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur.state] {
			// still allow if different caps? skip for speed
			continue
		}
		seen[cur.state] = true
		out = append(out, cur)
		for _, e := range n.states[cur.state].eps {
			nt := cloneThread(cur)
			nt.state = e.to
			applyOps(&nt, e.ops, tokens, source, -1)
			stack = append(stack, nt)
		}
	}
	return out
}

func captureScore(caps map[string][]Span) int {
	n := 0
	for _, v := range caps {
		n += len(v)
	}
	return n
}

// matchPattern runs the compiled NFA from each start position (same as old MatchFile loop).
func matchPattern(pat Node, tokens []tok, source []byte, root *grammar.Node) ([]Match, error) {
	n, err := compilePattern(pat)
	if err != nil {
		return nil, err
	}
	var out []Match
	for i := 0; i < len(tokens); {
		m, next, ok, err := matchNFA(n, tokens, i, source, root)
		if err != nil {
			return nil, err
		}
		if !ok {
			i++
			continue
		}
		out = append(out, m)
		if next <= i {
			i++
		} else {
			i = next
		}
	}
	return out, nil
}
