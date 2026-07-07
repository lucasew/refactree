package fuzzy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		return nil, fmt.Errorf("offline manifest missing at %s (run: rft fuzzy prefetch): %w", w.manifestPath(), err)
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
		sort.Strings(out.Images)
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
			return fmt.Errorf("offline manifest missing project %q (run: rft fuzzy prefetch --project %s)", p.ID, p.ID)
		}
		if p.LocalPath == "" && p.Ref != "" && mp.Ref != "" && p.Ref != mp.Ref {
			return fmt.Errorf("offline manifest pin mismatch for %s: catalog ref %s vs manifest %s (re-run prefetch)", p.ID, p.Ref, mp.Ref)
		}
		if p.LocalPath == "" {
			cache := ws.cachePath(p.ID)
			if st, err := os.Stat(cache); err != nil || !st.IsDir() {
				return fmt.Errorf("offline git cache missing for %s at %s (run: rft fuzzy prefetch --project %s)", p.ID, cache, p.ID)
			}
			if _, err := revParse(cache, p.Ref); err != nil {
				return fmt.Errorf("offline cache for %s missing ref %s (run: rft fuzzy prefetch --project %s): %w", p.ID, p.Ref, p.ID, err)
			}
		}
		if err := ws.requirePreserveSnapshot(p); err != nil {
			return err
		}
		key := ImageKey(p, mp.Commit)
		misePath := filepath.Join(ws.Root, "mise-data", sanitizeKey(key))
		if !miseDataPresent(misePath) && !miseDataPresent(mp.MiseDataPath) {
			return fmt.Errorf("offline mise-data missing for %s at %s (run: rft fuzzy prefetch --project %s)", p.ID, misePath, p.ID)
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
	sort.Strings(out)
	return out
}
