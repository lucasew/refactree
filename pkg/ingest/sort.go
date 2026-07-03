package ingest

import "sort"

// SortResult orders Files, Entities, Aliases, and Relations for stable compare.
func SortResult(r *Result) {
	if r == nil {
		return
	}
	sort.Slice(r.Files, func(i, j int) bool {
		return r.Files[i].Path < r.Files[j].Path
	})
	sort.Slice(r.Entities, func(i, j int) bool {
		if r.Entities[i].Reference != r.Entities[j].Reference {
			return r.Entities[i].Reference < r.Entities[j].Reference
		}
		return r.Entities[i].StartByte < r.Entities[j].StartByte
	})
	sort.Slice(r.Aliases, func(i, j int) bool {
		if r.Aliases[i].Reference != r.Aliases[j].Reference {
			return r.Aliases[i].Reference < r.Aliases[j].Reference
		}
		return r.Aliases[i].StartByte < r.Aliases[j].StartByte
	})
	sort.Slice(r.Relations, func(i, j int) bool {
		if r.Relations[i].Reference != r.Relations[j].Reference {
			return r.Relations[i].Reference < r.Relations[j].Reference
		}
		return r.Relations[i].StartByte < r.Relations[j].StartByte
	})
}
