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
	focus := graphNodeForRef(root, parsed)
	focusID := focus.ID
	// Use module-normalized focus for scope comparisons.
	parsed = ingest.ParseReference(focusID)

	nodes := map[string]*GraphNode{focusID: focus}
	var edges []*GraphEdge

	if parsed.Symbol != "" {
		result, err := seedForRef(root, parsed)
		if err != nil {
			return &Neighborhood{Focus: focus, Nodes: []*GraphNode{focus}, Edges: nil, Incomplete: true}, nil
		}
		// Parent module as structure (not a walk edge).
		if focus.ParentID != nil {
			parentID := *focus.ParentID
			if _, ok := nodes[parentID]; !ok {
				nodes[parentID] = graphNodeForRef(root, ingest.ParseReference(parentID))
			}
		}

		modFocus := scopeRef(parsed)
		addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
			if from == focusID || to == focusID {
				return true
			}
			return sameScope(modFocus, scopeRef(ingest.ParseReference(from))) ||
				sameScope(modFocus, scopeRef(ingest.ParseReference(to)))
		})
		addImportEdges(root, result, nodes, &edges, modFocus)

		return &Neighborhood{
			Focus:      focus,
			Nodes:      nodeList(nodes),
			Edges:      dedupeEdges(edges),
			Incomplete: true,
		}, nil
	}

	// Module focus (files already normalized to modules).
	abs, isDir, err := resolvePath(root, parsed)
	// After normalize, path may be a package directory.
	if err == nil && (parsed.Provider == "" || parsed.Provider == "path") {
		// Prefer seed from a representative path: if module is a dir, use dir materialize.
		var result *ingest.Result
		if isDir {
			result, err = materializeModule(root, parsed, abs, true)
		} else {
			// File-as-module (JS/Python): seed that file.
			result, err = ingest.SeedResult(root, abs)
		}
		if err != nil {
			return &Neighborhood{Focus: focus, Nodes: []*GraphNode{focus}, Incomplete: true}, nil
		}
		modFocus := scopeRef(parsed)
		// Atoms under this module.
		localAtoms := map[string]bool{}
		modID := projectScopeID(root, parsed)
		for _, ent := range result.Entities {
			er := ingest.ParseReference(ent.Reference)
			eid := graphRefIDString(root, ingest.CanonicalizeReference(root, er).String())
			em := projectScopeID(root, scopeRef(ingest.ParseReference(eid)))
			if em != modID {
				continue
			}
			ensureNode(nodes, root, eid)
			if n := nodes[eid]; n != nil {
				pid := focusID
				n.ParentID = &pid
			}
			localAtoms[eid] = true
		}
		addRelationEdges(root, result, nodes, &edges, func(from, to string) bool {
			return localAtoms[from] || sameScope(modFocus, scopeRef(ingest.ParseReference(from)))
		})
		return &Neighborhood{
			Focus:      focus,
			Nodes:      nodeList(nodes),
			Edges:      dedupeEdges(edges),
			Incomplete: true,
		}, nil
	}

	// Provider scope: import edges between modules.
	result, err := materializeModule(root, parsed, abs, isDir)
	if err != nil {
		return &Neighborhood{Focus: focus, Nodes: []*GraphNode{focus}, Incomplete: true}, nil
	}
	for _, a := range result.Aliases {
		fromID := graphRefIDString(root, a.Reference)
		toID := graphRefIDString(root, a.Target)
		// strip symbols for import map endpoints
		fromID = projectScopeID(root, scopeRef(ingest.ParseReference(fromID)))
		toID = projectScopeID(root, scopeRef(ingest.ParseReference(toID)))
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
		from := graphRefIDString(root, ingest.CanonicalizeReference(root, ingest.ParseReference(rel.Reference)).String())
		to := graphRefIDString(root, ingest.CanonicalizeReference(root, ingest.ParseReference(rel.Target)).String())
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
		from := graphRefIDString(root, ingest.CanonicalizeReference(root, ingest.ParseReference(a.Reference)).String())
		to := graphRefIDString(root, ingest.CanonicalizeReference(root, ingest.ParseReference(a.Target)).String())
		if from == "" || to == "" || from == to {
			continue
		}
		// sameScope on module path of endpoints
		if !sameScope(focus, scopeRef(ingest.ParseReference(from))) && !sameScope(focus, scopeRef(ingest.ParseReference(to))) {
			// also compare after module-normalizing focus path
			fr := scopeRef(ingest.ParseReference(from))
			tr := scopeRef(ingest.ParseReference(to))
			fm := ingest.ParseReference(projectScopeID(root, fr))
			tm := ingest.ParseReference(projectScopeID(root, tr))
			fo := ingest.ParseReference(projectScopeID(root, scopeRef(focus)))
			if !sameScope(fo, fm) && !sameScope(fo, tm) {
				continue
			}
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
	// Module-normalized path may be a package directory (e.g. path:./pkg/web::New).
	abs, isDir, err := resolvePath(root, scopeRef(parsed))
	if err != nil {
		return nil, err
	}
	if isDir {
		// Seed from all source files under the package dir (ExpandImports off).
		return ingest.MaterializeSource(ingest.ExtractSource{
			Kind:      ingest.ExtractDir,
			Root:      root,
			Dir:       abs,
			Recursive: false,
		}, ingest.MaterializeOptions{ExpandImports: false})
	}
	return ingest.SeedResult(root, abs)
}

func graphNodeForRef(root string, ref ingest.Reference) *GraphNode {
	// Normalize file paths to module scope (DirectoryModule → package dir, else file-as-module).
	id := graphRefID(root, ref)
	ref = ingest.ParseReference(id)

	kind := NodeKindModule
	moduleName := moduleDisplayName(root, ref)
	var parent *string
	var label string

	if ref.Symbol != "" {
		kind = NodeKindAtom
		// Two-line label: module\nsymbol (canvas splits on newline).
		label = moduleName + "\n" + ref.Symbol
		p := ref
		p.Symbol = ""
		ps := p.String()
		parent = &ps
	} else {
		label = moduleName
		// Never FILE in the graph — files are modules (or package dirs).
		kind = NodeKindModule
	}

	n := &GraphNode{ID: id, Kind: kind, Label: label, ParentID: parent, Language: languageForRef(root, ref)}
	return decorateNode(root, n)
}

// graphRefID normalizes a reference for the graph: file → module; atoms keep symbol on module path.
func graphRefID(root string, ref ingest.Reference) string {
	if ref.Symbol != "" {
		modID := projectScopeID(root, scopeRef(ref))
		mod := ingest.ParseReference(modID)
		mod.Symbol = ref.Symbol
		return mod.String()
	}
	return projectScopeID(root, ref)
}

// graphRefIDString normalizes an edge endpoint string.
func graphRefIDString(root, refStr string) string {
	if strings.TrimSpace(refStr) == "" {
		return ""
	}
	return graphRefID(root, ingest.ParseReference(refStr))
}

// moduleDisplayName is a short module label for the first line of a node.
func moduleDisplayName(root string, ref ingest.Reference) string {
	if isExternalRef(ref) {
		if ref.Path == "" {
			return ref.Provider
		}
		return ref.Provider + ":" + ref.Path
	}
	rel := strings.TrimPrefix(filepath.ToSlash(ref.Path), "./")
	if rel == "" || rel == "." {
		if base := filepath.Base(root); base != "" && base != "." {
			return base
		}
		return "."
	}
	return rel
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
	id = graphRefIDString(root, id)
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

// languageForRef infers a language id for coloring (path extension or provider).
func languageForRef(root string, ref ingest.Reference) string {
	p := strings.ToLower(ref.Provider)
	switch p {
	case "go":
		return "go"
	case "python":
		return "python"
	case "node", "javascript", "js":
		return "javascript"
	case "java":
		return "java"
	case "nix":
		return "nix"
	case "svelte":
		return "svelte"
	}
	path := ref.Path
	if path == "" {
		return ""
	}
	if lang, ok := ingest.LanguageForFile(path); ok {
		return lang
	}
	// Package directory: peek at child source files.
	abs, isDir, err := resolvePath(root, scopeRef(ref))
	if err != nil {
		return ""
	}
	if !isDir {
		return ""
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if lang, ok := ingest.LanguageForFile(e.Name()); ok {
			return lang
		}
	}
	return ""
}
