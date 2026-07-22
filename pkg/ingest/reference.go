package ingest

import (
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

func AtomRef(path, symbol string) string {
	return refpkg.AtomRef(path, symbol)
}

// LastPathComponent returns the final slash-separated path segment.
func LastPathComponent(s string) string {
	return refpkg.LastPathComponent(s)
}
