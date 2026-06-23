package ingestgo

import (
	"fmt"
	"os"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
	"github.com/lucasew/refactree/pkg/ingest"
)

func init() {
	ingest.RegisterMoveDriver("go", moveDriver{})
}

type moveDriver struct{}

func (moveDriver) Language() string { return "go" }

func (moveDriver) ExtractDecl(filePath string, entity ingest.Entity) (ingest.DeclExtract, error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return ingest.DeclExtract{}, err
	}

	lang, ok := grammar.GetByExtension(filePath)
	if !ok {
		return ingest.DeclExtract{}, fmt.Errorf("unsupported language for %s", filePath)
	}

	parser := grammar.NewParser()
	defer parser.Delete()
	if !parser.SetLanguage(lang) {
		return ingest.DeclExtract{}, fmt.Errorf("failed to set language for %s", filePath)
	}

	tree := parser.ParseString(string(source))
	defer tree.Delete()
	root := tree.RootNode()

	// Extract package name.
	pkg := ""
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Type() == "package_clause" {
			if id := ingest.ChildByType(child, "package_identifier"); id != nil {
				pkg = ingest.NodeText(id, source)
			}
		}
	}

	result := findGoDecl(root, entity.StartByte)
	if result == nil {
		return ingest.DeclExtract{}, fmt.Errorf("declaration not found in %s", filePath)
	}

	var declText string
	var removeStart, removeEnd uint32

	if result.Grouped {
		// Grouped type declaration: extract just the matching type_spec.
		// The output should be a standalone "type X struct {...}" declaration.
		// Dedent by one tab level since the spec was inside type (...).
		spec := result.TypeSpec
		specText := string(source[spec.StartByte():spec.EndByte()])
		declText = "type " + dedentOnce(specText)
		removeStart = spec.StartByte()
		removeEnd = spec.EndByte()
		// Remove trailing whitespace/newlines up to the next type_spec or ')'.
		for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r' || source[removeEnd] == '\t' || source[removeEnd] == ' ') {
			removeEnd++
		}
	} else {
		start := result.Node.StartByte()
		end := result.Node.EndByte()
		declText = string(source[start:end])
		removeStart = start
		removeEnd = end
		// Remove up to two trailing newlines.
		for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
			removeEnd++
			if removeEnd-end >= 2 {
				break
			}
		}
	}

	return ingest.DeclExtract{
		Preamble:    pkg,
		DeclText:    declText,
		RemoveStart: removeStart,
		RemoveEnd:   removeEnd,
	}, nil
}

func (moveDriver) InsertDecl(dstRelPath string, dstContent []byte, decl ingest.DeclExtract) ingest.Edit {
	insertAt := uint32(0)
	insertText := ""

	if dstContent != nil {
		insertAt = uint32(len(dstContent))
		if len(dstContent) > 0 && dstContent[len(dstContent)-1] != '\n' {
			insertText += "\n"
		}
		if len(dstContent) > 0 {
			insertText += "\n"
		}
		insertText += decl.DeclText + "\n"
	} else {
		insertText = fmt.Sprintf("package %s\n\n%s\n", decl.Preamble, decl.DeclText)
	}

	return ingest.Edit{
		File:      dstRelPath,
		StartByte: insertAt,
		EndByte:   insertAt,
		NewText:   insertText,
	}
}

func (moveDriver) RewriteImports(fileRelPath string, content []byte, result *ingest.Result, oldRef, newRef ingest.Reference) []ingest.Edit {
	// Determine old and new directory paths.
	oldPath := strings.TrimPrefix(oldRef.Path, "./")
	newPath := strings.TrimPrefix(newRef.Path, "./")

	// For symbol-level moves (cross-file), use the parent directories.
	oldDir := oldPath
	newDir := newPath
	if oldRef.Symbol != "" {
		oldDir = dirOf(oldPath)
		newDir = dirOf(newPath)
	}

	if oldDir == "" || newDir == "" || oldDir == newDir {
		return nil
	}

	var edits []ingest.Edit

	// For Go consumers, rewrite import path strings (not bare identifiers,
	// since the qualifier name comes from the package directive, not the dir).
	//
	// Try full subdir path first (e.g. "pkg/executil" -> "pkg/newutil"),
	// fall back to leaf-based replacement if the full path doesn't appear.
	didFull := false
	if strings.Contains(string(content), oldDir) {
		edits = ingest.FindAllOccurrencesInStrings(fileRelPath, content, oldDir, newDir)
		didFull = len(edits) > 0
	}

	if !didFull {
		oldBase := lastPathComponent(oldDir)
		newBase := lastPathComponent(newDir)
		if cp := ingest.CommonPathPrefix(oldDir, newDir); cp != "" {
			if rel := strings.Trim(strings.TrimPrefix(newDir, cp), "/"); rel != "" {
				newBase = rel
			}
		}
		if oldBase != newBase {
			edits = ingest.FindAllOccurrencesInStrings(fileRelPath, content, oldBase, newBase)
		}
	}

	// For cross-file symbol moves, also update the qualifier (e.g. pkga.Helper -> pkgb.Helper).
	// For package moves, the qualifier comes from the `package` directive and is handled
	// separately by planPackageMove's declaredName logic.
	if oldRef.Symbol != "" {
		oldQual := lastPathComponent(oldDir)
		newQual := lastPathComponent(newDir)
		if oldQual != newQual {
			qualEdits := ingest.FindAllOccurrences(fileRelPath, content, oldQual+".", newQual+".")
			edits = append(edits, qualEdits...)
		}
	}

	return edits
}

// dedentOnce removes one leading tab from each line of s.
func dedentOnce(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if len(line) > 0 && line[0] == '\t' {
			lines[i] = line[1:]
		}
	}
	return strings.Join(lines, "\n")
}

func dirOf(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i]
}

// goDeclResult holds the matched declaration node and, for grouped type
// declarations, the individual type_spec that matched (when the declaration
// contains multiple specs).
type goDeclResult struct {
	Node     *grammar.Node // the top-level declaration or type_spec
	Grouped  bool          // true when part of a type (...) group
	TypeSpec *grammar.Node // non-nil for grouped type declarations
}

// findGoDecl returns the declaration containing the entity whose name starts at nameStart.
func findGoDecl(root *grammar.Node, nameStart uint32) *goDeclResult {
	declTypes := map[string]bool{
		"function_declaration": true,
		"method_declaration":   true,
	}

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if declTypes[child.Type()] {
			if n := ingest.ChildByField(child, "name"); n != nil && n.StartByte() == nameStart {
				return &goDeclResult{Node: child}
			}
		}
		if child.Type() == "type_declaration" {
			specCount := 0
			var matchedSpec *grammar.Node
			for j := uint32(0); j < child.ChildCount(); j++ {
				spec := child.Child(j)
				if spec.Type() == "type_spec" {
					specCount++
					if id := ingest.ChildByType(spec, "type_identifier"); id != nil && id.StartByte() == nameStart {
						matchedSpec = spec
					}
				}
			}
			if matchedSpec != nil {
				if specCount > 1 {
					return &goDeclResult{Node: child, Grouped: true, TypeSpec: matchedSpec}
				}
				return &goDeclResult{Node: child}
			}
		}
	}
	return nil
}
