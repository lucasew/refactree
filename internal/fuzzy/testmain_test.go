package fuzzy

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates the process work-root for go test when the user did not pin
// RFT_FUZZY_WORK_ROOT. That keeps unit tests off the shared $TMPDIR/rft-fuzzy
// (or CI cache) default set in init.
//
// Catalog canvas / FuzzMvOneOp still work when the env is set to a warm root
// before the test process starts (mise run fuzzy:prefetch / fuzzy:run export it).
func TestMain(m *testing.M) {
	os.Exit(runFuzzyTests(m))
}

func runFuzzyTests(m *testing.M) int {
	var cleanup func()
	if !WorkRootPinnedByEnv() {
		dir, err := os.MkdirTemp("", "rft-fuzzy-gotest-")
		if err != nil {
			fmt.Fprintln(os.Stderr, "fuzzy TestMain:", err)
			return 1
		}
		SetDefaultWorkRoot(dir)
		cleanup = func() { _ = os.RemoveAll(dir) }
	}
	code := m.Run()
	if cleanup != nil {
		cleanup()
	}
	return code
}
