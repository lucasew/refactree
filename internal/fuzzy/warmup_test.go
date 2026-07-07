package fuzzy_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

// TestPrefetchWarmup runs catalog prefetch once into the shared work-root.
// Skipped unless RFT_FUZZY_WARMUP=1 so normal `go test ./...` stays offline-safe.
//
//	RFT_FUZZY_WARMUP=1 go test ./internal/fuzzy -run '^TestPrefetchWarmup$' -count=1 -timeout 0
//
// Optional env:
//
//	RFT_FUZZY_WORK_ROOT   durable work-root (default: $TMPDIR/rft-fuzzy)
//	RFT_FUZZY_NO_ISOLATE=1  host setup/check instead of Docker
//	RFT_FUZZY_PROJECT=slug  comma-separated project filter
func TestPrefetchWarmup(t *testing.T) {
	if !truthyEnv("RFT_FUZZY_WARMUP") {
		t.Skip("set RFT_FUZZY_WARMUP=1 to run catalog prefetch warmup")
	}
	root, err := fuzzy.PrefetchOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil || st.IsDir() {
		t.Fatalf("expected manifest under %s: %v", root, err)
	}
	t.Logf("prefetch warmup ok work-root=%s", root)
}

func truthyEnv(k string) bool {
	switch os.Getenv(k) {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}
