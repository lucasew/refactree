package pattern_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/go"
	"github.com/lucasew/refactree/pkg/pattern"
)

func TestRuleFromStrings_SetCapture(t *testing.T) {
	rule, err := pattern.RuleFromStrings(`$c:@go:context::Background`, `c=@go:testing::T`)
	if err != nil {
		t.Fatal(err)
	}
	if rule.SetCapture != "c" {
		t.Fatalf("SetCapture=%q", rule.SetCapture)
	}
	if rule.Pattern.Kind == "" || rule.Replacement.Kind == "" {
		t.Fatalf("empty IR: %+v", rule)
	}
}

func TestRefLeafRule_ExpandFile(t *testing.T) {
	dir := t.TempDir()
	src := []byte(`package p

import "fmt"

func f() {
	fmt.Println("x")
}
`)
	path := filepath.Join(dir, "p.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}

	fe, err := ingest.CollectExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractHop,
		Root:  dir,
		Paths: []string{path},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := ingest.Materialize(dir, fe, ingest.MaterializeOptions{ExpandImports: false})

	pf, err := ingest.ParseSourceFile(path, "go")
	if err != nil {
		t.Fatal(err)
	}
	defer pf.Close()

	target := ""
	for _, u := range result.Uses {
		if u.Target != "" {
			target = u.Target
			break
		}
	}
	if target == "" {
		t.Fatalf("no uses in result; atoms=%d uses=%d", len(result.Atoms), len(result.Uses))
	}

	rule, err := pattern.RefLeafRule(target, "Renamed")
	if err != nil {
		t.Fatal(err)
	}
	if !rule.NeedsLinks() {
		t.Fatal("RefLeafRule should need links")
	}

	matches, edits, err := rule.ExpandFile(dir, "p.go", src, pf.Root, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected matches for target %q", target)
	}
	if len(edits) == 0 {
		t.Fatal("expected edits")
	}
	for _, e := range edits {
		if e.File != "p.go" {
			t.Fatalf("edit file=%q", e.File)
		}
		if e.NewText != "Renamed" {
			t.Fatalf("NewText=%q", e.NewText)
		}
		if e.StartByte >= e.EndByte {
			t.Fatalf("empty span: %+v", e)
		}
	}
}

func TestRuleFromOp_Rewrite(t *testing.T) {
	op, err := pattern.OpFromCLI("rewrite", "go", `interface{}`, `any`)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := pattern.RuleFromOp(op)
	if err != nil {
		t.Fatal(err)
	}
	if err := rule.Valid(); err != nil {
		t.Fatal(err)
	}
}
