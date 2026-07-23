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
