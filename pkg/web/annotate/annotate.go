// Package annotate turns ingested source into ordered HTML segments.
// Relations and entities become hyperlinks/anchors keyed by reference strings.
package annotate

import (
	"cmp"
	"html"
	"slices"
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

// Options controls how Build maps references (e.g. provider-scoped rewrite).
type Options struct {
	// MapRef rewrites an ingest reference before anchors/links are built.
	// Nil keeps references as ingest produced them (path:./file::sym).
	MapRef func(ref string) string
}

// Build produces segments for one file's source using ingest facts for that path.
// filePath is the ingest-relative path (e.g. "main.go").
// codeURL builds a browser URL for a reference string.
func Build(source []byte, filePath string, result *ingest.Result, codeURL func(ref string) string) []Segment {
	return BuildWithOptions(source, filePath, result, codeURL, Options{})
}

// BuildWithOptions is Build plus optional reference remapping for provider views.
func BuildWithOptions(source []byte, filePath string, result *ingest.Result, codeURL func(ref string) string, opts Options) []Segment {
	if result == nil || len(source) == 0 {
		return []Segment{{Text: string(source)}}
	}

	mapRef := opts.MapRef
	if mapRef == nil {
		mapRef = func(ref string) string { return ref }
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
		displayRef := mapRef(ent.Reference)
		spans = append(spans, Span{
			Start:     ent.StartByte,
			End:       ent.EndByte,
			ID:        symbolAnchorID(displayRef),
			IsDef:     true,
			Reference: displayRef,
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
		displayRef := mapRef(alias.Reference)
		target := alias.Target
		if target != "" {
			target = mapRef(target)
		}
		href := ""
		if target != "" && codeURL != nil {
			href = codeURL(target)
		}
		// Import aliases are defs for display, but only anchor when they name a symbol
		// (avoids every import sharing path:./file as the same element id).
		titleRef := displayRef
		if target != "" {
			titleRef = target
		}
		spans = append(spans, Span{
			Start:     alias.StartByte,
			End:       alias.EndByte,
			Href:      href,
			ID:        symbolAnchorID(displayRef),
			IsLink:    href != "",
			IsDef:     true,
			Reference: titleRef,
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
		target := rel.Target
		if target != "" {
			target = mapRef(target)
		}
		href := ""
		if target != "" && codeURL != nil {
			href = codeURL(target)
		}
		spans = append(spans, Span{
			Start:     rel.StartByte,
			End:       rel.EndByte,
			Href:      href,
			IsLink:    href != "",
			Reference: target,
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

	slices.SortFunc(spans, func(a, b Span) int {
		if c := cmp.Compare(a.Start, b.Start); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Priority, a.Priority); c != 0 {
			return c
		}
		return cmp.Compare(b.End, a.End)
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

	slices.SortFunc(chosen, func(a, b Span) int {
		return cmp.Compare(a.Start, b.Start)
	})

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

// symbolAnchorID only emits an id for symbol refs (provider:path::symbol).
func symbolAnchorID(ref string) string {
	if !strings.Contains(ref, "::") {
		return ""
	}
	return AnchorID(ref)
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
