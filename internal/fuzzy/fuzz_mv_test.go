package fuzzy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
)

// FuzzMvOneOp stresses one catalog mv per execution (open canvas = projects.toml only).
//
// Requires a warm work-root (mise run fuzzy:prefetch). Skips when cold.
//
// Process logs are muted: go -fuzz workers own stdout for IPC; harness/git/mise
// output there aborts the worker with exit status 2.
//
//	mise run fuzzy:prefetch
//	FUZZTIME=30s mise run fuzzy:run
//	# or: go test ./internal/fuzzy -run '^$' -fuzz=FuzzMvOneOp -fuzztime=30s
//
// Fixtures are curated from bugs ($TMPDIR/rft-fuzzy-fuzz-fail/…), not used as canvas.
func FuzzMvOneOp(f *testing.F) {
	restore := MuteProcessLogs()
	f.Cleanup(restore)

	noIsolate := truthy(os.Getenv("RFT_FUZZY_NO_ISOLATE"))
	canvas, err := NewCatalogCanvas(DefaultWorkRoot(), DefaultCatalogPath(), noIsolate)
	if err != nil {
		f.Fatal(err)
	}
	canvas.Log = io.Discard
	canvas.runner.Log = io.Discard
	canvas.runner.Stdout = io.Discard
	canvas.runner.Stderr = io.Discard

	if err := canvas.Ready(); err != nil {
		f.Skip("catalog canvas not warm (run: mise run fuzzy:prefetch): ", err)
	}

	for i := range canvas.Projects {
		pi := uint8(i)
		f.Add(pi, uint8(0), uint32(0), uint32(1), uint32(0))
		f.Add(pi, uint8(1), uint32(1), uint32(2), uint32(1))
		f.Add(pi, uint8(2), uint32(3), uint32(4), uint32(2))
	}

	f.Fuzz(func(t *testing.T, projectIdx, opIdx uint8, entityIdx, entropy, fileIdx uint32) {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("panic: %v\n%s", rec, debug.Stack())
			}
		}()

		in := PlanInput{
			OpIndex:     opIdx,
			EntityIndex: entityIdx,
			Entropy:     entropy,
			FileIndex:   fileIdx,
		}
		p := canvas.Project(int(projectIdx))
		scaffold := filepath.Join(os.TempDir(), "rft-fuzzy-fuzz-fail",
			fmt.Sprintf("%s-%d-%d-%d-%d", p.ID, opIdx, entityIdx, entropy, fileIdx))

		res := canvas.Attempt(t.Context(), int(projectIdx), in, scaffold)
		switch res.Class {
		case classUnsupported, "pass", classEnv:
			// env = infra; unsupported = expected limits. Soft-return (no t.Skip).
			if res.Class == classEnv && res.Err != nil {
				t.Log("env soft-skip: ", res.Err)
			}
			return
		case classBug:
			t.Fatalf("catalog=%s plan=%s %s -> %s: %v (scaffold %s; curate into testdata/mv)",
				p.ID, res.Plan.Op, res.Plan.Src, res.Plan.Dst, res.Err, scaffold)
		default:
			t.Fatalf("unexpected class %q: %v", res.Class, res.Err)
		}
	})
}

// TestCatalogMvSeedCorpus runs the same seed matrix as FuzzMvOneOp f.Add entries
// as normal subtests (no fuzz worker). Useful when you want catalog coverage
// without -fuzz.
func TestCatalogMvSeedCorpus(t *testing.T) {
	noIsolate := truthy(os.Getenv("RFT_FUZZY_NO_ISOLATE"))
	canvas, err := NewCatalogCanvas(DefaultWorkRoot(), DefaultCatalogPath(), noIsolate)
	if err != nil {
		t.Fatal(err)
	}
	if err := canvas.Ready(); err != nil {
		t.Skip("catalog canvas not warm (run: mise run fuzzy:prefetch): ", err)
	}

	type seed struct {
		projectIdx int
		in         PlanInput
	}
	var seeds []seed
	for i := range canvas.Projects {
		seeds = append(seeds,
			seed{i, PlanInput{OpIndex: 0, EntityIndex: 0, Entropy: 1, FileIndex: 0}},
			seed{i, PlanInput{OpIndex: 1, EntityIndex: 1, Entropy: 2, FileIndex: 1}},
			seed{i, PlanInput{OpIndex: 2, EntityIndex: 3, Entropy: 4, FileIndex: 2}},
		)
	}

	for _, s := range seeds {
		s := s
		p := canvas.Project(s.projectIdx)
		name := fmt.Sprintf("%s/op%d_ent%d", p.ID, s.in.OpIndex, s.in.EntityIndex)
		t.Run(name, func(t *testing.T) {
			scaffold := filepath.Join(os.TempDir(), "rft-fuzzy-fuzz-fail", name)
			res := canvas.Attempt(t.Context(), s.projectIdx, s.in, scaffold)
			switch res.Class {
			case classUnsupported, "pass", classEnv:
				if res.Class == classEnv && res.Err != nil {
					t.Log("env soft-skip: ", res.Err)
				}
				return
			case classBug:
				t.Fatalf("catalog=%s plan=%s %s -> %s: %v (scaffold %s)",
					p.ID, res.Plan.Op, res.Plan.Src, res.Plan.Dst, res.Err, scaffold)
			default:
				t.Fatalf("unexpected class %q: %v", res.Class, res.Err)
			}
		})
	}
}
