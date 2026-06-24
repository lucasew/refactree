package reference

import "strings"

// Reference represents a fully qualified, parsed target entity within the system.
// It is the standard unified format used to identify files, packages, or specific
// symbols across all supported languages and providers (e.g., go, python, nix).
type Reference struct {
	Provider string // e.g. "path", "go", "python", "node"
	Path     string // e.g. "./main.go", "fmt"
	Symbol   string // e.g. "main", "Println"; empty for file/package refs
}

// Parse splits a "provider:path::symbol" string into a structural Reference.
// If the provider prefix is missing but the string looks like a relative file path,
// it automatically defaults to the "path" provider.
func Parse(s string) Reference {
	var r Reference

	base := s
	if j := strings.Index(base, "::"); j >= 0 {
		r.Symbol = base[j+2:]
		base = base[:j]
	}

	if i := strings.Index(base, ":"); i >= 0 {
		r.Provider = strings.ToLower(base[:i])
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

// String formats the reference back to its canonical string representation,
// ensuring the provider and double-colon symbol separators are correctly placed.
func (r Reference) String() string {
	base := r.Path
	if r.Provider != "" {
		base = strings.ToLower(r.Provider) + ":" + r.Path
	}
	if r.Symbol != "" {
		return base + "::" + r.Symbol
	}
	return base
}

// FileRef is a convenience helper that builds a "path" provider file reference.
// It generates a string in the format "path:<path>".
func FileRef(path string) string {
	return "path:" + path
}

// SymbolRef is a convenience helper that builds a "path" provider symbol reference.
// It generates a string in the format "path:<path>::<symbol>".
func SymbolRef(path, symbol string) string {
	return "path:" + path + "::" + symbol
}

// NormalizePathReference normalizes local "path" provider references, ensuring
// that relative paths are consistently prefixed with "./" for predictable matching.
// Non-path providers are returned completely unmodified.
func NormalizePathReference(ref Reference) Reference {
	if strings.ToLower(ref.Provider) != "path" {
		return ref
	}
	ref.Provider = "path"
	if ref.Path == "" || ref.Path == "." {
		ref.Path = "./"
		return ref
	}
	if strings.HasPrefix(ref.Path, "./") || strings.HasPrefix(ref.Path, "../") || strings.HasPrefix(ref.Path, "/") {
		return ref
	}
	ref.Path = "./" + ref.Path
	return ref
}
