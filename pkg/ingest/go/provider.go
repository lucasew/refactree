package ingestgo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// SymbolTarget represents a provider-backed symbol location.
type SymbolTarget struct {
	Dir    string
	Symbol string
}

// ResolveImport resolves a Go import path into a canonical reference string.
func ResolveImport(spec string, knownDirs map[string]bool) string {
	last := lastPathComponent(spec)
	if knownDirs[last] {
		return "path:./" + last
	}
	return "go:" + spec
}

// ResolveSymbolTarget resolves a go:<pkg>::<symbol> reference to a concrete
// package directory in the installed Go standard library.
func ResolveSymbolTarget(pkgPath, symbol string) (SymbolTarget, bool, error) {
	if symbol == "" {
		return SymbolTarget{}, false, nil
	}
	pkgDir, err := ResolvePackageDir(pkgPath)
	if err != nil {
		return SymbolTarget{}, true, err
	}

	return SymbolTarget{
		Dir:    pkgDir,
		Symbol: symbol,
	}, true, nil
}

// ResolvePackageDir resolves a Go import path to a concrete package directory.
func ResolvePackageDir(pkgPath string) (string, error) {
	pkgPath = strings.Trim(pkgPath, "/")
	if pkgPath == "" {
		return "", fmt.Errorf("go provider package path is empty")
	}

	if dir, ok := resolveStdlibPackageDir(pkgPath); ok {
		return dir, nil
	}

	if dir, ok := resolveModuleCachePackageDir(pkgPath); ok {
		return dir, nil
	}

	if dir, err := goListDir("list", "-f", "{{.Dir}}", pkgPath); err == nil {
		return dir, nil
	}
	return "", fmt.Errorf("go package not found: %s", pkgPath)
}

func resolveStdlibPackageDir(pkgPath string) (string, bool) {
	goRoot := runtime.GOROOT()
	if goRoot == "" {
		return "", false
	}
	pkgDir := filepath.Join(goRoot, "src", filepath.FromSlash(pkgPath))
	st, err := os.Stat(pkgDir)
	if err != nil || !st.IsDir() {
		return "", false
	}
	return pkgDir, true
}

func goListDir(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
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

	if v, err := goListDir("env", "GOMODCACHE"); err == nil {
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

