package ingestgo

import (
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

func init() {
	ingest.RegisterImportHygiene(goImportHygiene{})
}

type goImportHygiene struct{}

func (goImportHygiene) Language() string { return "go" }

// NeedsFromRef maps go:import/path::Symbol to import "import/path".
// Non-go providers and empty paths are ignored.
func (goImportHygiene) NeedsFromRef(ref string) (ingest.ImportNeed, bool) {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "@"))
	if ref == "" {
		return ingest.ImportNeed{}, false
	}
	r := ingest.ParseReference(ref)
	if r.Provider != "go" {
		return ingest.ImportNeed{}, false
	}
	path := strings.TrimSpace(r.Path)
	if path == "" || path == "." || path == "./" {
		return ingest.ImportNeed{}, false
	}
	// Skip path-shaped go refs that look like local files (not import paths).
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return ingest.ImportNeed{}, false
	}
	return ingest.ImportNeed{ImportPath: path}, true
}

func (goImportHygiene) EnsureImportEdits(fileRel string, content []byte, needs []ingest.ImportNeed) []ingest.Edit {
	if len(needs) == 0 || len(content) == 0 {
		return nil
	}
	var paths []string
	seen := map[string]bool{}
	for _, n := range needs {
		p := strings.TrimSpace(n.ImportPath)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil
	}
	next := EnsureImportsInContent(string(content), paths)
	return ingest.EditContentDiff(fileRel, content, []byte(next))
}

// PruneNamedUnusedEdits drops unused named import specs (never blank/dot).
// OnlyCandidates are Go import paths when non-empty.
func (goImportHygiene) PruneNamedUnusedEdits(fileRel string, content []byte, opts ingest.PruneImportOpts) []ingest.Edit {
	if len(content) == 0 {
		return nil
	}
	var want map[string]bool
	if len(opts.OnlyCandidates) > 0 {
		want = map[string]bool{}
		for _, p := range opts.OnlyCandidates {
			if p != "" {
				want[p] = true
			}
		}
		if len(want) == 0 {
			return nil
		}
	}
	specs := parseGoImportSpecs(content)
	if len(specs) == 0 {
		return nil
	}
	masked := append([]byte(nil), content...)
	for _, sp := range opts.MaskSpans {
		ingest.MaskNonNewlinesInPlace(masked, int(sp.StartByte), int(sp.EndByte))
	}
	seenBlocks := map[int]bool{}
	for _, spec := range specs {
		if spec.blockStart >= 0 && spec.blockEnd > 0 {
			if !seenBlocks[spec.blockStart] {
				ingest.MaskNonNewlinesInPlace(masked, spec.blockStart, spec.blockEnd)
				seenBlocks[spec.blockStart] = true
			}
			continue
		}
		ingest.MaskNonNewlinesInPlace(masked, spec.lineStart, spec.lineEnd)
	}
	bodyText := string(masked)
	var edits []ingest.Edit
	blockCounts := map[int]int{}
	blockRemove := map[int]int{}
	for _, spec := range specs {
		if spec.blockStart >= 0 {
			blockCounts[spec.blockStart]++
		}
	}
	for _, spec := range specs {
		// Named only: skip blank and dot (barrel / side-effect forms).
		if spec.local == "" || spec.local == "." || spec.local == "_" {
			continue
		}
		if want != nil && !want[spec.path] {
			continue
		}
		if goIdentUsed(bodyText, spec.local) {
			continue
		}
		edits = append(edits, ingest.Edit{
			File:    fileRel,
			Span:    ingest.Span{StartByte: uint32(spec.lineStart), EndByte: uint32(spec.lineEnd)},
			NewText: "",
		})
		if spec.blockStart >= 0 {
			blockRemove[spec.blockStart]++
		}
	}
	for blockStart, removed := range blockRemove {
		if removed == 0 || removed < blockCounts[blockStart] {
			continue
		}
		blockEnd := 0
		for _, spec := range specs {
			if spec.blockStart == blockStart && spec.blockEnd > 0 {
				blockEnd = spec.blockEnd
				break
			}
		}
		if blockEnd <= blockStart {
			continue
		}
		filtered := edits[:0]
		for _, e := range edits {
			if int(e.StartByte) >= blockStart && int(e.EndByte) <= blockEnd && e.NewText == "" {
				continue
			}
			filtered = append(filtered, e)
		}
		edits = filtered
		start, end := blockStart, blockEnd
		if start > 0 && content[start-1] == '\n' {
			start--
		}
		edits = append(edits, ingest.Edit{
			File:    fileRel,
			Span:    ingest.Span{StartByte: uint32(start), EndByte: uint32(end)},
			NewText: "",
		})
	}
	return edits
}
