package fuzzy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
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
// Flow: pre-ingest on ingest_roots → pick plan → apply on ingest root → rewrite
// consumers outside that root (e.g. boltons tests/) → post invariants → afterCheck.
//
// Apply stays scoped to the ingest root so monorepo-prefixed workDir paths do not
// break language resolution (java: vs path: under gson/src/…). External consumers
// get a second-pass RewriteImports only.
//
// Unsupported picks/applies return Class=unsupported and a nil Err (not a fuzzer crash).
// Bugs return Class=bug and a non-nil Err suitable for t.Fatal.
// afterCheck is typically the catalog project's mise test/build (via Session), not fixtures.
// log receives choose/result lines; nil defaults to os.Stdout.
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

	// Apply on the ingest root with plan paths as picked (no monorepo prefix).
	edits, err := ApplyMvPlan(ingestRoot, plan)
	if err != nil {
		class := classifyMvError(err)
		fmt.Fprintf(log, "mv result: project=%s class=%s apply_err=%v\n", p.ID, class, err)
		if class == classUnsupported {
			return MvAttemptResult{Plan: plan, Edits: edits, Class: classUnsupported}
		}
		return MvAttemptResult{Plan: plan, Edits: edits, Class: classBug, Err: fmt.Errorf("apply %s %s -> %s: %w", plan.Placement, plan.Source, plan.Destination, err)}
	}

	// Second pass: consumers outside ingest_roots (tests/, docs/, sibling packages).
	if extra, err := rewriteExternalConsumers(workDir, ingestRoot, plan); err != nil {
		return MvAttemptResult{Plan: plan, Edits: edits, Class: classBug, Err: fmt.Errorf("external consumers: %w", err)}
	} else if len(extra) > 0 {
		edits = append(edits, extra...)
	}

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

// rewriteExternalConsumers walks workDir excluding ingestRoot and rewrites
// import sites for the plan. Edit.File paths are workDir-relative.
//
// Only module/package moves (no symbol name): symbol moves need a full ingest
// Result for alias/use graph data, which we do not have for outside-root files.
// Passing result=nil into RewriteImports panics in python symbol import rewrite.
func rewriteExternalConsumers(workDir, ingestRoot string, plan movePlan) ([]ingest.Edit, error) {
	srcRef := ingest.ParseReference(plan.Source)
	if srcRef.Name != "" {
		return nil, nil
	}
	relIngest, err := filepath.Rel(workDir, ingestRoot)
	if err != nil || relIngest == "" || relIngest == "." {
		return nil, nil
	}
	if relIngest == ".." || strings.HasPrefix(relIngest, ".."+string(filepath.Separator)) {
		return nil, nil
	}
	relIngest = filepath.ToSlash(relIngest)

	src := plan.Source
	dst := plan.Destination
	var edits []ingest.Edit

	err = filepath.WalkDir(workDir, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workDir, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			// Skip the ingest tree (already rewritten by ApplyMvPlan).
			if rel == relIngest || strings.HasPrefix(rel, relIngest+"/") {
				return fs.SkipDir
			}
			switch d.Name() {
			case ".git", "node_modules", "target", ".venv", ".uv", "dist", "build":
				return fs.SkipDir
			}
			return nil
		}
		if rel == "." || strings.HasPrefix(rel, relIngest+"/") {
			return nil
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return nil
		}
		occs := ingest.RewriteImportsInFile(rel, content, nil, src, dst)
		if len(occs) == 0 {
			return nil
		}
		// Apply immediately under workDir so catalog check sees updates.
		if err := ingest.ApplyEdits(workDir, occs); err != nil {
			return err
		}
		edits = append(edits, occs...)
		return nil
	})
	return edits, err
}

// ScaffoldAttempt writes a fixture scaffold under destDir for a failed attempt.
// Curate into testdata/mv (or ingest) from these scaffolds — not the reverse.
func ScaffoldAttempt(workRoot, destDir string, res MvAttemptResult) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	return ScaffoldMvFixture(workRoot, destDir, res.Plan.Source, res.Plan.Destination, res.Edits)
}
