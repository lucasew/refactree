package ingestgo

import (
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// findPathSegmentOccurrencesInStrings replaces oldPath with newPath only
// inside string literals and only on import-path segment boundaries.
func findPathSegmentOccurrencesInStrings(file string, content []byte, oldPath, newPath string) []ingest.Edit {
	return findInStringLiterals(file, content, oldPath, newPath, true, "")
}

// findPathSegmentOccurrencesInStringsWithParent replaces a path leaf only when
// the preceding segment equals parentDir's final component.
func findPathSegmentOccurrencesInStringsWithParent(file string, content []byte, leaf, newLeaf, parentDir string) []ingest.Edit {
	return findInStringLiterals(file, content, leaf, newLeaf, true, parentDir)
}

func findInStringLiterals(file string, content []byte, oldBase, newBase string, pathSegments bool, parentDir string) []ingest.Edit {
	if oldBase == "" || oldBase == newBase {
		return nil
	}
	parentLeaf := ""
	if parentDir != "" {
		parentLeaf = lastPathComponent(parentDir)
	}
	text := string(content)
	var edits []ingest.Edit
	off := 0
	for off < len(text) {
		dq := strings.IndexByte(text[off:], '"')
		rq := strings.IndexByte(text[off:], '`')
		start := -1
		isRaw := false
		if dq >= 0 && (rq < 0 || dq < rq) {
			start = off + dq
		} else if rq >= 0 {
			start = off + rq
			isRaw = true
		}
		if start < 0 {
			break
		}
		end := -1
		if isRaw {
			e := strings.IndexByte(text[start+1:], '`')
			if e >= 0 {
				end = start + 1 + e
			}
		} else {
			i := start + 1
			for i < len(text) {
				if text[i] == '\\' && i+1 < len(text) {
					i += 2
					continue
				}
				if text[i] == '"' {
					end = i
					break
				}
				i++
			}
		}
		if end < 0 {
			break
		}
		seg := text[start : end+1]
		sOff := 0
		for {
			idx := strings.Index(seg[sOff:], oldBase)
			if idx < 0 {
				break
			}
			posInSeg := sOff + idx
			pos := start + posInSeg
			endPos := pos + len(oldBase)
			keep := true
			if pathSegments {
				keep = pathSegmentBounded(seg, posInSeg, posInSeg+len(oldBase))
				if keep && parentDir != "" {
					keep = pathSegmentHasParent(seg, posInSeg, parentLeaf)
				}
			}
			if keep {
				edits = append(edits, ingest.Edit{
					File:      file,
					StartByte: uint32(pos),
					EndByte:   uint32(endPos),
					NewText:   newBase,
				})
			}
			sOff = posInSeg + len(oldBase)
		}
		off = end + 1
	}
	return edits
}

func pathSegmentBounded(seg string, start, end int) bool {
	if start > 0 && isPathSegmentChar(seg[start-1]) {
		return false
	}
	if end < len(seg) && isPathSegmentChar(seg[end]) {
		return false
	}
	return true
}

func pathSegmentHasParent(seg string, leafStart int, parentLeaf string) bool {
	if parentLeaf == "" {
		return leafStart == 1
	}
	if leafStart < 2 || seg[leafStart-1] != '/' {
		return false
	}
	parentStart := leafStart - 1 - len(parentLeaf)
	if parentStart < 1 {
		return false
	}
	if seg[parentStart:leafStart-1] != parentLeaf {
		return false
	}
	return pathSegmentBounded(seg, parentStart, leafStart-1)
}

func isPathSegmentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
		b == '_' || b == '-' || b == '.' || b == '~' || b == '+'
}
