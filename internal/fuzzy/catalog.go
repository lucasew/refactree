package fuzzy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/pelletier/go-toml/v2"
)

// MvConfig controls which move grains (and optional placements) the harness may attempt.
type MvConfig struct {
	Enabled    bool     `toml:"enabled"`
	Grains     []string `toml:"grains"`
	Placements []string `toml:"placements,omitempty"`
	// Ops is rejected if present (legacy key removed in grain cutover).
	Ops []string `toml:"ops,omitempty"`
}

// DefaultMiseImage is the pinned testcontainers image (digest, not a floating tag).
const DefaultMiseImage = "jdxcode/mise@sha256:d536ef04425b3321dcbd60f6e7687994d5a1b8859574f0f9c2529dba620b74cb"

// CleanupImage is used for privileged host cleanup (ForceRemoveAll). Same pin as
// the session image so prefetch only needs one docker pull.
const CleanupImage = DefaultMiseImage

var (
	majorOnlyVersion  = regexp.MustCompile(`^[0-9]+$`)
	majorMinorVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
	floatingImageTag  = regexp.MustCompile(`:latest(?:@|$)`)
)

// IsolateConfig controls docker/testcontainers execution for setup/check.
type IsolateConfig struct {
	Image        string   `toml:"image"`         // default DefaultMiseImage
	SetupNetwork *bool    `toml:"setup_network"` // default true
	CheckNetwork *bool    `toml:"check_network"` // default false
	Env          []string `toml:"env"`           // KEY=VAL passed into the container
}

// ImageOrDefault returns the mise container image.
func (c IsolateConfig) ImageOrDefault() string {
	if c.Image != "" {
		return c.Image
	}
	return DefaultMiseImage
}

// SetupNetworkEnabled reports whether setup has network access (default true).
func (c IsolateConfig) SetupNetworkEnabled() bool {
	if c.SetupNetwork == nil {
		return true
	}
	return *c.SetupNetwork
}

// CheckNetworkEnabled reports whether checks have network access (default false).
func (c IsolateConfig) CheckNetworkEnabled() bool {
	if c.CheckNetwork == nil {
		return false
	}
	return *c.CheckNetwork
}

// Project is one real-world target from the fuzzy catalog.
type Project struct {
	ID            string         `toml:"-"` // map key under [projects.<slug>]
	URL           string         `toml:"url"`
	Ref           string         `toml:"ref"`
	Family        string         `toml:"family"` // ingest family id (ecma, jvm, go, …)
	Language      string         `toml:"language,omitempty"` // rejected if set (use family)
	Root          string         `toml:"root"`
	Mise          map[string]any `toml:"mise"`       // embedded mise.toml root ([projects.<slug>.mise]…)
	SetupTask     string         `toml:"setup_task"` // default "setup" when mise embedded; "-" skips
	CheckTask     string         `toml:"check_task"` // default "test" when mise embedded
	Setup         []string       `toml:"setup"`      // legacy argv fallback without embedded mise
	Check         []string       `toml:"check"`      // legacy argv fallback without embedded mise
	IngestRoots   []string       `toml:"ingest_roots"`
	Mv            MvConfig       `toml:"mv"`
	Isolate       IsolateConfig  `toml:"isolate"`
	PreserveGlobs []string       `toml:"preserve_globs"`
	SkipIf        string         `toml:"skip_if,omitempty"`
	LocalPath     string         `toml:"local_path,omitempty"` // test-only: skip clone, use path
}

type catalogFile struct {
	Projects map[string]Project `toml:"projects"`
}

// LoadCatalog reads and validates projects.toml.
// Projects are keyed by slug: [projects.<slug>].
func LoadCatalog(path string) ([]Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file catalogFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse catalog %s: %w", path, err)
	}
	if len(file.Projects) == 0 {
		return nil, fmt.Errorf("parse catalog %s: no [projects.<slug>] entries", path)
	}
	slugs := make([]string, 0, len(file.Projects))
	for slug := range file.Projects {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	out := make([]Project, 0, len(slugs))
	for _, slug := range slugs {
		p := file.Projects[slug]
		p.ID = slug
		if err := validateProject(&p); err != nil {
			return nil, fmt.Errorf("project %q: %w", slug, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// DefaultCatalogPath resolves testdata/fuzzy/projects.toml from cwd or module root.
func DefaultCatalogPath() string {
	candidates := []string{
		filepath.Join("testdata", "fuzzy", "projects.toml"),
		filepath.Join("..", "..", "testdata", "fuzzy", "projects.toml"),
		filepath.Join("..", "..", "..", "testdata", "fuzzy", "projects.toml"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return candidates[0]
}

func validateProject(p *Project) error {
	if p.ID == "" {
		return fmt.Errorf("slug is required")
	}
	if err := validateSlug(p.ID); err != nil {
		return err
	}
	if p.LocalPath == "" {
		if p.URL == "" {
			return fmt.Errorf("url is required")
		}
		if p.Ref == "" {
			return fmt.Errorf("ref is required")
		}
	}
	if p.Language != "" {
		return fmt.Errorf("project.language removed; use project.family")
	}
	if p.Family == "" {
		return fmt.Errorf("family is required")
	}
	if !ingest.IsKnownFamily(p.Family) {
		return fmt.Errorf("unknown family %q (register surfaces with LanguageRules.Family)", p.Family)
	}
	if p.Root == "" {
		p.Root = "."
	}
	if len(p.IngestRoots) == 0 {
		p.IngestRoots = []string{p.Root}
	}
	if HasEmbeddedMise(*p) {
		if p.CheckTask == "" && len(p.Check) == 0 {
			p.CheckTask = "test"
		}
		if p.SetupTask == "" && len(p.Setup) == 0 {
			p.SetupTask = "setup"
		}
		if err := validateMiseTools(p.Mise); err != nil {
			return err
		}
	}
	if len(p.Check) == 0 && p.CheckTask == "" && p.LocalPath == "" {
		return fmt.Errorf("check_task, check, or [projects.%s.mise] with default test task is required", p.ID)
	}
	if len(p.Mv.Ops) > 0 {
		return fmt.Errorf("mv.ops removed; use mv.grains (and optional mv.placements)")
	}
	if p.Mv.Enabled && len(p.Mv.Grains) == 0 {
		return fmt.Errorf("mv.enabled requires at least one grain")
	}
	for _, grain := range p.Mv.Grains {
		switch Grain(grain) {
		case GrainDeclaration, GrainModule, GrainPackage:
		default:
			return fmt.Errorf("unknown mv grain %q", grain)
		}
		if !grainAllowedForFamily(p.Family, Grain(grain)) {
			return fmt.Errorf("grain %q not valid for family %s", grain, p.Family)
		}
	}
	for _, placement := range p.Mv.Placements {
		switch Placement(placement) {
		case PlacementRename, PlacementLayout, PlacementModule, PlacementNewModule, PlacementPackage:
		default:
			return fmt.Errorf("unknown mv placement %q", placement)
		}
	}
	if err := validateIsolateImage(p.Isolate.Image); err != nil {
		return err
	}
	p.Isolate.SetupNetwork = boolPtr(p.Isolate.SetupNetworkEnabled())
	p.Isolate.CheckNetwork = boolPtr(p.Isolate.CheckNetworkEnabled())
	return nil
}

func validateMiseTools(mise map[string]any) error {
	raw, ok := mise["tools"]
	if !ok || raw == nil {
		return nil
	}
	tools, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("[projects.<slug>.mise.tools] must be a table")
	}
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ver, err := toolVersionString(tools[name])
		if err != nil {
			return fmt.Errorf("mise tool %q: %w", name, err)
		}
		if err := checkPinnedToolVersion(name, ver); err != nil {
			return err
		}
	}
	return nil
}

func toolVersionString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t), nil
	case int64, float64:
		return "", fmt.Errorf("version must be a quoted string (got %v)", v)
	default:
		return "", fmt.Errorf("version must be a quoted string (got %T)", v)
	}
}

// checkPinnedToolVersion rejects floating mise versions like "latest", "22", or "3.9".
func checkPinnedToolVersion(name, ver string) error {
	if ver == "" {
		return fmt.Errorf("mise tool %q: empty version", name)
	}
	lower := strings.ToLower(ver)
	if lower == "latest" || strings.HasPrefix(lower, "latest-") || strings.HasSuffix(lower, "-latest") {
		return fmt.Errorf("mise tool %q: pin an exact version (got %q)", name, ver)
	}
	if majorOnlyVersion.MatchString(ver) || majorMinorVersion.MatchString(ver) {
		return fmt.Errorf("mise tool %q: pin a full version (got %q)", name, ver)
	}
	return nil
}

func validateIsolateImage(image string) error {
	if image == "" {
		return nil
	}
	if floatingImageTag.MatchString(image) || strings.HasSuffix(image, ":latest") {
		return fmt.Errorf("isolate.image must be pinned (got %q)", image)
	}
	return nil
}

func validateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("empty slug")
	}
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return fmt.Errorf("invalid slug %q (use [a-zA-Z0-9_-])", slug)
		}
	}
	return nil
}

func boolPtr(v bool) *bool { return &v }

// miseVerboseArgv prefixes mise invocations with -v.
func miseVerboseArgv(args ...string) []string {
	out := make([]string, 0, len(args)+2)
	out = append(out, "mise", "-v")
	return append(out, args...)
}

// withVerbose returns argv with verbose flags for known tools.
func withVerbose(argv []string) []string {
	if len(argv) == 0 {
		return nil
	}
	out := append([]string(nil), argv...)
	switch out[0] {
	case "mise":
		if len(out) == 1 || out[1] != "-v" {
			out = append([]string{"mise", "-v"}, out[1:]...)
		}
	case "go":
		if len(out) >= 2 && out[1] == "test" && !containsArg(out, "-v") && !containsArg(out, "-verbose") {
			out = append([]string{"go", "test", "-v"}, out[2:]...)
		}
	}
	return out
}

func containsArg(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}

// SetupArgv returns the command run for project setup.
// Empty means setup is skipped.
func SetupArgv(p Project) []string {
	if p.SetupTask != "" && p.SetupTask != "-" {
		return miseVerboseArgv("run", p.SetupTask)
	}
	if HasEmbeddedMise(p) {
		return nil
	}
	return withVerbose(p.Setup)
}

// CheckArgv returns the command run for project checks.
func CheckArgv(p Project) []string {
	if p.CheckTask != "" && p.CheckTask != "-" {
		return miseVerboseArgv("run", p.CheckTask)
	}
	if HasEmbeddedMise(p) {
		return miseVerboseArgv("run", "test")
	}
	return withVerbose(p.Check)
}

// FilterProjects returns projects matching ids (empty ids = all).
func FilterProjects(projects []Project, ids []string) ([]Project, error) {
	if len(ids) == 0 {
		return projects, nil
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[strings.TrimSpace(id)] = true
	}
	var out []Project
	for _, p := range projects {
		if want[p.ID] {
			out = append(out, p)
			delete(want, p.ID)
		}
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for id := range want {
			missing = append(missing, id)
		}
		return nil, fmt.Errorf("unknown project id(s): %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// ProjectRoot joins workspace dir with the project's root subdir.
func ProjectRoot(workDir string, p Project) string {
	if p.Root == "" || p.Root == "." {
		return workDir
	}
	return filepath.Join(workDir, filepath.FromSlash(p.Root))
}

// ResolveIngestRoot joins workspace with one ingest_roots entry.
func ResolveIngestRoot(workDir string, rel string) string {
	if rel == "" || rel == "." {
		return workDir
	}
	return filepath.Join(workDir, filepath.FromSlash(rel))
}
