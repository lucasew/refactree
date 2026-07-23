package java

import (
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

func init() {
	ingest.RegisterImportHygiene(javaImportHygiene{})
}

// javaImportHygiene: prune single-type unused imports. Star/on-demand never.
type javaImportHygiene struct{}

func (javaImportHygiene) Language() string { return "java" }

func (javaImportHygiene) NeedsFromRef(string) (ingest.ImportNeed, bool) {
	return ingest.ImportNeed{}, false
}

func (javaImportHygiene) EnsureImportEdits(string, []byte, []ingest.ImportNeed) []ingest.Edit {
	return nil
}

// PruneNamedUnusedEdits removes unused single-type imports. Never star ("*").
// OnlyCandidates are full import statement texts when non-empty.
func (javaImportHygiene) PruneNamedUnusedEdits(fileRel string, content []byte, opts ingest.PruneImportOpts) []ingest.Edit {
	if len(content) == 0 {
		return nil
	}
	var want map[string]bool
	if len(opts.OnlyCandidates) > 0 {
		want = map[string]bool{}
		for _, s := range opts.OnlyCandidates {
			s = strings.TrimSpace(s)
			if s != "" {
				want[s] = true
			}
		}
		if len(want) == 0 {
			return nil
		}
	}
	specs := parseJavaImportSpecs(content)
	if len(specs) == 0 {
		return nil
	}
	masked := append([]byte(nil), content...)
	for _, sp := range opts.MaskSpans {
		ingest.MaskNonNewlinesInPlace(masked, int(sp.StartByte), int(sp.EndByte))
	}
	for _, spec := range specs {
		ingest.MaskNonNewlinesInPlace(masked, int(spec.startByte), int(spec.endByte))
	}
	rest := string(masked)

	var edits []ingest.Edit
	for _, spec := range specs {
		if want != nil && !want[spec.stmt] {
			continue
		}
		// Named only: never star / on-demand barrels.
		if spec.local == "" || spec.local == "*" {
			continue
		}
		if javaIdentUsed(rest, spec.local) {
			continue
		}
		edits = append(edits, ingest.Edit{
			File:    fileRel,
			Span:    ingest.Span{StartByte: uint32(spec.startByte), EndByte: uint32(spec.endByte)},
			NewText: "",
		})
	}
	return edits
}
