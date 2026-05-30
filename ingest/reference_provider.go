package ingest

import (
	"path/filepath"
	"strings"

	refpkg "github.com/lucasew/refactree/pkg/reference"
)

type ImportResolveContext = refpkg.ImportResolveContext
type ProviderSymbolTarget = refpkg.SymbolTarget
type ProviderScopeTarget = refpkg.ScopeTarget

func init() {
	refpkg.RegisterProvider("path", pathReferenceProvider{})
	refpkg.RegisterProvider("node", nodeReferenceProvider{})
	refpkg.RegisterProvider("go", goReferenceProvider{})
	refpkg.RegisterProvider("python", pythonReferenceProvider{})
}

func referenceProviderForName(name string) (refpkg.Provider, bool) {
	return refpkg.ProviderForName(name)
}

func resolveProviderSymbolTarget(ref Reference) (ProviderSymbolTarget, bool, error) {
	return refpkg.ResolveSymbolTarget(ref)
}

func resolveProviderScopeTarget(ref Reference) (ProviderScopeTarget, bool, error) {
	return refpkg.ResolveScopeTarget(ref)
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
