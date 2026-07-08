package fuzzy

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestCatalogFuzzCampaign is the open-canvas stress loop: warm catalog only,
// random PlanInput, full setup/mv/check. Failures print the plan and write a
// scaffold — no go -fuzz workers (those own stdout and die with exit 2 on
// multi-second host/docker work, with no useful failure text).
//
//	mise run fuzzy:prefetch
//	RFT_FUZZY_WORK_ROOT=… RFT_FUZZY_NO_ISOLATE=1 FUZZTIME=10m mise run fuzzy:run
//
// Env:
//
//	FUZZTIME          wall budget (Go duration, e.g. 10m, 30s). Default 1m if set empty with campaign forced.
//	RFT_FUZZY_ITERATIONS  optional hard cap on attempts (0 = only time budget)
//	RFT_FUZZY_SEED        RNG seed (default 1)
func TestCatalogFuzzCampaign(t *testing.T) {
	budget, iterations, ok := campaignBudget()
	if !ok {
		t.Skip("set FUZZTIME (e.g. 10m) or RFT_FUZZY_ITERATIONS to run the catalog campaign")
	}

	noIsolate := truthy(os.Getenv("RFT_FUZZY_NO_ISOLATE"))
	canvas, err := NewCatalogCanvas(DefaultWorkRoot(), DefaultCatalogPath(), noIsolate)
	if err != nil {
		t.Fatal(err)
	}
	if err := canvas.Ready(); err != nil {
		t.Fatalf("catalog canvas not warm (run: mise run fuzzy:prefetch): %v", err)
	}

	// Fresh seed each run unless RFT_FUZZY_SEED is set (repro).
	seed := time.Now().UnixNano()
	if s := strings.TrimSpace(os.Getenv("RFT_FUZZY_SEED")); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("RFT_FUZZY_SEED: %v", err)
		}
		seed = n
	}
	rng := rand.New(rand.NewSource(seed))

	// Shuffle project order so we do not always start on the same catalog slug
	// even when the RNG first draw would land on index 0.
	order := make([]int, len(canvas.Projects))
	for i := range order {
		order[i] = i
	}
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })

	deadline := time.Now().Add(budget)
	t.Logf("catalog campaign: projects=%d budget=%s iterations_cap=%d seed=%d work_root=%s no_isolate=%v",
		len(canvas.Projects), budget, iterations, seed, DefaultWorkRoot(), noIsolate)
	t.Logf("repro: RFT_FUZZY_SEED=%d", seed)

	for attempt := 1; ; attempt++ {
		if iterations > 0 && attempt > iterations {
			t.Logf("stopped: hit RFT_FUZZY_ITERATIONS=%d", iterations)
			return
		}
		if time.Now().After(deadline) {
			t.Logf("stopped: FUZZTIME budget %s after %d attempts", budget, attempt-1)
			return
		}

		// Round-robin across shuffled projects, then random PlanInput per attempt.
		projectIdx := order[(attempt-1)%len(order)]
		in := PlanInputFromRand(rng)
		p := canvas.Project(projectIdx)
		name := fmt.Sprintf("attempt=%d project=%s op_idx=%d entity_idx=%d entropy=%d file_idx=%d",
			attempt, p.ID, in.OpIndex, in.EntityIndex, in.Entropy, in.FileIndex)

		t.Run(name, func(t *testing.T) {
			scaffold := filepath.Join(os.TempDir(), "rft-fuzzy-fuzz-fail",
				fmt.Sprintf("%s-%d-%d-%d", p.ID, in.OpIndex, in.EntityIndex, in.Entropy))

			// Full choose/result lines to the test log (visible with -v).
			res := canvas.Attempt(t.Context(), projectIdx, in, scaffold)
			t.Logf("result class=%s plan op=%s\n  src=%s\n  dst=%s\n  err=%v\n  scaffold=%s",
				res.Class, res.Plan.Op, res.Plan.Src, res.Plan.Dst, res.Err, scaffold)

			switch res.Class {
			case classUnsupported, "pass", classEnv:
				if res.Class == classEnv && res.Err != nil {
					t.Logf("env soft-skip: %v", res.Err)
				}
				return
			case classBug:
				t.Fatalf("BUG catalog=%s op=%s\n  src=%s\n  dst=%s\n  err=%v\n  scaffold=%s (curate into testdata/mv)",
					p.ID, res.Plan.Op, res.Plan.Src, res.Plan.Dst, res.Err, scaffold)
			default:
				t.Fatalf("unexpected class %q: %v", res.Class, res.Err)
			}
		})
		if t.Failed() {
			return
		}
	}
}

// campaignBudget parses FUZZTIME / RFT_FUZZY_ITERATIONS. ok=false → skip campaign.
func campaignBudget() (budget time.Duration, iterations int, ok bool) {
	rawT := strings.TrimSpace(os.Getenv("FUZZTIME"))
	rawN := strings.TrimSpace(os.Getenv("RFT_FUZZY_ITERATIONS"))
	if rawT == "" && rawN == "" {
		return 0, 0, false
	}
	if rawT != "" {
		d, err := time.ParseDuration(rawT)
		if err != nil {
			// bare number → seconds (common mistake)
			if n, err2 := strconv.Atoi(rawT); err2 == nil && n > 0 {
				d = time.Duration(n) * time.Second
			} else {
				d = time.Minute
			}
		}
		if d <= 0 {
			d = time.Minute
		}
		budget = d
	} else {
		budget = 24 * time.Hour // iterations-only mode
	}
	if rawN != "" {
		n, err := strconv.Atoi(rawN)
		if err == nil && n > 0 {
			iterations = n
		}
	}
	return budget, iterations, true
}

// TestCatalogMvSeedCorpus runs a fixed seed matrix on warm catalog projects
// (normal go test, no -fuzz workers).
func TestCatalogMvSeedCorpus(t *testing.T) {
	if strings.TrimSpace(os.Getenv("FUZZTIME")) != "" || strings.TrimSpace(os.Getenv("RFT_FUZZY_ITERATIONS")) != "" {
		t.Skip("FUZZTIME/RFT_FUZZY_ITERATIONS set: use TestCatalogFuzzCampaign")
	}

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
			t.Logf("class=%s op=%s src=%s dst=%s err=%v",
				res.Class, res.Plan.Op, res.Plan.Src, res.Plan.Dst, res.Err)
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
