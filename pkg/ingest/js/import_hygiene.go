package js

import (
	"github.com/lucasew/refactree/pkg/ingest"
)

func init() {
	ingest.RegisterImportHygiene(jsImportHygiene{})
}

// jsImportHygiene: prune named-only unused imports. Ensure is not registered
// for JS rewrite refs yet (NeedsFromRef always false).
type jsImportHygiene struct{}

func (jsImportHygiene) Language() string { return "javascript" }

func (jsImportHygiene) NeedsFromRef(string) (ingest.ImportNeed, bool) {
	return ingest.ImportNeed{}, false
}

func (jsImportHygiene) EnsureImportEdits(string, []byte, []ingest.ImportNeed) []ingest.Edit {
	return nil
}

// PruneNamedUnusedEdits removes pure named import statements that are unused
// after masking opts.MaskSpans. Never removes side-effect, default, or
// namespace (barrel) imports. OnlyCandidates are full import statement texts
// (DeclExtract.Imports) when non-empty.
func (jsImportHygiene) PruneNamedUnusedEdits(fileRel string, content []byte, opts ingest.PruneImportOpts) []ingest.Edit {
	if len(content) == 0 {
		return nil
	}
	var want map[string]bool
	if len(opts.OnlyCandidates) > 0 {
		want = map[string]bool{}
		for _, s := range opts.OnlyCandidates {
			if s != "" {
				want[s] = true
			}
		}
		if len(want) == 0 {
			return nil
		}
	}

	pf, err := ingest.ParseSource(content, fileRel, "")
	if err != nil {
		return nil
	}
	defer pf.Close()
	stmts := parseJSImportStatements(pf.Root, content)
	if len(stmts) == 0 {
		return nil
	}

	masked := append([]byte(nil), content...)
	for _, sp := range opts.MaskSpans {
		ingest.MaskNonNewlinesInPlace(masked, int(sp.StartByte), int(sp.EndByte))
	}
	for _, stmt := range stmts {
		ingest.MaskNonNewlinesInPlace(masked, int(stmt.startByte), int(stmt.endByte))
	}
	restText := string(masked)

	var edits []ingest.Edit
	for _, stmt := range stmts {
		if want != nil && !want[stmt.text] {
			continue
		}
		// Named-only: refuse barrels (side-effect / default / namespace).
		if !stmt.namedOnly || len(stmt.namedLocals) == 0 {
			continue
		}
		stillUsed := false
		for _, local := range stmt.namedLocals {
			if jsIdentUsed(restText, local) {
				stillUsed = true
				break
			}
		}
		if stillUsed {
			continue
		}
		removeEnd := stmt.endByte
		for removeEnd < uint32(len(content)) && content[removeEnd] == '\n' {
			removeEnd++
			break
		}
		edits = append(edits, ingest.Edit{
			File:    fileRel,
			Span:    ingest.Span{StartByte: stmt.startByte, EndByte: removeEnd},
			NewText: "",
		})
	}
	return edits
}
