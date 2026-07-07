package fuzzy

import (
	"context"
	"fmt"
	"os"

	"github.com/lucasew/refactree/pkg/ingest"
)

// MvAttemptResult is the outcome of one open-canvas mv attempt.
type MvAttemptResult struct {
	Plan     mvPlan
	Edits    []ingest.Edit
	Class    string // bug | unsupported | pass | env
	Failures []InvariantFailure
	Err      error
}

// RunMvAttempt runs on an already-prepared mutable work dir (ingest root).
// Flow: pre-ingest → pick plan from PlanInput → apply → post invariants → optional afterCheck.
//
// Unsupported picks/applies return Class=unsupported and a nil Err (not a fuzzer crash).
// Bugs return Class=bug and a non-nil Err suitable for t.Fatal.
// afterCheck is typically the catalog project's mise test/build (via Session), not fixtures.
func RunMvAttempt(ctx context.Context, p Project, root string, in PlanInput, strict bool, afterCheck func(context.Context) error) MvAttemptResult {
	_ = ctx
	result, fails, err := RunIngestOnRoot(root, InvariantOptions{StrictRefs: strict})
	if err != nil {
		return MvAttemptResult{Class: classBug, Err: fmt.Errorf("pre-ingest: %w", err)}
	}
	if len(fails) > 0 {
		return MvAttemptResult{Class: classBug, Failures: fails, Err: fmt.Errorf("pre-ingest invariants: %v", fails)}
	}

	plan, err := pickMvPlanWith(in, p, root, result, nil)
	if err != nil {
		return MvAttemptResult{Class: classUnsupported, Plan: plan}
	}

	edits, err := ApplyMvPlan(root, plan)
	if err != nil {
		class := classifyMvError(err)
		if class == classUnsupported {
			return MvAttemptResult{Plan: plan, Edits: edits, Class: classUnsupported}
		}
		return MvAttemptResult{Plan: plan, Edits: edits, Class: classBug, Err: fmt.Errorf("apply %s %s -> %s: %w", plan.Op, plan.Src, plan.Dst, err)}
	}

	if post := postMvInvariants(root, plan, strict); len(post) > 0 {
		return MvAttemptResult{
			Plan:     plan,
			Edits:    edits,
			Class:    classBug,
			Failures: post,
			Err:      fmt.Errorf("post-mv invariants for %s %s -> %s: %v", plan.Op, plan.Src, plan.Dst, post),
		}
	}

	if afterCheck != nil {
		if err := afterCheck(ctx); err != nil {
			return MvAttemptResult{
				Plan:  plan,
				Edits: edits,
				Class: classBug,
				Err:   fmt.Errorf("check after %s %s -> %s: %w", plan.Op, plan.Src, plan.Dst, err),
			}
		}
	}

	return MvAttemptResult{Plan: plan, Edits: edits, Class: "pass"}
}

// ScaffoldAttempt writes a fixture scaffold under destDir for a failed attempt.
// Curate into testdata/mv (or ingest) from these scaffolds — not the reverse.
func ScaffoldAttempt(workRoot, destDir string, res MvAttemptResult) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	return ScaffoldMvFixture(workRoot, destDir, res.Plan.Src, res.Plan.Dst, res.Edits)
}
