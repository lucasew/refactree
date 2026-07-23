package pattern

import (
	"github.com/lucasew/refactree/pkg/ingest"
)

// ReplacementRefs collects product refs introduced by a replacement IR
// (kind "ref" nodes and @refs inside template text). Used for import ensure.
func ReplacementRefs(n Node) []string {
	var out []string
	var walk func(Node)
	walk = func(n Node) {
		switch n.Kind {
		case "ref":
			if n.Ref != "" {
				out = append(out, n.Ref)
			}
		case "template":
			out = append(out, refsInTemplateText(n.Text)...)
		}
		if n.Callee != nil {
			walk(*n.Callee)
		}
		for _, a := range n.Args {
			walk(a)
		}
	}
	walk(n)
	return out
}

func refsInTemplateText(tmpl string) []string {
	var out []string
	i := 0
	for i < len(tmpl) {
		if tmpl[i] != '@' {
			i++
			continue
		}
		ref, end, ok := scanTemplateRef(tmpl, i)
		if !ok {
			i++
			continue
		}
		out = append(out, ref)
		i = end
	}
	return out
}

// ImportNeedsForRule collects language import needs from static refs in the
// replacement. Unknown languages or refs yield no needs.
func ImportNeedsForRule(lang string, r Rule) []ingest.ImportNeed {
	h, ok := ingest.ImportHygieneForLanguage(lang)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var needs []ingest.ImportNeed
	for _, ref := range ReplacementRefs(r.Replacement) {
		n, ok := h.NeedsFromRef(ref)
		if !ok || n.ImportPath == "" || seen[n.ImportPath] {
			continue
		}
		seen[n.ImportPath] = true
		needs = append(needs, n)
	}
	return needs
}

// WithImportHygiene appends localized import-section edits for needs onto
// siteEdits. Import edits are computed on the original source so their offsets
// stay in the preamble; ApplyEdits (high offsets first) then applies body
// sites before the import region.
func WithImportHygiene(fileRel, lang string, source []byte, siteEdits []ingest.Edit, needs []ingest.ImportNeed) []ingest.Edit {
	if len(siteEdits) == 0 || len(needs) == 0 {
		return siteEdits
	}
	h, ok := ingest.ImportHygieneForLanguage(lang)
	if !ok {
		return siteEdits
	}
	imp := h.EnsureImportEdits(fileRel, source, needs)
	if len(imp) == 0 {
		return siteEdits
	}
	return append(append([]ingest.Edit(nil), siteEdits...), imp...)
}
