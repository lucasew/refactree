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
	if err := os.MkdirAll(filepath.Join(abs, "cache"), 0o755); err != nil {
		return nil, err
	}
	return &Workspace{Root: abs}, nil
}

func (w *Workspace) cachePath(id string) string {
	return filepath.Join(w.Root, "cache", id+".git")
}

func (w *Workspace) runPath(id, runID string) string {
	return filepath.Join(w.Root, "runs", id, runID)
}

// Prepare returns a clean work directory at the project pin.
func (w *Workspace) Prepare(p Project, runID string) (workDir string, commit string, err error) {
	if p.LocalPath != "" {
		src, err := filepath.Abs(p.LocalPath)
		if err != nil {
			return "", "", err
		}
		workDir = w.runPath(p.ID, runID)
		if err := os.RemoveAll(workDir); err != nil {
			return "", "", err
		}
		if err := copyDir(src, workDir); err != nil {
			return "", "", err
		}
		return workDir, "local", nil
	}

	if err := w.ensureBare(p); err != nil {
		return "", "", err
	}
	workDir = w.runPath(p.ID, runID)
	if err := os.RemoveAll(workDir); err != nil {
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
	return workDir, commit, nil
}

// Reset restores the workdir to the pinned ref, preserving configured globs.
func (w *Workspace) Reset(p Project, workDir string) error {
	if p.LocalPath != "" {
		// Re-copy from local source.
		preserved, err := stashPreserved(workDir, p.PreserveGlobs)
		if err != nil {
			return err
		}
		defer os.RemoveAll(preserved.dir)
		if err := os.RemoveAll(workDir); err != nil {
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
	defer os.RemoveAll(preserved.dir)

	cmds := [][]string{
		{"git", "reset", "--hard", p.Ref},
		{"git", "clean", "-fdx"},
	}
	for _, argv := range cmds {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(argv, " "), err, out)
		}
	}
	return preserved.restore(workDir)
}

func (w *Workspace) ensureBare(p Project) error {
	cache := w.cachePath(p.ID)
	if st, err := os.Stat(cache); err == nil && st.IsDir() {
		cmd := exec.Command("git", "fetch", "--force", "origin", p.Ref)
		cmd.Dir = cache
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch %s: %w\n%s", p.ID, err, out)
		}
		return nil
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
			_ = os.RemoveAll(src)
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
		_ = os.RemoveAll(dst)
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
