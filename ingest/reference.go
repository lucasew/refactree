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

	base := s
	if j := strings.Index(base, "::"); j >= 0 {
		r.Symbol = base[j+2:]
		base = base[:j]
	}

	if i := strings.Index(base, ":"); i >= 0 {
		r.Provider = base[:i]
		r.Path = base[i+1:]
		return r
	}

	// Shorthand reference: <path>::<symbol> or <path>
	if r.Symbol != "" || strings.Contains(base, "/") || strings.HasPrefix(base, ".") {
		r.Provider = "path"
		if strings.HasPrefix(base, "./") || strings.HasPrefix(base, "../") || strings.HasPrefix(base, "/") {
			r.Path = base
		} else {
			r.Path = "./" + base
		}
		return r
	}

	// Bare identifier (kept as-is).
	r.Path = base
	return r
}

// String formats the reference back to its canonical form.
func (r Reference) String() string {
	base := r.Path
	if r.Provider != "" {
		base = r.Provider + ":" + r.Path
	}
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
