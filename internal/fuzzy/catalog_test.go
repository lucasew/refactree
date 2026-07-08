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
	// DefaultCatalog is loaded from projects.toml at package init.
	if len(fuzzy.DefaultCatalog) == 0 {
		t.Fatal("DefaultCatalog is empty")
	}
	var prev string
	for _, p := range fuzzy.DefaultCatalog {
		if prev != "" && p.ID < prev {
			t.Fatalf("DefaultCatalog not sorted by slug: %q before %q", prev, p.ID)
		}
		prev = p.ID
		if p.Family == "" || !fuzzy.HasEmbeddedMise(p) {
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
family = "go"
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
family = "go"
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
	_, err := fuzzy.FilterProjects(fuzzy.DefaultCatalog, []string{"nope"})
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
family = "go"
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
family = "go"
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
family = "go"
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
