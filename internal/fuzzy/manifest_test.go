package fuzzy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestManifestRoundTrip(t *testing.T) {
	ws, err := fuzzy.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projects := []fuzzy.Project{{
		ID:            "demo",
		Ref:           "abc",
		PreserveGlobs: nil,
		Isolate:       fuzzy.IsolateConfig{},
	}}
	commits := map[string]string{"demo": "abc123"}
	m := ws.BuildManifest(projects, true, commits)
	if m.Isolation != "host" {
		t.Fatalf("isolation=%q", m.Isolation)
	}
	if err := ws.SaveManifest(m); err != nil {
		t.Fatal(err)
	}
	got, err := ws.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 1 || got.Projects[0].ID != "demo" || got.Projects[0].Commit != "abc123" {
		t.Fatalf("got %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("expected created_at")
	}
}

func TestValidateOfflineReadyMissingManifest(t *testing.T) {
	ws, err := fuzzy.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = fuzzy.ValidateOfflineReady(ws, []fuzzy.Project{{ID: "x"}}, true)
	if err == nil {
		t.Fatal("expected missing manifest error")
	}
}

func TestValidateOfflineReadyPinMismatch(t *testing.T) {
	ws, err := fuzzy.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := fuzzy.Project{ID: "demo", Ref: "newref", URL: "https://example.com/x.git"}
	m := ws.BuildManifest([]fuzzy.Project{{ID: "demo", Ref: "oldref"}}, true, map[string]string{"demo": "c1"})
	if err := ws.SaveManifest(m); err != nil {
		t.Fatal(err)
	}
	// seed mise-data so we fail on pin not mise-data
	key := fuzzy.ImageKey(fuzzy.Project{ID: "demo"}, "c1")
	dir := filepath.Join(ws.Root, "mise-data", key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = fuzzy.ValidateOfflineReady(ws, []fuzzy.Project{p}, true)
	if err == nil {
		t.Fatal("expected pin mismatch")
	}
}

func TestOfflineSessionEnv(t *testing.T) {
	env := fuzzy.OfflineSessionEnv()
	if env[fuzzy.OfflineEnvKey] != "1" || env["GOPROXY"] != "off" || env["MISE_OFFLINE"] != "1" {
		t.Fatalf("%+v", env)
	}
}

func TestRequiredImagesIncludesCleanup(t *testing.T) {
	imgs := fuzzy.RequiredImages([]fuzzy.Project{{ID: "a"}})
	foundCleanup, foundDefault := false, false
	for _, img := range imgs {
		if img == fuzzy.CleanupImage {
			foundCleanup = true
		}
		if img == fuzzy.DefaultMiseImage {
			foundDefault = true
		}
	}
	if !foundCleanup || !foundDefault {
		t.Fatalf("images=%v", imgs)
	}
}

func TestCleanupImageIsPinnedDefault(t *testing.T) {
	if fuzzy.CleanupImage != fuzzy.DefaultMiseImage {
		t.Fatalf("CleanupImage=%q Default=%q", fuzzy.CleanupImage, fuzzy.DefaultMiseImage)
	}
	if fuzzy.CleanupImage == "alpine:3.20" {
		t.Fatal("must not use floating alpine:3.20")
	}
}
