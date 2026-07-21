package graphql

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// BuildNeighborhood returns a lazy Seed-style neighborhood for ref.
func BuildNeighborhood(root, refStr string) (*Neighborhood, error) {
	parsed := ingest.CanonicalizeReference(root, ingest.ParseReference(refStr))
	focusID := parsed.String()
	focus := graphNodeForRef(root, parsed)

	nodes := map[string]*GraphNode{focusID: focus}
	var edges []*GraphEdge

	if parsed.Symbol != "" {
		result, err := seedForRef(root, parsed)
		if err != nil {
			return &Neighborhood{Focus: focus, Nodes: []*GraphNode{focus}, Edges: nil, Incomplete: true}, nil
		}
		// Parent file as structure (not a walk edge).
		parent := parsed
		parent.Symbol = ""
		parentID := parent.String()
		if _, ok := nodes[parentID]; !ok {
			nodes[parentID] = graphNodeForRef(root, parent)
		}
		focus.ParentID = &parentID

		addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
			// Ego: incident on focus, or both ends in the focus file (local structure).
			if from == focusID || to == focusID {
				return true
			}
			return sameScope(parsed, ingest.ParseReference(from)) || sameScope(parsed, ingest.ParseReference(to))
		})
		addImportEdges(root, result, nodes, &edges, parsed)

		return &Neighborhood{
			Focus:      focus,
			Nodes:      nodeList(nodes),
			Edges:      dedupeEdges(edges),
			Incomplete: true,
		}, nil
	}

	// No symbol: file or module.
	abs, isDir, err := resolvePath(root, parsed)
	if err == nil && !isDir && (parsed.Provider == "" || parsed.Provider == "path") {
		// File focus: atoms in file + their use edges (relations), not an import map.
		result, err := ingest.SeedResult(root, abs)
		if err != nil {
			return &Neighborhood{Focus: focus, Nodes: []*GraphNode{focus}, Incomplete: true}, nil
		}
		rel := strings.TrimPrefix(filepath.ToSlash(mustRel(root, abs)), "./")
		localAtoms := map[string]bool{}
		for _, ent := range result.Entities {
			er := ingest.ParseReference(ent.Reference)
			if normalizePath(er.Path) != normalizePath(rel) && filepath.Base(normalizePath(er.Path)) != filepath.Base(normalizePath(rel)) {
				continue
			}
			id := ingest.CanonicalizeReference(root, er).String()
			ensureNode(nodes, root, id)
			pid := focusID
			nodes[id].ParentID = &pid
			localAtoms[id] = true
		}
		// USES edges: from local atoms (and file-scoped relations) to their targets.
		addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
			return localAtoms[from] || sameScope(parsed, ingest.ParseReference(from))
		})
		return &Neighborhood{
			Focus:      focus,
			Nodes:      nodeList(nodes),
			Edges:      dedupeEdges(edges),
			Incomplete: true,
		}, nil
	}

	// Module / directory / provider scope: import edges between scopes.
	result, err := materializeModule(root, parsed, abs, isDir)
	if err != nil {
		return &Neighborhood{Focus: focus, Nodes: []*GraphNode{focus}, Incomplete: true}, nil
	}
	for _, a := range result.Aliases {
		fromRef := ingest.ParseReference(a.Reference)
		toRef := ingest.ParseReference(a.Target)
		fromScope := scopeRef(fromRef)
		toScope := scopeRef(toRef)
		fromID := ingest.CanonicalizeReference(root, fromScope).String()
		toID := ingest.CanonicalizeReference(root, toScope).String()
		if fromID == "" || toID == "" || fromID == toID {
			continue
		}
		ensureNode(nodes, root, fromID)
		ensureNode(nodes, root, toID)
		edges = append(edges, &GraphEdge{From: fromID, To: toID, Kind: EdgeKindImports})
	}
	return &Neighborhood{
		Focus:      focus,
		Nodes:      nodeList(nodes),
		Edges:      dedupeEdges(edges),
		Incomplete: true,
	}, nil
}

func addRelationEdges(
	root string,
	result *ingest.Result,
	nodes map[string]*GraphNode,
	edges *[]*GraphEdge,
	keep func(from, to string) bool,
) {
	if result == nil {
		return
	}
	for _, rel := range result.Relations {
		from := ingest.CanonicalizeReference(root, ingest.ParseReference(rel.Reference)).String()
		to := ingest.CanonicalizeReference(root, ingest.ParseReference(rel.Target)).String()
		if from == "" || to == "" || from == to {
			continue
		}
		if !keep(from, to) {
			continue
		}
		ensureNode(nodes, root, from)
		ensureNode(nodes, root, to)
		*edges = append(*edges, &GraphEdge{From: from, To: to, Kind: EdgeKindUses})
	}
}

func addImportEdges(root string, result *ingest.Result, nodes map[string]*GraphNode, edges *[]*GraphEdge, focus ingest.Reference) {
	if result == nil {
		return
	}
	for _, a := range result.Aliases {
		from := ingest.CanonicalizeReference(root, ingest.ParseReference(a.Reference)).String()
		to := ingest.CanonicalizeReference(root, ingest.ParseReference(a.Target)).String()
		if from == "" || to == "" || from == to {
			continue
		}
		if !sameScope(focus, ingest.ParseReference(from)) && !sameScope(focus, ingest.ParseReference(to)) {
			continue
		}
		ensureNode(nodes, root, from)
		ensureNode(nodes, root, to)
		*edges = append(*edges, &GraphEdge{From: from, To: to, Kind: EdgeKindImports})
	}
}

func dedupeEdges(in []*GraphEdge) []*GraphEdge {
	seen := map[string]bool{}
	out := make([]*GraphEdge, 0, len(in))
	for _, e := range in {
		if e == nil {
			continue
		}
		k := string(e.Kind) + "\x00" + e.From + "\x00" + e.To
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

func materializeModule(root string, parsed ingest.Reference, abs string, isDir bool) (*ingest.Result, error) {
	if parsed.Provider != "" && parsed.Provider != "path" {
		scope, ok, err := ingest.NewResolver(root).ResolveScopeTarget(parsed)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("no scope for %s", parsed.String())
		}
		return ingest.DirResult(scope.Dir, false)
	}
	if isDir && abs != "" {
		return ingest.MaterializeSource(ingest.ExtractSource{
			Kind:      ingest.ExtractDir,
			Root:      root,
			Dir:       abs,
			Recursive: false,
		}, ingest.MaterializeOptions{ExpandImports: false})
	}
	if abs != "" {
		return ingest.SeedResult(root, abs)
	}
	return nil, fmt.Errorf("cannot materialize %s", parsed.String())
}

func seedForRef(root string, parsed ingest.Reference) (*ingest.Result, error) {
	if parsed.Provider != "" && parsed.Provider != "path" {
		scope, ok, err := ingest.NewResolver(root).ResolveScopeTarget(ingest.Reference{
			Provider: parsed.Provider,
			Path:     parsed.Path,
		})
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("no scope")
		}
		result, err := ingest.DirResult(scope.Dir, false)
		if err != nil {
			return nil, err
		}
		for _, ent := range result.Entities {
			er := ingest.ParseReference(ent.Reference)
			if er.Symbol == parsed.Symbol {
				abs := filepath.Join(scope.Dir, filepath.FromSlash(strings.TrimPrefix(er.Path, "./")))
				if s, err := ingest.SeedResult(scope.Dir, abs); err == nil {
					return s, nil
				}
				return result, nil
			}
		}
		return result, nil
	}
	abs, _, err := resolvePath(root, parsed)
	if err != nil {
		return nil, err
	}
	return ingest.SeedResult(root, abs)
}

func graphNodeForRef(root string, ref ingest.Reference) *GraphNode {
	id := ref.String()
	kind := NodeKindFile
	label := ref.Path
	var parent *string
	if ref.Symbol != "" {
		kind = NodeKindAtom
		label = ref.Symbol
		p := ref
		p.Symbol = ""
		ps := p.String()
		parent = &ps
	} else if isModuleRef(root, ref) {
		kind = NodeKindModule
		label = ref.Path
		if label == "" {
			label = ref.Provider + ":" + ref.Path
		}
	} else {
		label = filepath.Base(strings.TrimSuffix(ref.Path, "/"))
		if label == "" || label == "." {
			label = ref.String()
		}
	}
	n := &GraphNode{ID: id, Kind: kind, Label: label, ParentID: parent}
	return decorateNode(root, n)
}

func isModuleRef(root string, ref ingest.Reference) bool {
	if ref.Symbol != "" {
		return false
	}
	if ref.Provider != "" && ref.Provider != "path" {
		return true
	}
	abs, isDir, err := resolvePath(root, ref)
	if err != nil {
		return false
	}
	if isDir {
		return true
	}
	if lang, ok := ingest.LanguageForFile(abs); ok && ingest.LanguageUsesDirectoryModule(lang) {
		return false
	}
	if lang, ok := ingest.LanguageForFile(abs); ok && !ingest.LanguageUsesDirectoryModule(lang) {
		return true
	}
	return false
}

func ensureNode(nodes map[string]*GraphNode, root, id string) {
	if id == "" {
		return
	}
	if _, ok := nodes[id]; ok {
		return
	}
	nodes[id] = graphNodeForRef(root, ingest.ParseReference(id))
}

func nodeList(m map[string]*GraphNode) []*GraphNode {
	out := make([]*GraphNode, 0, len(m))
	for _, n := range m {
		out = append(out, n)
	}
	return out
}

func sameScope(a, b ingest.Reference) bool {
	return a.Provider == b.Provider && normalizePath(a.Path) == normalizePath(b.Path)
}

func scopeRef(r ingest.Reference) ingest.Reference {
	r.Symbol = ""
	return r
}

func normalizePath(p string) string {
	p = strings.TrimPrefix(filepath.ToSlash(p), "./")
	return p
}

func resolvePath(root string, ref ingest.Reference) (abs string, isDir bool, err error) {
	if ref.Provider != "" && ref.Provider != "path" {
		return "", false, fmt.Errorf("not a path ref")
	}
	rel := strings.TrimPrefix(ref.Path, "./")
	if rel == "" || rel == "." {
		return root, true, nil
	}
	abs = filepath.Join(root, filepath.FromSlash(rel))
	st, err := os.Stat(abs)
	if err != nil {
		return abs, false, err
	}
	return abs, st.IsDir(), nil
}

func mustRel(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return rel
}
