package goref

import (
	"fmt"
	"os"
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

	pkgPath = strings.Trim(pkgPath, "/")
	if pkgPath == "" {
		return SymbolTarget{}, true, fmt.Errorf("go provider package path is empty")
	}

	goRoot := runtime.GOROOT()
	if goRoot == "" {
		return SymbolTarget{}, true, fmt.Errorf("go provider could not determine GOROOT")
	}

	pkgDir := filepath.Join(goRoot, "src", filepath.FromSlash(pkgPath))
	st, err := os.Stat(pkgDir)
	if err != nil || !st.IsDir() {
		return SymbolTarget{}, true, fmt.Errorf("go package not found in stdlib: %s", pkgPath)
	}

	return SymbolTarget{
		Dir:    pkgDir,
		Symbol: symbol,
	}, true, nil
}

func lastPathComponent(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
