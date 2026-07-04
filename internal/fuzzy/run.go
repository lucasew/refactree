package fuzzy

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Mode selects which phases to run.
type Mode string

const (
	ModeIngest   Mode = "ingest"
	ModeMv       Mode = "mv"
	ModeRun      Mode = "run"
	ModePrefetch Mode = "prefetch"
)

// Options configures a harness run.
type Options struct {
	CatalogPath string
	ProjectIDs  []string
	Mode        Mode
	Seed        int64
	Iterations  int
	WorkRoot    string
	ReportDir   string
	Allow       bool
	NoIsolate   bool // opt out of Docker; run setup/check on the host
	Offline     bool // use work-root caches only; no git fetch; container network=none
	StrictRefs  bool
	FailFast    bool
	Verbose     bool
	Ops         []string
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result summarizes a harness invocation.
type Result struct {
	ReportDir   string
	BugCount    int
	EnvFails    int
	Unsupported int
	Passed      int
}

// Run executes the fuzzy harness.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Iterations <= 0 {
		opts.Iterations = 1
	}
	if opts.Seed == 0 {
		opts.Seed = time.Now().UnixNano()
	}
	if opts.Mode == "" {
		opts.Mode = ModeRun
	}
	if opts.Mode == ModePrefetch && opts.Offline {
		return nil, fmt.Errorf("prefetch cannot use --offline; run prefetch online, then ingest/mv/run --offline")
	}
	if err := CheckAllowed(opts.Allow, opts.NoIsolate); err != nil {
		return nil, err
	}

	catalogPath := opts.CatalogPath
	if catalogPath == "" {
		catalogPath = DefaultCatalogPath()
	}
	projects, err := LoadCatalog(catalogPath)
	if err != nil {
		return nil, err
	}
	projects, err = FilterProjects(projects, opts.ProjectIDs)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects to run")
	}

	ws, err := NewWorkspace(opts.WorkRoot)
	if err != nil {
		return nil, err
	}

	if !opts.NoIsolate {
		if err := RequireDocker(ctx); err != nil {
			return nil, err
		}
		fmt.Fprintln(opts.Stdout, "isolate: docker (default); setup/check share one testcontainers session per project")
	} else {
		fmt.Fprintln(opts.Stdout, "isolate: disabled (--no-isolate); setup/check run on host")
	}
	if opts.Offline {
		fmt.Fprintln(opts.Stdout, "offline: using work-root git/mise/preserve caches only; container network disabled")
	}
	if opts.Mode != ModePrefetch {
		fmt.Fprintln(opts.Stdout, "note: ingest/mv always run on the host; only setup/check use the isolate session")
	} else {
		fmt.Fprintln(opts.Stdout, "prefetch: clone into work-root cache, run setup, save preserve snapshots and mise-data")
	}

	ids := make([]string, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	report, err := NewReport(opts.ReportDir, Meta{
		Seed:       opts.Seed,
		Iterations: opts.Iterations,
		Mode:       string(opts.Mode),
		Projects:   ids,
		WorkRoot:   ws.Root,
		Allow:      opts.Allow,
		NoIsolate:  opts.NoIsolate,
		Offline:    opts.Offline,
		StrictRefs: opts.StrictRefs,
	})
	if err != nil {
		return nil, err
	}
	defer report.Close()
	fmt.Fprintf(opts.Stdout, "report: %s\n", report.Dir)
	fmt.Fprintf(opts.Stdout, "work-root: %s\n", ws.Root)

	runner := Runner{
		NoIsolate: opts.NoIsolate,
		DataRoot:  filepath.Join(ws.Root, "mise-data"),
		Verbose:   opts.Verbose,
		Log:       opts.Stdout,
		Stdout:    opts.Stdout,
		Stderr:    opts.Stderr,
	}
	rng := rand.New(rand.NewSource(opts.Seed))
	out := &Result{ReportDir: report.Dir}

	for _, p := range projects {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if err := runProject(ctx, opts, p, ws, runner, rng, report, out); err != nil {
			if opts.FailFast {
				return out, err
			}
			fmt.Fprintf(opts.Stderr, "%s: %v\n", p.ID, err)
		}
	}
	if out.BugCount > 0 || out.EnvFails > 0 {
		return out, fmt.Errorf("fuzzy finished with bugs=%d env_fails=%d unsupported=%d; report %s",
			out.BugCount, out.EnvFails, out.Unsupported, report.Dir)
	}
	return out, nil
}

// sessionNetwork reports whether the isolate session should have network access.
func sessionNetwork(opts Options, p Project) bool {
	if opts.Offline {
		return false
	}
	if opts.Mode == ModePrefetch {
		return p.Isolate.SetupNetworkEnabled()
	}
	return p.Isolate.SetupNetworkEnabled() || p.Isolate.CheckNetworkEnabled()
}

type projectEnv struct {
	workDir  string
	root     string
	commit   string
	imageKey string
	session  *Session
}

func runProject(ctx context.Context, opts Options, p Project, ws *Workspace, runner Runner, rng *rand.Rand, report *Report, out *Result) error {
	fmt.Fprintf(opts.Stdout, "== project %s ==\n", p.ID)
	env, err := openProjectEnv(ctx, opts, p, ws, runner, report, out)
	if err != nil {
		return err
	}
	defer func() { _ = env.session.Close(ctx) }()

	if err := runLoggedSetup(ctx, opts, p, env.session, report, out, p.ID+"-setup", "setup", 0); err != nil {
		return err
	}

	if opts.Mode == ModePrefetch {
		return finishPrefetch(opts, p, ws, runner, env, report, out)
	}
	return runFuzzProject(ctx, opts, p, ws, rng, env, report, out)
}

func openProjectEnv(ctx context.Context, opts Options, p Project, ws *Workspace, runner Runner, report *Report, out *Result) (*projectEnv, error) {
	runID := fmt.Sprintf("%d", opts.Seed)
	prep := PrepareOptions{Offline: opts.Offline}
	if opts.Mode == ModePrefetch {
		runID = PrefetchRunID
		prep.Reuse = true
	}
	workDir, commit, err := ws.Prepare(p, runID, prep)
	if err != nil {
		out.EnvFails++
		_ = report.LogEvent(Event{Project: p.ID, Kind: "prepare", Outcome: "error", Class: "env", Error: err.Error()})
		return nil, fmt.Errorf("prepare: %w", err)
	}
	report.Meta.Commit = commit
	_ = report.LogEvent(Event{Project: p.ID, Kind: "prepare", Outcome: "pass", Error: commit})

	root := ProjectRoot(workDir, p)
	if err := ApplyProjectMise(p, root); err != nil {
		out.EnvFails++
		_ = report.LogEvent(Event{Project: p.ID, Kind: "mise", Outcome: "error", Class: "env", Error: err.Error()})
		return nil, fmt.Errorf("apply [projects.<slug>.mise]: %w", err)
	}
	if HasEmbeddedMise(p) {
		fmt.Fprintf(opts.Stdout, "mise: wrote [projects.<slug>.mise] to %s\n", filepath.Join(root, "mise.toml"))
	}

	imageKey := ImageKey(p, commit)
	session, err := runner.StartSession(ctx, p.Isolate, root, imageKey, sessionNetwork(opts, p))
	if err != nil {
		out.EnvFails++
		_ = report.LogEvent(Event{Project: p.ID, Kind: "session", Outcome: "error", Class: "env", Error: err.Error()})
		return nil, fmt.Errorf("start isolate session: %w", err)
	}
	return &projectEnv{workDir: workDir, root: root, commit: commit, imageKey: imageKey, session: session}, nil
}

func runLoggedSetup(ctx context.Context, opts Options, p Project, session *Session, report *Report, out *Result, logName, label string, iteration int) error {
	if argv := SetupArgv(p); len(argv) > 0 {
		fmt.Fprintf(opts.Stdout, "%s: %s\n", label, strings.Join(argv, " "))
	}
	res := RunSetup(ctx, session, p)
	return logCommandResult(opts, p, report, out, res, logName, label, "setup", iteration)
}

func runLoggedCheck(ctx context.Context, opts Options, p Project, session *Session, report *Report, out *Result, logName, label string, iteration int) error {
	if argv := CheckArgv(p); len(argv) > 0 {
		fmt.Fprintf(opts.Stdout, "%s: %s\n", label, strings.Join(argv, " "))
	}
	res := RunCheck(ctx, session, p)
	return logCommandResult(opts, p, report, out, res, logName, label, "check_before", iteration)
}

func logCommandResult(opts Options, p Project, report *Report, out *Result, res RunResult, logName, label, kind string, iteration int) error {
	logRel, _ := report.WriteRunResult(logName, res)
	ev := Event{
		Project:   p.ID,
		Iteration: iteration,
		Kind:      kind,
		Log:       logRel,
		ExitCode:  res.ExitCode,
		Error:     isolatedSuffix(res),
	}
	if err := requireOK(label, res); err != nil {
		out.EnvFails++
		printRunFailure(opts.Stdout, opts.Verbose, label, res, report.LogPath(logRel))
		ev.Outcome = "error"
		ev.Class = "env"
		ev.Error = shortRunErr(res)
		_ = report.LogEvent(ev)
		return err
	}
	if kind != "setup" || len(SetupArgv(p)) > 0 {
		ev.Outcome = "pass"
		_ = report.LogEvent(ev)
	}
	return nil
}

func finishPrefetch(opts Options, p Project, ws *Workspace, runner Runner, env *projectEnv, report *Report, out *Result) error {
	if err := ws.SavePreserveSnapshot(p, env.workDir); err != nil {
		out.EnvFails++
		_ = report.LogEvent(Event{Project: p.ID, Kind: "preserve_snapshot", Outcome: "error", Class: "env", Error: err.Error()})
		return fmt.Errorf("preserve snapshot: %w", err)
	}
	if len(p.PreserveGlobs) > 0 {
		snap := ws.preservePath(p.ID)
		_ = report.LogEvent(Event{Project: p.ID, Kind: "preserve_snapshot", Outcome: "pass", Error: snap})
		fmt.Fprintf(opts.Stdout, "preserve snapshot: %s\n", snap)
	}
	out.Passed++
	fmt.Fprintf(opts.Stdout, "prefetch: ready (cache=%s mise-data=%s worktree=%s)\n",
		ws.cachePath(p.ID), runner.miseDataDir(env.imageKey), env.workDir)
	return nil
}

func runFuzzProject(ctx context.Context, opts Options, p Project, ws *Workspace, rng *rand.Rand, env *projectEnv, report *Report, out *Result) error {
	doIngest := opts.Mode == ModeIngest || opts.Mode == ModeRun
	doMv := (opts.Mode == ModeMv || opts.Mode == ModeRun) && p.Mv.Enabled

	if doIngest {
		bugs, err := RunIngestProject(p, env.workDir, InvariantOptions{StrictRefs: opts.StrictRefs}, report)
		out.BugCount += bugs
		if err != nil {
			return err
		}
		out.Passed++
	}
	if !doMv {
		return nil
	}

	if err := runLoggedCheck(ctx, opts, p, env.session, report, out, p.ID+"-check-before", "baseline check", 0); err != nil {
		return err
	}

	ingestOpts := InvariantOptions{StrictRefs: opts.StrictRefs}
	for i := 0; i < opts.Iterations; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ws.Reset(p, env.workDir); err != nil {
			out.EnvFails++
			return fmt.Errorf("reset: %w", err)
		}
		if err := ApplyProjectMise(p, env.root); err != nil {
			out.EnvFails++
			return fmt.Errorf("re-apply [projects.<slug>.mise]: %w", err)
		}
		if len(p.PreserveGlobs) == 0 && len(SetupArgv(p)) > 0 {
			label := "setup after reset"
			if err := runLoggedSetup(ctx, opts, p, env.session, report, out, fmt.Sprintf("%s-setup-after-reset-%d", p.ID, i+1), label, i+1); err != nil {
				return err
			}
		}
		if err := runMvIteration(ctx, opts, p, rng, env, report, out, ingestOpts, i); err != nil {
			return err
		}
	}
	return nil
}

func runMvIteration(ctx context.Context, opts Options, p Project, rng *rand.Rand, env *projectEnv, report *Report, out *Result, ingestOpts InvariantOptions, i int) error {
	ingestRoot := primaryIngestRoot(p, env.workDir)
	result, fails, err := RunIngestOnRoot(ingestRoot, ingestOpts)
	if err != nil {
		out.BugCount++
		_ = report.LogEvent(Event{Project: p.ID, Iteration: i + 1, Kind: "mv_pre_ingest", Outcome: "error", Class: "bug", Error: err.Error()})
		if opts.FailFast {
			return err
		}
		return nil
	}
	if len(fails) > 0 {
		out.BugCount += len(fails)
		_ = report.LogEvent(Event{Project: p.ID, Iteration: i + 1, Kind: "mv_pre_ingest", Outcome: "fail", Class: "bug", Failures: fails})
		if opts.FailFast {
			return fmt.Errorf("pre-mv invariants: %v", fails)
		}
		return nil
	}

	plan, err := pickMvPlan(rng, p, ingestRoot, result, opts.Ops)
	if err != nil {
		out.Unsupported++
		fmt.Fprintf(opts.Stdout, "mv[%d/%d]: skip pick: %v\n", i+1, opts.Iterations, err)
		_ = report.LogEvent(Event{Project: p.ID, Iteration: i + 1, Kind: "mv_pick", Outcome: "skip", Class: "unsupported", Error: err.Error()})
		return nil
	}
	fmt.Fprintf(opts.Stdout, "mv[%d/%d]: %s %s -> %s\n", i+1, opts.Iterations, plan.Op, plan.Src, plan.Dst)

	start := time.Now()
	edits, err := ApplyMvPlan(ingestRoot, plan)
	ev := Event{
		Project:    p.ID,
		Iteration:  i + 1,
		Kind:       "mv",
		Op:         plan.Op,
		Source:     plan.Src,
		Dest:       plan.Dst,
		DurationMs: time.Since(start).Milliseconds(),
	}
	scaffold := func() {
		_ = ScaffoldMvFixture(ingestRoot, report.ScaffoldDir(p.ID, opts.Seed, i+1), plan.Src, plan.Dst, edits)
	}
	if err != nil {
		ev.Error = err.Error()
		ev.Class = classifyMvError(err)
		ev.Outcome = "error"
		if ev.Class == "bug" {
			out.BugCount++
			scaffold()
		} else {
			out.Unsupported++
		}
		fmt.Fprintf(opts.Stdout, "mv[%d/%d]: %s (%s): %v\n", i+1, opts.Iterations, ev.Outcome, ev.Class, err)
		_ = report.LogEvent(ev)
		if ev.Class == "bug" && opts.FailFast {
			return err
		}
		return nil
	}

	if postFails := postMvInvariants(ingestRoot, plan, opts.StrictRefs); len(postFails) > 0 {
		ev.Outcome = "fail"
		ev.Class = "bug"
		ev.Failures = postFails
		out.BugCount += len(postFails)
		scaffold()
		fmt.Fprintf(opts.Stdout, "mv[%d/%d]: fail (post-ingest): %v\n", i+1, opts.Iterations, postFails)
		_ = report.LogEvent(ev)
		if opts.FailFast {
			return fmt.Errorf("post-mv invariants: %v", postFails)
		}
		return nil
	}

	after := RunCheck(ctx, env.session, p)
	afterLog, _ := report.WriteRunResult(fmt.Sprintf("%s-check-after-%d", p.ID, i+1), after)
	ev.Log = afterLog
	ev.ExitCode = after.ExitCode
	if !after.OK() {
		label := fmt.Sprintf("mv[%d/%d] post-check", i+1, opts.Iterations)
		ev.Outcome = "fail"
		ev.Class = "bug"
		ev.Error = shortRunErr(after)
		out.BugCount++
		scaffold()
		fmt.Fprintf(opts.Stdout, "mv[%d/%d]: fail (check exit %d)\n", i+1, opts.Iterations, after.ExitCode)
		printRunFailure(opts.Stdout, opts.Verbose, label, after, report.LogPath(afterLog))
		_ = report.LogEvent(ev)
		if opts.FailFast {
			err := requireOK(label, after)
			if afterLog != "" {
				return fmt.Errorf("%w\nfull check log: %s", err, filepath.Join(report.LogPath(afterLog), "full.log"))
			}
			return err
		}
		return nil
	}
	ev.Outcome = "pass"
	out.Passed++
	fmt.Fprintf(opts.Stdout, "mv[%d/%d]: pass\n", i+1, opts.Iterations)
	_ = report.LogEvent(ev)
	return nil
}

func shortRunErr(res RunResult) string {
	if res.Err != nil {
		return fmt.Sprintf("exit %d: %v", res.ExitCode, res.Err)
	}
	if res.ExitCode != 0 {
		return fmt.Sprintf("exit %d", res.ExitCode)
	}
	return ""
}

func primaryIngestRoot(p Project, workDir string) string {
	if len(p.IngestRoots) > 0 {
		return ResolveIngestRoot(workDir, p.IngestRoots[0])
	}
	return ProjectRoot(workDir, p)
}

// ModuleRoot attempts to find the repo root containing testdata/fuzzy.
func ModuleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		cand := filepath.Join(dir, "testdata", "fuzzy", "projects.toml")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

func isolatedSuffix(res RunResult) string {
	if res.Isolated {
		return "isolated=true cmd=" + strings.Join(res.Args, " ")
	}
	return "isolated=false cmd=" + strings.Join(res.Args, " ")
}
