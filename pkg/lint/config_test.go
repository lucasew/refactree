package lint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/lint"
)

func TestLoad_RequiredFields(t *testing.T) {
	_, _, err := lint.Load([]byte(`
rules:
  - id: x
    language: go
    pattern: "interface{}"
    message: prefer any
`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_LanguageXORFamily(t *testing.T) {
	_, _, err := lint.Load([]byte(`
rules:
  - id: x
    language: go
    family: ecma
    pattern: "a"
    message: m
`))
	if err == nil {
		t.Fatal("expected error for both language and family")
	}

	_, _, err = lint.Load([]byte(`
rules:
  - id: x
    pattern: "a"
    message: m
`))
	if err == nil {
		t.Fatal("expected error for neither language nor family")
	}
}

func TestLoad_DuplicateID(t *testing.T) {
	_, _, err := lint.Load([]byte(`
rules:
  - id: x
    language: go
    pattern: "a"
    message: m
  - id: x
    language: go
    pattern: "b"
    message: n
`))
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestLoad_DefaultLevel(t *testing.T) {
	_, rules, err := lint.Load([]byte(`
rules:
  - id: x
    language: go
    pattern: "interface{}"
    message: prefer any
`))
	if err != nil {
		t.Fatal(err)
	}
	if rules[0].Level != "warning" {
		t.Fatalf("level=%q", rules[0].Level)
	}
}

func TestLoad_EmptyRules(t *testing.T) {
	cfg, rules, err := lint.Load([]byte(`rules: []`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 0 || len(rules) != 0 {
		t.Fatalf("want empty, got %d/%d", len(cfg.Rules), len(rules))
	}
}

func TestFindConfig_WalkUp(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, lint.ConfigFileName)
	if err := os.WriteFile(cfgPath, []byte("rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := lint.FindConfig(sub)
	if err != nil {
		t.Fatal(err)
	}
	if found != cfgPath {
		t.Fatalf("found %q want %q", found, cfgPath)
	}
}

func TestResolveConfigPath_Missing(t *testing.T) {
	path, err := lint.ResolveConfigPath(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("want empty path for missing config, got %q", path)
	}
}

func TestLoadCatalog_Defaults(t *testing.T) {
	path, rules, fromDefault, err := lint.LoadCatalog(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" || !fromDefault {
		t.Fatalf("path=%q fromDefault=%v", path, fromDefault)
	}
	if len(rules) != 1 || rules[0].Builtin != lint.BuiltinDeadImports {
		t.Fatalf("default rules: %+v", rules)
	}
}

func TestLoad_BuiltinDeadImports(t *testing.T) {
	_, rules, err := lint.Load([]byte(`
rules:
  - id: imports/unused-named
    builtin: dead-imports
    message: Unused named import
`))
	if err != nil {
		t.Fatal(err)
	}
	if rules[0].Builtin != lint.BuiltinDeadImports || rules[0].Rule != nil {
		t.Fatalf("%+v", rules[0])
	}
}

func TestLoad_BuiltinRejectsPattern(t *testing.T) {
	_, _, err := lint.Load([]byte(`
rules:
  - id: x
    builtin: dead-imports
    pattern: "a"
    message: m
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_UnknownBuiltin(t *testing.T) {
	_, _, err := lint.Load([]byte(`
rules:
  - id: x
    builtin: nope
    message: m
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveConfigPath_Override(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(p, []byte("rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := lint.ResolveConfigPath(t.TempDir(), p)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		// abs
		abs, _ := filepath.Abs(p)
		if got != abs {
			t.Fatalf("got %q want %q", got, abs)
		}
	}
}

func TestLoad_ReplacementOptional(t *testing.T) {
	_, rules, err := lint.Load([]byte(`
rules:
  - id: report
    language: go
    pattern: "interface{}"
    message: report only
  - id: fix
    language: go
    pattern: "interface{}"
    message: fix me
    replacement: any
`))
	if err != nil {
		t.Fatal(err)
	}
	if rules[0].Rule != nil {
		t.Fatal("report-only should have nil Rule")
	}
	if rules[1].Rule == nil {
		t.Fatal("fixable should have Rule")
	}
}
