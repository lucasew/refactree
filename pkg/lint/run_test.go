package lint_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/go"
	"github.com/lucasew/refactree/pkg/lint"
)

func TestRun_FindingsAndFix(t *testing.T) {
	dir := t.TempDir()
	src := []byte("package p\n\nfunc f(x interface{}) {}\n")
	if err := os.WriteFile(filepath.Join(dir, "p.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	_, rules, err := lint.Load([]byte(`
rules:
  - id: go/prefer-any
    language: go
    pattern: "interface{}"
    message: Prefer any over interface{}
    replacement: any
`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := lint.Run(dir, rules, lint.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("findings=%d %+v", len(res.Findings), res.Findings)
	}
	f := res.Findings[0]
	if !f.Fixable || f.FixSkipped {
		t.Fatalf("fixable=%v skipped=%v", f.Fixable, f.FixSkipped)
	}
	if len(res.ApplyEdits) == 0 {
		t.Fatal("expected apply edits")
	}

	// SARIF includes fixes without --fix
	var buf bytes.Buffer
	if err := lint.WriteSARIF(&buf, dir, res); err != nil {
		t.Fatal(err)
	}
	var log map[string]any
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	if log["version"] != "2.1.0" {
		t.Fatalf("version=%v", log["version"])
	}
	runs := log["runs"].([]any)
	run0 := runs[0].(map[string]any)
	results := run0["results"].([]any)
	r0 := results[0].(map[string]any)
	if _, ok := r0["fixes"]; !ok {
		t.Fatalf("missing fixes in SARIF: %s", buf.String())
	}
}

func TestRun_ConflictFirstRuleWins(t *testing.T) {
	dir := t.TempDir()
	// One token matched by two rules with different replacements.
	src := []byte("package p\n\nfunc f(x interface{}) {}\n")
	if err := os.WriteFile(filepath.Join(dir, "p.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	_, rules, err := lint.Load([]byte(`
rules:
  - id: first
    language: go
    pattern: "interface{}"
    message: first
    replacement: any
  - id: second
    language: go
    pattern: "interface{}"
    message: second
    replacement: "any /*x*/"
`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := lint.Run(dir, rules, lint.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 2 {
		t.Fatalf("findings=%d", len(res.Findings))
	}
	var first, second *lint.Finding
	for i := range res.Findings {
		switch res.Findings[i].RuleID {
		case "first":
			first = &res.Findings[i]
		case "second":
			second = &res.Findings[i]
		}
	}
	if first == nil || second == nil {
		t.Fatal("missing findings")
	}
	if first.FixSkipped {
		t.Fatal("first should not be skipped")
	}
	if !second.FixSkipped {
		t.Fatal("second should be skipped due to overlap")
	}
	// ApplyEdits should only reflect first
	for _, e := range res.ApplyEdits {
		if strings.Contains(e.NewText, "/*x*/") {
			t.Fatalf("second replacement leaked into apply: %q", e.NewText)
		}
	}
}

func TestRun_ReportOnly(t *testing.T) {
	dir := t.TempDir()
	src := []byte("package p\n\nfunc f(x interface{}) {}\n")
	if err := os.WriteFile(filepath.Join(dir, "p.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	_, rules, err := lint.Load([]byte(`
rules:
  - id: report
    language: go
    pattern: "interface{}"
    message: just report
`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := lint.Run(dir, rules, lint.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Fixable {
		t.Fatalf("%+v", res.Findings)
	}
	if len(res.ApplyEdits) != 0 {
		t.Fatal("report-only should not produce apply edits")
	}
	var buf bytes.Buffer
	if err := lint.WriteSARIF(&buf, dir, res); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"fixes"`) {
		t.Fatalf("report-only SARIF should not have fixes: %s", buf.String())
	}
}

func TestRun_DeadImportsBuiltin(t *testing.T) {
	dir := t.TempDir()
	src := []byte("package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc f() {\n\t_ = strings.TrimSpace(\"x\")\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "p.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	_, rules, err := lint.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	res, err := lint.Run(dir, rules, lint.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) == 0 {
		t.Fatal("expected unused fmt finding")
	}
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "imports/unused-named" && f.Fixable && !f.FixSkipped {
			found = true
		}
	}
	if !found {
		t.Fatalf("findings: %+v", res.Findings)
	}
	if len(res.ApplyEdits) == 0 {
		t.Fatal("expected apply edits")
	}
	out := ingest.ApplyEditsInMemory(src, res.ApplyEdits)
	if strings.Contains(string(out), `"fmt"`) {
		t.Fatalf("fmt should be pruned:\n%s", out)
	}
	if !strings.Contains(string(out), `"strings"`) {
		t.Fatalf("strings must remain:\n%s", out)
	}
}

func TestWriteText(t *testing.T) {
	var buf bytes.Buffer
	err := lint.WriteText(&buf, lint.Result{Findings: []lint.Finding{{
		File:    "p.go",
		Line:    3,
		Column:  10,
		Level:   "warning",
		RuleID:  "go/prefer-any",
		Message: "Prefer any",
		Fixable: true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "p.go:3:10: warning: go/prefer-any: Prefer any [fixable]") {
		t.Fatalf("got %q", got)
	}
}
