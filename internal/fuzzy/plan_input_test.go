package fuzzy

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanInputDeterministicPick(t *testing.T) {
	// Minimal local tree only to unit-test PlanInput determinism — not the canvas.
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "main.go"), []byte("package main\nfunc Helper() {}\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := Project{
		ID:     "unit",
		Family: "go",
		Mv:     MvConfig{Enabled: true, Grains: []string{"atom"}},
	}
	result, fails, err := RunIngestOnRoot(work, InvariantOptions{})
	if err != nil || len(fails) > 0 {
		t.Fatalf("ingest: err=%v fails=%v", err, fails)
	}
	in := PlanInput{GrainIndex: 0, SourceIndex: 0, PlacementIndex: 0, PeerIndex: 0, Entropy: 42}
	a, err := pickMvPlanWith(in, p, result)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pickMvPlanWith(in, p, result)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("same PlanInput produced different plans: %+v vs %+v", a, b)
	}
	if a.Placement != PlacementRename {
		t.Fatalf("expected rename placement, got %s plan=%+v", a.Placement, a)
	}
}

func TestPlanInputFromRandUsedByPick(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	in1 := PlanInputFromRand(rng)
	rng = rand.New(rand.NewSource(1))
	in2 := PlanInputFromRand(rng)
	if in1 != in2 {
		t.Fatalf("PlanInputFromRand not deterministic for seed: %+v vs %+v", in1, in2)
	}
}

func TestLoadCatalogCanvasMvEnabled(t *testing.T) {
	projects, err := LoadCatalogCanvas(DefaultCatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if !p.Mv.Enabled {
			t.Fatalf("canvas included mv-disabled project %s", p.ID)
		}
		if len(p.Mv.Grains) == 0 {
			t.Fatalf("canvas project %s has empty grains", p.ID)
		}
	}
}

func TestNewCatalogCanvasRespectsRFT_FUZZY_PROJECT(t *testing.T) {
	// Two mv-enabled local projects; filter must keep only the selected slug.
	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "main.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog := filepath.Join(dir, "projects.toml")
	data := fmt.Sprintf(`
[projects.alpha]
family = "go"
local_path = %q
root = "."
setup_task = "-"
ingest_roots = ["."]
[projects.alpha.mv]
enabled = true
grains = ["atom"]
[projects.alpha.isolate]
setup_network = true
check_network = false
[projects.alpha.mise.tasks.test]
run = "true"

[projects.beta]
family = "go"
local_path = %q
root = "."
setup_task = "-"
ingest_roots = ["."]
[projects.beta.mv]
enabled = true
grains = ["atom"]
[projects.beta.isolate]
setup_network = true
check_network = false
[projects.beta.mise.tasks.test]
run = "true"
`, filepath.Join(dir, "alpha"), filepath.Join(dir, "beta"))
	if err := os.WriteFile(catalog, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("RFT_FUZZY_PROJECT", "beta")
	canvas, err := NewCatalogCanvas(t.TempDir(), catalog, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(canvas.Projects) != 1 || canvas.Projects[0].ID != "beta" {
		t.Fatalf("projects=%v want only beta", canvas.Projects)
	}

	t.Setenv("RFT_FUZZY_PROJECT", "nope")
	if _, err := NewCatalogCanvas(t.TempDir(), catalog, true); err == nil {
		t.Fatal("expected error for unknown project id")
	}
}
