package ingest

import "strings"

// SpanOccupied reports whether [start,end) overlaps any already-covered rename span.
func SpanOccupied(occ map[[2]uint32]bool, start, end uint32) bool {
	if occ == nil {
		return false
	}
	if occ[[2]uint32{start, end}] {
		return true
	}
	for k := range occ {
		if start < k[1] && end > k[0] {
			return true
		}
	}
	return false
}

// MarkEntityRelationSpans returns per-file spans already covered by entity and
// relation renames for the given source references. Used by ExtraRenameEdits to
// avoid double-rewriting definition/usage sites.
func MarkEntityRelationSpans(result *Result, sourceSet map[string]bool) map[string]map[[2]uint32]bool {
	occupied := map[string]map[[2]uint32]bool{}
	if result == nil || len(sourceSet) == 0 {
		return occupied
	}
	mark := func(file string, start, end uint32) {
		file = strings.TrimPrefix(file, "./")
		if occupied[file] == nil {
			occupied[file] = map[[2]uint32]bool{}
		}
		occupied[file][[2]uint32{start, end}] = true
	}
	for _, ent := range result.Entities {
		if !sourceSet[ent.Reference] {
			continue
		}
		ref := ParseReference(ent.Reference)
		mark(ref.Path, ent.StartByte, ent.EndByte)
	}
	for _, rel := range result.Relations {
		if !sourceSet[rel.Target] {
			continue
		}
		ref := ParseReference(rel.Reference)
		mark(ref.Path, rel.StartByte, rel.EndByte)
	}
	return occupied
}

// IsIdentChar reports whether b is an ASCII letter, digit, or underscore.
func IsIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// IsIdentCharJava is IsIdentChar plus '$' (Java identifiers; also used by JS).
func IsIdentCharJava(b byte) bool {
	return IsIdentChar(b) || b == '$'
}
