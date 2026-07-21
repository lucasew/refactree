package graphql

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// StreamEvent is one progressive graph update (used by the WS explore session).
// Primary payload is edges; nodes (except focus) are hydrated on demand via GraphQL.
type StreamEvent struct {
	Type string `json:"type"` // focus | edge | done | error

	Node *GraphNode `json:"node,omitempty"`
	Edge *GraphEdge `json:"edge,omitempty"`

	Incomplete *bool  `json:"incomplete,omitempty"`
	Message    string `json:"message,omitempty"`
}

// StreamEmitter receives stream events. Return false to cancel.
type StreamEmitter func(StreamEvent) bool

func boolPtr(b bool) *bool { return &b }

func edgeKey(e *GraphEdge) string {
	if e == nil {
		return ""
	}
	return string(e.Kind) + "\x00" + e.From + "\x00" + e.To
}

// LookupNode returns cheap node metadata from a reference id (no Seed/Materialize).
func LookupNode(root, id string) *GraphNode {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return decorateNode(root, graphNodeForRef(root, ingest.ParseReference(id)))
}

// StreamNeighborhood is a one-shot helper (ephemeral corpus). Prefer
// SessionCorpus.StreamVisit for multi-visit sessions.
func StreamNeighborhood(ctx context.Context, root, ref string, emit StreamEmitter) error {
	c := NewSessionCorpus(root)
	return c.StreamVisit(ctx, ref, emit)
}

// StreamProjectGraph is a one-shot helper. Prefer SessionCorpus.StreamProject.
func StreamProjectGraph(ctx context.Context, root string, emit StreamEmitter) error {
	c := NewSessionCorpus(root)
	return c.StreamProject(ctx, emit)
}

// StreamVisit absorbs Seed extracts for ref (skipping paths already in the
// corpus), materializes once if the corpus grew, then emits edges.
// Emits: focus → edge* → done.
func (c *SessionCorpus) StreamVisit(ctx context.Context, ref string, emit StreamEmitter) error {
	if c == nil || emit == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	parsed := ingest.CanonicalizeReference(c.root, ingest.ParseReference(ref))
	focusID := parsed.String()
	focus := decorateNode(c.root, graphNodeForRef(c.root, parsed))
	if !emit(StreamEvent{Type: "focus", Node: focus, Incomplete: boolPtr(true)}) {
		return context.Canceled
	}

	if err := c.absorbForRef(parsed); err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	result := c.Result()
	edges := visitEdges(c.root, parsed, focusID, result)

	seen := map[string]bool{}
	for _, e := range edges {
		if err := ctx.Err(); err != nil {
			return err
		}
		if e == nil {
			continue
		}
		k := edgeKey(e)
		if seen[k] {
			continue
		}
		seen[k] = true
		if !emit(StreamEvent{Type: "edge", Edge: e, Incomplete: boolPtr(true)}) {
			return context.Canceled
		}
	}

	inc := true
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

// StreamProject absorbs the full project dir (new files only), materializes
// once if needed, emits IMPORT edges.
func (c *SessionCorpus) StreamProject(ctx context.Context, emit StreamEmitter) error {
	if c == nil || emit == nil {
		return nil
	}
	focus := &GraphNode{
		ID: "path:./", Kind: NodeKindModule, Label: filepath.Base(c.root),
		External: false, Expandable: false,
	}
	if focus.Label == "" || focus.Label == "." {
		focus.Label = "project"
	}
	if !emit(StreamEvent{Type: "focus", Node: focus, Incomplete: boolPtr(true)}) {
		return context.Canceled
	}

	if err := c.AbsorbDir("", true); err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	result := c.Result()
	seen := map[string]bool{}
	for _, a := range result.Aliases {
		if err := ctx.Err(); err != nil {
			return err
		}
		fromID := projectScopeID(c.root, ingest.ParseReference(a.Reference))
		toID := projectScopeID(c.root, ingest.ParseReference(a.Target))
		if fromID == "" || toID == "" || fromID == toID {
			continue
		}
		e := &GraphEdge{From: fromID, To: toID, Kind: EdgeKindImports}
		k := edgeKey(e)
		if seen[k] {
			continue
		}
		seen[k] = true
		if !emit(StreamEvent{Type: "edge", Edge: e, Incomplete: boolPtr(true)}) {
			return context.Canceled
		}
	}

	inc := true
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

func (c *SessionCorpus) absorbForRef(parsed ingest.Reference) error {
	// Provider scopes: materialize module dir into corpus.
	if parsed.Provider != "" && parsed.Provider != "path" {
		scope, ok, err := ingest.NewResolver(c.root).ResolveScopeTarget(ingest.Reference{
			Provider: parsed.Provider,
			Path:     parsed.Path,
		})
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		// Dir walk under scope — AbsorbDir with Dir=scope.Dir
		return c.AbsorbDir(scope.Dir, false)
	}

	abs, isDir, err := resolvePath(c.root, parsed)
	if err != nil {
		return err
	}
	if isDir {
		return c.AbsorbDir(abs, false)
	}
	// Seed neighborhood from this file (BFS); already-cached paths are not re-stored.
	return c.AbsorbSeed(abs)
}

// visitEdges builds the edge list for a focus ref from a closed Result.
func visitEdges(root string, focusRef ingest.Reference, focusID string, result *ingest.Result) []*GraphEdge {
	if result == nil {
		return nil
	}
	var edges []*GraphEdge
	nodes := map[string]*GraphNode{} // API filler for addRelationEdges

	if focusRef.Symbol != "" {
		addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
			if from == focusID || to == focusID {
				return true
			}
			return sameScope(focusRef, ingest.ParseReference(from)) ||
				sameScope(focusRef, ingest.ParseReference(to))
		})
		addImportEdges(root, result, nodes, &edges, focusRef)
		return dedupeEdges(edges)
	}

	// File / module focus.
	abs, isDir, err := resolvePath(root, focusRef)
	if err == nil && !isDir {
		focusPath := normalizePath(focusRef.Path)
		local := map[string]bool{}
		for _, ent := range result.Entities {
			er := ingest.ParseReference(ent.Reference)
			if normalizePath(er.Path) == focusPath || filepath.Base(normalizePath(er.Path)) == filepath.Base(focusPath) {
				local[ingest.CanonicalizeReference(root, er).String()] = true
			}
		}
		addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
			return local[from] || sameScope(focusRef, ingest.ParseReference(from))
		})
		return dedupeEdges(edges)
	}

	// Module/dir: import edges among scopes present in result.
	for _, a := range result.Aliases {
		fromID := projectScopeID(root, ingest.ParseReference(a.Reference))
		toID := projectScopeID(root, ingest.ParseReference(a.Target))
		if fromID == "" || toID == "" || fromID == toID {
			continue
		}
		edges = append(edges, &GraphEdge{From: fromID, To: toID, Kind: EdgeKindImports})
	}
	_ = abs
	_ = isDir
	return dedupeEdges(edges)
}

// EncodeStreamEventJSON is a small helper for tests/handlers.
func EncodeStreamEventJSON(ev StreamEvent) ([]byte, error) {
	return json.Marshal(ev)
}
