package web

// Blank-import language/reference providers so Resolver can dispatch to go/python/nix/node
// when the web package is used outside cmd/rft (tests, custom binaries).
import (
	_ "github.com/lucasew/refactree/pkg/ingest/go"
	_ "github.com/lucasew/refactree/pkg/ingest/js"
	_ "github.com/lucasew/refactree/pkg/ingest/nix"
	_ "github.com/lucasew/refactree/pkg/ingest/python"
)
