package fuzzy

import (
	"fmt"
	"os/exec"
	"strings"
)

// ImagePresent reports whether the local docker daemon has ref.
func ImagePresent(ref string) bool {
	if ref == "" {
		return false
	}
	cmd := exec.Command("docker", "image", "inspect", ref)
	return cmd.Run() == nil
}

// EnsureImages makes sure each image ref is available locally.
// When pull is true, missing images are docker-pulled (progress streamed live).
// When pull is false (offline), missing images return an error pointing at prefetch.
func EnsureImages(refs []string, pull bool) error {
	seen := map[string]struct{}{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		if ImagePresent(ref) {
			continue
		}
		if !pull {
			return fmt.Errorf("docker image %q not present locally (run prefetch warmup while online)", ref)
		}
		logCmdLine(nil, "docker", "pull", ref)
		cmd := exec.Command("docker", "pull", ref)
		out, err := runStreamingCombined(cmd, nil)
		if err != nil {
			return fmt.Errorf("docker pull %s: %w\n%s", ref, err, out)
		}
		if !ImagePresent(ref) {
			return fmt.Errorf("docker pull %s succeeded but image still missing", ref)
		}
	}
	return nil
}
