package pattern_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/go"
	"github.com/lucasew/refactree/pkg/pattern"
)

func TestReplacementRefs(t *testing.T) {
	rule, err := pattern.RuleFromStrings(`$x`, `@go:fmt::Errorf($x)`)
	if err != nil {
		// template form
		rule, err = pattern.RuleFromStrings(`foo`, `bar`)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Direct template with @ref
	repl, err := pattern.ParseReplacement(`@go:context::Background()`)
	if err != nil {
		t.Fatal(err)
	}
	refs := pattern.ReplacementRefs(repl)
	if len(refs) != 1 || !strings.Contains(refs[0], "context") {
		t.Fatalf("refs=%v", refs)
	}
	_ = rule
}

func TestWithImportHygiene_AddsFmt(t *testing.T) {
	dir := t.TempDir()
	// File has no fmt import; rewrite will emit fmt.Errorf via @ref.
	src := []byte("package p\n\nfunc f() error {\n\treturn nil\n}\n")
	path := filepath.Join(dir, "p.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}

	op, err := pattern.OpFromCLI("rewrite", "go",
		`return nil`,
		`return @go:fmt::Errorf("x")`,
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err := pattern.RunWithOptions(dir, op, pattern.RunOptions{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Edits) == 0 {
		t.Fatal("expected edits")
	}
	if err := ingest.ApplyEdits(dir, res.Edits); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "fmt.Errorf") {
		t.Fatalf("missing fmt.Errorf:\n%s", text)
	}
	if !strings.Contains(text, `"fmt"`) {
		t.Fatalf("missing import fmt:\n%s", text)
	}
}

func TestImportNeedsForRule(t *testing.T) {
	rule, err := pattern.RuleFromStrings(`x`, `@go:net/http::Get`)
	if err != nil {
		t.Fatal(err)
	}
	needs := pattern.ImportNeedsForRule("go", rule)
	if len(needs) != 1 || needs[0].ImportPath != "net/http" {
		t.Fatalf("needs=%+v", needs)
	}
	if len(pattern.ImportNeedsForRule("python", rule)) != 0 {
		t.Fatal("python should have no hygiene registered")
	}
}
