package ingest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"

	_ "github.com/lucasew/ccgo-tree-sitter/grammar/go"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/python"
)

func TestWalkSymbols_NonRecursiveDirectory(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "root.go"), "package main\n\nfunc Root() {}\n")
	mustWrite(t, filepath.Join(dir, "sub", "sub.go"), "package main\n\nfunc Sub() {}\n")

	refs, err := collectRefs(dir, "path:./", ingest.ListOptions{IncludeHidden: true})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}

	if !containsRef(refs, "path:./root.go::Root") {
		t.Fatalf("expected root symbol, got %v", refs)
	}
	if containsRef(refs, "path:./sub/sub.go::Sub") {
		t.Fatalf("did not expect nested symbol in non-recursive listing, got %v", refs)
	}
}

func TestWalkSymbols_RecursiveDirectory(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "root.go"), "package main\n\nfunc Root() {}\n")
	mustWrite(t, filepath.Join(dir, "sub", "sub.go"), "package main\n\nfunc Sub() {}\n")

	refs, err := collectRefs(dir, "path:./", ingest.ListOptions{IncludeHidden: true, Recursive: true})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}

	if !containsRef(refs, "path:./root.go::Root") || !containsRef(refs, "path:./sub/sub.go::Sub") {
		t.Fatalf("expected recursive symbols, got %v", refs)
	}
}

func TestWalkSymbols_HiddenFilter(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package main\n\nfunc visible() {}\nfunc _private() {}\nfunc Visible() {}\n")

	refs, err := collectRefs(dir, "path:./", ingest.ListOptions{})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if containsRef(refs, "path:./a.go::visible") {
		t.Fatalf("did not expect hidden symbol without IncludeHidden, got %v", refs)
	}
	if containsRef(refs, "path:./a.go::_private") {
		t.Fatalf("did not expect hidden underscore symbol without IncludeHidden, got %v", refs)
	}
	if !containsRef(refs, "path:./a.go::Visible") {
		t.Fatalf("expected exported symbol, got %v", refs)
	}

	refs, err = collectRefs(dir, "path:./", ingest.ListOptions{IncludeHidden: true})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if !containsRef(refs, "path:./a.go::visible") {
		t.Fatalf("expected hidden symbol with IncludeHidden, got %v", refs)
	}
	if !containsRef(refs, "path:./a.go::_private") {
		t.Fatalf("expected hidden underscore symbol with IncludeHidden, got %v", refs)
	}
}

func TestWalkSymbols_FileScope(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package main\n\nfunc A() {}\n")
	mustWrite(t, filepath.Join(dir, "b.go"), "package main\n\nfunc B() {}\n")

	refs, err := collectRefs(dir, "path:./a.go", ingest.ListOptions{IncludeHidden: true})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if len(refs) != 1 || refs[0] != "path:./a.go::A" {
		t.Fatalf("expected only a.go symbol, got %v", refs)
	}
}

func TestWalkSymbols_StopEarly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package main\n\nfunc A() {}\nfunc B() {}\n")

	count := 0
	err := ingest.WalkSymbols(dir, "path:./", ingest.ListOptions{IncludeHidden: true}, func(sym ingest.SymbolInfo) bool {
		_ = sym
		count++
		return false
	})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected early stop after 1 item, got %d", count)
	}
}

func TestWalkSymbols_GoProviderScope(t *testing.T) {
	refs, err := collectRefs(".", "go:fmt", ingest.ListOptions{})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if !containsSymbol(refs, "Printf") {
		t.Fatalf("expected Printf in go:fmt listing, got %d refs", len(refs))
	}
}

func TestWalkSymbols_UnsupportedProvider(t *testing.T) {
	_, err := collectRefs(".", "node:react", ingest.ListOptions{})
	if err == nil {
		t.Fatal("expected error for unsupported provider listing")
	}
	if !strings.Contains(err.Error(), `listing not supported for provider "node"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWalkSymbols_GoProviderReferenceShape(t *testing.T) {
	refs, err := collectRefs(".", "go:fmt", ingest.ListOptions{})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("expected symbols")
	}

	for _, r := range refs {
		if !strings.HasPrefix(r, "go:fmt::") {
			t.Fatalf("unexpected provider reference: %q", r)
		}
		if strings.Contains(r, "::Test") || strings.Contains(r, "::Example") {
			t.Fatalf("did not expect go test/example symbols in provider listing: %q", r)
		}
	}
}

func TestWalkSymbols_ReferenceFixtures(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "testdata", "ingest")
	input, err := os.ReadFile(filepath.Join(fixtureRoot, "list_reference_cases.json"))
	if err != nil {
		t.Fatalf("reading list_reference_cases.json: %v", err)
	}

	var cases []struct {
		Name      string `json:"name"`
		Fixture   string `json:"fixture"`
		Reference string `json:"reference"`
		Options   struct {
			IncludeHidden bool `json:"include_hidden"`
			Recursive     bool `json:"recursive"`
		} `json:"options"`
		ExpectedRefs   []string `json:"expected_refs"`
		UnexpectedRefs []string `json:"unexpected_refs"`
	}
	if err := json.Unmarshal(input, &cases); err != nil {
		t.Fatalf("parsing list_reference_cases.json: %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			dir := filepath.Join(fixtureRoot, tc.Fixture)
			refs, err := collectRefs(dir, tc.Reference, ingest.ListOptions{
				IncludeHidden: tc.Options.IncludeHidden,
				Recursive:     tc.Options.Recursive,
			})
			if err != nil {
				t.Fatalf("walk symbols: %v", err)
			}

			for _, expected := range tc.ExpectedRefs {
				if !containsRef(refs, expected) {
					t.Fatalf("expected symbol %q not found, got %v", expected, refs)
				}
			}

			for _, unexpected := range tc.UnexpectedRefs {
				if containsRef(refs, unexpected) {
					t.Fatalf("unexpected symbol %q found, got %v", unexpected, refs)
				}
			}
		})
	}
}

func mustWrite(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func collectRefs(dir, ref string, opts ingest.ListOptions) ([]string, error) {
	out := []string{}
	err := ingest.WalkSymbols(dir, ref, opts, func(sym ingest.SymbolInfo) bool {
		out = append(out, sym.Entity.Reference)
		return true
	})
	return out, err
}

func containsRef(refs []string, needle string) bool {
	for _, r := range refs {
		if strings.TrimSpace(r) == needle {
			return true
		}
	}
	return false
}

func containsSymbol(refs []string, symbol string) bool {
	for _, r := range refs {
		ref := ingest.ParseReference(strings.TrimSpace(r))
		if ref.Symbol == symbol {
			return true
		}
	}
	return false
}
