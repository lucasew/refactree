package fuzzy

import (
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/go"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/java"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/python"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/tsx"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/typescript"

	_ "github.com/lucasew/refactree/pkg/ingest/go"
	_ "github.com/lucasew/refactree/pkg/ingest/java"
	_ "github.com/lucasew/refactree/pkg/ingest/js"
	_ "github.com/lucasew/refactree/pkg/ingest/nix"
	_ "github.com/lucasew/refactree/pkg/ingest/python"
	_ "github.com/lucasew/refactree/pkg/ingest/svelte"
	_ "github.com/lucasew/refactree/pkg/pattern"
)
