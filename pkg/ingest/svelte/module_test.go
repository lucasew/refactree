package svelte_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lucasew/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/svelte"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/typescript"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/svelte"
)

func TestSvelteIngestScriptSymbols(t *testing.T) {
	dir := t.TempDir()
	src := `<script lang="ts">
export let initialQuery = '';
let query = initialQuery;
function handleSearch(event: Event) {
  event.preventDefault();
}
</script>
<form></form>
`
	if err := os.WriteFile(filepath.Join(dir, "SearchBar.svelte"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ingest.Ingest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Language != "svelte" {
		t.Fatalf("files=%#v", result.Files)
	}
	names := map[string]bool{}
	for _, e := range result.Entities {
		ref := ingest.ParseReference(e.Reference)
		names[ref.Symbol] = true
	}
	for _, want := range []string{"initialQuery", "query", "handleSearch"} {
		if !names[want] {
			t.Fatalf("missing %q in %#v", want, names)
		}
	}
}

func TestSvelteLanguageForFile(t *testing.T) {
	lang, ok := ingest.LanguageForFile("X.svelte")
	if !ok || lang != "svelte" {
		t.Fatalf("got %q ok=%v", lang, ok)
	}
}
