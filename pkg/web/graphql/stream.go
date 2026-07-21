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

// StreamNeighborhood streams edges as Seed discovery grows.
// Emits: focus (entry node only) → edge* → done.
func StreamNeighborhood(ctx context.Context, root, ref string, emit StreamEmitter) error {
	if emit == nil {
		return nil
	}
	parsed := ingest.CanonicalizeReference(root, ingest.ParseReference(ref))
	focusID := parsed.String()
	focus := decorateNode(root, graphNodeForRef(root, parsed))
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

	if parsed.Provider == "" || parsed.Provider == "path" {
		abs, isDir, err := resolvePath(root, parsed)
		if err == nil && !isDir {
			if err := streamPathSeedEdges(ctx, root, parsed, focusID, abs, emitEdge); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				emit(StreamEvent{Type: "error", Message: err.Error()})
				return err
			}
			inc := true
			if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
				return context.Canceled
			}
			return nil
		}
	}

	// Provider / dir / module: build once, stream edges only.
	nb, err := BuildNeighborhood(root, ref)
	if err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	for _, e := range nb.Edges {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !emitEdge(e) {
			return context.Canceled
		}
	}
	inc := true
	if nb != nil {
		inc = nb.Incomplete
	}
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

func streamPathSeedEdges(
	ctx context.Context,
	root string,
	focusRef ingest.Reference,
	focusID string,
	seedAbs string,
	emitEdge func(*GraphEdge) bool,
) error {
	focusPath := normalizePath(focusRef.Path)
	var extracts []*ingest.FileExtract

	return ingest.WalkExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractSeed,
		Root:  root,
		Paths: []string{seedAbs},
	}, func(fe *ingest.FileExtract) bool {
		if ctx.Err() != nil {
			return false
		}
		if fe == nil {
			return true
		}
		cp := *fe
		extracts = append(extracts, &cp)

		result := ingest.Materialize(root, extracts, ingest.MaterializeOptions{ExpandImports: false})
		var edges []*GraphEdge
		nodes := map[string]*GraphNode{} // unused except addRelationEdges API

		if focusRef.Symbol != "" {
			addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
				if from == focusID || to == focusID {
					return true
				}
				return sameScope(focusRef, ingest.ParseReference(from)) ||
					sameScope(focusRef, ingest.ParseReference(to))
			})
			addImportEdges(root, result, nodes, &edges, focusRef)
		} else {
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
		}

		for _, e := range dedupeEdges(edges) {
			if !emitEdge(e) {
				return false
			}
		}
		return true
	})
}

// StreamProjectGraph streams IMPORT edges as the project dir is walked.
// Emits: focus → edge* → done. Nodes are on-demand.
func StreamProjectGraph(ctx context.Context, root string, emit StreamEmitter) error {
	if emit == nil {
		return nil
	}
	focus := &GraphNode{
		ID: "path:./", Kind: NodeKindModule, Label: filepath.Base(root),
		External: false, Expandable: false,
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

	var extracts []*ingest.FileExtract
	err := ingest.WalkExtracts(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      root,
		Recursive: true,
	}, func(fe *ingest.FileExtract) bool {
		if ctx.Err() != nil {
			return false
		}
		if fe == nil {
			return true
		}
		cp := *fe
		extracts = append(extracts, &cp)

		result := ingest.Materialize(root, extracts, ingest.MaterializeOptions{ExpandImports: false})
		for _, a := range result.Aliases {
			fromID := projectScopeID(root, ingest.ParseReference(a.Reference))
			toID := projectScopeID(root, ingest.ParseReference(a.Target))
			if fromID == "" || toID == "" || fromID == toID {
				continue
			}
			if !emitEdge(&GraphEdge{From: fromID, To: toID, Kind: EdgeKindImports}) {
				return false
			}
		}
		return true
	})
	if err != nil && ctx.Err() == nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}

	inc := true
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

// EncodeStreamEventJSON is a small helper for tests/handlers.
func EncodeStreamEventJSON(ev StreamEvent) ([]byte, error) {
	return json.Marshal(ev)
}
