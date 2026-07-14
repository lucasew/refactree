package fuzzy_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lucasew/refactree/internal/fuzzy"
)

// TestPrefetchWarmup ensures the shared work-root is warm (no-op when already
// complete; otherwise downloads/setup only what is missing). Skipped unless
// RFT_FUZZY_WARMUP=1 so normal `go test ./...` stays offline-safe.
//
//	RFT_FUZZY_WARMUP=1 go test ./internal/fuzzy -run '^TestPrefetchWarmup$' -count=1 -timeout 0 -v
//
// Prefetch uses t.Context() so host mise/uv children are cancelled when the test
// ends or the go test timeout fires (CommandContext on the harness path).
//
// Optional: RFT_FUZZY_WORK_ROOT, RFT_FUZZY_NO_ISOLATE=1, RFT_FUZZY_PROJECT=slug,...
//
// After this, catalog tests should use WorkRoot: fuzzy.DefaultWorkRoot() and Offline: true.
func TestPrefetchWarmup(t *testing.T) {
	if !truthyEnv("RFT_FUZZY_WARMUP") {
		t.Skip("set RFT_FUZZY_WARMUP=1 to run catalog prefetch warmup")
	}
	ctx := t.Context()
	start := time.Now()
	root, err := fuzzy.PrefetchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Since(start)
	if st, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil || st.IsDir() {
		t.Fatalf("expected manifest under %s: %v", root, err)
	}

	// Second call must be a cheap no-op (warm check only).
	start = time.Now()
	root2, err := fuzzy.PrefetchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second := time.Since(start)
	if root2 != root {
		t.Fatalf("work-root changed %q -> %q", root, root2)
	}
	t.Logf("prefetch warmup ok work-root=%s first=%s second(no-op)=%s", root, first, second)
}

func truthyEnv(k string) bool {
	switch os.Getenv(k) {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}
