package ingest

import (
	"fmt"
	"strings"
	"sync"
)

// DeclExtract holds the result of extracting a top-level declaration from a source file.
type DeclExtract struct {
	// Preamble is language-specific context needed to recreate the file
	// (e.g. Go package name). Empty for languages that don't need it.
	Preamble string
	// DeclText is the full source text of the declaration.
	DeclText string
	// Imports lists import specs the declaration needs in the destination
	// file. InsertDecl should ensure they exist; interpretation is language-specific.
	Imports []string
	// RemoveStart is the start byte of the range to delete from the source file.
	RemoveStart uint32
	// RemoveEnd is the end byte of the range to delete from the source file.
	RemoveEnd uint32
}

// AppendDeclText appends declText to content with blank-line separation.
// Ensures content ends with a newline before the blank line when non-empty.
func AppendDeclText(content, declText string) string {
	out := content
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out += "\n"
	}
	if len(out) > 0 {
		out += "\n"
	}
	return out + declText + "\n"
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

// RenameExpander is an optional MoveDriver capability that expands a single
// symbol rename to related entities that must change together (for example a
// Go interface method and all implementations in the same package tree).
type RenameExpander interface {
	ExpandRenameSources(result *Result, sourceRef string) []string
}

// CrossFileMoveFinisher is an optional MoveDriver capability for extra edits
// after declaration extract/insert and consumer import rewrites (for example
// qualifying same-package call sites when a Go symbol leaves its package).
type CrossFileMoveFinisher interface {
	FinishCrossFileMove(rootDir string, result *Result, src, dst Reference, decl DeclExtract) ([]Edit, error)
}

// RenameSpanExpander is an optional MoveDriver capability that adds rename
// edits beyond entity/relation/alias spans (for example Go interface method
// names that are not modeled as standalone entities).
type RenameSpanExpander interface {
	ExtraRenameEdits(rootDir string, result *Result, sourceRefs []string, oldLeaf, newLeaf string) []Edit
}

// PackageMovePlanner is an optional MoveDriver capability for languages that
// need multi-root package relocation and non-source support-file rewrites
// (for example Java src/main/java + src/test/java and proguard/pom paths).
type PackageMovePlanner interface {
	// ExpandPackageDirs returns all (srcDir, dstDir) pairs to relocate for a
	// package move. Pairs use slash paths relative to the ingest root without a
	// leading "./". The slice must include the primary pair.
	ExpandPackageDirs(result *Result, srcDir, dstDir string) [][2]string

	// RewriteSupportFiles returns edits for non-ingested support files under
	// rootDir (build configs, proguard rules, etc.). movedFiles lists source
	// paths already relocated by the core planner.
	RewriteSupportFiles(rootDir string, result *Result, movedFiles map[string]bool, srcDir, dstDir string) ([]Edit, error)
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

// packageMovePlannerFor picks a PackageMovePlanner for files under srcDir.
func packageMovePlannerFor(result *Result, srcDir string) (PackageMovePlanner, bool) {
	srcDir = strings.TrimSuffix(strings.TrimPrefix(srcDir, "./"), "/")
	seen := map[string]bool{}
	for _, f := range result.Files {
		rel := strings.TrimPrefix(f.Path, "./")
		if f.Language == "" || seen[f.Language] {
			continue
		}
		if rel != srcDir && !strings.HasPrefix(rel, srcDir+"/") {
			continue
		}
		seen[f.Language] = true
		driver, ok := moveDriverForLanguage(f.Language)
		if !ok {
			continue
		}
		if planner, ok := driver.(PackageMovePlanner); ok {
			return planner, true
		}
	}
	return nil, false
}
