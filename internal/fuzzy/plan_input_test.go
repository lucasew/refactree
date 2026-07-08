package fuzzy

import (
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
		Mv:     MvConfig{Enabled: true, Grains: []string{"declaration"}},
	}
	result, fails, err := RunIngestOnRoot(work, InvariantOptions{})
	if err != nil || len(fails) > 0 {
		t.Fatalf("ingest: err=%v fails=%v", err, fails)
	}
	in := PlanInput{GrainIndex: 0, SourceIndex: 0, PlacementIndex: 0, PeerIndex: 0, Entropy: 42}
	a, err := pickMvPlanWith(in, p, work, result)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pickMvPlanWith(in, p, work, result)
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
