package fuzzy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// FuzzMvOneOp is the Go-native fuzz entry for open-canvas mv correctness on
// catalog projects (testdata/fuzzy/projects.toml), not on curated fixtures.
//
// Each execution:
//  1. picks one mv-enabled catalog project
//  2. prepares a fresh offline worktree from the warm work-root
//  3. runs catalog setup, one mv (PlanInput), graph invariants, catalog check
//  4. on class=bug, writes a scaffold under $TMPDIR/rft-fuzzy-fuzz-fail/ for
//     curation into testdata/mv (or ingest)
//
// Requires a warm work-root (mise run fuzzy:prefetch). If not warm, the test
// skips so normal CI without catalog caches stays green.
//
// Run (long, after prefetch):
//
//	FUZZTIME=30s mise run fuzzy:run
//	# or: go test ./internal/fuzzy -fuzz=FuzzMvOneOp -fuzztime=30s -timeout 0
//
// Seed-only (when warm): each f.Add runs once under plain mise run fuzzy:run.
func FuzzMvOneOp(f *testing.F) {
	noIsolate := truthy(os.Getenv("RFT_FUZZY_NO_ISOLATE"))
	canvas, err := NewCatalogCanvas(DefaultWorkRoot(), DefaultCatalogPath(), noIsolate)
	if err != nil {
		f.Fatal(err)
	}
	if err := canvas.Ready(); err != nil {
		f.Skip("catalog canvas not warm (run: mise run fuzzy:prefetch): ", err)
	}

	for i := range canvas.Projects {
		pi := uint8(i)
		// A few seeds per project; fuzzer mutates from here when -fuzz is set.
		f.Add(pi, uint8(0), uint32(0), uint32(1), uint32(0))
		f.Add(pi, uint8(1), uint32(1), uint32(2), uint32(1))
		f.Add(pi, uint8(2), uint32(3), uint32(4), uint32(2))
	}

	f.Fuzz(func(t *testing.T, projectIdx, opIdx uint8, entityIdx, entropy, fileIdx uint32) {
		in := PlanInput{
			OpIndex:     opIdx,
			EntityIndex: entityIdx,
			Entropy:     entropy,
			FileIndex:   fileIdx,
		}
		p := canvas.Project(int(projectIdx))
		scaffold := filepath.Join(os.TempDir(), "rft-fuzzy-fuzz-fail",
			fmt.Sprintf("%s-%d-%d-%d-%d", p.ID, opIdx, entityIdx, entropy, fileIdx))

		res := canvas.Attempt(context.Background(), int(projectIdx), in, scaffold)
		switch res.Class {
		case classUnsupported, "pass":
			return
		case classEnv:
			// Infra/setup noise: do not treat as an implementation bug for the fuzzer.
			t.Skip("env: ", res.Err)
		case classBug:
			t.Fatalf("catalog=%s plan=%s %s -> %s: %v (scaffold %s; curate into testdata/mv)",
				p.ID, res.Plan.Op, res.Plan.Src, res.Plan.Dst, res.Err, scaffold)
		default:
			t.Fatalf("unexpected class %q: %v", res.Class, res.Err)
		}
	})
}
