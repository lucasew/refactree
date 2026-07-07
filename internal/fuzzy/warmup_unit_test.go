package fuzzy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultWorkRootEnv(t *testing.T) {
	t.Setenv("RFT_FUZZY_WORK_ROOT", "/var/cache/rft-fuzzy-test")
	if got := DefaultWorkRoot(); got != "/var/cache/rft-fuzzy-test" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("RFT_FUZZY_WORK_ROOT", "")
	got := DefaultWorkRoot()
	want := filepath.Join(os.TempDir(), "rft-fuzzy")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSplitCommaIDs(t *testing.T) {
	got := splitCommaIDs(" a, b ,c,,")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("%v", got)
	}
}
