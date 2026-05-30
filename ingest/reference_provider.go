package ingest

import (
	"path/filepath"
	"strings"
)

// ImportResolveContext carries filesystem and index metadata needed by
// provider-backed import resolution.
type ImportResolveContext struct {
	RootDir      string
	ImporterPath string
	KnownFiles   map[string]bool
	KnownDirs    map[string]bool
}

// ReferenceProvider resolves import specs into canonical references.
type ReferenceProvider interface {
	Name() string
	Resolve(spec string, ctx ImportResolveContext) (string, bool)
}

func referenceProviderForName(name string) (ReferenceProvider, bool) {
	p, ok := builtInReferenceProviders[name]
	return p, ok
}

var builtInReferenceProviders = map[string]ReferenceProvider{
	"path":   pathReferenceProvider{},
	"node":   nodeReferenceProvider{},
	"go":     goReferenceProvider{},
	"python": pythonReferenceProvider{},
}

func pathRefForAbs(rootDir, absPath string) string {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return FileRef(filepath.ToSlash(absPath))
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return FileRef(filepath.ToSlash(absPath))
	}

	rel, err := filepath.Rel(rootAbs, abs)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return FileRef("./" + filepath.ToSlash(rel))
	}
	return FileRef(filepath.ToSlash(abs))
}

func relImportPath(importerPath, spec string) string {
	importerDir := filepath.ToSlash(filepath.Dir(importerPath))
	if importerDir == "." {
		importerDir = ""
	}
	if strings.HasPrefix(spec, "/") {
		return strings.TrimPrefix(filepath.ToSlash(spec), "/")
	}
	joined := filepath.ToSlash(filepath.Clean(filepath.Join(importerDir, spec)))
	joined = strings.TrimPrefix(joined, "./")
	return joined
}

func splitNodePackageSpecifier(spec string) (pkgName, subpath string) {
	parts := strings.Split(spec, "/")
	if spec == "" {
		return "", ""
	}
	if strings.HasPrefix(spec, "@") {
		if len(parts) < 2 {
			return spec, ""
		}
		pkgName = parts[0] + "/" + parts[1]
		if len(parts) > 2 {
			subpath = strings.Join(parts[2:], "/")
		}
		return pkgName, subpath
	}
	pkgName = parts[0]
	if len(parts) > 1 {
		subpath = strings.Join(parts[1:], "/")
	}
	return pkgName, subpath
}
