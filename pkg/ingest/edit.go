package ingest

import "strings"

// ReplaceSpan builds one leaf/text replacement edit at sp in file.
// file is stored without a leading "./". Empty or inverted spans are skipped
// (returns a zero Edit with empty File).
func ReplaceSpan(file string, sp Span, newText string) Edit {
	if sp.Empty() || sp.StartByte > sp.EndByte {
		return Edit{}
	}
	return Edit{
		File:    strings.TrimPrefix(file, "./"),
		Span:    sp,
		NewText: newText,
	}
}

// ReplaceSpans builds edits that write newText over each span in file.
// Skips empty/inverted spans. Order follows spans.
func ReplaceSpans(file string, spans []Span, newText string) []Edit {
	if len(spans) == 0 {
		return nil
	}
	file = strings.TrimPrefix(file, "./")
	edits := make([]Edit, 0, len(spans))
	for _, sp := range spans {
		if sp.Empty() || sp.StartByte > sp.EndByte {
			continue
		}
		edits = append(edits, Edit{File: file, Span: sp, NewText: newText})
	}
	return edits
}

// AppendReplaceSpan appends a ReplaceSpan edit when the span is non-empty.
func AppendReplaceSpan(edits []Edit, file string, sp Span, newText string) []Edit {
	e := ReplaceSpan(file, sp, newText)
	if e.File == "" {
		return edits
	}
	return append(edits, e)
}
