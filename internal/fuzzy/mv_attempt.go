package fuzzy

import (
	"context"
	"fmt"
	"io"
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
// log receives one line per chosen op (and skip/result); nil defaults to os.Stdout.
func RunMvAttempt(ctx context.Context, p Project, root string, in PlanInput, strict bool, afterCheck func(context.Context) error, log io.Writer) MvAttemptResult {
	_ = ctx
	if log == nil {
		log = os.Stdout
	}
	result, fails, err := RunIngestOnRoot(root, InvariantOptions{StrictRefs: strict})
	if err != nil {
		return MvAttemptResult{Class: classBug, Err: fmt.Errorf("pre-ingest: %w", err)}
	}
	if len(fails) > 0 {
		return MvAttemptResult{Class: classBug, Failures: fails, Err: fmt.Errorf("pre-ingest invariants: %v", fails)}
	}

	plan, err := pickMvPlanWith(in, p, root, result, nil)
	if err != nil {
		fmt.Fprintf(log, "mv choose: project=%s skip pick (op_idx=%d entity_idx=%d entropy=%d file_idx=%d): %v\n",
			p.ID, in.OpIndex, in.EntityIndex, in.Entropy, in.FileIndex, err)
		return MvAttemptResult{Class: classUnsupported, Plan: plan}
	}
	fmt.Fprintf(log, "mv choose: project=%s op=%s\n  src=%s\n  dst=%s\n  input={op_idx=%d entity_idx=%d entropy=%d file_idx=%d}\n",
		p.ID, plan.Op, plan.Src, plan.Dst, in.OpIndex, in.EntityIndex, in.Entropy, in.FileIndex)

	edits, err := ApplyMvPlan(root, plan)
	if err != nil {
		class := classifyMvError(err)
		fmt.Fprintf(log, "mv result: project=%s class=%s apply_err=%v\n", p.ID, class, err)
		if class == classUnsupported {
			return MvAttemptResult{Plan: plan, Edits: edits, Class: classUnsupported}
		}
		return MvAttemptResult{Plan: plan, Edits: edits, Class: classBug, Err: fmt.Errorf("apply %s %s -> %s: %w", plan.Op, plan.Src, plan.Dst, err)}
	}

	if post := postMvInvariants(root, plan, strict); len(post) > 0 {
		fmt.Fprintf(log, "mv result: project=%s class=bug post_invariants=%v\n", p.ID, post)
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
			fmt.Fprintf(log, "mv result: project=%s class=bug check_err=%v\n", p.ID, err)
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
