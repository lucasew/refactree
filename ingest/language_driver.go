package ingest

import "github.com/lucasew/ccgo-tree-sitter/grammar"

// LanguageDriver defines a consistent adapter interface for language-specific
// ingestion and filesystem conventions used by refactoring.
type LanguageDriver interface {
	Language() string
	Extract(root *grammar.Node, source []byte, relPath string) *fileExtract
	ResolveImport(sourcePath string, ctx ImportResolveContext) string

	// DirectoryEntryFile returns the canonical file used when a directory is
	// referenced as a symbol container (for example __init__.py or index.js).
	// Return empty string when the language has no single canonical entry file.
	DirectoryEntryFile(dirRel string) string

	// DestinationFileInDirectory maps a destination directory reference to a
	// concrete file path for this language.
	DestinationFileInDirectory(dstDirRel string, srcRef Reference) string
}

func languageDriverForName(name string) (LanguageDriver, bool) {
	d, ok := builtInLanguageDrivers[name]
	return d, ok
}

func languageDriverForFile(filename string) (LanguageDriver, bool) {
	lang := languageNameByExt(filename)
	return languageDriverForName(lang)
}

var builtInLanguageDrivers = map[string]LanguageDriver{
	"go":         goLanguageDriver{},
	"python":     pythonLanguageDriver{},
	"javascript": javascriptLanguageDriver{},
}
