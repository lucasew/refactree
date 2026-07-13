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
// Failures always stop the project (caller decides multi-project FailFast).
func RunIngestProject(p Project, workDir string, opts InvariantOptions, report *Report, out *Result) error {
	for _, rel := range p.IngestRoots {
		root := ResolveIngestRoot(workDir, rel)
		start := time.Now()
		_, fails, err := RunIngestOnRoot(root, opts)
		ev := Event{
			Project:    p.ID,
			Kind:       "ingest",
			DurationMs: time.Since(start).Milliseconds(),
		}
		if err != nil || len(fails) > 0 {
			_ = out.ingestBug(report, ev, err, fails)
			if err != nil {
				return fmt.Errorf("ingest %s (%s): %w", p.ID, rel, err)
			}
			return fmt.Errorf("ingest invariants failed for %s (%s): %v", p.ID, rel, fails)
		}
		ev.Outcome = "pass"
		logEvent(report, ev)
	}
	return nil
}
