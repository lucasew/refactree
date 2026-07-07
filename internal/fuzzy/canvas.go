package fuzzy

import (
	"context"
	"fmt"
	"io"
	"os"
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
		if len(p.Mv.Ops) == 0 {
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
func NewCatalogCanvas(workRoot, catalogPath string, noIsolate bool) (*CatalogCanvas, error) {
	projects, err := LoadCatalogCanvas(catalogPath)
	if err != nil {
		return nil, err
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
	if c.ready {
		return c.readyEr
	}
	c.readyEr = ValidateOfflineReady(c.Workspace, c.Projects, c.NoIsolate)
	c.ready = true
	return c.readyEr
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
func (c *CatalogCanvas) Attempt(ctx context.Context, projectIdx int, in PlanInput, scaffoldDir string) MvAttemptResult {
	if err := c.Ready(); err != nil {
		return MvAttemptResult{Class: classEnv, Err: fmt.Errorf("canvas not ready: %w", err)}
	}
	p := c.Project(projectIdx)

	runID := fmt.Sprintf("gofuzz-%d-%d-%d-%d", projectIdx, in.OpIndex, in.EntityIndex, in.Entropy)
	workDir, commit, err := c.Workspace.Prepare(p, runID, PrepareOptions{Offline: c.Offline, Reuse: false})
	if err != nil {
		return MvAttemptResult{Class: classEnv, Err: fmt.Errorf("prepare %s: %w", p.ID, err)}
	}
	// Best-effort cleanup of the disposable worktree after the attempt.
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

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
	defer func() { _ = session.Close(ctx) }()

	if setup := RunSetup(ctx, session, p); !setup.OK() {
		return MvAttemptResult{Class: classEnv, Err: fmt.Errorf("setup %s: %s", p.ID, shortRunErr(setup))}
	}

	ingestRoot := primaryIngestRoot(p, workDir)
	res := RunMvAttempt(ctx, p, ingestRoot, in, c.Strict, nil)
	if res.Class != "pass" {
		if res.Class == classBug && scaffoldDir != "" {
			_ = ScaffoldAttempt(ingestRoot, scaffoldDir, res)
		}
		return res
	}

	check := RunCheck(ctx, session, p)
	if !check.OK() {
		res.Class = classBug
		res.Err = fmt.Errorf("catalog check after %s %s -> %s: %s", res.Plan.Op, res.Plan.Src, res.Plan.Dst, shortRunErr(check))
		if scaffoldDir != "" {
			_ = ScaffoldAttempt(ingestRoot, scaffoldDir, res)
		}
		return res
	}
	return res
}
