package fuzzy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultWorkRoot is the single on-disk root for the fuzzy harness:
// cache/, preserve/, runs/, mise-data/, reports/.
// It is RFT_FUZZY_WORK_ROOT when set, otherwise $TMPDIR/rft-fuzzy.
func DefaultWorkRoot() string {
	if root := os.Getenv("RFT_FUZZY_WORK_ROOT"); root != "" {
		return root
	}
	return filepath.Join(os.TempDir(), "rft-fuzzy")
}

var prefetchMu sync.Mutex

// PrefetchOnce ensures DefaultWorkRoot has everything needed for offline catalog
// runs, then returns that path.
//
// Behaviour:
//   - If the work-root is already warm for the selected projects (manifest, git
//     pins, preserve snapshots, mise-data, local docker images when isolating),
//     this is a no-op and returns immediately.
//   - Otherwise it runs ModePrefetch, which skips individual projects that are
//     already warm and only downloads/setup what is missing.
//
// Safe for concurrent callers (mutex). Unlike sync.Once, a failed attempt does
// not permanently block later retries in the same process.
//
// Env:
//
//	RFT_FUZZY_WORK_ROOT     durable work-root
//	RFT_FUZZY_NO_ISOLATE=1  host setup/check (no Docker)
//	RFT_FUZZY_PROJECT       comma-separated project slugs (default: all catalog)
func PrefetchOnce(ctx context.Context) (workRoot string, err error) {
	return Prefetch(ctx, PrefetchOptions{
		WorkRoot:   DefaultWorkRoot(),
		NoIsolate:  truthy(os.Getenv("RFT_FUZZY_NO_ISOLATE")),
		ProjectIDs: splitCommaIDs(os.Getenv("RFT_FUZZY_PROJECT")),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	})
}

// PrefetchOptions configures Prefetch / PrefetchOnce.
type PrefetchOptions struct {
	WorkRoot    string
	CatalogPath string
	ProjectIDs  []string
	NoIsolate   bool
	Stdout      io.Writer
	Stderr      io.Writer
}

// Prefetch fills gaps in work-root or no-ops when already warm. Concurrent calls
// for the same process are serialized on a process-wide lock (one warm at a time).
func Prefetch(ctx context.Context, opts PrefetchOptions) (workRoot string, err error) {
	prefetchMu.Lock()
	defer prefetchMu.Unlock()

	if opts.WorkRoot == "" {
		opts.WorkRoot = DefaultWorkRoot()
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	catalogPath := opts.CatalogPath
	if catalogPath == "" {
		catalogPath = DefaultCatalogPath()
	}
	projects, err := LoadCatalog(catalogPath)
	if err != nil {
		return opts.WorkRoot, err
	}
	projects, err = FilterProjects(projects, opts.ProjectIDs)
	if err != nil {
		return opts.WorkRoot, err
	}
	if len(projects) == 0 {
		return opts.WorkRoot, fmt.Errorf("no projects to prefetch")
	}

	ws, err := NewWorkspace(opts.WorkRoot)
	if err != nil {
		return opts.WorkRoot, err
	}

	if err := ValidateOfflineReady(ws, projects, opts.NoIsolate); err == nil {
		fmt.Fprintf(opts.Stdout, "prefetch: no-op (work-root warm) %s\n", ws.Root)
		return ws.Root, nil
	} else {
		fmt.Fprintf(opts.Stdout, "prefetch: filling gaps (%v)\n", err)
	}

	_, err = Run(ctx, Options{
		CatalogPath: catalogPath,
		ProjectIDs:  opts.ProjectIDs,
		Mode:        ModePrefetch,
		WorkRoot:    ws.Root,
		Allow:       true,
		NoIsolate:   opts.NoIsolate,
		Stdout:      opts.Stdout,
		Stderr:      opts.Stderr,
	})
	if err != nil {
		return ws.Root, fmt.Errorf("prefetch (work-root %s): %w", ws.Root, err)
	}
	// Confirm offline-ready after fill.
	if err := ValidateOfflineReady(ws, projects, opts.NoIsolate); err != nil {
		return ws.Root, fmt.Errorf("prefetch finished but work-root still not offline-ready: %w", err)
	}
	fmt.Fprintf(opts.Stdout, "prefetch: ready %s\n", ws.Root)
	return ws.Root, nil
}

// SharedWorkRoot is an alias for DefaultWorkRoot (stable path used by Prefetch).
func SharedWorkRoot() string {
	return DefaultWorkRoot()
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
