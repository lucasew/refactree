package ingest

import (
	"fmt"
	"sync"
)

// DeclExtract holds the result of extracting a top-level declaration from a source file.
type DeclExtract struct {
	// Preamble is language-specific context needed to recreate the file
	// (e.g. Go package name). Empty for languages that don't need it.
	Preamble string
	// DeclText is the full source text of the declaration.
	DeclText string
	// RemoveStart is the start byte of the range to delete from the source file.
	RemoveStart uint32
	// RemoveEnd is the end byte of the range to delete from the source file.
	RemoveEnd uint32
}

// MoveDriver defines language-specific operations for cross-file moves.
// Each language registers one via RegisterMoveDriver.
type MoveDriver interface {
	Language() string

	// ExtractDecl extracts a top-level declaration containing the entity
	// from the file at filePath (absolute).
	ExtractDecl(filePath string, entity Entity) (DeclExtract, error)

	// InsertDecl produces an edit that inserts a declaration into dstRelPath.
	// dstContent is the current file content, or nil if the file doesn't exist.
	InsertDecl(dstRelPath string, dstContent []byte, decl DeclExtract) Edit

	// RewriteImports produces edits to update import statements in a consumer
	// file when a symbol or package moves from oldRef to newRef.
	RewriteImports(fileRelPath string, content []byte, result *Result, oldRef, newRef Reference) []Edit
}

var (
	moveDriversMu sync.RWMutex
	moveDrivers   = map[string]MoveDriver{}
)

// RegisterMoveDriver registers a move driver by language name.
// It panics on empty names, nil drivers, or duplicate names.
func RegisterMoveDriver(name string, driver MoveDriver) {
	if name == "" {
		panic("ingest: RegisterMoveDriver with empty name")
	}
	if driver == nil {
		panic("ingest: RegisterMoveDriver with nil driver")
	}

	moveDriversMu.Lock()
	defer moveDriversMu.Unlock()
	if _, exists := moveDrivers[name]; exists {
		panic(fmt.Sprintf("ingest: move driver %q already registered", name))
	}
	moveDrivers[name] = driver
}

func moveDriverForLanguage(lang string) (MoveDriver, bool) {
	moveDriversMu.RLock()
	defer moveDriversMu.RUnlock()
	d, ok := moveDrivers[lang]
	return d, ok
}
