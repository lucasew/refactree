// Package annotate turns ingested source into ordered HTML segments.
// Relations and entities become hyperlinks/anchors keyed by reference strings.
package annotate

import (
	"html"
	"sort"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// Segment is one contiguous slice of source with optional link/anchor metadata.
type Segment struct {
	Text      string
	Href      string // non-empty → render as <a>
	ID        string // non-empty → element id (definition anchor)
	IsLink    bool
	IsDef     bool
	Reference string // symbol/file reference this segment points to or defines
}

// Span marks a byte range in source with link/anchor intent.
type Span struct {
	Start     uint32
	End       uint32
	Href      string
	ID        string
	IsLink    bool
	IsDef     bool
	Reference string
	Priority  int // higher wins on overlap
}

// Build produces segments for one file's source using ingest facts for that path.
// filePath is the ingest-relative path (e.g. "main.go").
// codeURL builds a browser URL for a reference string.
func Build(source []byte, filePath string, result *ingest.Result, codeURL func(ref string) string) []Segment {
	if result == nil || len(source) == 0 {
		return []Segment{{Text: string(source)}}
	}

	normPath := normalizePath(filePath)
	var spans []Span

	for _, ent := range result.Entities {
		ref := ingest.ParseReference(ent.Reference)
		if normalizePath(ref.Path) != normPath {
			continue
		}
		if ent.StartByte >= ent.EndByte || int(ent.EndByte) > len(source) {
			continue
		}
		spans = append(spans, Span{
			Start:     ent.StartByte,
			End:       ent.EndByte,
			ID:        AnchorID(ent.Reference),
			IsDef:     true,
			Reference: ent.Reference,
			Priority:  2,
		})
	}

	for _, alias := range result.Aliases {
		ref := ingest.ParseReference(alias.Reference)
		if normalizePath(ref.Path) != normPath {
			continue
		}
		if alias.StartByte >= alias.EndByte || int(alias.EndByte) > len(source) {
			continue
		}
		href := ""
		if alias.Target != "" && codeURL != nil {
			href = codeURL(alias.Target)
		}
		spans = append(spans, Span{
			Start:     alias.StartByte,
			End:       alias.EndByte,
			Href:      href,
			ID:        AnchorID(alias.Reference),
			IsLink:    href != "",
			IsDef:     true,
			Reference: alias.Reference,
			Priority:  2,
		})
	}

	for _, rel := range result.Relations {
		ref := ingest.ParseReference(rel.Reference)
		if normalizePath(ref.Path) != normPath {
			continue
		}
		if rel.StartByte >= rel.EndByte || int(rel.EndByte) > len(source) {
			continue
		}
		href := ""
		if rel.Target != "" && codeURL != nil {
			href = codeURL(rel.Target)
		}
		spans = append(spans, Span{
			Start:     rel.StartByte,
			End:       rel.EndByte,
			Href:      href,
			IsLink:    href != "",
			Reference: rel.Target,
			Priority:  1,
		})
	}

	return Apply(source, spans)
}

// Apply merges non-overlapping highest-priority spans into ordered segments.
func Apply(source []byte, spans []Span) []Segment {
	if len(spans) == 0 {
		return []Segment{{Text: string(source)}}
	}

	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Start != spans[j].Start {
			return spans[i].Start < spans[j].Start
		}
		if spans[i].Priority != spans[j].Priority {
			return spans[i].Priority > spans[j].Priority
		}
		return spans[i].End > spans[j].End
	})

	chosen := make([]Span, 0, len(spans))
	var cursor uint32
	for _, s := range spans {
		if s.Start < cursor || s.End <= s.Start {
			continue
		}
		if int(s.End) > len(source) {
			continue
		}
		chosen = append(chosen, s)
		cursor = s.End
	}

	sort.Slice(chosen, func(i, j int) bool { return chosen[i].Start < chosen[j].Start })

	var out []Segment
	var pos uint32
	for _, s := range chosen {
		if s.Start > pos {
			out = append(out, Segment{Text: string(source[pos:s.Start])})
		}
		out = append(out, Segment{
			Text:      string(source[s.Start:s.End]),
			Href:      s.Href,
			ID:        s.ID,
			IsLink:    s.IsLink,
			IsDef:     s.IsDef,
			Reference: s.Reference,
		})
		pos = s.End
	}
	if int(pos) < len(source) {
		out = append(out, Segment{Text: string(source[pos:])})
	}
	return out
}

// AnchorID turns a reference into a safe HTML element id.
func AnchorID(ref string) string {
	if ref == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("sym-")
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Escape segments for direct template use when not using html/template auto-escape on fields.
func EscapeText(s string) string {
	return html.EscapeString(s)
}

func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "./")
	return filepathToSlash(p)
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
