package ingest

import (
	"fmt"
	"sync"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
)

// SymbolListOptions controls per-language symbol visibility policy.
type SymbolListOptions struct {
	IncludeHidden bool
}

// LanguageDriver defines a consistent adapter interface for language-specific
// ingestion and filesystem conventions used by refactoring.
type LanguageDriver interface {
	Language() string
	Extract(root *grammar.Node, source []byte, relPath string) *FileExtract
	ResolveImport(sourcePath string, ctx ImportResolveContext) string
	AllowListSymbol(name string, opts SymbolListOptions) bool

	// DirectoryEntryFile returns the canonical file used when a directory is
	// referenced as a symbol container (for example __init__.py or index.js).
	// Return empty string when the language has no single canonical entry file.
	DirectoryEntryFile(dirRel string) string

	// DestinationFileInDirectory maps a destination directory reference to a
	// concrete file path for this language.
	DestinationFileInDirectory(dstDirRel string, srcRef Reference) string
}

func languageDriverForName(name string) (LanguageDriver, bool) {
	languageDriversMu.RLock()
	defer languageDriversMu.RUnlock()
	d, ok := languageDrivers[name]
	return d, ok
}

func languageDriverForFile(filename string) (LanguageDriver, bool) {
	lang := languageNameByExt(filename)
	return languageDriverForName(lang)
}

var (
	languageDriversMu sync.RWMutex
	languageDrivers   = map[string]LanguageDriver{}
)

// RegisterLanguageDriver registers a language driver by name.
// It panics on empty names, nil drivers, or duplicate names.
func RegisterLanguageDriver(name string, driver LanguageDriver) {
	if name == "" {
		panic("ingest: RegisterLanguageDriver with empty name")
	}
	if driver == nil {
		panic("ingest: RegisterLanguageDriver with nil driver")
	}
	if driver.Language() != "" && driver.Language() != name {
		panic(fmt.Sprintf("ingest: RegisterLanguageDriver name mismatch: key=%q driver=%q", name, driver.Language()))
	}

	languageDriversMu.Lock()
	defer languageDriversMu.Unlock()
	if _, exists := languageDrivers[name]; exists {
		panic(fmt.Sprintf("ingest: language driver %q already registered", name))
	}
	languageDrivers[name] = driver
}
