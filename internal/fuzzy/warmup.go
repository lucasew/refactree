package fuzzy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultWorkRoot returns RFT_FUZZY_WORK_ROOT, or <tmpdir>/rft-fuzzy when unset.
func DefaultWorkRoot() string {
	if root := os.Getenv("RFT_FUZZY_WORK_ROOT"); root != "" {
		return root
	}
	return filepath.Join(os.TempDir(), "rft-fuzzy")
}

var (
	prefetchOnce sync.Once
	prefetchRoot string
	prefetchErr  error
)

// PrefetchOnce runs catalog ModePrefetch exactly once per process into
// DefaultWorkRoot (override with RFT_FUZZY_WORK_ROOT). Concurrent callers share
// the same result. Use this to warm git/mise/preserve/docker caches before
// offline catalog tests in the same process, or via TestPrefetchWarmup alone.
//
// Isolation defaults to Docker. Set RFT_FUZZY_NO_ISOLATE=1 for host setup/check
// (implies allow). Extra project filters: RFT_FUZZY_PROJECT (comma-separated slugs).
func PrefetchOnce(ctx context.Context) (workRoot string, err error) {
	prefetchOnce.Do(func() {
		prefetchRoot = DefaultWorkRoot()
		noIsolate := truthy(os.Getenv("RFT_FUZZY_NO_ISOLATE"))
		opts := Options{
			Mode:      ModePrefetch,
			WorkRoot:  prefetchRoot,
			Allow:     true,
			NoIsolate: noIsolate,
			Stdout:    os.Stdout,
			Stderr:    os.Stderr,
		}
		if raw := os.Getenv("RFT_FUZZY_PROJECT"); raw != "" {
			opts.ProjectIDs = splitCommaIDs(raw)
		}
		_, prefetchErr = Run(ctx, opts)
		if prefetchErr != nil {
			prefetchErr = fmt.Errorf("prefetch once (work-root %s): %w", prefetchRoot, prefetchErr)
		}
	})
	return prefetchRoot, prefetchErr
}

// SharedWorkRoot returns the work-root used by PrefetchOnce after it has run.
// Empty if PrefetchOnce has not completed successfully yet (or never called).
func SharedWorkRoot() string {
	return prefetchRoot
}

func splitCommaIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
