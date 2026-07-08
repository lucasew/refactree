package fuzzy_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestLoadCatalog(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "testdata", "fuzzy", "projects.toml")
	projects, err := fuzzy.LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) < 3 {
		t.Fatalf("expected >=3 projects, got %d", len(projects))
	}
	// stable slug order
	if projects[0].ID != "astro" {
		t.Fatalf("expected astro first (sorted), got %q", projects[0].ID)
	}
	ids := map[string]bool{}
	for _, p := range projects {
		ids[p.ID] = true
		if p.Language == "" || !fuzzy.HasEmbeddedMise(p) {
			t.Fatalf("invalid project %#v", p)
		}
		if p.CheckTask != "test" || p.SetupTask != "setup" {
			t.Fatalf("%s tasks setup=%q check=%q", p.ID, p.SetupTask, p.CheckTask)
		}
		if p.Isolate.ImageOrDefault() != fuzzy.DefaultMiseImage {
			t.Fatalf("%s image %q want default pin", p.ID, p.Isolate.ImageOrDefault())
		}
		content, err := fuzzy.ResolveMiseTOML(p)
		if err != nil || content == "" {
			t.Fatalf("%s resolve mise: %v %q", p.ID, err, content)
		}
		if !strings.Contains(content, "[tools]") && !strings.Contains(content, "tools") {
			t.Fatalf("%s missing tools in:\n%s", p.ID, content)
		}
		if strings.Contains(content, "latest") {
			t.Fatalf("%s mise tools must be pinned:\n%s", p.ID, content)
		}
	}
	for _, id := range []string{"workspaced", "ritm_annotation", "astro", "gson"} {
		if !ids[id] {
			t.Fatalf("missing %s", id)
		}
	}
}

func TestMiseToolsMustBePinned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	catalog := filepath.Join(dir, "projects.toml")
	cases := []struct {
		name string
		ver  string
	}{
		{"latest", `uv = "latest"`},
		{"major", `node = "22"`},
		{"major_minor", `maven = "3.9"`},
		{"unquoted", `java = 21`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := fmt.Sprintf(`
[projects.x]
language = "go"
local_path = "/tmp/x"
ingest_roots = ["."]
[projects.x.mise.tools]
%s
[projects.x.mise.tasks.test]
run = "true"
`, tc.ver)
			if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := fuzzy.LoadCatalog(catalog); err == nil {
				t.Fatal("expected pin validation error")
			}
		})
	}
}

func TestIsolateImageRejectsLatest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	catalog := filepath.Join(dir, "projects.toml")
	data := `
[projects.x]
language = "go"
local_path = "/tmp/x"
ingest_roots = ["."]
[projects.x.isolate]
image = "jdxcode/mise:latest"
[projects.x.mise.tools]
go = "1.26.4"
[projects.x.mise.tasks.test]
run = "true"
`
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fuzzy.LoadCatalog(catalog); err == nil {
		t.Fatal("expected isolate.image pin error")
	}
}

func TestFilterProjectsUnknown(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "testdata", "fuzzy", "projects.toml")
	projects, err := fuzzy.LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fuzzy.FilterProjects(projects, []string{"nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateLocalProjectLegacyCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	catalog := filepath.Join(dir, "projects.toml")
	data := `
[projects.local]
language = "go"
local_path = "/tmp/x"
check = ["true"]
ingest_roots = ["."]
setup_task = "-"
[projects.local.mv]
enabled = false
[projects.local.isolate]
engine = "auto"
`
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	projects, err := fuzzy.LoadCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if projects[0].ID != "local" || projects[0].LocalPath == "" {
		t.Fatalf("unexpected %#v", projects[0])
	}
}

func TestInvalidSlugRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	catalog := filepath.Join(dir, "projects.toml")
	data := `
[projects."bad slug"]
language = "go"
local_path = "/tmp/x"
check = ["true"]
`
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := fuzzy.LoadCatalog(catalog)
	if err == nil {
		t.Fatal("expected invalid slug error")
	}
}

func TestMvOpsRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	catalog := filepath.Join(dir, "projects.toml")
	data := `
[projects.x]
language = "go"
local_path = "/tmp/x"
ingest_roots = ["."]
[projects.x.mv]
enabled = true
ops = ["rename"]
[projects.x.mise.tools]
go = "1.26.4"
[projects.x.mise.tasks.test]
run = "true"
`
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fuzzy.LoadCatalog(catalog); err == nil {
		t.Fatal("expected mv.ops rejection")
	}
}
