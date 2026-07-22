package js_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/tsx"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/typescript"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/js"
)

func TestECMAIngestTypeScriptAndTSX(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types.ts"), []byte(`
export interface User {
  id: string
}
export type ID = string
export function greet(name: string): string {
  return name
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Button.tsx"), []byte(`
export default function Button(props: { label: string }) {
  return <button>{props.label}</button>
}
export const variant = "primary"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ingest.ProjectResult(dir)
	if err != nil {
		t.Fatal(err)
	}

	langs := map[string]string{}
	for _, f := range result.Files {
		langs[strings.TrimPrefix(f.Path, "./")] = f.Language
	}
	if langs["types.ts"] != "javascript" || langs["Button.tsx"] != "javascript" {
		t.Fatalf("expected ECMA files as language=javascript, got %#v", langs)
	}

	names := map[string]bool{}
	for _, e := range result.Atoms {
		ref := ingest.ParseReference(e.Reference)
		if ref.Name != "" {
			names[ref.Name] = true
		}
	}
	for _, want := range []string{"User", "ID", "greet", "Button", "variant"} {
		if !names[want] {
			t.Fatalf("missing entity %q in %#v (entities=%d files=%d)", want, names, len(result.Atoms), len(result.Files))
		}
	}
}

func TestECMALanguageForExtensions(t *testing.T) {
	for _, file := range []string{"a.ts", "b.tsx", "c.jsx", "d.js"} {
		lang, ok := ingest.LanguageForFile(file)
		if !ok || lang != "javascript" {
			t.Fatalf("LanguageForFile(%q)=%q ok=%v", file, lang, ok)
		}
	}
	if _, ok := ingest.LanguageForFile("x.vue"); ok {
		t.Fatal("vue must not be ingested")
	}
	if _, ok := ingest.LanguageForFile("x.astro"); ok {
		t.Fatal("astro must not be ingested")
	}
}
