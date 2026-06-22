package pythonref

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ModuleTarget struct {
	Dir       string
	File      string
	IsPackage bool
}

type SymbolTarget struct {
	Dir    string
	Symbol string
}

var moduleTargetCache sync.Map

type moduleResolveResult struct {
	Origin     string   `json:"origin"`
	Locations  []string `json:"locations"`
	ModuleFile string   `json:"module_file"`
	SourceFile string   `json:"source_file"`
}

const moduleResolveScript = `
import importlib.util
import importlib
import inspect
import json
import sys

name = sys.argv[1]
spec = importlib.util.find_spec(name)
if spec is None:
    print(json.dumps({"error": "not_found", "name": name}))
    sys.exit(2)

origin = spec.origin
locations = list(spec.submodule_search_locations or [])
module_file = None
source_file = None
try:
    mod = importlib.import_module(name)
    module_file = getattr(mod, "__file__", None)
    try:
        source_file = inspect.getsourcefile(mod)
    except Exception:
        source_file = None
except Exception:
    pass

print(json.dumps({
    "origin": origin,
    "locations": locations,
    "module_file": module_file,
    "source_file": source_file,
}))
`

// ResolveModuleTarget resolves a python:<module> spec into a concrete ingest
// scope directory plus the module file name used for symbol/doc filtering.
// workDir is the project root (serve/browse --dir); when set, resolution uses
// that tree's .venv/bin/python if present so site-packages match the project.
func ResolveModuleTarget(spec, workDir string) (ModuleTarget, error) {
	module := normalizeModuleSpec(spec)
	if module == "" {
		return ModuleTarget{}, fmt.Errorf("python provider module path is empty")
	}

	cacheKey := workDir + "\x00" + module
	if cached, ok := moduleTargetCache.Load(cacheKey); ok {
		return cached.(ModuleTarget), nil
	}

	target, err := resolveModuleTarget(module, workDir)
	if err != nil {
		return ModuleTarget{}, err
	}

	moduleTargetCache.Store(cacheKey, target)
	return target, nil
}

// ResolveSymbolTarget resolves python:<module>::<symbol> to an ingest target.
func ResolveSymbolTarget(spec, symbol, workDir string) (SymbolTarget, bool, error) {
	if symbol == "" {
		return SymbolTarget{}, false, nil
	}
	target, err := ResolveModuleTarget(spec, workDir)
	if err != nil {
		return SymbolTarget{}, true, err
	}
	return SymbolTarget{Dir: target.Dir, Symbol: symbol}, true, nil
}

// MatchesEntityPath reports whether entPath (relative to target.Dir) belongs to
// the resolved module target.
func MatchesEntityPath(target ModuleTarget, entPath string) bool {
	rel := filepath.ToSlash(entPath)
	if target.File == "" {
		return true
	}
	return rel == filepath.ToSlash(target.File)
}

func resolveModuleTarget(module, workDir string) (ModuleTarget, error) {
	cmdPath, err := pythonCommand(workDir)
	if err != nil {
		return ModuleTarget{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdPath, "-c", moduleResolveScript, module)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = pythonCommandEnv(workDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ModuleTarget{}, fmt.Errorf("python module resolution timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return ModuleTarget{}, fmt.Errorf("%s", msg)
		}
		return ModuleTarget{}, fmt.Errorf("python module %q not found", module)
	}

	var result moduleResolveResult
	if err := json.Unmarshal(out, &result); err != nil {
		return ModuleTarget{}, fmt.Errorf("invalid python resolver output: %w", err)
	}

	if len(result.Locations) > 0 {
		dir := result.Locations[0]
		if dir == "" {
			return ModuleTarget{}, fmt.Errorf("python module %q resolved to empty package directory", module)
		}
		file := ""
		origin := strings.TrimSpace(result.Origin)
		if origin != "" && origin != "built-in" && origin != "frozen" {
			originDir := filepath.Dir(origin)
			if samePath(originDir, dir) {
				file = filepath.Base(origin)
			}
		}
		if file == "" {
			for _, candidate := range []string{result.ModuleFile, result.SourceFile} {
				if candidate == "" {
					continue
				}
				candidateDir := filepath.Dir(candidate)
				if samePath(candidateDir, dir) {
					file = filepath.Base(candidate)
					break
				}
			}
		}
		if file == "" {
			initFile := filepath.Join(dir, "__init__.py")
			if st, err := os.Stat(initFile); err == nil && !st.IsDir() {
				file = "__init__.py"
			}
		}
		return ModuleTarget{Dir: dir, File: file, IsPackage: true}, nil
	}

	origin := strings.TrimSpace(result.Origin)
	if origin == "" || origin == "built-in" || origin == "frozen" {
		for _, candidate := range []string{result.ModuleFile, result.SourceFile} {
			if candidate == "" {
				continue
			}
			st, err := os.Stat(candidate)
			if err == nil && !st.IsDir() {
				return ModuleTarget{Dir: filepath.Dir(candidate), File: filepath.Base(candidate), IsPackage: false}, nil
			}
		}
		return ModuleTarget{}, fmt.Errorf("python module %q has no filesystem source", module)
	}
	st, err := os.Stat(origin)
	if err != nil || st.IsDir() {
		return ModuleTarget{}, fmt.Errorf("python module %q resolved to invalid source %q", module, origin)
	}
	return ModuleTarget{Dir: filepath.Dir(origin), File: filepath.Base(origin), IsPackage: false}, nil
}

func pythonCommand(workDir string) (string, error) {
	if workDir != "" {
		for _, rel := range []string{
			filepath.Join(".venv", "bin", "python"),
			filepath.Join(".venv", "bin", "python3"),
			filepath.Join("venv", "bin", "python"),
			filepath.Join("venv", "bin", "python3"),
		} {
			candidate := filepath.Join(workDir, rel)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, nil
			}
		}
	}
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python executable not found (tried project .venv and python3/python on PATH)")
}

func pythonCommandEnv(workDir string) []string {
	env := os.Environ()
	base := filepath.Join(os.TempDir(), "refactree-python")
	uvCache := filepath.Join(base, "uv-cache")
	xdgCache := filepath.Join(base, "xdg-cache")
	_ = os.MkdirAll(uvCache, 0755)
	_ = os.MkdirAll(xdgCache, 0755)
	env = append(env, "UV_CACHE_DIR="+uvCache)
	env = append(env, "XDG_CACHE_HOME="+xdgCache)
	// Prefer project-local imports when resolving inside workDir.
	if workDir != "" {
		pyPath := workDir
		if existing := os.Getenv("PYTHONPATH"); existing != "" {
			pyPath = workDir + string(os.PathListSeparator) + existing
		}
		env = append(env, "PYTHONPATH="+pyPath)
		// Ensure venv site-packages are used when we invoke venv python (automatic);
		// VIRTUAL_ENV helps some tools that inspect it.
		for _, name := range []string{".venv", "venv"} {
			v := filepath.Join(workDir, name)
			if st, err := os.Stat(v); err == nil && st.IsDir() {
				env = append(env, "VIRTUAL_ENV="+v)
				break
			}
		}
	}
	return env
}

func normalizeModuleSpec(spec string) string {
	module := strings.TrimSpace(spec)
	module = strings.Trim(module, "/")
	module = strings.ReplaceAll(module, "/", ".")
	module = strings.Trim(module, ".")
	return module
}

func samePath(a, b string) bool {
	aa := filepath.Clean(a)
	bb := filepath.Clean(b)
	return aa == bb
}
