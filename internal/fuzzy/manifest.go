package fuzzy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const manifestFileName = "manifest.json"

// Manifest records what prefetch populated so offline runs can fail fast.
type Manifest struct {
	Version   string            `json:"version"`
	CreatedAt time.Time         `json:"created_at"`
	WorkRoot  string            `json:"work_root"`
	Isolation string            `json:"isolation"` // "docker" or "host"
	Images    []string          `json:"images,omitempty"`
	Projects  []ManifestProject `json:"projects"`
}

// ManifestProject is one catalog entry as prefetched.
type ManifestProject struct {
	ID              string   `json:"id"`
	Ref             string   `json:"ref"`
	Commit          string   `json:"commit"`
	Image           string   `json:"image"`
	ImageKey        string   `json:"image_key"`
	PreserveGlobs   []string `json:"preserve_globs,omitempty"`
	PreserveOK      bool     `json:"preserve_ok"`
	MiseDataPath    string   `json:"mise_data_path"`
	MiseDataPresent bool     `json:"mise_data_present"`
	SetupTask       string   `json:"setup_task,omitempty"`
	CheckTask       string   `json:"check_task,omitempty"`
}

func (w *Workspace) manifestPath() string {
	return filepath.Join(w.Root, manifestFileName)
}

// SaveManifest writes manifest.json under work-root.
func (w *Workspace) SaveManifest(m Manifest) error {
	m.WorkRoot = w.Root
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(w.manifestPath(), data, 0o644)
}

// LoadManifest reads work-root/manifest.json.
func (w *Workspace) LoadManifest() (*Manifest, error) {
	data, err := os.ReadFile(w.manifestPath())
	if err != nil {
		return nil, fmt.Errorf("offline manifest missing at %s (run prefetch warmup): %w", w.manifestPath(), err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", w.manifestPath(), err)
	}
	return &m, nil
}

// BuildManifest constructs a manifest from the current work-root state for projects.
func (w *Workspace) BuildManifest(projects []Project, noIsolate bool, commits map[string]string) Manifest {
	isolation := "docker"
	if noIsolate {
		isolation = "host"
	}
	imageSet := map[string]struct{}{}
	if !noIsolate {
		imageSet[CleanupImage] = struct{}{}
	}
	out := Manifest{
		Version:   "1",
		CreatedAt: time.Now().UTC(),
		WorkRoot:  w.Root,
		Isolation: isolation,
		Projects:  make([]ManifestProject, 0, len(projects)),
	}
	for _, p := range projects {
		commit := commits[p.ID]
		if commit == "" {
			commit = p.Ref
		}
		key := ImageKey(p, commit)
		misePath := filepath.Join(w.Root, "mise-data", sanitizeKey(key))
		mp := ManifestProject{
			ID:              p.ID,
			Ref:             p.Ref,
			Commit:          commit,
			Image:           p.Isolate.ImageOrDefault(),
			ImageKey:        key,
			PreserveGlobs:   append([]string(nil), p.PreserveGlobs...),
			PreserveOK:      len(p.PreserveGlobs) == 0 || w.HasPreserveSnapshot(p),
			MiseDataPath:    misePath,
			MiseDataPresent: miseDataPresent(misePath),
			SetupTask:       p.SetupTask,
			CheckTask:       p.CheckTask,
		}
		out.Projects = append(out.Projects, mp)
		if !noIsolate {
			imageSet[p.Isolate.ImageOrDefault()] = struct{}{}
		}
	}
	if len(imageSet) > 0 {
		out.Images = make([]string, 0, len(imageSet))
		for img := range imageSet {
			out.Images = append(out.Images, img)
		}
		slices.Sort(out.Images)
	}
	return out
}

func miseDataPresent(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

// ValidateOfflineReady checks work-root + optional local Docker images for offline use.
func ValidateOfflineReady(ws *Workspace, projects []Project, noIsolate bool) error {
	m, err := ws.LoadManifest()
	if err != nil {
		return err
	}
	byID := map[string]ManifestProject{}
	for _, mp := range m.Projects {
		byID[mp.ID] = mp
	}

	var imageNeed []string
	imageSeen := map[string]struct{}{}
	for _, p := range projects {
		mp, ok := byID[p.ID]
		if !ok {
			return fmt.Errorf("offline manifest missing project %q (run prefetch warmup)", p.ID)
		}
		if p.LocalPath == "" && p.Ref != "" && mp.Ref != "" && p.Ref != mp.Ref {
			return fmt.Errorf("offline manifest pin mismatch for %s: catalog ref %s vs manifest %s (re-run prefetch)", p.ID, p.Ref, mp.Ref)
		}
		commit, err := projectGitReady(ws, p)
		if err != nil {
			return err
		}
		if err := ws.requirePreserveSnapshot(p); err != nil {
			return err
		}
		if err := projectMiseReady(ws, p, commit, mp); err != nil {
			return err
		}
		if !noIsolate {
			img := p.Isolate.ImageOrDefault()
			if _, ok := imageSeen[img]; !ok {
				imageSeen[img] = struct{}{}
				imageNeed = append(imageNeed, img)
			}
		}
	}
	if !noIsolate {
		if _, ok := imageSeen[CleanupImage]; !ok {
			imageNeed = append(imageNeed, CleanupImage)
		}
		if err := EnsureImages(imageNeed, false); err != nil {
			return err
		}
	}
	return nil
}

// projectPrefetchReady reports whether a single project already has durable
// caches for offline use (git pin, preserve, mise-data, and local image when isolating).
// commit is the resolved pin when ok.
func projectPrefetchReady(ws *Workspace, p Project, noIsolate bool) (commit string, ok bool) {
	commit, err := projectGitReady(ws, p)
	if err != nil {
		return "", false
	}
	if err := ws.requirePreserveSnapshot(p); err != nil {
		return "", false
	}
	if err := projectMiseReady(ws, p, commit, ManifestProject{}); err != nil {
		return "", false
	}
	if !noIsolate {
		if !ImagePresent(p.Isolate.ImageOrDefault()) || !ImagePresent(CleanupImage) {
			return "", false
		}
	}
	return commit, true
}

func projectGitReady(ws *Workspace, p Project) (commit string, err error) {
	if p.LocalPath != "" {
		return "local", nil
	}
	cache := ws.cachePath(p.ID)
	if st, err := os.Stat(cache); err != nil || !st.IsDir() {
		return "", fmt.Errorf("git cache missing for %s at %s", p.ID, cache)
	}
	commit, err = revParse(cache, p.Ref)
	if err != nil {
		return "", fmt.Errorf("git cache for %s missing ref %s: %w", p.ID, p.Ref, err)
	}
	return commit, nil
}

func projectMiseReady(ws *Workspace, p Project, commit string, mp ManifestProject) error {
	key := ImageKey(p, commit)
	misePath := filepath.Join(ws.Root, "mise-data", sanitizeKey(key))
	if miseDataPresent(misePath) {
		return nil
	}
	if mp.MiseDataPath != "" && miseDataPresent(mp.MiseDataPath) {
		return nil
	}
	// Fallback: any key for this project id under mise-data (older layouts).
	entries, err := os.ReadDir(filepath.Join(ws.Root, "mise-data"))
	if err == nil {
		prefix := sanitizeKey(p.ID) + "-"
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), prefix) && miseDataPresent(filepath.Join(ws.Root, "mise-data", e.Name())) {
				return nil
			}
		}
	}
	return fmt.Errorf("mise-data missing for %s at %s", p.ID, misePath)
}

// RequiredImages returns the unique docker image refs needed for projects.
func RequiredImages(projects []Project) []string {
	seen := map[string]struct{}{CleanupImage: {}}
	for _, p := range projects {
		seen[p.Isolate.ImageOrDefault()] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for img := range seen {
		out = append(out, img)
	}
	slices.Sort(out)
	return out
}
