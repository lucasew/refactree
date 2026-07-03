package fuzzy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	projectMiseName = "mise.toml"
	projectMiseBak  = "mise.toml.refactree-upstream"
)

// HasEmbeddedMise reports whether the project supplies a [projects.<slug>.mise] table.
func HasEmbeddedMise(p Project) bool {
	return len(p.Mise) > 0
}

// ResolveMiseTOML marshals [projects.<slug>.mise] into mise.toml contents.
func ResolveMiseTOML(p Project) (string, error) {
	if len(p.Mise) == 0 {
		return "", nil
	}
	data, err := toml.Marshal(p.Mise)
	if err != nil {
		return "", fmt.Errorf("marshal projects.%s.mise: %w", p.ID, err)
	}
	return string(data), nil
}

// ApplyProjectMise writes [projects.<slug>.mise] as mise.toml in the project root.
// An existing mise.toml is moved aside to mise.toml.refactree-upstream once.
func ApplyProjectMise(p Project, projectRoot string) error {
	content, err := ResolveMiseTOML(p)
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(projectRoot, projectMiseName)
	bak := filepath.Join(projectRoot, projectMiseBak)
	if _, err := os.Stat(dst); err == nil {
		if _, err := os.Stat(bak); os.IsNotExist(err) {
			if err := os.Rename(dst, bak); err != nil {
				return fmt.Errorf("backup upstream mise.toml: %w", err)
			}
		} else {
			_ = os.Remove(dst)
		}
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write embedded mise.toml: %w", err)
	}
	for _, name := range []string{"mise.local.toml", ".mise.toml", "mise.lock"} {
		_ = os.Remove(filepath.Join(projectRoot, name))
	}
	return nil
}
