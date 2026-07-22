package ingest

// Result is the output of ingesting a directory of source files.
type Result struct {
	Files   []File  `json:"files"`
	Atoms   []Atom  `json:"atoms"`
	Aliases []Alias `json:"aliases,omitempty"`
	Uses    []Use   `json:"uses"`
}

// File records a source file and its language.
type File struct {
	Language string `json:"language"`
	Path     string `json:"path"`
}

// Atom is a named symbol definition (function, class, type).
type Atom struct {
	Reference string `json:"reference"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
	// Exported is omitted from fixture JSON; used by listing when a language
	// driver opts into explicit export flags (see UseExportedFlag).
	Exported bool `json:"-"`
}

// Alias is an import binding that introduces a name in file scope.
type Alias struct {
	Reference string `json:"reference"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
	Target    string `json:"target"`
}

// Use is a usage of a symbol at a specific source location.
type Use struct {
	Reference      string `json:"reference"`
	StartByte      uint32 `json:"start_byte"`
	EndByte        uint32 `json:"end_byte"`
	Target         string `json:"target"`
	ViaImportAlias bool   `json:"via_import_alias,omitempty"`
}

// --- intermediate types produced by per-file extraction ---

// FileExtract holds raw facts from parsing a single source file.
type FileExtract struct {
	Language string
	Path     string
	Package  string // Go package name; empty for Python/JS

	Atoms     []AtomDef
	Imports   []ImportDef
	Usages    []UsageDef
	Reexports []ReexportDef // language-neutral forwarding hops (barrels / re-exports)
	// DefaultExport is the primary symbol this module exposes when imported as a
	// whole (no member). Language drivers set it from their syntax; core only
	// materializes it as a file→file::symbol Alias for CanonicalizeInResult.
	// Empty if the driver did not identify a single primary export.
	DefaultExport string
}

// ReexportDef is a language-neutral "this module forwards X from Y" fact.
// Language drivers fill it from their syntax (JS export … from, Python from … import re-export, etc.).
// Resolved into Result.Aliases for CanonicalizeInResult. When SourceStartByte/SourceEndByte
// are set, the alias span is the source-name token so renames rewrite export { name } from
// (and export { name as alias } from) without relying on zero-span synthetic sites.
type ReexportDef struct {
	ExportName      string // name this module exposes; empty for star/wildcard forward
	SourceName      string // name in the source module; empty for star or when same as ExportName
	SourcePath      string // module specifier ("./foo", "pkg", …)
	Star            bool   // true for wildcard forward (export * / from … import *)
	SourceStartByte uint32 // byte span of SourceName token when known (rename site)
	SourceEndByte   uint32
}

// AtomDef is a symbol definition found during extraction.
type AtomDef struct {
	Name      string
	StartByte uint32
	EndByte   uint32
	Exported  bool
}

// ImportDef is an import declaration found during extraction.
type ImportDef struct {
	LocalName  string // name bound in local scope
	SourcePath string // raw import path ("fmt", "./helper.js", "helpers")
	MemberName string // specific symbol imported; empty for module/package imports
	StartByte  uint32 // byte span of the local binding name
	EndByte    uint32

	// Byte span of the token that names the referenced target symbol.
	// For member imports this is the imported member token ("helper").
	// For module/package imports this falls back to StartByte/EndByte.
	TargetStartByte uint32
	TargetEndByte   uint32

	// True when import syntax used an explicit alias binding (`as`/import alias),
	// even if alias text equals the imported symbol name.
	HasAliasBinding bool
}

// UsageDef is a call-site identifier found during extraction.
type UsageDef struct {
	Scope     string // enclosing entity name; empty for file-level
	Name      string // identifier text
	StartByte uint32
	EndByte   uint32

	// For qualified access (obj.member):
	Qualifier     string
	QualStartByte uint32
	QualEndByte   uint32
}
