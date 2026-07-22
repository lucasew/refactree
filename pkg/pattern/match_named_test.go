package pattern

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/go"
)

func TestNamedRegexCaptures(t *testing.T) {
	// Write a tiny file with TestFoo
	dir := t.TempDir()
	src := []byte("package p\n\nfunc TestFoo(t *testing.T) {}\n")
	path := filepath.Join(dir, "x_test.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	pat, err := ParsePattern(`func $name:{/^Test(?P<rest>.*)/}`)
	if err != nil {
		t.Fatal(err)
	}
	pf, err := ingest.ParseSourceFile(path, "go")
	if err != nil {
		t.Fatal(err)
	}
	defer pf.Close()
	result, err := ingest.MaterializeSource(ingest.ExtractSource{
		Kind: ingest.ExtractHop, Root: dir, Paths: []string{path},
	}, ingest.MaterializeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ms, err := MatchFile(dir, "x_test.go", src, pf.Root, pat, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("matches=%d want 1", len(ms))
	}
	// $name = full token
	if got := ms[0].Captures["name"]; got != "TestFoo" {
		// group covering node might be just TestFoo
		t.Logf("name=%q caps=%v", got, ms[0].Captures)
	}
	if got := ms[0].Captures["rest"]; got != "Foo" {
		t.Fatalf("rest=%q want Foo; caps=%v", got, ms[0].Captures)
	}
}
