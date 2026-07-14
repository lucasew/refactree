package ingest

import "strings"

// ForEachStringLiteral walks double-quoted and raw backtick string literals in
// content. lit includes the opening and closing delimiters; start is the byte
// offset of the opening quote. Iteration stops early if fn returns false.
//
// Escape handling for double-quoted strings matches Go/JSON-style \" and \\
// pairs; raw strings end at the next unescaped backtick (no escapes).
func ForEachStringLiteral(content []byte, fn func(lit string, start int) bool) {
	text := string(content)
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
			return
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
			return
		}
		if !fn(text[start:end+1], start) {
			return
		}
		off = end + 1
	}
}
