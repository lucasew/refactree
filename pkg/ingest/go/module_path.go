package ingestgo

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// Ensure moveDriver implements PackageImportMatcher.
var _ ingest.PackageImportMatcher = moveDriver{}

// ImportPathUnderPackageTree reports whether a Go import path refers to
// project-relative packageDir or a subpackage (module/packageDir/...).
func (moveDriver) ImportPathUnderPackageTree(rootDir, importPath, packageDir string) bool {
	packageDir = ingest.CleanRelDir(packageDir)
	p := strings.Trim(strings.TrimPrefix(importPath, "./"), "/")
	if packageDir == "" || p == "" {
		return false
	}
	if mod := readGoModulePath(rootDir); mod != "" {
		prefix := strings.Trim(mod, "/") + "/" + packageDir
		return p == prefix || strings.HasPrefix(p, prefix+"/")
	}
	// Fixtures without go.mod: example/helperpkg, example/cmd/codegen, …
	return p == packageDir || strings.HasPrefix(p, packageDir+"/") || strings.HasSuffix(p, "/"+packageDir)
}

// ImportPathIsPackage reports whether importPath is exactly the package at
// packageDir (module/packageDir), not a subpackage.
func (moveDriver) ImportPathIsPackage(rootDir, importPath, packageDir string) bool {
	packageDir = ingest.CleanRelDir(packageDir)
	p := strings.Trim(strings.TrimPrefix(importPath, "./"), "/")
	if p == "" {
		return false
	}
	if mod := readGoModulePath(rootDir); mod != "" {
		want := strings.Trim(mod, "/")
		if packageDir != "" {
			want = want + "/" + packageDir
		}
		return p == want
	}
	if packageDir == "" {
		return false
	}
	return p == packageDir
}

// readGoModulePath returns the module path from rootDir/go.mod, or "".
// Only rootDir is consulted (no parent walk — temp fixture roots must not
// inherit an unrelated module from outer directories).
func readGoModulePath(rootDir string) string {
	if rootDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
