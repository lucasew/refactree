package fuzzy

import (
	"fmt"
	"os"
	"strings"
)

const allowEnv = "RFT_FUZZY_ALLOW"

// GuardError is returned when host-side execution is refused.
type GuardError struct {
	Reason string
}

func (e *GuardError) Error() string {
	return fmt.Sprintf("fuzzy harness refused: %s (setup/check run on the host with --no-isolate; use Docker isolation, set %s=1, pass --allow, or run in CI)", e.Reason, allowEnv)
}

// CheckAllowed enforces a host-safety policy only when isolation is disabled.
// With testcontainers (default), untrusted setup/check run inside Docker, so
// an ephemeral host is not required.
func CheckAllowed(allow, noIsolate bool) error {
	if !noIsolate {
		return nil
	}
	if allow {
		return nil
	}
	if truthy(os.Getenv(allowEnv)) {
		return nil
	}
	if truthy(os.Getenv("CI")) {
		return nil
	}
	return &GuardError{Reason: "--no-isolate on a non-ephemeral host"}
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
