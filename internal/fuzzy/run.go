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
	// VerifyOffline, when true after prefetch, re-validates offline readiness.
	// Prefetch defaults this to true unless NoVerifyOffline is set.
	VerifyOffline   bool
	NoVerifyOffline bool // skip post-prefetch offline verification
	StrictRefs      bool
	FailFast        bool
	Verbose         bool
	Grains          []string // optional override of project mv grains
	Stdout          io.Writer
	Stderr          io.Writer
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
	if err := normalizeOptions(&opts); err != nil {
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

	if opts.WorkRoot == "" {
		opts.WorkRoot = DefaultWorkRoot()
	}
	ws, err := NewWorkspace(opts.WorkRoot)
	if err != nil {
		return nil, err
	}
	// All on-disk harness state lives under work-root (reports, mise-data, caches, …).
	if opts.ReportDir == "" {
		opts.ReportDir = ws.ReportsDir()
	}

	// Prefetch full no-op when work-root already supports offline runs.
	if opts.Mode.isPrefetch() {
		if err := ValidateOfflineReady(ws, projects, opts.NoIsolate); err == nil {
			fmt.Fprintf(opts.Stdout, "prefetch: no-op (work-root warm) %s\n", ws.Root)
			return &Result{Passed: len(projects)}, nil
		}
	}

	if !opts.NoIsolate {
		if err := RequireDocker(ctx); err != nil {
			return nil, err
		}
		disableRyuk()
	}
	printRunBanner(opts)

	if opts.Offline {
		if err := ValidateOfflineReady(ws, projects, opts.NoIsolate); err != nil {
			return nil, fmt.Errorf("%w (run: RFT_FUZZY_WARMUP=1 go test ./internal/fuzzy -run '^TestPrefetchWarmup$')", err)
		}
		fmt.Fprintln(opts.Stdout, "offline preflight: ok")
	}
	if opts.Mode.isPrefetch() && !opts.NoIsolate {
		imgs := RequiredImages(projects)
		fmt.Fprintf(opts.Stdout, "prefetch: ensuring docker images (%d)\n", len(imgs))
		if err := EnsureImages(imgs, true); err != nil {
			return nil, err
		}
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
		Offline:   opts.Offline,
		DataRoot:  ws.MiseDataRoot(),
		Verbose:   opts.Verbose,
		Log:       opts.Stdout,
		Stdout:    opts.Stdout,
		Stderr:    opts.Stderr,
	}
	var rng *rand.Rand
	if opts.Mode.fuzzesMv() {
		rng = rand.New(rand.NewSource(opts.Seed))
	}
	out := &Result{ReportDir: report.Dir}
	commits := map[string]string{}

	for _, p := range projects {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if opts.Mode.isPrefetch() {
			if commit, ok := projectPrefetchReady(ws, p, opts.NoIsolate); ok {
				fmt.Fprintf(opts.Stdout, "== project %s ==\nprefetch: skip (already warm)\n", p.ID)
				commits[p.ID] = commit
				out.Passed++
				_ = report.LogEvent(Event{Project: p.ID, Kind: "prefetch_skip", Outcome: "pass", Error: commit})
				continue
			}
		}
		if err := runProject(ctx, opts, p, ws, runner, rng, report, out, commits); err != nil {
			if opts.FailFast {
				return out, err
			}
			fmt.Fprintf(opts.Stderr, "%s: %v\n", p.ID, err)
		}
	}
	if opts.Mode.isPrefetch() && out.EnvFails == 0 && out.BugCount == 0 {
		if err := finishPrefetchAll(opts, ws, projects, commits); err != nil {
			return out, err
		}
	}
	if out.BugCount > 0 || out.EnvFails > 0 {
		return out, fmt.Errorf("fuzzy finished with bugs=%d env_fails=%d unsupported=%d; report %s",
			out.BugCount, out.EnvFails, out.Unsupported, report.Dir)
	}
	return out, nil
}

func normalizeOptions(opts *Options) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Mode == "" {
		opts.Mode = ModeRun
	}
	if opts.Seed == 0 {
		opts.Seed = time.Now().UnixNano()
	}
	if opts.Mode.fuzzesMv() && opts.Iterations <= 0 {
		opts.Iterations = 1
	}
	if opts.Mode.isPrefetch() && opts.Offline {
		return fmt.Errorf("prefetch cannot use --offline; run prefetch online, then ingest/mv/run --offline")
	}
	if opts.Mode.isPrefetch() && !opts.NoVerifyOffline {
		opts.VerifyOffline = true
	}
	return CheckAllowed(opts.Allow, opts.NoIsolate)
}

func printRunBanner(opts Options) {
	if opts.NoIsolate {
		fmt.Fprintln(opts.Stdout, "isolate: disabled (--no-isolate); setup/check run on host")
	} else {
		fmt.Fprintln(opts.Stdout, "isolate: docker (default); setup/check share one testcontainers session per project")
	}
	if opts.Offline {
		fmt.Fprintln(opts.Stdout, "offline: work-root caches only; no git fetch/pull; container network=none; package managers offline")
	}
	if opts.Mode.isPrefetch() {
		fmt.Fprintln(opts.Stdout, "prefetch: no-op if warm; else fill missing git/mise/preserve/images and write manifest")
		return
	}
	fmt.Fprintln(opts.Stdout, "note: ingest/mv always run on the host; only setup/check use the isolate session")
}

// sessionNetwork reports whether the isolate session should have network access.
func sessionNetwork(opts Options, p Project) bool {
	if opts.Offline {
		return false
	}
	if opts.Mode.isPrefetch() {
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

func runProject(ctx context.Context, opts Options, p Project, ws *Workspace, runner Runner, rng *rand.Rand, report *Report, out *Result, commits map[string]string) error {
	fmt.Fprintf(opts.Stdout, "== project %s ==\n", p.ID)
	env, err := openProjectEnv(ctx, opts, p, ws, runner, report, out)
	if err != nil {
		return err
	}
	defer func() { _ = env.session.Close(ctx) }()
	if commits != nil {
		commits[p.ID] = env.commit
	}

	if err := runLoggedSetup(ctx, opts, p, env.session, report, out, p.ID+"-setup", "setup", 0); err != nil {
		return err
	}

	if opts.Mode.isPrefetch() {
		return finishPrefetch(opts, p, ws, runner, env, report, out)
	}
	return runFuzzProject(ctx, opts, p, ws, rng, env, report, out)
}

func openProjectEnv(ctx context.Context, opts Options, p Project, ws *Workspace, runner Runner, report *Report, out *Result) (*projectEnv, error) {
	runID, reuse := opts.Mode.worktreeID(opts.Seed)
	workDir, commit, err := ws.Prepare(p, runID, PrepareOptions{Offline: opts.Offline, Reuse: reuse})
	if err != nil {
		return nil, out.envErrorf(report, p.ID, "prepare", "prepare", err)
	}
	report.Meta.Commit = commit
	_ = report.LogEvent(Event{Project: p.ID, Kind: "prepare", Outcome: "pass", Error: commit})

	root := ProjectRoot(workDir, p)
	if err := ApplyProjectMise(p, root); err != nil {
		return nil, out.envErrorf(report, p.ID, "mise", "apply [projects.<slug>.mise]", err)
	}
	if HasEmbeddedMise(p) {
		fmt.Fprintf(opts.Stdout, "mise: wrote [projects.<slug>.mise] to %s\n", filepath.Join(root, "mise.toml"))
	}

	imageKey := ImageKey(p, commit)
	session, err := runner.StartSession(ctx, p.Isolate, root, imageKey, sessionNetwork(opts, p))
	if err != nil {
		return nil, out.envErrorf(report, p.ID, "session", "start isolate session", err)
	}
	return &projectEnv{workDir: workDir, root: root, commit: commit, imageKey: imageKey, session: session}, nil
}

func runLoggedSetup(ctx context.Context, opts Options, p Project, session *Session, report *Report, out *Result, logName, label string, iteration int) error {
	if argv := SetupArgv(p); len(argv) > 0 {
		fmt.Fprintf(opts.Stdout, "%s: %s\n", label, strings.Join(argv, " "))
	}
	return logCommandResult(opts, p, report, out, RunSetup(ctx, session, p), logName, label, "setup", iteration)
}

func runLoggedCheck(ctx context.Context, opts Options, p Project, session *Session, report *Report, out *Result, logName, label string, iteration int) error {
	if argv := CheckArgv(p); len(argv) > 0 {
		fmt.Fprintf(opts.Stdout, "%s: %s\n", label, strings.Join(argv, " "))
	}
	return logCommandResult(opts, p, report, out, RunCheck(ctx, session, p), logName, label, "check_before", iteration)
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
		ev.Class = classEnv
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
		return out.envErrorf(report, p.ID, "preserve_snapshot", "preserve snapshot", err)
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

func finishPrefetchAll(opts Options, ws *Workspace, projects []Project, commits map[string]string) error {
	m := ws.BuildManifest(projects, opts.NoIsolate, commits)
	if err := ws.SaveManifest(m); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	fmt.Fprintf(opts.Stdout, "manifest: %s (%d projects)\n", ws.manifestPath(), len(m.Projects))
	if !opts.VerifyOffline {
		return nil
	}
	fmt.Fprintln(opts.Stdout, "prefetch: verify-offline preflight")
	if err := ValidateOfflineReady(ws, projects, opts.NoIsolate); err != nil {
		return fmt.Errorf("prefetch verify-offline failed: %w", err)
	}
	fmt.Fprintln(opts.Stdout, "prefetch: verify-offline ok")
	return nil
}

func runFuzzProject(ctx context.Context, opts Options, p Project, ws *Workspace, rng *rand.Rand, env *projectEnv, report *Report, out *Result) error {
	if opts.Mode.checksIngest() {
		if err := RunIngestProject(p, env.workDir, InvariantOptions{StrictRefs: opts.StrictRefs}, report, out); err != nil {
			return err
		}
		out.Passed++
	}
	if !opts.Mode.fuzzesMv() || !p.Mv.Enabled {
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
		if err := resetForMvIteration(ctx, opts, p, ws, env, report, out, i); err != nil {
			return err
		}
		if err := runMvIteration(ctx, opts, p, rng, env, report, out, ingestOpts, i); err != nil {
			return err
		}
	}
	return nil
}

func resetForMvIteration(ctx context.Context, opts Options, p Project, ws *Workspace, env *projectEnv, report *Report, out *Result, i int) error {
	if err := ws.Reset(p, env.workDir); err != nil {
		out.EnvFails++
		return fmt.Errorf("reset: %w", err)
	}
	if err := ApplyProjectMise(p, env.root); err != nil {
		out.EnvFails++
		return fmt.Errorf("re-apply [projects.<slug>.mise]: %w", err)
	}
	if len(p.PreserveGlobs) == 0 && len(SetupArgv(p)) > 0 {
		return runLoggedSetup(ctx, opts, p, env.session, report, out,
			fmt.Sprintf("%s-setup-after-reset-%d", p.ID, i+1), "setup after reset", i+1)
	}
	return nil
}

func runMvIteration(ctx context.Context, opts Options, p Project, rng *rand.Rand, env *projectEnv, report *Report, out *Result, ingestOpts InvariantOptions, i int) error {
	iter := i + 1
	label := fmt.Sprintf("mv[%d/%d]", iter, opts.Iterations)
	ingestRoot := primaryIngestRoot(p, env.workDir)
	in := PlanInputFromRand(rng)
	if len(opts.Grains) > 0 {
		p.Mv.Grains = append([]string(nil), opts.Grains...)
	}

	// afterCheck runs the catalog project check so choose/result lines stay in one place.
	var checkRes RunResult
	attempt := RunMvAttempt(ctx, p, ingestRoot, in, opts.StrictRefs, func(ctx context.Context) error {
		checkRes = RunCheck(ctx, env.session, p)
		if !checkRes.OK() {
			return fmt.Errorf("%s", shortRunErr(checkRes))
		}
		return nil
	}, opts.Stdout)

	plan := attempt.Plan
	ev := Event{
		Project:   p.ID,
		Iteration: iter,
		Kind:      "mv",
		Placement: string(plan.Placement),
		Source:    plan.Source,
		Dest:      plan.Destination,
	}
	scaffold := func() {
		_ = ScaffoldMvFixture(ingestRoot, report.ScaffoldDir(p.ID, opts.Seed, iter), plan.Source, plan.Destination, attempt.Edits)
	}

	switch attempt.Class {
	case classUnsupported:
		errMsg := "unsupported"
		if attempt.Err != nil {
			errMsg = attempt.Err.Error()
		} else if plan.Placement == "" {
			errMsg = "pick failed"
		}
		out.recordUnsupported(report, Event{
			Project:   p.ID,
			Iteration: iter,
			Kind:      "mv_pick",
			Outcome:   "skip",
			Class:     classUnsupported,
			Error:     errMsg,
			Placement: string(plan.Placement),
			Source:    plan.Source,
			Dest:      plan.Destination,
		})
		return nil
	case classBug:
		// Distinguish pre-ingest failures (no plan) from apply/post/check bugs.
		if plan.Placement == "" && len(attempt.Failures) > 0 {
			ev := Event{Project: p.ID, Iteration: iter, Kind: "mv_pre_ingest"}
			failErr := out.ingestBug(report, ev, attempt.Err, attempt.Failures)
			if opts.FailFast {
				return failErr
			}
			return nil
		}
		ev.Outcome = "fail"
		ev.Class = classBug
		if attempt.Err != nil {
			ev.Error = attempt.Err.Error()
		}
		ev.Failures = attempt.Failures
		if checkRes.Args != nil || checkRes.ExitCode != 0 || checkRes.Err != nil {
			afterLog, _ := report.WriteRunResult(fmt.Sprintf("%s-check-after-%d", p.ID, iter), checkRes)
			ev.Log = afterLog
			ev.ExitCode = checkRes.ExitCode
			printRunFailure(opts.Stdout, opts.Verbose, label+" post-check", checkRes, report.LogPath(afterLog))
		}
		scaffold()
		return out.bugErr(opts, report, ev, attempt.Err)
	case classPass:
		fmt.Fprintf(opts.Stdout, "mv result: project=%s class=pass iteration=%d/%d placement=%s\n",
			p.ID, iter, opts.Iterations, plan.Placement)
		out.recordPass(report, ev)
		return nil
	default:
		if attempt.Err != nil {
			return attempt.Err
		}
		return nil
	}
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
