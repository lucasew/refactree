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
	for _, ent := range result.Atoms {
		if !sourceSet[ent.Reference] {
			continue
		}
		ref := ParseReference(ent.Reference)
		mark(ref.Path, ent.StartByte, ent.EndByte)
	}
	for _, rel := range result.Uses {
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

// MaskNonNewlinesInPlace replaces bytes in [start, end) with spaces, keeping
// newlines so residual whole-word scans still see line structure. Indices are
// clamped to buf. Used by strip-unused-import paths after a declaration move.
func MaskNonNewlinesInPlace(buf []byte, start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(buf) {
		end = len(buf)
	}
	for i := start; i < end; i++ {
		if buf[i] != '\n' {
			buf[i] = ' '
		}
	}
}

// IdentUsed reports whether ident appears as a whole identifier in text.
// isIdent classifies identifier characters (IsIdentChar or IsIdentCharJava).
// Empty ident or a nil isIdent always returns false.
func IdentUsed(text, ident string, isIdent func(byte) bool) bool {
	if ident == "" || isIdent == nil {
		return false
	}
	off := 0
	for {
		idx := strings.Index(text[off:], ident)
		if idx < 0 {
			return false
		}
		pos := off + idx
		end := pos + len(ident)
		if pos > 0 && isIdent(text[pos-1]) {
			off = end
			continue
		}
		if end < len(text) && isIdent(text[end]) {
			off = end
			continue
		}
		return true
	}
}
