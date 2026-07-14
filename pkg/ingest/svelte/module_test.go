package svelte_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/svelte"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/typescript"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/svelte"
)

func TestSvelteIngestScriptSymbols(t *testing.T) {
	dir := t.TempDir()
	src := `<script lang="ts">
import { Search } from 'lucide-svelte';
export let initialQuery = '';
let query = initialQuery;
function handleSearch(event: Event) {
  event.preventDefault();
}
</script>
<form on:submit={handleSearch}>
<input bind:value={query} />
<Search size={18} />
</form>
`
	if err := os.WriteFile(filepath.Join(dir, "SearchBar.svelte"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ingest.ProjectResult(dir)
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
			t.Fatalf("missing entity %q in %#v", want, names)
		}
	}

	// Markup usages should resolve to script symbols (or component import).
	usageTargets := map[string]bool{}
	for _, r := range result.Relations {
		ref := ingest.ParseReference(r.Reference)
		// usage site path
		tgt := ingest.ParseReference(r.Target)
		usageTargets[tgt.Symbol] = true
		_ = ref
	}
	for _, want := range []string{"handleSearch", "query", "Search"} {
		if !usageTargets[want] {
			// dump for debug
			var dump []string
			for _, r := range result.Relations {
				dump = append(dump, r.Reference+" -> "+r.Target)
			}
			t.Fatalf("missing markup relation to %q; relations=%v", want, dump)
		}
	}
}

func TestSvelteLanguageForFile(t *testing.T) {
	lang, ok := ingest.LanguageForFile("X.svelte")
	if !ok || lang != "svelte" {
		t.Fatalf("got %q ok=%v", lang, ok)
	}
}
