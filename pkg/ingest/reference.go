package ingest

import (
	"strings"

	refpkg "github.com/lucasew/refactree/pkg/reference"
)

// Reference is the canonical reference model shared across packages.
type Reference = refpkg.Reference

func ParseReference(s string) Reference {
	return refpkg.Parse(s)
}

func FileRef(path string) string {
	return refpkg.FileRef(path)
}

func SymbolRef(path, symbol string) string {
	return refpkg.SymbolRef(path, symbol)
}

// lastPathComponent returns the substring after the last '/'.
func lastPathComponent(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
