package fuzzy

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestModePredicates(t *testing.T) {
	cases := []struct {
		mode         Mode
		fuzzesMv     bool
		checksIngest bool
		prefetch     bool
		wantID       string
		wantReuse    bool
	}{
		{ModeRun, true, true, false, "42", false},
		{ModeMv, true, false, false, "42", false},
		{ModeIngest, false, true, false, IngestRunID, true},
		{ModePrefetch, false, false, true, PrefetchRunID, true},
	}
	for _, tc := range cases {
		if got := tc.mode.fuzzesMv(); got != tc.fuzzesMv {
			t.Fatalf("%s fuzzesMv=%v want %v", tc.mode, got, tc.fuzzesMv)
		}
		if got := tc.mode.checksIngest(); got != tc.checksIngest {
			t.Fatalf("%s checksIngest=%v want %v", tc.mode, got, tc.checksIngest)
		}
		if got := tc.mode.isPrefetch(); got != tc.prefetch {
			t.Fatalf("%s isPrefetch=%v want %v", tc.mode, got, tc.prefetch)
		}
		id, reuse := tc.mode.worktreeID(42)
		if id != tc.wantID || reuse != tc.wantReuse {
			t.Fatalf("%s worktreeID=(%s,%v) want (%s,%v)", tc.mode, id, reuse, tc.wantID, tc.wantReuse)
		}
	}
}

func TestBugErrFailFast(t *testing.T) {
	rep, err := NewReport(t.TempDir(), Meta{Seed: 1, Mode: "mv"})
	if err != nil {
		t.Fatal(err)
	}
	defer rep.Close()

	out := &Result{}
	boom := errors.New("boom")
	ev := Event{Project: "p", Kind: "mv", Outcome: "fail", Class: classBug}

	if err := out.bugErr(Options{}, rep, ev, boom); err != nil {
		t.Fatalf("without fail-fast: %v", err)
	}
	if out.BugCount != 1 {
		t.Fatalf("BugCount=%d", out.BugCount)
	}

	if err := out.bugErr(Options{FailFast: true}, rep, ev, boom); !errors.Is(err, boom) {
		t.Fatalf("fail-fast err=%v", err)
	}
	if out.BugCount != 2 {
		t.Fatalf("BugCount=%d after second", out.BugCount)
	}

	ev.Failures = []InvariantFailure{{Check: "a"}, {Check: "b"}}
	_ = out.bugErr(Options{}, rep, ev, boom)
	if out.BugCount != 4 {
		t.Fatalf("BugCount=%d want 4 after two failures", out.BugCount)
	}
}

func TestIngestBugAndEnvErrorf(t *testing.T) {
	rep, err := NewReport(filepath.Join(t.TempDir(), "r"), Meta{Seed: 2, Mode: "ingest"})
	if err != nil {
		t.Fatal(err)
	}
	defer rep.Close()
	out := &Result{}

	boom := errors.New("ingest broke")
	if err := out.ingestBug(rep, Event{Project: "p", Kind: "ingest"}, boom, nil); !errors.Is(err, boom) {
		t.Fatalf("ingest err: %v", err)
	}
	if out.BugCount != 1 {
		t.Fatalf("BugCount=%d", out.BugCount)
	}

	fails := []InvariantFailure{{Check: "x"}, {Check: "y"}}
	if err := out.ingestBug(rep, Event{Project: "p", Kind: "ingest"}, nil, fails); err == nil {
		t.Fatal("expected invariants error")
	}
	if out.BugCount != 3 {
		t.Fatalf("BugCount=%d want 3", out.BugCount)
	}

	wrapped := out.envErrorf(rep, "p", "prepare", "prepare", errors.New("disk"))
	if wrapped == nil || out.EnvFails != 1 {
		t.Fatalf("env err=%v fails=%d", wrapped, out.EnvFails)
	}
}

func TestMvApplyOutcomeClassifies(t *testing.T) {
	rep, err := NewReport(t.TempDir(), Meta{Seed: 3, Mode: "mv"})
	if err != nil {
		t.Fatal(err)
	}
	defer rep.Close()

	out := &Result{}
	scaffolded := false
	ev := Event{Project: "p", Kind: "mv"}
	err = out.mvApplyOutcome(Options{}, rep, ev, errors.New("not supported yet"), func() { scaffolded = true })
	if err != nil || scaffolded || out.Unsupported != 1 || out.BugCount != 0 {
		t.Fatalf("unsupported: err=%v scaffold=%v unsup=%d bugs=%d", err, scaffolded, out.Unsupported, out.BugCount)
	}

	scaffolded = false
	boom := errors.New("rewrite panicked")
	err = out.mvApplyOutcome(Options{FailFast: true}, rep, Event{Project: "p", Kind: "mv"}, boom, func() { scaffolded = true })
	if !errors.Is(err, boom) || !scaffolded || out.BugCount != 1 {
		t.Fatalf("bug: err=%v scaffold=%v bugs=%d", err, scaffolded, out.BugCount)
	}
}
