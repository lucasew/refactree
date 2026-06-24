package goref

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SymbolTarget represents the physical directory mapped to a Go package,
// used for symbol ingestion and documentation generation.
type SymbolTarget struct {
	Dir    string
	Symbol string
}

// ResolveImport translates a Go-native import statement (e.g. "fmt" or "github.com/org/repo")
// into a standardized reference string (e.g. "go:fmt"). If the target corresponds
// to a known local directory, it resolves as a local "path" reference instead.
func ResolveImport(spec string, knownDirs map[string]bool) string {
	last := lastPathComponent(spec)
	if knownDirs[last] {
		return "path:./" + last
	}
	return "go:" + spec
}

// ResolveSymbolTarget resolves a "go:<pkg>::<symbol>" reference down to a concrete
// physical directory on disk. The workDir parameter provides the module context
// (e.g. the project root) for the underlying `go list` toolchain invocation.
func ResolveSymbolTarget(pkgPath, symbol, workDir string) (SymbolTarget, bool, error) {
	if symbol == "" {
		return SymbolTarget{}, false, nil
	}
	pkgDir, err := ResolvePackageDir(pkgPath, workDir)
	if err != nil {
		return SymbolTarget{}, true, err
	}

	return SymbolTarget{
		Dir:    pkgDir,
		Symbol: symbol,
	}, true, nil
}

// ResolvePackageDir translates a Go package path into its absolute filesystem location.
// It prioritizes the local module context (workDir) to ensure workspace or replaced
// dependencies are correctly identified via `go list`, falling back to the standard
// library or the global module cache.
func ResolvePackageDir(pkgPath, workDir string) (string, error) {
	pkgPath = strings.Trim(pkgPath, "/")
	if pkgPath == "" {
		return "", fmt.Errorf("go provider package path is empty")
	}

	if dir, ok := resolveStdlibPackageDir(pkgPath); ok {
		return dir, nil
	}

	// Prefer go list from the project root so the current module and its
	// replace/workspace deps win over a random module-cache hit.
	if dir, err := goListDir(workDir, "list", "-f", "{{.Dir}}", pkgPath); err == nil {
		return dir, nil
	}

	if dir, ok := resolveModuleCachePackageDir(pkgPath); ok {
		return dir, nil
	}

	return "", fmt.Errorf("go package not found: %s", pkgPath)
}

var (
	goRootOnce sync.Once
	goRoot     string
)

func goEnvGOROOT() string {
	goRootOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "env", "GOROOT")
		out, err := cmd.Output()
		if err != nil {
			return
		}
		goRoot = strings.TrimSpace(string(out))
	})
	return goRoot
}

func resolveStdlibPackageDir(pkgPath string) (string, bool) {
	root := goEnvGOROOT()
	if root == "" {
		return "", false
	}
	pkgDir := filepath.Join(root, "src", filepath.FromSlash(pkgPath))
	st, err := os.Stat(pkgDir)
	if err != nil || !st.IsDir() {
		return "", false
	}
	return pkgDir, true
}

func goListDir(workDir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("go command timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		return "", err
	}

	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("go list returned empty directory")
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("go list directory is invalid: %s", dir)
	}
	return dir, nil
}

func resolveModuleCachePackageDir(pkgPath string) (string, bool) {
	modCache, ok := moduleCacheDir()
	if !ok {
		return "", false
	}

	parts := strings.Split(pkgPath, "/")
	for i := len(parts); i >= 1; i-- {
		modPath := strings.Join(parts[:i], "/")
		subPath := strings.Join(parts[i:], "/")

		pattern := filepath.Join(modCache, escapeModulePath(modPath)+"@*")
		matches, _ := filepath.Glob(pattern)
		if len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		for j := len(matches) - 1; j >= 0; j-- {
			candidate := matches[j]
			if subPath != "" {
				candidate = filepath.Join(candidate, filepath.FromSlash(subPath))
			}
			st, err := os.Stat(candidate)
			if err == nil && st.IsDir() {
				return candidate, true
			}
		}
	}

	return "", false
}

func moduleCacheDir() (string, bool) {
	if v := strings.TrimSpace(os.Getenv("GOMODCACHE")); v != "" {
		if st, err := os.Stat(v); err == nil && st.IsDir() {
			return v, true
		}
	}

	if v := strings.TrimSpace(os.Getenv("GOPATH")); v != "" {
		for _, gp := range filepath.SplitList(v) {
			if gp == "" {
				continue
			}
			candidate := filepath.Join(gp, "pkg", "mod")
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				return candidate, true
			}
		}
	}

	if v, err := goListDir("", "env", "GOMODCACHE"); err == nil {
		return v, true
	}

	return "", false
}

func escapeModulePath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b.WriteByte('!')
			b.WriteByte(c + ('a' - 'A'))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func lastPathComponent(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
