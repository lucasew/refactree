package ingest

// Result is the output of ingesting a directory of source files.
type Result struct {
	Files     []File     `json:"files"`
	Entities  []Entity   `json:"entities"`
	Aliases   []Alias    `json:"aliases,omitempty"`
	Relations []Relation `json:"relations"`
}

// File records a source file and its language.
type File struct {
	Language string `json:"language"`
	Path     string `json:"path"`
}

// Entity is a named symbol definition (function, class, type).
type Entity struct {
	Reference string `json:"reference"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
}

// Alias is an import binding that introduces a name in file scope.
type Alias struct {
	Reference string `json:"reference"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
	Target    string `json:"target"`
}

// Relation is a usage of a symbol at a specific source location.
type Relation struct {
	Reference string `json:"reference"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
	Target    string `json:"target"`
}

// --- intermediate types produced by per-file extraction ---

// fileExtract holds raw facts from parsing a single source file.
type fileExtract struct {
	language string
	path     string
	pkg      string // Go package name; empty for Python/JS

	entities []entityDef
	imports  []importDef
	usages   []usageDef
}

// entityDef is a symbol definition found during extraction.
type entityDef struct {
	name      string
	startByte uint32
	endByte   uint32
	exported  bool
}

// importDef is an import declaration found during extraction.
type importDef struct {
	localName  string // name bound in local scope
	sourcePath string // raw import path ("fmt", "./helper.js", "helpers")
	memberName string // specific symbol imported; empty for module/package imports
	startByte  uint32 // byte span of the local binding name
	endByte    uint32
}

// usageDef is a call-site identifier found during extraction.
type usageDef struct {
	scope     string // enclosing entity name; empty for file-level
	name      string // identifier text
	startByte uint32
	endByte   uint32

	// For qualified access (obj.member):
	qualifier     string
	qualStartByte uint32
	qualEndByte   uint32
}
