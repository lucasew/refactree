package ingest

import "strings"

// Reference is a parsed provider:path::symbol string.
type Reference struct {
	Provider string // e.g. "path", "go", "python", "node"
	Path     string // e.g. "./main.go", "fmt"
	Symbol   string // e.g. "main", "Println"; empty for file/package refs
}

// ParseReference splits a "provider:path::symbol" string.
func ParseReference(s string) Reference {
	var r Reference
	i := strings.Index(s, ":")
	if i < 0 {
		r.Path = s
		return r
	}
	r.Provider = s[:i]
	rest := s[i+1:]
	if j := strings.Index(rest, "::"); j >= 0 {
		r.Path = rest[:j]
		r.Symbol = rest[j+2:]
	} else {
		r.Path = rest
	}
	return r
}

// String formats the reference back to its canonical form.
func (r Reference) String() string {
	base := r.Provider + ":" + r.Path
	if r.Symbol != "" {
		return base + "::" + r.Symbol
	}
	return base
}

// FileRef builds a path-provider file reference: path:./file
func FileRef(path string) string {
	return "path:" + path
}

// SymbolRef builds a path-provider symbol reference: path:./file::symbol
func SymbolRef(path, symbol string) string {
	return "path:" + path + "::" + symbol
}

// lastPathComponent returns the substring after the last '/'.
func lastPathComponent(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
