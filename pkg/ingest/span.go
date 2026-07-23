package ingest

// Span is a half-open UTF-8 byte range [StartByte, EndByte) in a source buffer.
// Offsets match tree-sitter StartByte/EndByte (not rune indexes). Used for
// tokens, match roots, pattern captures, hyperlink leaves, and edits. Text is
// always derived from the original file []byte — never stored on the span.
type Span struct {
	StartByte uint32
	EndByte   uint32
}

// Text returns src[StartByte:EndByte].
func (s Span) Text(src []byte) string {
	b := s.Bytes(src)
	if b == nil {
		return ""
	}
	return string(b)
}

// Bytes returns src[StartByte:EndByte], or nil if out of range / empty invalid.
func (s Span) Bytes(src []byte) []byte {
	if src == nil || s.EndByte > uint32(len(src)) || s.StartByte > s.EndByte {
		return nil
	}
	return src[s.StartByte:s.EndByte]
}

// Empty reports whether the span has zero length (StartByte == EndByte).
func (s Span) Empty() bool { return s.StartByte >= s.EndByte }

// Eq reports whether s.Text(src) equals want without an intermediate string
// when lengths differ.
func (s Span) Eq(src []byte, want string) bool {
	b := s.Bytes(src)
	return len(b) == len(want) && string(b) == want
}
