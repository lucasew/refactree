package fuzzy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestApplyProjectMiseTable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	upstream := "tools = { go = \"1.20\" }\n"
	if err := os.WriteFile(filepath.Join(root, "mise.toml"), []byte(upstream), 0o644); err != nil {
		t.Fatal(err)
	}
	p := fuzzy.Project{
		ID: "demo",
		Mise: map[string]any{
			"tools": map[string]any{"go": "1.26.4"},
			"tasks": map[string]any{
				"test": map[string]any{"run": "true"},
			},
		},
	}
	if err := fuzzy.ApplyProjectMise(p, root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "1.26.4") || !strings.Contains(text, "true") {
		t.Fatalf("unexpected mise.toml:\n%s", text)
	}
	bak, err := os.ReadFile(filepath.Join(root, "mise.toml.refactree-upstream"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bak) != upstream {
		t.Fatalf("backup mismatch: %q", bak)
	}
}

func TestSetupCheckArgvDefaultsFromMiseTable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	catalog := filepath.Join(dir, "projects.toml")
	data := `
[projects.x]
family = "go"
local_path = "/tmp/x"
ingest_roots = ["."]
[projects.x.mv]
enabled = false
[projects.x.mise.tools]
go = "1.26.4"
[projects.x.mise.tasks.setup]
run = "echo setup"
[projects.x.mise.tasks.test]
run = "true"
`
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	projects, err := fuzzy.LoadCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	p := projects[0]
	if p.ID != "x" || !fuzzy.HasEmbeddedMise(p) {
		t.Fatalf("unexpected %#v", p)
	}
	if got := fuzzy.SetupArgv(p); len(got) != 4 || got[1] != "-v" || got[3] != "setup" {
		t.Fatalf("setup argv: %#v", got)
	}
	if got := fuzzy.CheckArgv(p); len(got) != 4 || got[1] != "-v" || got[3] != "test" {
		t.Fatalf("check argv: %#v", got)
	}
	content, err := fuzzy.ResolveMiseTOML(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "true") {
		t.Fatalf("marshal missing test run:\n%s", content)
	}
}
