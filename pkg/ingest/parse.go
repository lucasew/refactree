package ingest

import (
	"fmt"
	"os"

	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// ParsedFile is a one-shot tree-sitter parse of a source file.
// Call Close when finished to free the parser and tree.
type ParsedFile struct {
	Source []byte
	Root   *grammar.Node

	tree   *grammar.Tree
	parser *grammar.Parser
}

// Close releases native parser/tree resources. Safe on nil or double-close.
func (p *ParsedFile) Close() {
	if p == nil {
		return
	}
	if p.tree != nil {
		p.tree.Delete()
		p.tree = nil
	}
	if p.parser != nil {
		p.parser.Delete()
		p.parser = nil
	}
	p.Root = nil
}

// ParseSourceFile reads path and parses it with the tree-sitter grammar for the
// file extension. If the extension is unknown and fallbackLang is non-empty,
// grammar.Get(fallbackLang) is tried (java move drivers use "java").
func ParseSourceFile(path, fallbackLang string) (*ParsedFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lang, ok := grammar.GetByExtension(path)
	if !ok && fallbackLang != "" {
		lang, ok = grammar.Get(fallbackLang)
	}
	if !ok {
		return nil, fmt.Errorf("unsupported language for %s", path)
	}

	parser := grammar.NewParser()
	if !parser.SetLanguage(lang) {
		parser.Delete()
		return nil, fmt.Errorf("failed to set language for %s", path)
	}

	tree := parser.ParseString(string(source))
	return &ParsedFile{
		Source: source,
		Root:   tree.RootNode(),
		tree:   tree,
		parser: parser,
	}, nil
}
