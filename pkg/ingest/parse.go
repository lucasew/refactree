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

// ParseSource parses content with the tree-sitter grammar for pathForLang
// (extension-based). fallbackLang is used when the extension is unknown.
// On success the returned ParsedFile.Source is content.
func ParseSource(content []byte, pathForLang, fallbackLang string) (*ParsedFile, error) {
	lang, ok := grammar.GetByExtension(pathForLang)
	if !ok && fallbackLang != "" {
		lang, ok = grammar.Get(fallbackLang)
	}
	if !ok {
		return nil, fmt.Errorf("unsupported language for %s", pathForLang)
	}
	return ParseSourceLanguage(content, lang, pathForLang)
}

// ParseSourceLanguage parses content with an already-resolved language.
// label is used only in error messages.
func ParseSourceLanguage(content []byte, lang grammar.Language, label string) (*ParsedFile, error) {
	parser := grammar.NewParser()
	if !parser.SetLanguage(lang) {
		parser.Delete()
		return nil, fmt.Errorf("failed to set language for %s", label)
	}
	tree := parser.ParseString(string(content))
	return &ParsedFile{
		Source: content,
		Root:   tree.RootNode(),
		tree:   tree,
		parser: parser,
	}, nil
}

// ParseSourceFile reads path and parses it with the tree-sitter grammar for the
// file extension. If the extension is unknown and fallbackLang is non-empty,
// grammar.Get(fallbackLang) is tried (java move drivers use "java").
func ParseSourceFile(path, fallbackLang string) (*ParsedFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSource(source, path, fallbackLang)
}
