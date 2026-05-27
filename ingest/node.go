package ingest

import "github.com/lucasew/ccgo-tree-sitter/grammar"

// nodeText returns the source text covered by a node.
func nodeText(n *grammar.Node, source []byte) string {
	s, e := n.StartByte(), n.EndByte()
	if s <= e && int(e) <= len(source) {
		return string(source[s:e])
	}
	return ""
}

// childByField returns the first child whose field name matches, or nil.
func childByField(n *grammar.Node, field string) *grammar.Node {
	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.FieldNameForChild(i) == field {
			c := n.Child(i)
			if !c.IsNull() {
				return c
			}
		}
	}
	return nil
}

// childByType returns the first child whose node type matches, or nil.
func childByType(n *grammar.Node, typ string) *grammar.Node {
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Type() == typ {
			return c
		}
	}
	return nil
}
