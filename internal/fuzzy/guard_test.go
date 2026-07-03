package fuzzy_test

import (
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestCheckAllowedIsolatedSkipsGuard(t *testing.T) {
	t.Setenv("RFT_FUZZY_ALLOW", "")
	t.Setenv("CI", "")
	if err := fuzzy.CheckAllowed(false, false); err != nil {
		t.Fatalf("docker isolation should not require ephemeral host: %v", err)
	}
}

func TestCheckAllowedNoIsolateRequiresAllow(t *testing.T) {
	t.Setenv("RFT_FUZZY_ALLOW", "")
	t.Setenv("CI", "")
	if err := fuzzy.CheckAllowed(false, true); err == nil {
		t.Fatal("expected refusal for --no-isolate without allow")
	}
	if err := fuzzy.CheckAllowed(true, true); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RFT_FUZZY_ALLOW", "1")
	if err := fuzzy.CheckAllowed(false, true); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RFT_FUZZY_ALLOW", "")
	t.Setenv("CI", "true")
	if err := fuzzy.CheckAllowed(false, true); err != nil {
		t.Fatal(err)
	}
}
