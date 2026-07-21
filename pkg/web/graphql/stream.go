package graphql

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// StreamEvent is one progressive graph update (WS explore session).
// Edges are the primary stream; nodes (except focus) hydrate via GraphQL.
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

// LookupNode returns cheap node metadata from a reference id (no graph walk).
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

// StreamVisit is the core explore path:
//
//  1. focus event
//  2. discover visit closure (one multi-seed BFS or dir walk) → Touch into session corpus
//  3. MaterializeVisit(closure only) — not whole session history
//  4. stream edges from that Result
//  5. done
//
// Module visits seed all direct package files in one WalkExtracts call.
// No per-file MaterializeOne; no N independent Seeds.
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

	visit := make(map[string]*ingest.FileExtract)
	if err := c.discoverVisit(parsed, visit); err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	if err := ctx.Err(); err != nil {
		// Preempted after discover (e.g. user clicked another package) — skip Materialize.
		return err
	}

	result := c.MaterializeVisit(visit)
	if err := ctx.Err(); err != nil {
		// Preempted during Materialize — do not stream stale edges for this package.
		return err
	}
	return emitVisitEdges(ctx, c.root, parsed, focusID, result, emit)
}

// StreamProject walks the project tree and streams package-level IMPORTS + USES
// edges progressively (parse → materialize → emit per file / small batch).
// Local go: packages under the project root are rewritten to path:./… .
//
// Unlike a bulk DiscoverDir+Materialize of the whole tree, edges are pushed as
// soon as each file is ready so the UI is not blocked for a minute on large repos.
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
	emitEdge := func(from, to string, kind EdgeKind) bool {
		from = graphRefIDString(c.root, from)
		to = graphRefIDString(c.root, to)
		from = projectScopeID(c.root, scopeRef(ingest.ParseReference(from)))
		to = projectScopeID(c.root, scopeRef(ingest.ParseReference(to)))
		if from == "" || to == "" || from == to {
			return true
		}
		e := &GraphEdge{From: from, To: to, Kind: kind}
		k := edgeKey(e)
		if seen[k] {
			return true
		}
		seen[k] = true
		return emit(StreamEvent{Type: "edge", Edge: e, Incomplete: boolPtr(true)})
	}

	flush := func(batch map[string]*ingest.FileExtract) bool {
		if len(batch) == 0 {
			return true
		}
		if err := ctx.Err(); err != nil {
			return false
		}
		result := c.MaterializeVisit(batch)
		for _, a := range result.Aliases {
			if !emitEdge(a.Reference, a.Target, EdgeKindImports) {
				return false
			}
		}
		for _, rel := range result.Relations {
			if !emitEdge(rel.Reference, rel.Target, EdgeKindUses) {
				return false
			}
		}
		return true
	}

	// Small batches: enough to amortize Materialize, small enough to stream early.
	const batchSize = 3
	batch := make(map[string]*ingest.FileExtract, batchSize)

	err := ingest.WalkExtracts(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      c.root,
		Recursive: true,
	}, func(fe *ingest.FileExtract) bool {
		if fe == nil {
			return true
		}
		if err := ctx.Err(); err != nil {
			return false
		}
		stored := c.Touch(fe)
		key := extractRelKey(stored)
		if key == "" {
			return true
		}
		batch[key] = stored
		if len(batch) < batchSize {
			return true
		}
		ok := flush(batch)
		batch = make(map[string]*ingest.FileExtract, batchSize)
		return ok
	})
	if err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	if !flush(batch) {
		return context.Canceled
	}

	inc := true
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

// discoverVisit fills visit with the extract closure for this focus.
func (c *SessionCorpus) discoverVisit(parsed ingest.Reference, visit map[string]*ingest.FileExtract) error {
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
		files, err := directSourceFilesAbs(scope.Dir)
		if err != nil {
			return err
		}
		return c.DiscoverSeeds(files, visit)
	}

	abs, isDir, err := resolvePath(c.root, scopeRef(parsed))
	if err != nil {
		return err
	}

	// Module (dir or file-as-module): one multi-seed from all direct source files.
	if parsed.Symbol == "" {
		var seeds []string
		if isDir {
			seeds, err = directSourceFilesAbs(abs)
			if err != nil {
				return err
			}
		} else {
			seeds = []string{abs}
		}
		return c.DiscoverSeeds(seeds, visit)
	}

	// Atom: if path is package dir, multi-seed package files; else seed that file.
	if isDir {
		files, err := directSourceFilesAbs(abs)
		if err != nil {
			return err
		}
		return c.DiscoverSeeds(files, visit)
	}
	return c.DiscoverSeeds([]string{abs}, visit)
}

func emitVisitEdges(
	ctx context.Context,
	root string,
	parsed ingest.Reference,
	focusID string,
	result *ingest.Result,
	emit StreamEmitter,
) error {
	seen := map[string]bool{}
	for _, e := range visitEdges(root, parsed, focusID, result) {
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
			continue
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
