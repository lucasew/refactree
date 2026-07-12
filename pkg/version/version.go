package version

import (
	"runtime/debug"
	"strings"
)

var version = "dev"

// Version returns the refactree version.
// It defaults to "dev" when ldflags injection is not provided.
func Version() string {
	v := strings.TrimSpace(version)
	if v == "" {
		return "dev"
	}
	return v
}

// BuildID returns the build ID from buildinfo
func BuildID() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	// Try to get vcs.revision for commit hash
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			if len(setting.Value) > 8 {
				return setting.Value[:8] // short hash
			}
			return setting.Value
		}
	}

	return "dev"
}

// GetBuildID returns a build identifier combining version and commit hash
func GetBuildID() string {
	return Version() + "-" + BuildID()
}
