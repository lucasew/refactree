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
	ModeIngest Mode = "ingest"
	ModeMv     Mode = "mv"
	ModeRun    Mode = "run"
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
	NoIsolate   bool
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
		fmt.Fprintln(opts.Stdout, "isolate: testcontainers session per project (jdxcode/mise); setup+check share one container")
	} else {
		fmt.Fprintln(opts.Stdout, "isolate: disabled (--no-isolate); setup/check run on host")
	}
	fmt.Fprintln(opts.Stdout, "note: ingest/mv always run on the host; only setup/check use the container session")

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
		StrictRefs: opts.StrictRefs,
	})
	if err != nil {
		return nil, err
	}
	defer report.Close()
	fmt.Fprintf(opts.Stdout, "report: %s\n", report.Dir)

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

func runProject(ctx context.Context, opts Options, p Project, ws *Workspace, runner Runner, rng *rand.Rand, report *Report, out *Result) error {
	runID := fmt.Sprintf("%d", opts.Seed)
	fmt.Fprintf(opts.Stdout, "== project %s ==\n", p.ID)
	workDir, commit, err := ws.Prepare(p, runID)
	if err != nil {
		out.EnvFails++
		_ = report.LogEvent(Event{Project: p.ID, Kind: "prepare", Outcome: "error", Class: "env", Error: err.Error()})
		return fmt.Errorf("prepare: %w", err)
	}
	report.Meta.Commit = commit
	imageKey := ImageKey(p, commit)
	_ = report.LogEvent(Event{Project: p.ID, Kind: "prepare", Outcome: "pass", Error: commit})

	root := ProjectRoot(workDir, p)
	if err := ApplyProjectMise(p, root); err != nil {
		out.EnvFails++
		_ = report.LogEvent(Event{Project: p.ID, Kind: "mise", Outcome: "error", Class: "env", Error: err.Error()})
		return fmt.Errorf("apply [projects.<slug>.mise]: %w", err)
	}
	if HasEmbeddedMise(p) {
		fmt.Fprintf(opts.Stdout, "mise: wrote [projects.<slug>.mise] to %s\n", filepath.Join(root, "mise.toml"))
	}

	// One container for setup + all checks on this project.
	network := p.Isolate.SetupNetworkEnabled() || p.Isolate.CheckNetworkEnabled()
	session, err := runner.StartSession(ctx, p.Isolate, root, imageKey, network)
	if err != nil {
		out.EnvFails++
		_ = report.LogEvent(Event{Project: p.ID, Kind: "session", Outcome: "error", Class: "env", Error: err.Error()})
		return fmt.Errorf("start isolate session: %w", err)
	}
	defer func() { _ = session.Close(ctx) }()

	if argv := SetupArgv(p); len(argv) > 0 {
		fmt.Fprintf(opts.Stdout, "setup: %s\n", strings.Join(argv, " "))
	}
	setupRes := RunSetup(ctx, session, p)
	setupLog, _ := report.WriteRunResult(p.ID+"-setup", setupRes)
	if err := requireOK("setup", setupRes); err != nil {
		out.EnvFails++
		printRunFailure(opts.Stdout, opts.Verbose, "setup", setupRes, report.LogPath(setupLog))
		_ = report.LogEvent(Event{Project: p.ID, Kind: "setup", Outcome: "error", Class: "env", Error: shortRunErr(setupRes), Log: setupLog, ExitCode: setupRes.ExitCode})
		return err
	}
	if len(SetupArgv(p)) > 0 {
		_ = report.LogEvent(Event{Project: p.ID, Kind: "setup", Outcome: "pass", Log: setupLog, ExitCode: setupRes.ExitCode, Error: isolatedSuffix(setupRes)})
	}

	doIngest := opts.Mode == ModeIngest || opts.Mode == ModeRun
	doMv := (opts.Mode == ModeMv || opts.Mode == ModeRun) && p.Mv.Enabled

	if doIngest {
		bugs, err := RunIngestProject(p, workDir, InvariantOptions{StrictRefs: opts.StrictRefs}, report)
		out.BugCount += bugs
		if err != nil {
			return err
		}
		out.Passed++
	}

	if !doMv {
		return nil
	}

	fmt.Fprintf(opts.Stdout, "check: %s\n", strings.Join(CheckArgv(p), " "))
	checkRes := RunCheck(ctx, session, p)
	checkLog, _ := report.WriteRunResult(p.ID+"-check-before", checkRes)
	if err := requireOK("baseline check", checkRes); err != nil {
		out.EnvFails++
		printRunFailure(opts.Stdout, opts.Verbose, "baseline check", checkRes, report.LogPath(checkLog))
		_ = report.LogEvent(Event{Project: p.ID, Kind: "check_before", Outcome: "error", Class: "env", Error: shortRunErr(checkRes), Log: checkLog, ExitCode: checkRes.ExitCode})
		return err
	}
	_ = report.LogEvent(Event{Project: p.ID, Kind: "check_before", Outcome: "pass", Log: checkLog, ExitCode: checkRes.ExitCode, Error: isolatedSuffix(checkRes)})

	ingestOpts := InvariantOptions{StrictRefs: opts.StrictRefs}
	for i := 0; i < opts.Iterations; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ws.Reset(p, workDir); err != nil {
			out.EnvFails++
			return fmt.Errorf("reset: %w", err)
		}
		if err := ApplyProjectMise(p, root); err != nil {
			out.EnvFails++
			return fmt.Errorf("re-apply [projects.<slug>.mise]: %w", err)
		}
		// Re-run setup in the same session if deps were wiped.
		if len(p.PreserveGlobs) == 0 && len(SetupArgv(p)) > 0 {
			sr := RunSetup(ctx, session, p)
			srLog, _ := report.WriteRunResult(fmt.Sprintf("%s-setup-after-reset-%d", p.ID, i+1), sr)
			if err := requireOK("setup after reset", sr); err != nil {
				out.EnvFails++
				printRunFailure(opts.Stdout, opts.Verbose, "setup after reset", sr, report.LogPath(srLog))
				_ = report.LogEvent(Event{Project: p.ID, Iteration: i + 1, Kind: "setup", Outcome: "error", Class: "env", Error: shortRunErr(sr), Log: srLog, ExitCode: sr.ExitCode})
				return err
			}
		}

		ingestRoot := primaryIngestRoot(p, workDir)
		result, fails, err := RunIngestOnRoot(ingestRoot, ingestOpts)
		if err != nil {
			out.BugCount++
			_ = report.LogEvent(Event{Project: p.ID, Iteration: i + 1, Kind: "mv_pre_ingest", Outcome: "error", Class: "bug", Error: err.Error()})
			if opts.FailFast {
				return err
			}
			continue
		}
		if len(fails) > 0 {
			out.BugCount += len(fails)
			_ = report.LogEvent(Event{Project: p.ID, Iteration: i + 1, Kind: "mv_pre_ingest", Outcome: "fail", Class: "bug", Failures: fails})
			if opts.FailFast {
				return fmt.Errorf("pre-mv invariants: %v", fails)
			}
			continue
		}

		plan, err := pickMvPlan(rng, p, ingestRoot, result, opts.Ops)
		if err != nil {
			out.Unsupported++
			fmt.Fprintf(opts.Stdout, "mv[%d/%d]: skip pick: %v\n", i+1, opts.Iterations, err)
			_ = report.LogEvent(Event{Project: p.ID, Iteration: i + 1, Kind: "mv_pick", Outcome: "skip", Class: "unsupported", Error: err.Error()})
			continue
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
		if err != nil {
			ev.Error = err.Error()
			ev.Class = classifyMvError(err)
			ev.Outcome = "error"
			if ev.Class == "bug" {
				out.BugCount++
				scaffold := report.ScaffoldDir(p.ID, opts.Seed, i+1)
				_ = ScaffoldMvFixture(ingestRoot, scaffold, plan.Src, plan.Dst, edits)
			} else {
				out.Unsupported++
			}
			fmt.Fprintf(opts.Stdout, "mv[%d/%d]: %s (%s): %v\n", i+1, opts.Iterations, ev.Outcome, ev.Class, err)
			_ = report.LogEvent(ev)
			if ev.Class == "bug" && opts.FailFast {
				return err
			}
			continue
		}

		postFails := postMvInvariants(ingestRoot, plan, opts.StrictRefs)
		if len(postFails) > 0 {
			ev.Outcome = "fail"
			ev.Class = "bug"
			ev.Failures = postFails
			out.BugCount += len(postFails)
			scaffold := report.ScaffoldDir(p.ID, opts.Seed, i+1)
			_ = ScaffoldMvFixture(ingestRoot, scaffold, plan.Src, plan.Dst, edits)
			fmt.Fprintf(opts.Stdout, "mv[%d/%d]: fail (post-ingest): %v\n", i+1, opts.Iterations, postFails)
			_ = report.LogEvent(ev)
			if opts.FailFast {
				return fmt.Errorf("post-mv invariants: %v", postFails)
			}
			continue
		}

		after := RunCheck(ctx, session, p)
		afterLog, _ := report.WriteRunResult(fmt.Sprintf("%s-check-after-%d", p.ID, i+1), after)
		ev.Log = afterLog
		ev.ExitCode = after.ExitCode
		if !after.OK() {
			label := fmt.Sprintf("mv[%d/%d] post-check", i+1, opts.Iterations)
			ev.Outcome = "fail"
			ev.Class = "bug"
			ev.Error = shortRunErr(after)
			out.BugCount++
			scaffold := report.ScaffoldDir(p.ID, opts.Seed, i+1)
			_ = ScaffoldMvFixture(ingestRoot, scaffold, plan.Src, plan.Dst, edits)
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
			continue
		}
		ev.Outcome = "pass"
		out.Passed++
		fmt.Fprintf(opts.Stdout, "mv[%d/%d]: pass\n", i+1, opts.Iterations)
		_ = report.LogEvent(ev)
	}
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
