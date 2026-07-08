package fuzzy

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// Grain is a level in the language module lattice.
type Grain string

const (
	GrainDeclaration Grain = "declaration"
	GrainModule      Grain = "module"
	GrainPackage     Grain = "package"
)

// Placement is where a node goes after a move.
type Placement string

const (
	PlacementRename    Placement = "rename"
	PlacementLayout    Placement = "layout"
	PlacementModule    Placement = "module"
	PlacementNewModule Placement = "new_module"
	PlacementPackage   Placement = "package"
)

// MoveNode is an enumerable source at a grain.
type MoveNode struct {
	Grain     Grain
	// Reference is a full path reference (with symbol for declaration grain).
	Reference string
	// Path is the slash path with leading ./ (file or directory).
	Path string
	// Symbol is non-empty for declaration grain.
	Symbol string
}

// languageMoveModel describes grains and module boundaries for one language.
type languageMoveModel interface {
	Grains() []Grain
	ListNodes(result *ingest.Result, grain Grain, projectLanguage string) []MoveNode
	// SameModule reports whether two paths (./rel) share a module boundary.
	SameModule(pathA, pathB string) bool
	// ModuleKey returns a stable module identity for a file path (./rel).
	ModuleKey(filePath string) string
}

func moveModelForLanguage(language string) (languageMoveModel, error) {
	switch language {
	case "go":
		return goMoveModel{}, nil
	case "java":
		return javaMoveModel{}, nil
	case "python":
		return pythonMoveModel{}, nil
	case "javascript":
		return javascriptMoveModel{}, nil
	default:
		return nil, errUnsupportedLanguage(language)
	}
}

func errUnsupportedLanguage(language string) error {
	return &moveModelError{msg: "unsupported project language for move model: " + language}
}

type moveModelError struct{ msg string }

func (e *moveModelError) Error() string { return e.msg }

// defaultGrainsForLanguage is the full grain set when catalog does not filter.
func defaultGrainsForLanguage(language string) []Grain {
	m, err := moveModelForLanguage(language)
	if err != nil {
		return nil
	}
	return m.Grains()
}

func grainAllowed(language string, grain Grain) bool {
	for _, g := range defaultGrainsForLanguage(language) {
		if g == grain {
			return true
		}
	}
	return false
}

// --- Go: package directory is the module; file is layout only. ---

type goMoveModel struct{}

func (goMoveModel) Grains() []Grain {
	return []Grain{GrainDeclaration, GrainPackage}
}

func (goMoveModel) ModuleKey(filePath string) string {
	return dirKey(filePath)
}

func (m goMoveModel) SameModule(pathA, pathB string) bool {
	return m.ModuleKey(pathA) == m.ModuleKey(pathB)
}

func (goMoveModel) ListNodes(result *ingest.Result, grain Grain, projectLanguage string) []MoveNode {
	switch grain {
	case GrainDeclaration:
		return listDeclarationNodes(result, projectLanguage)
	case GrainPackage:
		return listPackageNodes(result, projectLanguage)
	default:
		return nil
	}
}

// --- Java: package directory is the module. ---

type javaMoveModel struct{}

func (javaMoveModel) Grains() []Grain {
	return []Grain{GrainDeclaration, GrainPackage}
}

func (javaMoveModel) ModuleKey(filePath string) string {
	return dirKey(filePath)
}

func (m javaMoveModel) SameModule(pathA, pathB string) bool {
	return m.ModuleKey(pathA) == m.ModuleKey(pathB)
}

func (javaMoveModel) ListNodes(result *ingest.Result, grain Grain, projectLanguage string) []MoveNode {
	switch grain {
	case GrainDeclaration:
		return listDeclarationNodes(result, projectLanguage)
	case GrainPackage:
		return listPackageNodes(result, projectLanguage)
	default:
		return nil
	}
}

// --- Python: file is a module; directory packages are a separate grain. ---

type pythonMoveModel struct{}

func (pythonMoveModel) Grains() []Grain {
	return []Grain{GrainDeclaration, GrainModule, GrainPackage}
}

func (pythonMoveModel) ModuleKey(filePath string) string {
	return normalizePath(filePath)
}

func (m pythonMoveModel) SameModule(pathA, pathB string) bool {
	return m.ModuleKey(pathA) == m.ModuleKey(pathB)
}

func (pythonMoveModel) ListNodes(result *ingest.Result, grain Grain, projectLanguage string) []MoveNode {
	switch grain {
	case GrainDeclaration:
		return listDeclarationNodes(result, projectLanguage)
	case GrainModule:
		return listModuleFileNodes(result, projectLanguage)
	case GrainPackage:
		return listPackageNodes(result, projectLanguage)
	default:
		return nil
	}
}

// --- JavaScript: file is the module. ---

type javascriptMoveModel struct{}

func (javascriptMoveModel) Grains() []Grain {
	return []Grain{GrainDeclaration, GrainModule}
}

func (javascriptMoveModel) ModuleKey(filePath string) string {
	return normalizePath(filePath)
}

func (m javascriptMoveModel) SameModule(pathA, pathB string) bool {
	return m.ModuleKey(pathA) == m.ModuleKey(pathB)
}

func (javascriptMoveModel) ListNodes(result *ingest.Result, grain Grain, projectLanguage string) []MoveNode {
	switch grain {
	case GrainDeclaration:
		return listDeclarationNodes(result, projectLanguage)
	case GrainModule:
		return listModuleFileNodes(result, projectLanguage)
	default:
		return nil
	}
}

// --- shared listing ---

func listDeclarationNodes(result *ingest.Result, projectLanguage string) []MoveNode {
	var out []MoveNode
	for _, e := range result.Entities {
		ref := ingest.ParseReference(e.Reference)
		if ref.Provider != "path" || ref.Symbol == "" {
			continue
		}
		rel := strings.TrimPrefix(ref.Path, "./")
		lang, ok := ingest.LanguageForFile(rel)
		if !ok || lang != projectLanguage {
			continue
		}
		if isFuzzSymbol(ref.Symbol) {
			continue
		}
		path := ref.Path
		if !strings.HasPrefix(path, "./") {
			path = "./" + path
		}
		out = append(out, MoveNode{
			Grain:     GrainDeclaration,
			Reference: e.Reference,
			Path:      path,
			Symbol:    ref.Symbol,
		})
	}
	return out
}

func listPackageNodes(result *ingest.Result, projectLanguage string) []MoveNode {
	dirs := map[string]bool{}
	for _, f := range result.Files {
		if f.Language != projectLanguage {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		dir := path.Dir(rel)
		if dir == "." || dir == "" {
			continue
		}
		dirs[dir] = true
	}
	keys := make([]string, 0, len(dirs))
	for d := range dirs {
		keys = append(keys, d)
	}
	sort.Strings(keys)
	var out []MoveNode
	for _, d := range keys {
		p := "./" + d
		out = append(out, MoveNode{
			Grain:     GrainPackage,
			Reference: "path:" + p,
			Path:      p,
		})
	}
	return out
}

func listModuleFileNodes(result *ingest.Result, projectLanguage string) []MoveNode {
	var out []MoveNode
	for _, f := range result.Files {
		if f.Language != projectLanguage {
			continue
		}
		p := f.Path
		if !strings.HasPrefix(p, "./") {
			p = "./" + p
		}
		out = append(out, MoveNode{
			Grain:     GrainModule,
			Reference: "path:" + p,
			Path:      p,
		})
	}
	return out
}

func isFuzzSymbol(symbol string) bool {
	return strings.HasPrefix(symbol, "fuzz_") ||
		strings.HasPrefix(symbol, "Fuzz") ||
		strings.HasPrefix(symbol, "FUZZ")
}

func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "./")
	return filepath.ToSlash(p)
}

func dirKey(filePath string) string {
	rel := normalizePath(filePath)
	d := path.Dir(rel)
	if d == "." {
		return ""
	}
	return d
}

func listLanguageFiles(result *ingest.Result, projectLanguage string) []string {
	var files []string
	for _, f := range result.Files {
		if f.Language != projectLanguage {
			continue
		}
		p := f.Path
		if !strings.HasPrefix(p, "./") {
			p = "./" + p
		}
		files = append(files, p)
	}
	sort.Strings(files)
	return files
}

func filesInModule(result *ingest.Result, model languageMoveModel, projectLanguage, moduleKey string) []string {
	var files []string
	for _, f := range listLanguageFiles(result, projectLanguage) {
		if model.ModuleKey(f) == moduleKey {
			files = append(files, f)
		}
	}
	return files
}

func modulesOtherThan(result *ingest.Result, model languageMoveModel, projectLanguage, moduleKey string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, f := range listLanguageFiles(result, projectLanguage) {
		k := model.ModuleKey(f)
		if k == "" || k == moduleKey || seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// peerFileInModule returns a layout file path in the same module, or "".
func peerFileInModule(files []string, sourcePath string, peerIndex uint32) string {
	var peers []string
	for _, f := range files {
		if f != sourcePath {
			peers = append(peers, f)
		}
	}
	if len(peers) == 0 {
		return ""
	}
	return peers[int(peerIndex)%len(peers)]
}

// fileInModuleKey picks a representative file path for an existing module key.
func fileInModuleKey(result *ingest.Result, model languageMoveModel, projectLanguage, moduleKey string, peerIndex uint32) string {
	files := filesInModule(result, model, projectLanguage, moduleKey)
	if len(files) == 0 {
		return ""
	}
	return files[int(peerIndex)%len(files)]
}
