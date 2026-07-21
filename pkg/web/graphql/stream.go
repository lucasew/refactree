package graphql

import (
	"context"
	"encoding/json"
	"os"
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

// StreamVisit explores ref into the session corpus and streams edges.
//
// When the focus is a module (no ::symbol), every direct source file of that
// module is visited (Seed from each file) so package members and their
// relations are absorbed and streamed—not only a bare dir listing.
//
// Emits: focus → edge* (as files are first absorbed / re-visited) → edge* (full) → done.
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

	// Progressive edges for a single extract under the current focus filter.
	streamLocal := func(fe *ingest.FileExtract) bool {
		if ctx.Err() != nil || fe == nil {
			return ctx.Err() == nil
		}
		local := c.MaterializeOne(fe)
		for _, e := range visitEdges(c.root, parsed, focusID, local) {
			if !emitEdge(e) {
				return false
			}
		}
		return true
	}

	onNew := func(fe *ingest.FileExtract) bool {
		return streamLocal(fe)
	}

	if parsed.Symbol != "" {
		// Atom visit: seed from defining module/file only.
		if err := c.absorbForRef(parsed, onNew); err != nil {
			emit(StreamEvent{Type: "error", Message: err.Error()})
			return err
		}
	} else {
		// Module visit: Seed from every direct source file in the module.
		files, err := directSourceFiles(c.root, parsed)
		if err != nil {
			emit(StreamEvent{Type: "error", Message: err.Error()})
			return err
		}
		for _, abs := range files {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Seed BFS from this file (new paths only enter corpus).
			if err := c.AbsorbSeed(abs, onNew); err != nil {
				emit(StreamEvent{Type: "error", Message: err.Error()})
				return err
			}
			// Always re-stream local edges for the file itself (session edge
			// dedupe drops already-sent ones). Covers "file already in corpus".
			if fe := c.GetByAbs(abs); fe != nil {
				if !streamLocal(fe) {
					return context.Canceled
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Full corpus resolve for cross-file edges.
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
		return emitImportAliases(c.MaterializeOne(fe))
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

// absorbForRef is used for atom visits (and helpers).
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
		// Provider package: visit each direct source file under the scope dir.
		files, err := directSourceFilesAbs(scope.Dir)
		if err != nil {
			return err
		}
		for _, abs := range files {
			if err := c.AbsorbSeed(abs, onNew); err != nil {
				return err
			}
			if fe := c.GetByAbs(abs); fe != nil && onNew != nil {
				if !onNew(fe) {
					return context.Canceled
				}
			}
		}
		return nil
	}

	abs, isDir, err := resolvePath(c.root, parsed)
	if err != nil {
		return err
	}
	if isDir {
		files, err := directSourceFilesAbs(abs)
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := c.AbsorbSeed(f, onNew); err != nil {
				return err
			}
			if fe := c.GetByAbs(f); fe != nil && onNew != nil {
				if !onNew(fe) {
					return context.Canceled
				}
			}
		}
		return nil
	}
	return c.AbsorbSeed(abs, onNew)
}

// directSourceFiles lists absolute paths of direct source files in a module.
func directSourceFiles(root string, modRef ingest.Reference) ([]string, error) {
	abs, isDir, err := resolvePath(root, scopeRef(modRef))
	if err != nil {
		return nil, err
	}
	if !isDir {
		return []string{abs}, nil
	}
	return directSourceFilesAbs(abs)
}

func directSourceFilesAbs(dirAbs string) ([]string, error) {
	entries, err := os.ReadDir(dirAbs)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue // only direct files, not nested packages
		}
		name := e.Name()
		if _, ok := ingest.LanguageForFile(name); !ok {
			continue
		}
		out = append(out, filepath.Join(dirAbs, name))
	}
	return out, nil
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
			return sameScope(focusRef, scopeRef(ingest.ParseReference(from))) ||
				sameScope(focusRef, scopeRef(ingest.ParseReference(to)))
		})
		addImportEdges(root, result, nodes, &edges, scopeRef(focusRef))
		return dedupeEdges(edges)
	}

	// Module focus: all atoms/relations under this module.
	modID := projectScopeID(root, scopeRef(focusRef))
	modFocus := scopeRef(focusRef)
	localAtoms := map[string]bool{}
	for _, ent := range result.Entities {
		er := ingest.ParseReference(ent.Reference)
		eid := graphRefIDString(root, ingest.CanonicalizeReference(root, er).String())
		if projectScopeID(root, scopeRef(ingest.ParseReference(eid))) != modID {
			continue
		}
		localAtoms[eid] = true
	}
	addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
		return localAtoms[from] || sameScope(modFocus, scopeRef(ingest.ParseReference(from)))
	})
	// Import edges from this module outward.
	for _, a := range result.Aliases {
		fromID := projectScopeID(root, ingest.ParseReference(a.Reference))
		toID := projectScopeID(root, ingest.ParseReference(a.Target))
		if fromID == "" || toID == "" || fromID == toID {
			continue
		}
		if fromID != modID && toID != modID {
			continue
		}
		edges = append(edges, &GraphEdge{From: fromID, To: toID, Kind: EdgeKindImports})
	}
	return dedupeEdges(edges)
}

// EncodeStreamEventJSON is a small helper for tests/handlers.
func EncodeStreamEventJSON(ev StreamEvent) ([]byte, error) {
	return json.Marshal(ev)
}
