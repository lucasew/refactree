package javaref

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SymbolTarget points at a directory to ingest for a java: type reference.
type SymbolTarget struct {
	Dir  string
	Name string
}

// ModuleTarget is a resolved package or compilation unit on disk.
type ModuleTarget struct {
	Dir  string
	File string // basename when the target is a single .java file
}

// ResolveImport maps a Java type/package spec to a path: or java: reference.
// knownFiles keys are slash-separated paths relative to the ingest root.
func ResolveImport(spec string, knownFiles map[string]bool) string {
	spec = normalizeTypeSpec(spec)
	if spec == "" {
		return ""
	}
	if ref, ok := resolveKnownType(spec, knownFiles); ok {
		return ref
	}
	if ref, ok := resolveKnownPackage(spec, knownFiles); ok {
		return ref
	}
	return "java:" + spec
}

// ResolveTypeFile returns the relative path of the .java file for spec, if known.
func ResolveTypeFile(spec string, knownFiles map[string]bool) (string, bool) {
	spec = normalizeTypeSpec(spec)
	if spec == "" {
		return "", false
	}
	for _, candidate := range typeFileCandidates(spec) {
		if knownFiles[candidate] {
			return candidate, true
		}
	}
	return "", false
}

// ResolvePackageDir resolves java:<package> to a filesystem directory under rootDir
// using package-path mapping and common source roots (src/main/java, src/test/java, src).
func ResolvePackageDir(spec, rootDir string) (string, error) {
	pkg := normalizeTypeSpec(spec)
	if pkg == "" {
		return "", fmt.Errorf("java provider package path is empty")
	}
	rel := strings.ReplaceAll(pkg, ".", "/")
	for _, dir := range packageDirCandidates(rootDir, rel) {
		st, err := os.Stat(dir)
		if err == nil && st.IsDir() && dirHasJavaSources(dir) {
			return dir, nil
		}
	}
	return "", fmt.Errorf("java package not found: %s", pkg)
}

// ResolveSymbolTarget resolves java:<type-or-package>::<symbol>.
func ResolveSymbolTarget(spec, symbol, rootDir string) (SymbolTarget, bool, error) {
	if symbol == "" {
		return SymbolTarget{}, false, nil
	}
	spec = normalizeTypeSpec(spec)
	if spec == "" {
		return SymbolTarget{}, false, nil
	}

	if file, ok := resolveTypeFileOnDisk(spec, rootDir); ok {
		return SymbolTarget{Dir: filepath.Dir(file), Name: symbol}, true, nil
	}

	pkg := spec
	if idx := strings.LastIndex(spec, "."); idx >= 0 {
		pkg = spec[:idx]
	}
	dir, err := ResolvePackageDir(pkg, rootDir)
	if err != nil {
		dir, err = ResolvePackageDir(spec, rootDir)
		if err != nil {
			return SymbolTarget{}, true, err
		}
	}
	return SymbolTarget{Dir: dir, Name: symbol}, true, nil
}

// ResolveModuleTarget resolves java:<spec> to a package dir and optional file.
func ResolveModuleTarget(spec, rootDir string) (ModuleTarget, error) {
	spec = normalizeTypeSpec(spec)
	if spec == "" {
		return ModuleTarget{}, fmt.Errorf("java provider module path is empty")
	}
	if file, ok := resolveTypeFileOnDisk(spec, rootDir); ok {
		return ModuleTarget{Dir: filepath.Dir(file), File: filepath.Base(file)}, nil
	}
	dir, err := ResolvePackageDir(spec, rootDir)
	if err != nil {
		return ModuleTarget{}, err
	}
	return ModuleTarget{Dir: dir}, nil
}

// MatchesEntityPath reports whether entPath (relative to target.Dir) belongs to target.
func MatchesEntityPath(target ModuleTarget, entPath string) bool {
	rel := filepath.ToSlash(entPath)
	if target.File == "" {
		return true
	}
	return rel == filepath.ToSlash(target.File)
}

func resolveKnownType(spec string, knownFiles map[string]bool) (string, bool) {
	if file, ok := ResolveTypeFile(spec, knownFiles); ok {
		return "path:./" + file, true
	}
	return "", false
}

func resolveKnownPackage(spec string, knownFiles map[string]bool) (string, bool) {
	rel := strings.ReplaceAll(spec, ".", "/")
	prefix := rel + "/"
	for file := range knownFiles {
		if file == rel+".java" {
			continue
		}
		if strings.HasPrefix(file, prefix) && strings.HasSuffix(file, ".java") {
			return "path:./" + rel, true
		}
		for _, root := range sourceRootPrefixes(file) {
			if strings.HasPrefix(file, root+prefix) && strings.HasSuffix(file, ".java") {
				return "path:./" + root + rel, true
			}
		}
	}
	return "", false
}

func typeFileCandidates(spec string) []string {
	rel := strings.ReplaceAll(spec, ".", "/") + ".java"
	out := []string{rel}
	for _, root := range []string{"src/main/java/", "src/test/java/", "src/"} {
		out = append(out, root+rel)
	}
	return out
}

func sourceRootPrefixes(file string) []string {
	var roots []string
	for _, root := range []string{"src/main/java/", "src/test/java/", "src/"} {
		if strings.HasPrefix(file, root) {
			roots = append(roots, root)
		}
	}
	return roots
}

func packageDirCandidates(rootDir, rel string) []string {
	relFS := filepath.FromSlash(rel)
	var out []string
	if rootDir == "" {
		return out
	}
	rootAbs := rootDir
	if abs, err := filepath.Abs(rootDir); err == nil {
		rootAbs = abs
	}
	for _, root := range []string{"", "src/main/java", "src/test/java", "src"} {
		if root == "" {
			out = append(out, filepath.Join(rootAbs, relFS))
			continue
		}
		out = append(out, filepath.Join(rootAbs, filepath.FromSlash(root), relFS))
	}
	return out
}

func resolveTypeFileOnDisk(spec, rootDir string) (string, bool) {
	if rootDir == "" {
		return "", false
	}
	rootAbs := rootDir
	if abs, err := filepath.Abs(rootDir); err == nil {
		rootAbs = abs
	}
	for _, candidate := range typeFileCandidates(spec) {
		path := filepath.Join(rootAbs, filepath.FromSlash(candidate))
		st, err := os.Stat(path)
		if err == nil && !st.IsDir() {
			return path, true
		}
	}
	return "", false
}

func dirHasJavaSources(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".java") {
			return true
		}
	}
	return false
}

func normalizeTypeSpec(spec string) string {
	spec = strings.TrimSpace(spec)
	spec = strings.Trim(spec, "/")
	spec = strings.ReplaceAll(spec, "/", ".")
	spec = strings.Trim(spec, ".")
	return spec
}
