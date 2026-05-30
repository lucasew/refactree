package goref

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

	if dir, err := goListDir("list", "-f", "{{.Dir}}", pkgPath); err == nil {
		return dir, nil
	}

	if dir, err := goListDir("list", "-m", "-f", "{{.Dir}}", pkgPath); err == nil {
		return dir, nil
	}
	return "", fmt.Errorf("go package or module not found: %s", pkgPath)
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
	cmd := exec.Command("go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
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

func lastPathComponent(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
