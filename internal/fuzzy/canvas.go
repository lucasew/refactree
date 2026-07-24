package fuzzy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

// LoadCatalogCanvas returns catalog projects with mv enabled — the open canvas
// for Go native fuzz and for catalog ModeMv/ModeRun. Fixtures under testdata/
// are not canvas inputs; they are curated later from bug scaffolds.
func LoadCatalogCanvas(catalogPath string) ([]Project, error) {
	if catalogPath == "" {
		catalogPath = DefaultCatalogPath()
	}
	projects, err := LoadCatalog(catalogPath)
	if err != nil {
		return nil, err
	}
	var out []Project
	for _, p := range projects {
		if !p.Mv.Enabled {
			continue
		}
		if len(p.Mv.Grains) == 0 {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no mv-enabled projects in catalog %s", catalogPath)
	}
	return out, nil
}

// CatalogCanvas runs one-shot mv attempts against warm catalog worktrees.
// It is safe for sequential use (Go fuzz default). Prefer one canvas per process.
type CatalogCanvas struct {
	Projects  []Project
	Workspace *Workspace
	NoIsolate bool
	Offline   bool
	Strict    bool
	Log       io.Writer

	mu      sync.Mutex
	runner  Runner
	ready   bool
	readyEr error
}

// NewCatalogCanvas builds a canvas over DefaultWorkRoot (or workRoot) and the
// mv-enabled catalog. Call Ready before Attempt; Ready requires a warm offline
// work-root (mise run fuzzy:prefetch).
//
// When RFT_FUZZY_PROJECT is set (comma-separated slugs), the canvas is limited
// to those projects — same filter as Prefetch/Run. That lets campaigns and the
// seed matrix run against a partially warm work-root (e.g. only workspaced).
func NewCatalogCanvas(workRoot, catalogPath string, noIsolate bool) (*CatalogCanvas, error) {
	projects, err := LoadCatalogCanvas(catalogPath)
	if err != nil {
		return nil, err
	}
	if ids := splitCommaIDs(os.Getenv("RFT_FUZZY_PROJECT")); len(ids) > 0 {
		projects, err = FilterProjects(projects, ids)
		if err != nil {
			return nil, fmt.Errorf("RFT_FUZZY_PROJECT: %w", err)
		}
		if len(projects) == 0 {
			return nil, fmt.Errorf("RFT_FUZZY_PROJECT matched no mv-enabled projects")
		}
	}
	if workRoot == "" {
		workRoot = DefaultWorkRoot()
	}
	ws, err := NewWorkspace(workRoot)
	if err != nil {
		return nil, err
	}
	log := io.Writer(os.Stdout)
	return &CatalogCanvas{
		Projects:  projects,
		Workspace: ws,
		NoIsolate: noIsolate,
		Offline:   true,
		Log:       log,
		runner: Runner{
			NoIsolate: noIsolate,
			Offline:   true,
			DataRoot:  ws.MiseDataRoot(),
			Log:       log,
			Stdout:    log,
			Stderr:    os.Stderr,
		},
	}, nil
}

// Ready validates the work-root can run offline against the canvas projects.
func (c *CatalogCanvas) Ready() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readyLocked()
}

// Project returns projects[i%len] for fuzz projectIdx.
func (c *CatalogCanvas) Project(i int) Project {
	if len(c.Projects) == 0 {
		return Project{}
	}
	if i < 0 {
		i = -i
	}
	return c.Projects[i%len(c.Projects)]
}

// Attempt prepares a fresh offline worktree for projectIdx, runs setup, one mv
// from PlanInput, post-ingest invariants, then the project's catalog check
// (build/test via mise isolate or host). On bug-class failures, scaffoldDir
// receives a fixture scaffold for later curation into testdata/mv or ingest.
//
// Serialized: bare git caches are not safe for concurrent worktree add/remove
// (Go fuzz workers share this canvas).
func (c *CatalogCanvas) Attempt(ctx context.Context, projectIdx int, in PlanInput, scaffoldDir string) (res MvAttemptResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.readyLocked(); err != nil {
		return MvAttemptResult{Class: classEnv, Err: fmt.Errorf("canvas not ready: %w", err)}
	}
	if err := ctx.Err(); err != nil {
		return MvAttemptResult{Class: classEnv, Err: err}
	}
	p := c.Project(projectIdx)

	runID := fmt.Sprintf("gofuzz-%d-%d-%d-%d-%d", projectIdx, in.GrainIndex, in.SourceIndex, in.PlacementIndex, in.Entropy)
	workDir, commit, err := c.Workspace.Prepare(p, runID, PrepareOptions{Offline: c.Offline, Reuse: false})
	if err != nil {
		return MvAttemptResult{Class: classEnv, Err: fmt.Errorf("prepare %s: %w", p.ID, err)}
	}
	cache := c.Workspace.cachePath(p.ID)
	// Proper worktree teardown — plain RemoveAll leaves the bare repo corrupted
	// and can abort later seeds (fuzz worker "terminated unexpectedly").
	defer removeGitWorktree(cache, workDir)

	root := ProjectRoot(workDir, p)
	if err := ApplyProjectMise(p, root); err != nil {
		return MvAttemptResult{Class: classEnv, Err: fmt.Errorf("mise %s: %w", p.ID, err)}
	}

	// Reuse prefetch mise-data keyed by real pin commit (same as ModeRun).
	imageKey := ImageKey(p, commit)
	session, err := c.runner.StartSession(ctx, p.Isolate, root, imageKey, false)
	if err != nil {
		return MvAttemptResult{Class: classEnv, Err: fmt.Errorf("session %s: %w", p.ID, err)}
	}
	defer func() {
		if cerr := session.Close(ctx); cerr != nil {
			if res.Class == classPass {
				res = MvAttemptResult{Class: classEnv, Err: fmt.Errorf("session close %s: %w", p.ID, cerr)}
			} else {
				slog.Warn("fuzzy: session close failed", "project", p.ID, "err", cerr)
			}
		}
	}()

	if setup := RunSetup(ctx, session, p); !setup.OK() {
		res = MvAttemptResult{Class: classEnv, Err: fmt.Errorf("setup %s: %s", p.ID, formatRunFailureDetail(setup, 4096))}
		return res
	}

	log := c.Log
	if log == nil {
		log = os.Stdout
	}
	// workDir = full checkout. RunMvAttempt applies on ingest_roots then rewrites
	// external consumers (e.g. boltons tests/) without rebasing plan paths.
	res = RunMvAttempt(ctx, p, workDir, in, c.Strict, nil, log)
	if res.Class != classPass {
		if res.Class == classBug && scaffoldDir != "" {
			// Primary edits are ingest-root-relative.
			_ = ScaffoldAttempt(primaryIngestRoot(p, workDir), scaffoldDir, res)
		}
		return res
	}

	check := RunCheck(ctx, session, p)
	if !check.OK() {
		detail := formatRunFailureDetail(check, 4096)
		res.Class = classBug
		res.Err = fmt.Errorf("catalog check after %s %s -> %s: %s", res.Plan.Placement, res.Plan.Source, res.Plan.Destination, detail)
		fmt.Fprintf(log, "mv result: project=%s class=bug catalog_check=%s\n", p.ID, shortRunErr(check))
		if scaffoldDir != "" {
			_ = ScaffoldAttempt(primaryIngestRoot(p, workDir), scaffoldDir, res)
		}
		return res
	}
	fmt.Fprintf(log, "mv result: project=%s class=pass placement=%s (catalog check ok)\n", p.ID, res.Plan.Placement)
	return res
}

// readyLocked is Ready() with c.mu already held.
func (c *CatalogCanvas) readyLocked() error {
	if c.ready {
		return c.readyEr
	}
	c.readyEr = ValidateOfflineReady(c.Workspace, c.Projects, c.NoIsolate)
	c.ready = true
	return c.readyEr
}

// removeGitWorktree unregisters a linked worktree then deletes the directory.
func removeGitWorktree(bareCache, workDir string) {
	if workDir == "" {
		return
	}
	if bareCache != "" {
		// Prefer git's own remove so the bare repo stays consistent.
		cmd := exec.Command("git", "-C", bareCache, "worktree", "remove", "--force", workDir)
		_ = cmd.Run()
		prune := exec.Command("git", "-C", bareCache, "worktree", "prune")
		_ = prune.Run()
	}
	_ = os.RemoveAll(workDir)
}
