package ingest

import (
	"cmp"
	"slices"
)

// SortResult orders Files, Entities, Aliases, and Relations for stable compare.
func SortResult(r *Result) {
	if r == nil {
		return
	}
	slices.SortFunc(r.Files, func(a, b File) int {
		return cmp.Compare(a.Path, b.Path)
	})
	slices.SortFunc(r.Entities, func(a, b Entity) int {
		return cmp.Or(
			cmp.Compare(a.Reference, b.Reference),
			cmp.Compare(a.StartByte, b.StartByte),
		)
	})
	slices.SortFunc(r.Aliases, func(a, b Alias) int {
		return cmp.Or(
			cmp.Compare(a.Reference, b.Reference),
			cmp.Compare(a.StartByte, b.StartByte),
		)
	})
	slices.SortFunc(r.Relations, func(a, b Relation) int {
		return cmp.Or(
			cmp.Compare(a.Reference, b.Reference),
			cmp.Compare(a.StartByte, b.StartByte),
		)
	})
}
