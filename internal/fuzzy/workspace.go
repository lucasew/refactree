package fuzzy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Workspace manages cached bare clones and mutable worktrees.
type Workspace struct {
	Root string
}

func NewWorkspace(root string) (*Workspace, error) {
	if root == "" {
		root = filepath.Join(os.TempDir(), "rft-fuzzy")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"cache", "preserve", "runs", "mise-data"} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Workspace{Root: abs}, nil
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

// PrepareOptions controls clone and snapshot restore behavior.
type PrepareOptions struct {
	Offline bool
}

// Prepare returns a clean work directory at the project pin.
// When offline, uses the bare cache only (no git fetch/clone from URL) and
// restores preserve snapshots written by prefetch.
func (w *Workspace) Prepare(p Project, runID string, opts PrepareOptions) (workDir string, commit string, err error) {
	if p.LocalPath != "" {
		src, err := filepath.Abs(p.LocalPath)
		if err != nil {
			return "", "", err
		}
		workDir = w.runPath(p.ID, runID)
		if err := ForceRemoveAll(workDir); err != nil {
			return "", "", err
		}
		if err := copyDir(src, workDir); err != nil {
			return "", "", err
		}
		if err := w.RestorePreserveSnapshot(p, workDir); err != nil {
			return "", "", err
		}
		if opts.Offline {
			if err := w.requirePreserveSnapshot(p); err != nil {
				return "", "", err
			}
		}
		return workDir, "local", nil
	}

	if err := w.ensureBare(p, opts.Offline); err != nil {
		return "", "", err
	}
	workDir = w.runPath(p.ID, runID)
	if err := ForceRemoveAll(workDir); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(workDir), 0o755); err != nil {
		return "", "", err
	}
	cmd := exec.Command("git", "clone", "--no-checkout", w.cachePath(p.ID), workDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git clone worktree: %w\n%s", err, out)
	}
	co := exec.Command("git", "checkout", "--force", p.Ref)
	co.Dir = workDir
	if out, err := co.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git checkout %s: %w\n%s", p.Ref, err, out)
	}
	commit, err = revParse(workDir, "HEAD")
	if err != nil {
		return "", "", err
	}
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
func (w *Workspace) SavePreserveSnapshot(p Project, workDir string) error {
	if len(p.PreserveGlobs) == 0 {
		return nil
	}
	dstRoot := w.preservePath(p.ID)
	if err := ForceRemoveAll(dstRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return err
	}
	saved := 0
	for _, g := range p.PreserveGlobs {
		src := filepath.Join(workDir, filepath.FromSlash(g))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(dstRoot, filepath.FromSlash(g))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("snapshot %s: %w", g, err)
		}
		saved++
	}
	if saved == 0 {
		return fmt.Errorf("preserve snapshot for %s: none of %v present after setup", p.ID, p.PreserveGlobs)
	}
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
	for _, g := range p.PreserveGlobs {
		src := filepath.Join(srcRoot, filepath.FromSlash(g))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(workDir, filepath.FromSlash(g))
		if err := ForceRemoveAll(dst); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("restore snapshot %s: %w", g, err)
		}
	}
	return nil
}

func (w *Workspace) requirePreserveSnapshot(p Project) error {
	if len(p.PreserveGlobs) == 0 {
		return nil
	}
	srcRoot := w.preservePath(p.ID)
	st, err := os.Stat(srcRoot)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("offline preserve snapshot missing for %s (run: rft fuzzy prefetch --project %s)", p.ID, p.ID)
	}
	for _, g := range p.PreserveGlobs {
		src := filepath.Join(srcRoot, filepath.FromSlash(g))
		if _, err := os.Stat(src); err == nil {
			return nil
		}
	}
	return fmt.Errorf("offline preserve snapshot for %s has none of %v (re-run prefetch)", p.ID, p.PreserveGlobs)
}

// Reset restores the workdir to the pinned ref, preserving configured globs.
func (w *Workspace) Reset(p Project, workDir string) error {
	if p.LocalPath != "" {
		// Re-copy from local source.
		preserved, err := stashPreserved(workDir, p.PreserveGlobs)
		if err != nil {
			return err
		}
		defer func() { _ = ForceRemoveAll(preserved.dir) }()
		if err := ForceRemoveAll(workDir); err != nil {
			return err
		}
		if err := copyDir(p.LocalPath, workDir); err != nil {
			return err
		}
		return preserved.restore(workDir)
	}

	preserved, err := stashPreserved(workDir, p.PreserveGlobs)
	if err != nil {
		return err
	}
	defer func() { _ = ForceRemoveAll(preserved.dir) }()

	cmds := [][]string{
		{"git", "reset", "--hard", p.Ref},
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
	return preserved.restore(workDir)
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
		if offline {
			if _, err := revParse(cache, p.Ref); err != nil {
				return fmt.Errorf("offline cache for %s missing ref %s (run: rft fuzzy prefetch --project %s): %w", p.ID, p.Ref, p.ID, err)
			}
			return nil
		}
		cmd := exec.Command("git", "fetch", "--force", "origin", p.Ref)
		cmd.Dir = cache
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch %s: %w\n%s", p.ID, err, out)
		}
		return nil
	}
	if offline {
		return fmt.Errorf("offline git cache missing for %s at %s (run: rft fuzzy prefetch --project %s)", p.ID, cache, p.ID)
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "--bare", p.URL, cache)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone --bare %s: %w\n%s", p.URL, err, out)
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
	for _, g := range globs {
		src := filepath.Join(workDir, filepath.FromSlash(g))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(tmp, filepath.FromSlash(g))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(src, dst); err != nil {
			if err := copyDir(src, dst); err != nil {
				return nil, err
			}
			if err := ForceRemoveAll(src); err != nil {
				return nil, err
			}
		}
		ps.items = append(ps.items, g)
	}
	return ps, nil
}

func (ps *preservedSet) restore(workDir string) error {
	if ps == nil || ps.dir == "" {
		return nil
	}
	for _, g := range ps.items {
		src := filepath.Join(ps.dir, filepath.FromSlash(g))
		dst := filepath.Join(workDir, filepath.FromSlash(g))
		if err := ForceRemoveAll(dst); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
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
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
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
