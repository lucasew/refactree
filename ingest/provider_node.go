package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type nodeReferenceProvider struct{}

func (nodeReferenceProvider) Name() string { return "node" }

func (nodeReferenceProvider) Resolve(spec string, ctx ImportResolveContext) (string, bool) {
	if strings.HasPrefix(spec, "node:") {
		return spec, true
	}
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") {
		return "", false
	}

	pkgName, subpath := splitNodePackageSpecifier(spec)
	if pkgName == "" {
		return "node:" + spec, true
	}

	rootAbs, err := filepath.Abs(ctx.RootDir)
	if err != nil {
		rootAbs = ctx.RootDir
	}
	importerAbs := filepath.Join(rootAbs, filepath.FromSlash(filepath.Dir(ctx.ImporterPath)))

	for _, pkgRoot := range nodeModuleCandidates(importerAbs, pkgName) {
		targetBase := pkgRoot
		preferPackageMain := subpath == ""
		if subpath != "" {
			targetBase = filepath.Join(pkgRoot, filepath.FromSlash(subpath))
			preferPackageMain = false
		}

		if resolved, ok := resolveJSFileOnDisk(targetBase, preferPackageMain); ok {
			return pathRefForAbs(rootAbs, resolved), true
		}
		if st, err := os.Stat(targetBase); err == nil && st.IsDir() {
			return pathRefForAbs(rootAbs, targetBase), true
		}
	}

	return "node:" + spec, true
}

func nodeModuleCandidates(importerAbsDir, pkgName string) []string {
	seen := map[string]bool{}
	out := []string{}

	for dir := importerAbsDir; ; {
		candidate := filepath.Join(dir, "node_modules", filepath.FromSlash(pkgName))
		if !seen[candidate] {
			seen[candidate] = true
			out = append(out, candidate)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if nodePath := os.Getenv("NODE_PATH"); nodePath != "" {
		for _, entry := range filepath.SplitList(nodePath) {
			if entry == "" {
				continue
			}
			candidate := filepath.Join(entry, filepath.FromSlash(pkgName))
			if !seen[candidate] {
				seen[candidate] = true
				out = append(out, candidate)
			}
		}
	}

	return out
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
