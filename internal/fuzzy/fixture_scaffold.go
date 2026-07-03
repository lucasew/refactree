package fuzzy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// ScaffoldMvFixture copies touched files and writes op.json under destDir.
func ScaffoldMvFixture(workRoot, destDir, source, destination string, edits []ingest.Edit) error {
	if err := os.MkdirAll(filepath.Join(destDir, "input"), 0o755); err != nil {
		return err
	}
	seen := map[string]bool{}
	copyOne := func(rel string) error {
		rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
		if rel == "" || seen[rel] {
			return nil
		}
		seen[rel] = true
		src := filepath.Join(workRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(src)
		if err != nil {
			return nil // skip missing
		}
		dst := filepath.Join(destDir, "input", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	}
	for _, e := range edits {
		if err := copyOne(e.File); err != nil {
			return err
		}
	}
	for _, ref := range []string{source, destination} {
		r := ingest.ParseReference(ref)
		if r.Provider == "path" && r.Path != "" && r.Path != "./" {
			_ = copyOne(r.Path)
		}
	}
	op := map[string]string{"source": source, "destination": destination}
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destDir, "op.json"), append(data, '\n'), 0o644)
}
