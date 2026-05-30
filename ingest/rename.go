package ingest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

// Edit describes a text replacement in a source file.
type Edit struct {
	File      string // path relative to the ingest root
	StartByte uint32
	EndByte   uint32
	NewText   string
}

// Rename computes the edits needed to rename or move a symbol from sourceRef to
// destRef. Same-file operations are treated as renames; cross-file operations
// are treated as moves.
func Rename(dir, sourceRef, destRef string) ([]Edit, error) {
	src := ParseReference(sourceRef)
	dst := ParseReference(destRef)
	if src.Symbol == "" || dst.Symbol == "" {
		return nil, fmt.Errorf("source and destination references must include symbols")
	}

	result, err := Ingest(dir)
	if err != nil {
		return nil, err
	}

	src, err = canonicalSourceReference(dir, result, src)
	if err != nil {
		return nil, err
	}
	dst, err = canonicalDestinationReference(dir, result, src, dst)
	if err != nil {
		return nil, err
	}

	sourceRef = src.String()
	destRef = dst.String()

	sourceEntity, ok := findEntityByReference(result, sourceRef)
	if !ok {
		return nil, fmt.Errorf("no entity found for reference %s", sourceRef)
	}

	if src.Path != dst.Path {
		if src.Symbol != dst.Symbol {
			return nil, fmt.Errorf("cross-file move with symbol rename is not supported yet")
		}
		srcLang := languageForRefPath(result, src.Path)
		driver, ok := moveDriverForLanguage(srcLang)
		if !ok {
			return nil, fmt.Errorf("cross-file move is not supported for language %q", srcLang)
		}
		return driver.PlanCrossFileMove(dir, result, src, dst, sourceEntity)
	}

	return planSymbolRename(result, sourceRef, dst.Symbol)
}

func findEntityByReference(result *Result, ref string) (Entity, bool) {
	for _, ent := range result.Entities {
		if ent.Reference == ref {
			return ent, true
		}
	}
	return Entity{}, false
}

func planSymbolRename(result *Result, sourceRef, destSymbol string) ([]Edit, error) {
	var edits []Edit

	// 1. Rename the entity definition.
	for _, ent := range result.Entities {
		if ent.Reference == sourceRef {
			ref := ParseReference(ent.Reference)
			edits = append(edits, Edit{
				File:      strings.TrimPrefix(ref.Path, "./"),
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
				NewText:   destSymbol,
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
				NewText:   destSymbol,
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
				NewText:   destSymbol,
			})
		}
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("no entity found for reference %s", sourceRef)
	}

	return edits, nil
}

func planGoCrossFileMove(dir string, result *Result, src, dst Reference, sourceEntity Entity) ([]Edit, error) {
	srcRel := strings.TrimPrefix(src.Path, "./")
	dstRel := strings.TrimPrefix(dst.Path, "./")
	if path.Dir(srcRel) != path.Dir(dstRel) {
		return nil, fmt.Errorf("cross-directory Go move is not supported yet")
	}

	pkg, declText, removeStart, removeEnd, err := extractGoDeclFromEntity(filepath.Join(dir, srcRel), sourceEntity.StartByte)
	if err != nil {
		return nil, err
	}

	dstPath := filepath.Join(dir, dstRel)
	dstContent, err := os.ReadFile(dstPath)
	dstExists := true
	if err != nil {
		if os.IsNotExist(err) {
			dstExists = false
		} else {
			return nil, fmt.Errorf("reading %s: %w", dstRel, err)
		}
	}

	insertAt := uint32(0)
	insertText := ""
	if dstExists {
		insertAt = uint32(len(dstContent))
		if len(dstContent) > 0 && dstContent[len(dstContent)-1] != '\n' {
			insertText += "\n"
		}
		if len(dstContent) > 0 {
			insertText += "\n"
		}
		insertText += declText + "\n"
	} else {
		insertText = fmt.Sprintf("package %s\n\n%s\n", pkg, declText)
	}

	return []Edit{
		{
			File:      srcRel,
			StartByte: removeStart,
			EndByte:   removeEnd,
			NewText:   "",
		},
		{
			File:      dstRel,
			StartByte: insertAt,
			EndByte:   insertAt,
			NewText:   insertText,
		},
	}, nil
}

func extractGoDeclFromEntity(filePath string, nameStart uint32) (pkg, declText string, start, end uint32, err error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", 0, 0, err
	}

	lang, ok := grammar.GetByExtension(filePath)
	if !ok {
		return "", "", 0, 0, fmt.Errorf("unsupported language for %s", filePath)
	}

	parser := grammar.NewParser()
	defer parser.Delete()
	if !parser.SetLanguage(lang) {
		return "", "", 0, 0, fmt.Errorf("failed to set language for %s", filePath)
	}

	tree := parser.ParseString(string(source))
	defer tree.Delete()
	root := tree.RootNode()

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Type() == "package_clause" {
			if id := childByType(child, "package_identifier"); id != nil {
				pkg = nodeText(id, source)
			}
		}
	}

	declNode := findDeclContaining(root, nameStart, "go")
	if declNode == nil {
		return "", "", 0, 0, fmt.Errorf("declaration not found in %s", filePath)
	}

	start = declNode.StartByte()
	end = declNode.EndByte()
	declText = string(source[start:end])

	// Also remove up to two trailing newlines to avoid leaving extra gaps.
	removeEnd := end
	for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
		removeEnd++
		if removeEnd-end >= 2 {
			break
		}
	}

	return pkg, declText, start, removeEnd, nil
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
