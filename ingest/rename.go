package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Edit describes a text replacement in a source file.
type Edit struct {
	File      string // path relative to the ingest root
	StartByte uint32
	EndByte   uint32
	NewText   string
}

// Rename computes the edits needed to rename a symbol from sourceRef to destRef.
// Both references must use the same provider and path; only the symbol changes.
func Rename(dir, sourceRef, destRef string) ([]Edit, error) {
	src := ParseReference(sourceRef)
	dst := ParseReference(destRef)
	sourceRef = src.String()
	destRef = dst.String()

	result, err := Ingest(dir)
	if err != nil {
		return nil, err
	}

	var edits []Edit

	// 1. Rename the entity definition.
	for _, ent := range result.Entities {
		if ent.Reference == sourceRef {
			ref := ParseReference(ent.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
				NewText:   dst.Symbol,
			})
		}
	}

	// 2. Rename at every call site that targets this entity.
	for _, rel := range result.Relations {
		if rel.Target == sourceRef {
			// Usage through explicit import alias bindings (`as`) should keep
			// the local alias name unchanged when renaming the imported symbol.
			if rel.ViaImportAlias {
				continue
			}
			ref := ParseReference(rel.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: rel.StartByte,
				EndByte:   rel.EndByte,
				NewText:   dst.Symbol,
			})
		}
	}

	// 3. Rename in import bindings that target this entity.
	for _, alias := range result.Aliases {
		if alias.Target == sourceRef {
			ref := ParseReference(alias.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: alias.StartByte,
				EndByte:   alias.EndByte,
				NewText:   dst.Symbol,
			})
		}
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("no entity found for reference %s", sourceRef)
	}

	if src.Symbol == "" || dst.Symbol == "" {
		return nil, fmt.Errorf("source and destination references must include symbols")
	}

	return edits, nil
}

// ApplyEdits applies text replacements to files under dir.
// Edits are grouped by file and applied from last to first byte offset
// so that earlier offsets remain valid.
func ApplyEdits(dir string, edits []Edit) error {
	byFile := map[string][]Edit{}
	for _, e := range edits {
		byFile[e.File] = append(byFile[e.File], e)
	}

	for file, fileEdits := range byFile {
		path := filepath.Join(dir, file)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", file, err)
		}

		sort.Slice(fileEdits, func(i, j int) bool {
			return fileEdits[i].StartByte > fileEdits[j].StartByte
		})

		for _, e := range fileEdits {
			if int(e.EndByte) > len(content) {
				return fmt.Errorf("edit out of bounds in %s: end %d > len %d", file, e.EndByte, len(content))
			}
			content = append(content[:e.StartByte], append([]byte(e.NewText), content[e.EndByte:]...)...)
		}

		if err := os.WriteFile(path, content, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}
	}

	return nil
}
