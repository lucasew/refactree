package ingest

import (
	"strings"
	"sync"
)

// UseSiteRenamer expands leaf renames at call/use sites for symbols in
// sourceSet. newLeaf is the replacement identifier text (AtomName form).
// root is the project ingest root; result is a full project materialize.
//
// The default walks result.Uses (graph). Linking pkg/pattern registers
// pattern.UseSiteRenames (RefLeafRule / NFA) so mv use sites share the same
// site-transform backbone as rewrite.
type UseSiteRenamer func(root string, result *Result, sourceSet map[string]bool, newLeaf string) []Edit

var (
	useSiteRenamerMu sync.RWMutex
	useSiteRenamer   UseSiteRenamer
)

// RegisterUseSiteRenamer sets the use-site rename expander.
// Passing nil restores the graph default. Panics if called twice with non-nil
// without an intervening nil clear (same idea as one global site engine).
func RegisterUseSiteRenamer(fn UseSiteRenamer) {
	useSiteRenamerMu.Lock()
	defer useSiteRenamerMu.Unlock()
	useSiteRenamer = fn
}

func expandUseSiteRenames(root string, result *Result, sourceSet map[string]bool, newLeaf string) []Edit {
	useSiteRenamerMu.RLock()
	fn := useSiteRenamer
	useSiteRenamerMu.RUnlock()
	if fn != nil {
		return fn(root, result, sourceSet, newLeaf)
	}
	return useSiteRenamesFromGraph(result, sourceSet, newLeaf)
}

// useSiteRenamesFromGraph rewrites identifier leaves at Uses that target any
// ref in sourceSet. Import-alias bindings (ViaImportAlias) are left unchanged.
func useSiteRenamesFromGraph(result *Result, sourceSet map[string]bool, newLeaf string) []Edit {
	var edits []Edit
	for _, rel := range result.Uses {
		if !sourceSet[rel.Target] {
			continue
		}
		if rel.ViaImportAlias {
			continue
		}
		ref := ParseReference(rel.Reference)
		edits = append(edits, Edit{
			File:    strings.TrimPrefix(ref.Path, "./"),
			Span:    Span{StartByte: rel.StartByte, EndByte: rel.EndByte},
			NewText: newLeaf,
		})
	}
	return edits
}
