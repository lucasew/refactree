package fuzzy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

// Java public type renames relocate Type.java → NewName.java while the plan
// destination still names the pre-rename path. dest_present must accept the
// atom under the new file stem (gson ExclusionStrategy → Fuzz1 seed).
func TestPostMvInvariantsDestPresentFileRelocate(t *testing.T) {
	root := t.TempDir()
	// Post-rename tree: defining file already moved to Fuzz1.java.
	if err := os.WriteFile(filepath.Join(root, "Fuzz1.java"), []byte(`package demo;
public interface Fuzz1 {
  boolean shouldSkip();
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "User.java"), []byte(`package demo;
public class User implements Fuzz1 {
  public boolean shouldSkip() { return false; }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Empty leftover at the plan path (applyRenameFileMoves truncates old file).
	if err := os.WriteFile(filepath.Join(root, "ExclusionStrategy.java"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := movePlan{
		Placement:   PlacementRename,
		Source:      "path:./ExclusionStrategy.java::ExclusionStrategy",
		Destination: "path:./ExclusionStrategy.java::Fuzz1",
	}
	fails := postMvInvariants(root, plan, false)
	for _, f := range fails {
		if f.Check == "dest_present" {
			t.Fatalf("dest_present after file relocate: %v (fails=%v)", f, fails)
		}
	}

	// Sanity: atom really lives under the new path.
	res, err := ingest.ProjectResult(root)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, a := range res.Atoms {
		if a.Reference == "path:./Fuzz1.java::Fuzz1" || strings.HasSuffix(a.Reference, "Fuzz1.java::Fuzz1") {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("expected Fuzz1 atom in ingest; got %v", res.Atoms)
	}
}

func TestPostMvInvariantsDestPresentSameFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Types.java"), []byte(`package demo;
class Renamed { int value; }
class Other { Renamed h = new Renamed(); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := movePlan{
		Placement:   PlacementRename,
		Source:      "path:./Types.java::Helper",
		Destination: "path:./Types.java::Renamed",
	}
	fails := postMvInvariants(root, plan, false)
	for _, f := range fails {
		if f.Check == "dest_present" {
			t.Fatalf("same-file rename dest_present: %v", f)
		}
	}
}

func TestFormatFuzzNamePreservesExportCase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		hint string
		want string
	}{
		// Exported Go/Java-style (catalog Token → must stay exported)
		{"Token", "Fuzza"},
		{"Helper", "Fuzza"},
		{"HTTPClient", "Fuzza"},
		// SCREAMING_SNAKE
		{"CI_INSTRUCTIONS", "FUZZ_A"},
		{"ERR_NOT_FOUND", "FUZZ_A"},
		// ALLCAPS
		{"ID", "FUZZA"},
		// unexported camelCase → fuzz + hex
		{"token", "fuzza"},
		{"helper", "fuzza"},
		// snake_case
		{"err_not_found", "fuzz_a"},
		{"", "fuzz_a"},
	}
	const n = 10 // hex "a"
	for _, tc := range cases {
		got := formatFuzzName(tc.hint, n)
		if got != tc.want {
			t.Errorf("formatFuzzName(%q, %d)=%q want %q", tc.hint, n, got, tc.want)
		}
	}
}

func TestUniqueSymbolFromExportedStaysExported(t *testing.T) {
	t.Parallel()
	existing := map[string]bool{}
	name := uniqueSymbolFrom(0xf7017, existing, "Token")
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		t.Fatalf("uniqueSymbolFrom for Token must be exported-style, got %q", name)
	}
	if strings.HasPrefix(name, "fuzz") {
		t.Fatalf("must not produce unexported fuzz* for Token, got %q", name)
	}
}
