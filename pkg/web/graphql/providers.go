package graphql

// Blank-import language drivers so neighborhood/Seed work outside cmd/rft.
import (
	_ "github.com/lucasew/refactree/pkg/ingest/go"
	_ "github.com/lucasew/refactree/pkg/ingest/java"
	_ "github.com/lucasew/refactree/pkg/ingest/js"
	_ "github.com/lucasew/refactree/pkg/ingest/nix"
	_ "github.com/lucasew/refactree/pkg/ingest/python"
	_ "github.com/lucasew/refactree/pkg/ingest/svelte"
)
