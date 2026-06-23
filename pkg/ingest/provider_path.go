package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	refpkg "github.com/lucasew/refactree/pkg/reference"
)

type pathReferenceProvider struct{}

func NewPathReferenceProvider() refpkg.Provider { return pathReferenceProvider{} }

func init() {
	RegisterReferenceProvider("path", NewPathReferenceProvider())
}

func (pathReferenceProvider) Name() string { return "path" }

func (pathReferenceProvider) Resolve(spec string, ctx ImportResolveContext) (string, bool) {
	if !(strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/")) {
		return "", false
	}

	rel := relImportPath(ctx.ImporterPath, spec)
	if rel == "" {
		return "", false
	}

	if ref, ok := resolveKnownJSPath(rel, ctx.KnownFiles); ok {
		return ref, true
	}

	rootAbs, err := filepath.Abs(ctx.RootDir)
	if err != nil {
		return FileRef("./" + rel), true
	}
	candidate := filepath.Join(rootAbs, filepath.FromSlash(rel))
	resolved, ok := resolveJSFileOnDisk(candidate, false)
	if ok {
		return pathRefForAbs(rootAbs, resolved), true
	}

	return FileRef("./" + rel), true
}

func resolveKnownJSPath(rel string, knownFiles map[string]bool) (string, bool) {
	if knownFiles[rel] {
		return FileRef("./" + rel), true
	}
	for _, ext := range []string{".js", ".mjs", ".cjs"} {
		if knownFiles[rel+ext] {
			return FileRef("./" + rel + ext), true
		}
	}
	for _, indexName := range []string{"index.js", "index.mjs", "index.cjs"} {
		p := filepath.ToSlash(filepath.Join(rel, indexName))
		if knownFiles[p] {
			return FileRef("./" + p), true
		}
	}
	return "", false
}

func resolveJSFileOnDisk(baseAbs string, preferPackageMain bool) (string, bool) {
	if st, err := os.Stat(baseAbs); err == nil {
		if !st.IsDir() {
			return baseAbs, true
		}

		if preferPackageMain {
			if mainEntry, ok := readPackageMain(filepath.Join(baseAbs, "package.json")); ok {
				mainAbs := filepath.Join(baseAbs, filepath.FromSlash(mainEntry))
				if resolved, ok := resolveJSFileOnDisk(mainAbs, false); ok {
					return resolved, true
				}
			}
		}

		for _, indexName := range []string{"index.js", "index.mjs", "index.cjs"} {
			candidate := filepath.Join(baseAbs, indexName)
			if st2, err := os.Stat(candidate); err == nil && !st2.IsDir() {
				return candidate, true
			}
		}
	}

	for _, ext := range []string{".js", ".mjs", ".cjs"} {
		candidate := baseAbs + ext
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, true
		}
	}

	return "", false
}

func readPackageMain(packageJSONPath string) (string, bool) {
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return "", false
	}
	var pkg struct {
		Main string `json:"main"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false
	}
	if pkg.Main == "" {
		return "", false
	}
	return filepath.ToSlash(pkg.Main), true
}
