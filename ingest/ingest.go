package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

// Ingest discovers source files under dir, parses them with tree-sitter,
// and resolves cross-file references.
func Ingest(dir string) (*Result, error) {
	var extracts []*fileExtract

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		fe, parseErr := parseFile(dir, path)
		if parseErr != nil {
			return parseErr
		}
		if fe != nil {
			extracts = append(extracts, fe)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resolve(dir, extracts), nil
}

// parseFile parses a single source file and returns its fileExtract.
// Returns nil (no error) for unsupported file types.
func parseFile(dir, absPath string) (*fileExtract, error) {
	lang, ok := grammar.GetByExtension(absPath)
	if !ok {
		return nil, nil
	}
	driver, ok := languageDriverForFile(absPath)
	if !ok {
		return nil, nil
	}

	relPath, err := filepath.Rel(dir, absPath)
	if err != nil {
		return nil, err
	}

	source, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", relPath, err)
	}

	parser := grammar.NewParser()
	defer parser.Delete()

	if !parser.SetLanguage(lang) {
		return nil, fmt.Errorf("failed to set language for %s", relPath)
	}

	tree := parser.ParseString(string(source))
	defer tree.Delete()

	root := tree.RootNode()
	return driver.Extract(root, source, relPath), nil
}

func languageNameByExt(filename string) string {
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	switch ext {
	case "py":
		return "python"
	case "js":
		return "javascript"
	default:
		return ext
	}
}
