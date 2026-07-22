package ingest

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lucasew/refactree/pkg/projectfs"
)

// StageEdits applies edits in memory on top of base FS (nil = disk) under dir.
// Returns an overlay with staged file contents (absolute paths). Does not write disk.
func StageEdits(dir string, base projectfs.FS, edits []Edit) (*projectfs.Overlay, error) {
	if base == nil {
		base = projectfs.OS{}
	}
	rootAbs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	ov := projectfs.NewOverlay(base)

	byFile := map[string][]Edit{}
	for _, e := range edits {
		byFile[e.File] = append(byFile[e.File], e)
	}

	for file, fileEdits := range byFile {
		rel := strings.TrimPrefix(filepath.ToSlash(file), "./")
		abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
		content, err := ov.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				createAllowed := true
				for _, e := range fileEdits {
					if e.EndByte != 0 {
						createAllowed = false
						break
					}
				}
				if !createAllowed {
					return nil, fmt.Errorf("reading %s: %w", file, err)
				}
				content = []byte{}
			} else {
				return nil, fmt.Errorf("reading %s: %w", file, err)
			}
		}

		slices.SortFunc(fileEdits, func(a, b Edit) int {
			return cmp.Compare(b.StartByte, a.StartByte)
		})

		for _, e := range fileEdits {
			if int(e.EndByte) > len(content) {
				return nil, fmt.Errorf("edit out of bounds in %s: end %d > len %d", file, e.EndByte, len(content))
			}
			content = append(content[:e.StartByte], append([]byte(e.NewText), content[e.EndByte:]...)...)
		}
		ov.Set(abs, content)
	}
	return ov, nil
}

// CommitOverlay writes overlay entries that differ from base to disk.
// Used after a successful staged validation. Empty content removes the file.
func CommitOverlay(ov *projectfs.Overlay) error {
	if ov == nil {
		return nil
	}
	for _, abs := range ov.Paths() {
		content, ok := ov.Get(abs)
		if !ok {
			continue
		}
		if len(content) == 0 {
			_ = os.Remove(abs)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", abs, err)
		}
		if err := os.WriteFile(abs, content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", abs, err)
		}
	}
	return nil
}

// ValidateStagedProject materializes the project through staged FS and checks
// that entity identifier spans still match symbol leaves (basic post-edit gate).
// Clears the process extract cache first so overlay content is not masked.
func ValidateStagedProject(dir string, fsys projectfs.FS) error {
	ClearExtractCache()
	result, err := MaterializeSource(ExtractSource{
		Kind:      ExtractDir,
		Root:      dir,
		Recursive: true,
		FS:        fsys,
	}, MaterializeOptions{ExpandImports: true, FS: fsys})
	if err != nil {
		return err
	}
	rootAbs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, ent := range result.Atoms {
		ref := ParseReference(ent.Reference)
		if ref.Name == "" {
			continue
		}
		rel := strings.TrimPrefix(ref.Path, "./")
		abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
		data, err := fsys.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("staged missing %s: %w", rel, err)
		}
		if int(ent.EndByte) > len(data) || ent.StartByte > ent.EndByte {
			return fmt.Errorf("staged bad span for %s", ent.Reference)
		}
		got := string(data[ent.StartByte:ent.EndByte])
		want := AtomName(ref.Name)
		if got != want {
			return fmt.Errorf("staged entity text mismatch %s: %q != %q", ent.Reference, got, want)
		}
	}
	return nil
}

// ProjectResultFS is ProjectResult reading through fsys (nil = disk).
func ProjectResultFS(root string, fsys projectfs.FS) (*Result, error) {
	return MaterializeSource(ExtractSource{
		Kind:      ExtractDir,
		Root:      root,
		Recursive: true,
		FS:        fsys,
	}, MaterializeOptions{ExpandImports: true, FS: fsys})
}
