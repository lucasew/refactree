package fuzzy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// MvAttemptResult is the outcome of one open-canvas mv attempt.
type MvAttemptResult struct {
	Plan     movePlan
	Edits    []ingest.Edit
	Class    string // bug | unsupported | pass | env
	Failures []InvariantFailure
	Err      error
}

// RunMvAttempt runs on an already-prepared mutable work dir (repo / project root).
// Flow: pre-ingest on ingest_roots → pick plan → apply on workDir (so consumers
// outside the ingest root, e.g. boltons tests/, are rewritten) → post invariants
// → optional afterCheck.
//
// Unsupported picks/applies return Class=unsupported and a nil Err (not a fuzzer crash).
// Bugs return Class=bug and a non-nil Err suitable for t.Fatal.
// afterCheck is typically the catalog project's mise test/build (via Session), not fixtures.
// log receives choose/result lines; nil defaults to os.Stdout.
//
// workDir is the prepared project checkout. Plans are picked from primaryIngestRoot
// (paths relative to that root) and rebased onto workDir for apply when they differ.
func RunMvAttempt(ctx context.Context, p Project, workDir string, in PlanInput, strict bool, afterCheck func(context.Context) error, log io.Writer) MvAttemptResult {
	if err := ctx.Err(); err != nil {
		return MvAttemptResult{Class: classEnv, Err: err}
	}
	if log == nil {
		log = os.Stdout
	}
	ingestRoot := primaryIngestRoot(p, workDir)
	result, fails, err := RunIngestOnRoot(ingestRoot, InvariantOptions{StrictRefs: strict})
	if err != nil {
		return MvAttemptResult{Class: classBug, Err: fmt.Errorf("pre-ingest: %w", err)}
	}
	if len(fails) > 0 {
		return MvAttemptResult{Class: classBug, Failures: fails, Err: fmt.Errorf("pre-ingest invariants: %v", fails)}
	}

	plan, err := pickMvPlanWith(in, p, result)
	if err != nil {
		fmt.Fprintf(log, "mv choose: project=%s skip pick (grain_idx=%d source_idx=%d placement_idx=%d peer_idx=%d entropy=%d): %v\n",
			p.ID, in.GrainIndex, in.SourceIndex, in.PlacementIndex, in.PeerIndex, in.Entropy, err)
		return MvAttemptResult{Class: classUnsupported, Plan: plan}
	}
	fmt.Fprintf(log, "mv choose: project=%s placement=%s\n  source=%s\n  destination=%s\n  input={grain_idx=%d source_idx=%d placement_idx=%d peer_idx=%d entropy=%d}\n",
		p.ID, plan.Placement, plan.Source, plan.Destination,
		in.GrainIndex, in.SourceIndex, in.PlacementIndex, in.PeerIndex, in.Entropy)

	// Apply on the full workDir so files outside ingest_roots (tests/, docs/)
	// that import the moved module still get import rewrites. Plan paths from
	// pick are relative to ingestRoot — rebase when that root is a subdirectory.
	applyPlan := rebasePlanToWorkDir(plan, ingestRoot, workDir)
	edits, err := ApplyMvPlan(workDir, applyPlan)
	if err != nil {
		class := classifyMvError(err)
		fmt.Fprintf(log, "mv result: project=%s class=%s apply_err=%v\n", p.ID, class, err)
		if class == classUnsupported {
			return MvAttemptResult{Plan: plan, Edits: edits, Class: classUnsupported}
		}
		return MvAttemptResult{Plan: plan, Edits: edits, Class: classBug, Err: fmt.Errorf("apply %s %s -> %s: %w", plan.Placement, plan.Source, plan.Destination, err)}
	}

	// Post-invariants on the ingest root with original (ingest-relative) plan paths.
	if post := postMvInvariants(ingestRoot, plan, strict); len(post) > 0 {
		fmt.Fprintf(log, "mv result: project=%s class=bug post_invariants=%v\n", p.ID, post)
		return MvAttemptResult{
			Plan:     plan,
			Edits:    edits,
			Class:    classBug,
			Failures: post,
			Err:      fmt.Errorf("post-mv invariants for %s %s -> %s: %v", plan.Placement, plan.Source, plan.Destination, post),
		}
	}

	if afterCheck != nil {
		if err := afterCheck(ctx); err != nil {
			fmt.Fprintf(log, "mv result: project=%s class=bug check_err=%v\n", p.ID, err)
			return MvAttemptResult{
				Plan:  plan,
				Edits: edits,
				Class: classBug,
				Err:   fmt.Errorf("check after %s %s -> %s: %w", plan.Placement, plan.Source, plan.Destination, err),
			}
		}
	}

	return MvAttemptResult{Plan: plan, Edits: edits, Class: classPass}
}

// rebasePlanToWorkDir prefixes plan path: refs with the ingest root's path relative
// to workDir when they differ (e.g. boltons/fileutils.py under workDir).
// No-op when ingestRoot is workDir itself.
func rebasePlanToWorkDir(plan movePlan, ingestRoot, workDir string) movePlan {
	rel, err := filepath.Rel(workDir, ingestRoot)
	if err != nil || rel == "" || rel == "." {
		return plan
	}
	// Refuse to leave workDir.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return plan
	}
	rel = filepath.ToSlash(rel)
	return movePlan{
		Placement:   plan.Placement,
		Source:      prefixPathRef(plan.Source, rel),
		Destination: prefixPathRef(plan.Destination, rel),
	}
}

// prefixPathRef rewrites path:./file.py[::sym] → path:./prefix/file.py[::sym].
func prefixPathRef(ref, prefix string) string {
	r := ingest.ParseReference(ref)
	if r.Provider != "" && r.Provider != "path" {
		return ref
	}
	p := strings.TrimPrefix(r.Path, "./")
	joined := path.Join(prefix, p)
	if !strings.HasPrefix(joined, "./") {
		joined = "./" + joined
	}
	if r.Name != "" {
		return ingest.AtomRef(joined, r.Name)
	}
	return "path:" + joined
}

// ScaffoldAttempt writes a fixture scaffold under destDir for a failed attempt.
// Curate into testdata/mv (or ingest) from these scaffolds — not the reverse.
func ScaffoldAttempt(workRoot, destDir string, res MvAttemptResult) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	return ScaffoldMvFixture(workRoot, destDir, res.Plan.Source, res.Plan.Destination, res.Edits)
}
