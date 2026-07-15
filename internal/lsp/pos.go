package lsp

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	"go.lsp.dev/protocol"
)

// byteOffset returns the UTF-8 byte offset for an LSP position (UTF-16 columns).
func byteOffset(text string, pos protocol.Position) int {
	line := int(pos.Line)
	char := int(pos.Character)
	if line < 0 {
		line = 0
	}
	if char < 0 {
		char = 0
	}
	off := 0
	curLine := 0
	for off <= len(text) && curLine < line {
		if off == len(text) {
			return len(text)
		}
		r, size := utf8.DecodeRuneInString(text[off:])
		off += size
		if r == '\n' {
			curLine++
		}
	}
	u16 := 0
	for off < len(text) && u16 < char {
		r, size := utf8.DecodeRuneInString(text[off:])
		if r == '\n' {
			break
		}
		off += size
		if r == utf8.RuneError && size == 1 {
			u16++
			continue
		}
		u16 += utf16.RuneLen(r)
	}
	return off
}

// rangeFromBytes maps exclusive-end byte span to an LSP Range (UTF-16).
func rangeFromBytes(text string, start, end int, lines *grammar.LineIndex) protocol.Range {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(text) {
		end = len(text)
	}
	if lines == nil {
		lines = grammar.NewLineIndex(text)
	}
	sl, sc := lines.LineColumnAt(start)
	el, ec := lines.LineColumnAt(end)
	return protocol.Range{
		Start: protocol.Position{Line: uint32(max(0, sl-1)), Character: uint32(byteColToUTF16(text, sl, sc))},
		End:   protocol.Position{Line: uint32(max(0, el-1)), Character: uint32(byteColToUTF16(text, el, ec))},
	}
}

func byteColToUTF16(text string, line1, byteCol int) int {
	if line1 < 1 {
		line1 = 1
	}
	off := 0
	cur := 1
	for off < len(text) && cur < line1 {
		if text[off] == '\n' {
			cur++
		}
		off++
	}
	target := off + byteCol
	if target > len(text) {
		target = len(text)
	}
	u16 := 0
	for off < target {
		r, size := utf8.DecodeRuneInString(text[off:])
		if r == '\n' {
			break
		}
		off += size
		if r == utf8.RuneError && size == 1 {
			u16++
			continue
		}
		u16 += utf16.RuneLen(r)
	}
	return u16
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
