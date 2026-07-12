package fuzzy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PrefetchRunID is the stable worktree name used by prefetch so reruns reuse state.
const PrefetchRunID = "prefetch"

// IngestRunID is the stable worktree name used by ingest-only runs so reruns reuse state.
const IngestRunID = "ingest"

// Workspace manages cached bare clones and mutable worktrees.
type Workspace struct {
	Root string
}

func NewWorkspace(root string) (*Workspace, error) {
	if root == "" {
		root = DefaultWorkRoot()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"cache", "preserve", "runs", "mise-data", "reports"} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Workspace{Root: abs}, nil
}

// ReportsDir is work-root/reports (event logs and scaffolds for harness runs).
func (w *Workspace) ReportsDir() string {
	return filepath.Join(w.Root, "reports")
}

// MiseDataRoot is work-root/mise-data (tool and package-manager caches).
func (w *Workspace) MiseDataRoot() string {
	return filepath.Join(w.Root, "mise-data")
}

func (w *Workspace) cachePath(id string) string {
	return filepath.Join(w.Root, "cache", id+".git")
}

func (w *Workspace) preservePath(id string) string {
	return filepath.Join(w.Root, "preserve", id)
}

func (w *Workspace) runPath(id, runID string) string {
	return filepath.Join(w.Root, "runs", id, runID)
}

// PrepareOptions controls clone, snapshot restore, and reuse behavior.
type PrepareOptions struct {
	Offline bool
	// Reuse keeps an existing worktree at the pinned ref (reset in place) instead
	// of deleting it. Prefetch sets this so repeated runs are idempotent.
	Reuse bool
}

// Prepare returns a work directory at the project pin.
// When offline, uses the bare cache only (no git fetch/clone from URL) and
// requires preserve snapshots written by prefetch.
func (w *Workspace) Prepare(p Project, runID string, opts PrepareOptions) (workDir string, commit string, err error) {
	if opts.Reuse {
		workDir, commit, err = w.reuseWorkDir(p, runID, opts)
		if err != nil {
			return "", "", err
		}
		if workDir != "" {
			return workDir, commit, nil
		}
	}
	return w.prepareFresh(p, runID, opts)
}

func (w *Workspace) reuseWorkDir(p Project, runID string, opts PrepareOptions) (workDir string, commit string, err error) {
	workDir = w.runPath(p.ID, runID)
	st, err := os.Stat(workDir)
	if err != nil || !st.IsDir() {
		return "", "", nil
	}

	if p.LocalPath != "" {
		if err := w.Reset(p, workDir); err != nil {
			return "", "", fmt.Errorf("reuse reset: %w", err)
		}
		return w.finishPrepare(p, workDir, "local", opts)
	}

	if err := w.ensureBare(p, opts.Offline); err != nil {
		return "", "", err
	}
	want, err := revParse(w.cachePath(p.ID), p.Ref)
	if err != nil {
		return "", "", fmt.Errorf("resolve pin %s: %w", p.Ref, err)
	}
	have, err := revParse(workDir, "HEAD")
	if err != nil || have != want {
		return "", "", nil
	}
	if err := w.Reset(p, workDir); err != nil {
		return "", "", fmt.Errorf("reuse reset: %w", err)
	}
	return w.finishPrepare(p, workDir, have, opts)
}

func (w *Workspace) prepareFresh(p Project, runID string, opts PrepareOptions) (workDir string, commit string, err error) {
	workDir = w.runPath(p.ID, runID)
	if err := ForceRemoveAll(workDir); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(workDir), 0o755); err != nil {
		return "", "", err
	}

	if p.LocalPath != "" {
		src, err := filepath.Abs(p.LocalPath)
		if err != nil {
			return "", "", err
		}
		if err := copyDir(src, workDir); err != nil {
			return "", "", err
		}
		return w.finishPrepare(p, workDir, "local", opts)
	}

	if err := w.ensureBare(p, opts.Offline); err != nil {
		return "", "", err
	}
	// Linked worktree from the shallow bare cache (avoids a second full object copy
	// and works when the only tip is refs/rft/pin from a depth-1 fetch).
	cache := w.cachePath(p.ID)
	// Drop stale registrations if a prior run deleted the path with ForceRemoveAll.
	prune := exec.Command("git", "worktree", "prune", "--verbose")
	prune.Dir = cache
	_, _ = prune.CombinedOutput()
	logCmdLine(nil, "git", "-C", cache, "worktree", "add", "--detach", "--force", workDir, p.Ref)
	wt := exec.Command("git", "worktree", "add", "--detach", "--force", workDir, p.Ref)
	wt.Dir = cache
	if out, err := runStreamingCombined(wt, nil); err != nil {
		return "", "", fmt.Errorf("git worktree add %s: %w\n%s", p.Ref, err, out)
	}
	commit, err = revParse(workDir, "HEAD")
	if err != nil {
		return "", "", err
	}
	return w.finishPrepare(p, workDir, commit, opts)
}

func (w *Workspace) finishPrepare(p Project, workDir, commit string, opts PrepareOptions) (string, string, error) {
	if err := w.RestorePreserveSnapshot(p, workDir); err != nil {
		return "", "", err
	}
	if opts.Offline {
		if err := w.requirePreserveSnapshot(p); err != nil {
			return "", "", err
		}
	}
	return workDir, commit, nil
}

// SavePreserveSnapshot copies configured preserve_globs from workDir into the
// durable work-root snapshot used by later Prepare calls (especially --offline).
// The swap is atomic so a failed save leaves the previous snapshot intact.
func (w *Workspace) SavePreserveSnapshot(p Project, workDir string) error {
	if len(p.PreserveGlobs) == 0 {
		return nil
	}
	final := w.preservePath(p.ID)
	tmp := final + ".tmp"
	old := final + ".old"
	if err := ForceRemoveAll(tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	saved, err := copyGlobs(workDir, tmp, p.PreserveGlobs)
	if err != nil {
		_ = ForceRemoveAll(tmp)
		return err
	}
	if len(saved) == 0 {
		_ = ForceRemoveAll(tmp)
		return fmt.Errorf("preserve snapshot for %s: none of %v present after setup", p.ID, p.PreserveGlobs)
	}
	if err := ForceRemoveAll(old); err != nil {
		_ = ForceRemoveAll(tmp)
		return err
	}
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, old); err != nil {
			_ = ForceRemoveAll(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, final); err != nil {
		if _, statErr := os.Stat(old); statErr == nil {
			_ = os.Rename(old, final)
		}
		_ = ForceRemoveAll(tmp)
		return err
	}
	_ = ForceRemoveAll(old)
	return nil
}

// RestorePreserveSnapshot overlays durable preserve snapshots onto workDir.
func (w *Workspace) RestorePreserveSnapshot(p Project, workDir string) error {
	if len(p.PreserveGlobs) == 0 {
		return nil
	}
	srcRoot := w.preservePath(p.ID)
	st, err := os.Stat(srcRoot)
	if err != nil || !st.IsDir() {
		return nil
	}
	_, err = copyGlobs(srcRoot, workDir, p.PreserveGlobs)
	return err
}

func (w *Workspace) requirePreserveSnapshot(p Project) error {
	if len(p.PreserveGlobs) == 0 {
		return nil
	}
	srcRoot := w.preservePath(p.ID)
	st, err := os.Stat(srcRoot)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("offline preserve snapshot missing for %s (run prefetch warmup)", p.ID)
	}
	for _, g := range p.PreserveGlobs {
		if _, err := os.Stat(filepath.Join(srcRoot, filepath.FromSlash(g))); err == nil {
			return nil
		}
	}
	return fmt.Errorf("offline preserve snapshot for %s has none of %v (re-run prefetch warmup)", p.ID, p.PreserveGlobs)
}

// HasPreserveSnapshot reports whether a usable durable snapshot exists.
func (w *Workspace) HasPreserveSnapshot(p Project) bool {
	return w.requirePreserveSnapshot(p) == nil
}

// Reset restores the workdir to the pinned ref, preserving configured globs.
func (w *Workspace) Reset(p Project, workDir string) error {
	preserved, err := stashPreserved(workDir, p.PreserveGlobs)
	if err != nil {
		return err
	}
	defer func() { _ = ForceRemoveAll(preserved.dir) }()

	if p.LocalPath != "" {
		if err := replaceDirContents(workDir, p.LocalPath); err != nil {
			return err
		}
	} else if err := resetGitWorkDir(workDir, p.Ref); err != nil {
		return err
	}
	return preserved.restore(workDir)
}

// replaceDirContents clears dst's entries without removing dst itself, then
// copies src into it. Keeps the directory inode stable for idempotent reuse.
func replaceDirContents(dst, src string) error {
	entries, err := os.ReadDir(dst)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ForceRemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return copyDir(src, dst)
}

func resetGitWorkDir(workDir, ref string) error {
	cmds := [][]string{
		{"git", "reset", "--hard", ref},
		{"git", "clean", "-fdx"},
	}
	for _, argv := range cmds {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			if cleanErr := forceCleanUntracked(workDir); cleanErr != nil {
				return fmt.Errorf("%s: %w\n%s\nforce clean: %v", strings.Join(argv, " "), err, out, cleanErr)
			}
		}
	}
	return nil
}

// forceCleanUntracked removes untracked files that git clean cannot delete
// (typically root-owned build outputs from older container runs).
func forceCleanUntracked(workDir string) error {
	cmd := exec.Command("git", "clean", "-fdxn")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clean -fdxn: %w\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		rel, ok := strings.CutPrefix(line, "Would remove ")
		if !ok || rel == "" {
			continue
		}
		if err := ForceRemoveAll(filepath.Join(workDir, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	cmd = exec.Command("git", "clean", "-fdx")
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean -fdx after force remove: %w\n%s", err, out)
	}
	return nil
}

func (w *Workspace) ensureBare(p Project, offline bool) error {
	cache := w.cachePath(p.ID)
	if st, err := os.Stat(cache); err == nil && st.IsDir() {
		// Already have the pin: do not fetch (idempotent / airgap-friendly).
		if _, err := revParse(cache, p.Ref); err == nil {
			return nil
		}
		if offline {
			return fmt.Errorf("offline cache for %s missing ref %s (run: go test ./internal/fuzzy -run TestPrefetchWarmup with RFT_FUZZY_WARMUP=1): %w", p.ID, p.Ref, err)
		}
		// Pin not in cache yet: shallow-fetch only that tip.
		if err := shallowFetchPin(cache, p.Ref); err != nil {
			return fmt.Errorf("git fetch %s: %w", p.ID, err)
		}
		if _, err := revParse(cache, p.Ref); err != nil {
			return fmt.Errorf("git cache for %s still missing ref %s after fetch: %w", p.ID, p.Ref, err)
		}
		return nil
	}
	if offline {
		return fmt.Errorf("offline git cache missing for %s at %s (run prefetch warmup)", p.ID, cache)
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return err
	}
	// Shallow bare: only the catalog pin (full history is never needed).
	if err := shallowBareClone(p.URL, cache, p.Ref); err != nil {
		_ = ForceRemoveAll(cache)
		return fmt.Errorf("git shallow clone %s: %w", p.URL, err)
	}
	return nil
}

// shallowBareClone creates a bare repo and fetches only ref at depth 1.
// Works for full commit SHAs, tags, and branch names (GitHub and most forges).
func shallowBareClone(url, cache, ref string) error {
	logCmdLine(nil, "git", "init", "--bare", cache)
	initCmd := exec.Command("git", "init", "--bare", cache)
	if out, err := runStreamingCombined(initCmd, nil); err != nil {
		return fmt.Errorf("git init --bare: %w\n%s", err, out)
	}
	logCmdLine(nil, "git", "-C", cache, "remote", "add", "origin", url)
	remoteCmd := exec.Command("git", "remote", "add", "origin", url)
	remoteCmd.Dir = cache
	if out, err := runStreamingCombined(remoteCmd, nil); err != nil {
		return fmt.Errorf("git remote add: %w\n%s", err, out)
	}
	if err := shallowFetchPin(cache, ref); err != nil {
		return err
	}
	if _, err := revParse(cache, ref); err != nil {
		return fmt.Errorf("pin %s missing after shallow fetch: %w", ref, err)
	}
	return nil
}

// shallowFetchPin pulls a single tip into an existing bare (or partial) cache.
// The tip is stored at refs/rft/pin so worktree add / rev-parse always find it
// (a FETCH_HEAD-only fetch leaves no durable ref).
func shallowFetchPin(cache, ref string) error {
	const pinRef = "refs/rft/pin"
	// depth 1: only the catalog pin tip; no full history.
	spec := ref + ":" + pinRef
	args := []string{"fetch", "--depth", "1", "--force", "origin", spec}
	logCmdLine(nil, append([]string{"git", "-C", cache}, args...)...)
	cmd := exec.Command("git", args...)
	cmd.Dir = cache
	if out, err := runStreamingCombined(cmd, nil); err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	// Point HEAD at the pin so tools that read the default branch see a tip.
	logCmdLine(nil, "git", "-C", cache, "symbolic-ref", "HEAD", pinRef)
	head := exec.Command("git", "symbolic-ref", "HEAD", pinRef)
	head.Dir = cache
	if out, err := head.CombinedOutput(); err != nil {
		return fmt.Errorf("symbolic-ref HEAD: %w\n%s", err, out)
	}
	return nil
}

func revParse(dir, rev string) (string, error) {
	cmd := exec.Command("git", "rev-parse", rev)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type preservedSet struct {
	dir   string
	items []string
}

func stashPreserved(workDir string, globs []string) (*preservedSet, error) {
	ps := &preservedSet{}
	if len(globs) == 0 {
		return ps, nil
	}
	tmp, err := os.MkdirTemp("", "rft-fuzzy-preserve-*")
	if err != nil {
		return nil, err
	}
	ps.dir = tmp
	items, err := moveGlobs(workDir, tmp, globs)
	if err != nil {
		_ = ForceRemoveAll(tmp)
		return nil, err
	}
	ps.items = items
	return ps, nil
}

func (ps *preservedSet) restore(workDir string) error {
	if ps == nil || ps.dir == "" || len(ps.items) == 0 {
		return nil
	}
	_, err := moveGlobs(ps.dir, workDir, ps.items)
	return err
}

// copyGlobs copies each present glob from srcRoot to dstRoot, replacing destinations.
func copyGlobs(srcRoot, dstRoot string, globs []string) ([]string, error) {
	return transferGlobs(srcRoot, dstRoot, globs, false)
}

// moveGlobs renames each present glob from srcRoot to dstRoot, copying on cross-device failure.
func moveGlobs(srcRoot, dstRoot string, globs []string) ([]string, error) {
	return transferGlobs(srcRoot, dstRoot, globs, true)
}

func transferGlobs(srcRoot, dstRoot string, globs []string, move bool) ([]string, error) {
	var done []string
	for _, g := range globs {
		rel := filepath.FromSlash(g)
		src := filepath.Join(srcRoot, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(dstRoot, rel)
		if err := ForceRemoveAll(dst); err != nil {
			return done, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return done, err
		}
		if move {
			if err := os.Rename(src, dst); err != nil {
				if err := copyDir(src, dst); err != nil {
					return done, fmt.Errorf("%s: %w", g, err)
				}
				if err := ForceRemoveAll(src); err != nil {
					return done, err
				}
			}
		} else if err := copyDir(src, dst); err != nil {
			return done, fmt.Errorf("%s: %w", g, err)
		}
		done = append(done, g)
	}
	return done, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		// Symlinks first: Type() has mode bits without a full stat; WalkDir does not follow links.
		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode())
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
