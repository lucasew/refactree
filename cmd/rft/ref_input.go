package main

import "github.com/lucasew/refactree/pkg/ingest"

// coerceLocalPathRef converts provider-less references into path references
// when they point to an existing local filesystem entry.
func coerceLocalPathRef(ref ingest.Reference) ingest.Reference {
	return ingest.CoerceLocalPathReference(".", ref)
}
