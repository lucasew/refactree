package graphql

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// StreamEvent is one progressive graph update (SSE).
type StreamEvent struct {
	Type string `json:"type"` // focus | node | edge | done | error

	Node *GraphNode `json:"node,omitempty"`
	Edge *GraphEdge `json:"edge,omitempty"`

	Incomplete *bool  `json:"incomplete,omitempty"`
	Message    string `json:"message,omitempty"`
}

// StreamEmitter receives stream events. Return false to cancel.
type StreamEmitter func(StreamEvent) bool

func boolPtr(b bool) *bool { return &b }

// StreamNeighborhood progressively emits focus, discovery nodes, then edges.
func StreamNeighborhood(ctx context.Context, root, ref string, emit StreamEmitter) error {
	if emit == nil {
		return nil
	}
	parsed := ingest.CanonicalizeReference(root, ingest.ParseReference(ref))
	focus := decorateNode(root, graphNodeForRef(root, parsed))
	if !emit(StreamEvent{Type: "focus", Node: focus, Incomplete: boolPtr(true)}) {
		return context.Canceled
	}

	// Prefer full BuildNeighborhood (correct edges), but emit nodes/edges incrementally after.
	// For path seeds, also emit file atoms as Seed BFS discovers files (pre-edge preview).
	if parsed.Provider == "" || parsed.Provider == "path" {
		if err := streamSeedPreview(ctx, root, parsed, emit); err != nil && ctx.Err() != nil {
			return err
		}
	}

	nb, err := BuildNeighborhood(root, ref)
	if err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	return streamNeighborhoodResult(ctx, nb, emit, true /* skip re-sending focus */)
}

// StreamProjectGraph progressively emits the project import map.
func StreamProjectGraph(ctx context.Context, root string, emit StreamEmitter) error {
	if emit == nil {
		return nil
	}
	// Focus shell immediately.
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

	nb, err := BuildProjectGraph(root)
	if err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	return streamNeighborhoodResult(ctx, nb, emit, true)
}

// streamSeedPreview walks Seed extracts and emits entity nodes as files parse.
func streamSeedPreview(ctx context.Context, root string, parsed ingest.Reference, emit StreamEmitter) error {
	abs, isDir, err := resolvePath(root, parsed)
	if err != nil || isDir {
		return nil
	}
	_ = ingest.WalkExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractSeed,
		Root:  root,
		Paths: []string{abs},
	}, func(fe *ingest.FileExtract) bool {
		if ctx.Err() != nil {
			return false
		}
		if fe == nil {
			return true
		}
		fileRef := ingest.FileRef("./" + strings.TrimPrefix(filepath.ToSlash(fe.Path), "./"))
		fn := decorateNode(root, graphNodeForRef(root, ingest.ParseReference(fileRef)))
		if !emit(StreamEvent{Type: "node", Node: fn, Incomplete: boolPtr(true)}) {
			return false
		}
		for _, ent := range fe.Entities {
			er := ingest.ParseReference(ingest.SymbolRef("./"+strings.TrimPrefix(filepath.ToSlash(fe.Path), "./"), ent.Name))
			n := decorateNode(root, graphNodeForRef(root, er))
			if !emit(StreamEvent{Type: "node", Node: n, Incomplete: boolPtr(true)}) {
				return false
			}
		}
		return true
	})
	return ctx.Err()
}

func streamNeighborhoodResult(ctx context.Context, nb *Neighborhood, emit StreamEmitter, skipFocus bool) error {
	if nb == nil {
		inc := true
		if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
			return context.Canceled
		}
		return nil
	}

	if !skipFocus && nb.Focus != nil {
		if !emit(StreamEvent{Type: "focus", Node: nb.Focus, Incomplete: boolPtr(true)}) {
			return context.Canceled
		}
	}

	for _, n := range nb.Nodes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if n == nil {
			continue
		}
		if skipFocus && nb.Focus != nil && n.ID == nb.Focus.ID {
			continue
		}
		if !emit(StreamEvent{Type: "node", Node: n, Incomplete: boolPtr(true)}) {
			return context.Canceled
		}
	}

	for _, e := range nb.Edges {
		if err := ctx.Err(); err != nil {
			return err
		}
		if e == nil {
			continue
		}
		if !emit(StreamEvent{Type: "edge", Edge: e, Incomplete: boolPtr(true)}) {
			return context.Canceled
		}
	}

	inc := nb.Incomplete
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

// EncodeStreamEventJSON is a small helper for tests/handlers.
func EncodeStreamEventJSON(ev StreamEvent) ([]byte, error) {
	return json.Marshal(ev)
}
