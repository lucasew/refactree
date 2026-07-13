package fuzzy

import (
	"fmt"
	"log/slog"
	"strconv"
)

const (
	classBug         = "bug"
	classEnv         = "env"
	classUnsupported = "unsupported"
	classPass        = "pass"
)

// fuzzesMv reports whether this mode runs mv iterations.
func (m Mode) fuzzesMv() bool { return m == ModeMv || m == ModeRun }

// checksIngest reports whether this mode runs ingest invariant checks.
func (m Mode) checksIngest() bool { return m == ModeIngest || m == ModeRun }

// isPrefetch reports whether this mode only warms caches.
func (m Mode) isPrefetch() bool { return m == ModePrefetch }

// worktreeID returns the runs/<slug>/<id> name and whether Prepare should reuse it.
func (m Mode) worktreeID(seed int64) (id string, reuse bool) {
	switch m {
	case ModePrefetch:
		return PrefetchRunID, true
	case ModeIngest:
		return IngestRunID, true
	default:
		return strconv.FormatInt(seed, 10), false
	}
}

// envErrorf records an environment failure and returns a wrapped error.
func (out *Result) envErrorf(report *Report, project, kind, wrap string, err error) error {
	out.EnvFails++
	logEvent(report, Event{
		Project: project,
		Kind:    kind,
		Outcome: "error",
		Class:   classEnv,
		Error:   err.Error(),
	})
	return fmt.Errorf("%s: %w", wrap, err)
}

// recordUnsupported logs a skip/unsupported event and increments the counter.
func (out *Result) recordUnsupported(report *Report, ev Event) {
	out.Unsupported++
	logEvent(report, ev)
}

// recordPass logs a passing event and increments the counter.
func (out *Result) recordPass(report *Report, ev Event) {
	out.Passed++
	ev.Outcome = "pass"
	logEvent(report, ev)
}

// bugErr records a bug-class event. With FailFast it returns err; otherwise nil.
func (out *Result) bugErr(opts Options, report *Report, ev Event, err error) error {
	n := len(ev.Failures)
	if n == 0 {
		n = 1
	}
	out.BugCount += n
	logEvent(report, ev)
	if opts.FailFast {
		return err
	}
	return nil
}

// ingestBug records an ingest error or invariant failure and returns a caller-facing error.
// Callers wrap the message and decide whether FailFast applies.
func (out *Result) ingestBug(report *Report, ev Event, err error, fails []InvariantFailure) (failErr error) {
	if err != nil {
		ev.Outcome = "error"
		ev.Class = classBug
		ev.Error = err.Error()
		out.BugCount++
		logEvent(report, ev)
		return err
	}
	ev.Outcome = "fail"
	ev.Class = classBug
	ev.Failures = fails
	out.BugCount += len(fails)
	logEvent(report, ev)
	return fmt.Errorf("invariants: %v", fails)
}

// logEvent writes an events.jsonl line; failures are secondary and only warned.
func logEvent(report *Report, ev Event) {
	if report == nil {
		return
	}
	if err := report.LogEvent(ev); err != nil {
		slog.Warn("fuzzy: log event failed", "project", ev.Project, "kind", ev.Kind, "err", err)
	}
}

// mvApplyOutcome records an ApplyMvPlan error as bug or unsupported.
// scaffold runs only for bug-class failures.
func (out *Result) mvApplyOutcome(opts Options, report *Report, ev Event, err error, scaffold func()) error {
	ev.Error = err.Error()
	ev.Class = classifyMvError(err)
	ev.Outcome = "error"
	if ev.Class == classBug {
		scaffold()
		return out.bugErr(opts, report, ev, err)
	}
	out.recordUnsupported(report, ev)
	return nil
}
