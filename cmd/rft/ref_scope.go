package main

import "github.com/lucasew/refactree/pkg/ingest"

// normalizeRefForCommandScope derives ingest dir and normalized reference
// using the same path-scoping primitive used by ls/doc/mv/browse commands.
func normalizeRefForCommandScope(ref ingest.Reference) (string, ingest.Reference) {
	scope := ingest.ResolveReferenceScope(".", ref)
	return scope.Dir, scope.Reference
}

func normalizeRefForIngestDir(dir string, ref ingest.Reference) ingest.Reference {
	return ingest.NormalizeReferenceForScope(dir, dir, ref)
}

// resolvePathRefForBrowse turns command-scoped path refs into absolute paths
// so browse opens the same scope that ls/doc/mv ingest against.
func resolvePathRefForBrowse(dir string, ref ingest.Reference) ingest.Reference {
	return ingest.AbsolutePathReferenceForScope(ingest.ReferenceScope{
		Dir:       dir,
		Reference: ref,
	})
}
