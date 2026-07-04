package fuzzy

import (
	"fmt"
	"time"

	"github.com/lucasew/refactree/pkg/ingest"
)

// RunIngestOnRoot ingests one directory and checks invariants.
func RunIngestOnRoot(root string, opts InvariantOptions) (result *ingest.Result, fails []InvariantFailure, err error) {
	result, err = ingest.Ingest(root)
	if err != nil {
		return nil, nil, err
	}
	fails = append(fails, CheckInvariants(root, result, opts)...)
	fails = append(fails, CheckIdempotentIngest(root, result)...)
	fails = append(fails, CheckWalkSymbolsSubset(root, result)...)
	return result, fails, nil
}

// RunIngestProject runs ingest checks for every configured root.
func RunIngestProject(p Project, workDir string, opts InvariantOptions, report *Report) (bugFails int, err error) {
	for _, rel := range p.IngestRoots {
		root := ResolveIngestRoot(workDir, rel)
		start := time.Now()
		_, fails, ierr := RunIngestOnRoot(root, opts)
		ev := Event{
			Project:    p.ID,
			Kind:       "ingest",
			DurationMs: time.Since(start).Milliseconds(),
		}
		if ierr != nil {
			ev.Outcome = "error"
			ev.Class = "bug"
			ev.Error = ierr.Error()
			bugFails++
			_ = report.LogEvent(ev)
			return bugFails, fmt.Errorf("ingest %s (%s): %w", p.ID, rel, ierr)
		}
		if len(fails) > 0 {
			ev.Outcome = "fail"
			ev.Class = "bug"
			ev.Failures = fails
			bugFails += len(fails)
			_ = report.LogEvent(ev)
			return bugFails, fmt.Errorf("ingest invariants failed for %s (%s): %v", p.ID, rel, fails)
		}
		ev.Outcome = "pass"
		_ = report.LogEvent(ev)
	}
	return 0, nil
}
