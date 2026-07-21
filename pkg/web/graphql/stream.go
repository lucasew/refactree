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

// StreamNeighborhood is a one-shot helper (ephemeral corpus).
func StreamNeighborhood(ctx context.Context, root, ref string, emit StreamEmitter) error {
	return NewSessionCorpus(root).StreamVisit(ctx, ref, emit)
}

// StreamProjectGraph is a one-shot helper.
func StreamProjectGraph(ctx context.Context, root string, emit StreamEmitter) error {
	return NewSessionCorpus(root).StreamProject(ctx, emit)
}

// StreamVisit explores ref into the session corpus and streams edges as files
// are first absorbed (single-file resolve), then emits remaining cross-file
// edges after one full Materialize of the corpus.
//
// Emits: focus → edge* (live) → edge* (cross-file) → done.
// Already-cached paths are not re-absorbed; no second full explore of known files.
func (c *SessionCorpus) StreamVisit(ctx context.Context, ref string, emit StreamEmitter) error {
	if c == nil || emit == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	parsed := ingest.CanonicalizeReference(c.root, ingest.ParseReference(ref))
	focus := decorateNode(c.root, graphNodeForRef(c.root, parsed))
	focusID := focus.ID
	// Align keep-filters with module-normalized focus path.
	parsed = ingest.ParseReference(focusID)
	if !emit(StreamEvent{Type: "focus", Node: focus, Incomplete: boolPtr(true)}) {
		return context.Canceled
	}

	seen := map[string]bool{}
	emitEdge := func(e *GraphEdge) bool {
		if e == nil || e.From == "" || e.To == "" || e.From == e.To {
			return true
		}
		k := edgeKey(e)
		if seen[k] {
			return true
		}
		seen[k] = true
		return emit(StreamEvent{Type: "edge", Edge: e, Incomplete: boolPtr(true)})
	}

	// Progressive: each newly absorbed file → local edges immediately.
	onNew := func(fe *ingest.FileExtract) bool {
		if ctx.Err() != nil {
			return false
		}
		local := c.MaterializeOne(fe)
		for _, e := range visitEdges(c.root, parsed, focusID, local) {
			if !emitEdge(e) {
				return false
			}
		}
		return true
	}

	if err := c.absorbForRef(parsed, onNew); err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// One full resolve for cross-file edges not visible in single-file passes.
	full := c.Result()
	for _, e := range visitEdges(c.root, parsed, focusID, full) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !emitEdge(e) {
			return context.Canceled
		}
	}

	inc := true
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

// StreamProject streams IMPORT edges as new project files are absorbed,
// then one full Materialize pass for any remaining aliases.
func (c *SessionCorpus) StreamProject(ctx context.Context, emit StreamEmitter) error {
	if c == nil || emit == nil {
		return nil
	}
	focus := &GraphNode{
		ID: "path:./", Kind: NodeKindModule, Label: filepath.Base(c.root),
		External: false, Expandable: false, Language: "",
	}
	if focus.Label == "" || focus.Label == "." {
		focus.Label = "project"
	}
	if !emit(StreamEvent{Type: "focus", Node: focus, Incomplete: boolPtr(true)}) {
		return context.Canceled
	}

	seen := map[string]bool{}
	emitEdge := func(e *GraphEdge) bool {
		if e == nil || e.From == "" || e.To == "" || e.From == e.To {
			return true
		}
		k := edgeKey(e)
		if seen[k] {
			return true
		}
		seen[k] = true
		return emit(StreamEvent{Type: "edge", Edge: e, Incomplete: boolPtr(true)})
	}

	emitImportAliases := func(result *ingest.Result) bool {
		if result == nil {
			return true
		}
		for _, a := range result.Aliases {
			fromID := projectScopeID(c.root, ingest.ParseReference(a.Reference))
			toID := projectScopeID(c.root, ingest.ParseReference(a.Target))
			if fromID == "" || toID == "" || fromID == toID {
				continue
			}
			if !emitEdge(&GraphEdge{From: fromID, To: toID, Kind: EdgeKindImports}) {
				return false
			}
		}
		return true
	}

	if err := c.AbsorbDir("", true, func(fe *ingest.FileExtract) bool {
		if ctx.Err() != nil {
			return false
		}
		local := c.MaterializeOne(fe)
		return emitImportAliases(local)
	}); err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if !emitImportAliases(c.Result()) {
		return context.Canceled
	}

	inc := true
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

func (c *SessionCorpus) absorbForRef(parsed ingest.Reference, onNew func(*ingest.FileExtract) bool) error {
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
		return c.AbsorbDir(scope.Dir, false, onNew)
	}

	abs, isDir, err := resolvePath(c.root, parsed)
	if err != nil {
		return err
	}
	if isDir {
		return c.AbsorbDir(abs, false, onNew)
	}
	return c.AbsorbSeed(abs, onNew)
}

// visitEdges builds the edge list for a focus ref from a closed Result.
func visitEdges(root string, focusRef ingest.Reference, focusID string, result *ingest.Result) []*GraphEdge {
	if result == nil {
		return nil
	}
	var edges []*GraphEdge
	nodes := map[string]*GraphNode{}

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
