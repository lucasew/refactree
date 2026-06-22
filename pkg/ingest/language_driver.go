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

	// DestinationFileInDirectory maps a destination directory reference to a
	// concrete file path for this language.
	DestinationFileInDirectory(dstDirRel string, srcRef Reference) string
}

// DirectoryModuleResolver is an optional LanguageDriver capability: map a
// directory on disk to the source file that backs it as a module (e.g. Python
// __init__.py, JS index.js / package.json main).
type DirectoryModuleResolver interface {
	// ResolveDirectoryModule returns a path relative to absDir (forward slashes
	// ok; typically a basename) for the backing file, or ok=false if this
	// language does not treat absDir as its directory module.
	ResolveDirectoryModule(absDir string) (entryRel string, ok bool)
}

// ResolveDirectoryModuleFile asks registered drivers for a directory-module
// entry under absDir. First successful resolver wins (registration order is
// not guaranteed; languages should not claim foreign dirs).
func ResolveDirectoryModuleFile(absDir string) (entryRel string, ok bool) {
	languageDriversMu.RLock()
	defer languageDriversMu.RUnlock()
	for _, driver := range languageDrivers {
		resolver, has := driver.(DirectoryModuleResolver)
		if !has {
			continue
		}
		if entry, ok := resolver.ResolveDirectoryModule(absDir); ok && entry != "" {
			return entry, true
		}
	}
	return "", false
}

func languageDriverForName(name string) (LanguageDriver, bool) {
	languageDriversMu.RLock()
	defer languageDriversMu.RUnlock()
	d, ok := languageDrivers[name]
	return d, ok
}

func languageDriverForFile(filename string) (LanguageDriver, bool) {
	lang, ok := LanguageForFile(filename)
	if !ok {
		return nil, false
	}
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
