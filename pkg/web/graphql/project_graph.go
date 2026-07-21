package graphql

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// BuildProjectGraph builds a lazy project-local import map:
// path-scoped modules/files as nodes, IMPORTS edges from aliases,
// non-path targets as external expandable stubs (not crawled until neighborhood).
func BuildProjectGraph(root string) (*Neighborhood, error) {
	result, err := ingest.MaterializeSource(ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      root,
		Recursive: true,
	}, ingest.MaterializeOptions{ExpandImports: false})
	if err != nil {
		return nil, err
	}

	focusID := "path:./"
	focus := &GraphNode{
		ID:         focusID,
		Kind:       NodeKindModule,
		Label:      filepath.Base(root),
		External:   false,
		Expandable: false,
		Language:   "",
	}
	if focus.Label == "" || focus.Label == "." {
		focus.Label = "project"
	}

	nodes := map[string]*GraphNode{focusID: focus}
	var edges []*GraphEdge

	// Path files as module/file nodes.
	for _, f := range result.Files {
		rel := strings.TrimPrefix(filepath.ToSlash(f.Path), "./")
		id := projectScopeID(root, ingest.ParseReference("path:./"+rel))
		if _, ok := nodes[id]; ok {
			continue
		}
		ref := ingest.ParseReference(id)
		nodes[id] = decorateNode(root, graphNodeForRef(root, ref))
	}

	// IMPORTS from aliases (external targets stay stubs).
	for _, a := range result.Aliases {
		fromID := projectScopeID(root, ingest.ParseReference(a.Reference))
		toID := projectScopeID(root, ingest.ParseReference(a.Target))
		if fromID == "" || toID == "" || fromID == toID {
			continue
		}
		if _, ok := nodes[fromID]; !ok {
			nodes[fromID] = decorateNode(root, graphNodeForRef(root, ingest.ParseReference(fromID)))
		}
		if _, ok := nodes[toID]; !ok {
			nodes[toID] = decorateNode(root, graphNodeForRef(root, ingest.ParseReference(toID)))
		}
		edges = append(edges, &GraphEdge{From: fromID, To: toID, Kind: EdgeKindImports})
	}

	for _, n := range nodes {
		decorateNode(root, n)
	}

	return &Neighborhood{
		Focus:      focus,
		Nodes:      nodeList(nodes),
		Edges:      dedupeEdges(edges),
		Incomplete: true,
	}, nil
}

func decorateNode(root string, n *GraphNode) *GraphNode {
	if n == nil {
		return n
	}
	// Re-key through graph id so go:example.com/app/pkg → path:./pkg when local.
	if n.ID != "" {
		n.ID = graphRefIDString(root, n.ID)
	}
	r := ingest.ParseReference(n.ID)
	n.External = isExternalRef(r)
	// External stubs are expandable until the client has loaded neighborhood(ref).
	n.Expandable = n.External
	return n
}

func isExternalRef(ref ingest.Reference) bool {
	p := strings.ToLower(ref.Provider)
	return p != "" && p != "path"
}

// tryLocalizeProviderToPath rewrites provider refs (go:example.com/app/pkg/lib) that
// resolve inside the project tree to path:./pkg/lib. Absolute package dir under root
// is the stable identity; relative path:./ form is the graph id.
func tryLocalizeProviderToPath(root string, ref ingest.Reference) (ingest.Reference, bool) {
	p := strings.ToLower(ref.Provider)
	if p == "" || p == "path" {
		return ref, false
	}
	scope, ok, err := ingest.NewResolver(root).ResolveScopeTarget(ingest.Reference{
		Provider: ref.Provider,
		Path:     ref.Path,
	})
	if err != nil || !ok || strings.TrimSpace(scope.Dir) == "" {
		return ref, false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return ref, false
	}
	dirAbs, err := filepath.Abs(scope.Dir)
	if err != nil {
		return ref, false
	}
	rel, err := filepath.Rel(rootAbs, dirAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ref, false
	}
	rel = filepath.ToSlash(rel)
	path := "./"
	if rel != "." {
		path = "./" + rel
	}
	return ingest.Reference{Provider: "path", Path: path, Symbol: ref.Symbol}, true
}

// projectScopeID maps a ref to a stable graph id for the project map.
// Local provider packages (same module / under root) become path:./… so they
// share identity with filesystem packages (not external orange stubs).
func projectScopeID(root string, ref ingest.Reference) string {
	if loc, ok := tryLocalizeProviderToPath(root, ref); ok {
		ref = loc
	}
	ref.Symbol = ""
	if isExternalRef(ref) {
		return ingest.Reference{Provider: ref.Provider, Path: ref.Path}.String()
	}
	rel := strings.TrimPrefix(filepath.ToSlash(ref.Path), "./")
	if rel == "" || rel == "." {
		return "path:./"
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		return "path:./" + rel
	}
	if lang, ok := ingest.LanguageForFile(abs); ok && ingest.LanguageUsesDirectoryModule(lang) {
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "." || dir == "" {
			return "path:./"
		}
		return "path:./" + dir
	}
	return "path:./" + rel
}
