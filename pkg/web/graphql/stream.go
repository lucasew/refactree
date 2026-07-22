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
//  2. hop-discover direct package/file extracts only (no import BFS)
//  3. MaterializeVisit(closure only) — not whole session history
//  4. stream edges from that Result
//  5. done
//
// Module visits hop all direct package files. Import-graph expansion is the
// crawl's job — Seed BFS here used to walk most of the repo on every click
// and peg the CPU while crawl stayed paused.
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

// ProjectBatchSize is how many files are materialized before edges are emitted.
const ProjectBatchSize = 3

// ProjectFocusNode is the root focus event for a project crawl.
func ProjectFocusNode(root string) *GraphNode {
	focus := &GraphNode{
		ID: "path:./", Kind: NodeKindModule, Label: filepath.Base(root),
		External: false, Expandable: false, Language: "",
	}
	if focus.Label == "" || focus.Label == "." {
		focus.Label = "project"
	}
	return focus
}

// EmitProjectBatch materializes a small set of extracts and emits module IMPORTS
// and atom USES edges. Containment is parentId structure, not USED_BY.
// Wire-level dedupe is the emitter's job (session seen map). Returns false if emit cancels.
func (c *SessionCorpus) EmitProjectBatch(
	ctx context.Context,
	batch map[string]*ingest.FileExtract,
	emit StreamEmitter,
) bool {
	if c == nil || emit == nil || len(batch) == 0 {
		return true
	}
	if err := ctx.Err(); err != nil {
		return false
	}
	emitEdge := func(from, to string, kind EdgeKind) bool {
		from = graphRefIDString(c.root, from)
		to = graphRefIDString(c.root, to)
		if kind == EdgeKindImports {
			from = projectScopeID(c.root, scopeRef(ingest.ParseReference(from)))
			to = projectScopeID(c.root, scopeRef(ingest.ParseReference(to)))
		}
		if from == "" || to == "" || from == to {
			return true
		}
		e := &GraphEdge{From: from, To: to, Kind: kind}
		return emit(StreamEvent{Type: "edge", Edge: e, Incomplete: boolPtr(true)})
	}

	result := c.MaterializeVisit(batch)
	for _, a := range result.Aliases {
		if !emitEdge(a.Reference, a.Target, EdgeKindImports) {
			return false
		}
	}
	for _, rel := range result.Uses {
		if !emitEdge(rel.Reference, rel.Target, EdgeKindUses) {
			return false
		}
	}
	return true
}

// StreamProject walks the project tree and streams edges progressively.
// Prefer the pump+worker path in graph_session for cooperative preemption;
// this remains for tests and one-shot callers.
func (c *SessionCorpus) StreamProject(ctx context.Context, emit StreamEmitter) error {
	if c == nil || emit == nil {
		return nil
	}
	if !emit(StreamEvent{Type: "focus", Node: ProjectFocusNode(c.root), Incomplete: boolPtr(true)}) {
		return context.Canceled
	}

	batch := make(map[string]*ingest.FileExtract, ProjectBatchSize)

	flush := func() bool {
		ok := c.EmitProjectBatch(ctx, batch, emit)
		batch = make(map[string]*ingest.FileExtract, ProjectBatchSize)
		return ok
	}

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
		key := extractRelKey(fe)
		// Skip paths already in the corpus (file browser / prior visit primed).
		if key != "" && c.Has(key) {
			return true
		}
		stored := c.Touch(fe)
		key = extractRelKey(stored)
		if key == "" {
			return true
		}
		batch[key] = stored
		if len(batch) < ProjectBatchSize {
			return true
		}
		return flush()
	})
	if err != nil {
		emit(StreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	if !flush() {
		return context.Canceled
	}

	inc := true
	if !emit(StreamEvent{Type: "done", Incomplete: &inc}) {
		return context.Canceled
	}
	return nil
}

// discoverVisit fills visit with direct package/file extracts for this focus.
// No import/neighbor BFS — that was unbounded on large repos and blocked crawl.
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
		return c.DiscoverHop(files, visit)
	}

	abs, isDir, err := resolvePath(c.root, scopeRef(parsed))
	if err != nil {
		return err
	}

	// Module (dir or file-as-module): hop all direct source files in that package.
	if parsed.Name == "" {
		var seeds []string
		if isDir {
			seeds, err = directSourceFilesAbs(abs)
			if err != nil {
				return err
			}
		} else {
			seeds = []string{abs}
		}
		return c.DiscoverHop(seeds, visit)
	}

	// Atom: hop package files if path is a dir; else that file only.
	if isDir {
		files, err := directSourceFilesAbs(abs)
		if err != nil {
			return err
		}
		return c.DiscoverHop(files, visit)
	}
	return c.DiscoverHop([]string{abs}, visit)
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

	if focusRef.Name != "" {
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
	for _, ent := range result.Atoms {
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
