package fuzzy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteRunResultFullOutput(t *testing.T) {
	dir := t.TempDir()
	rep, err := NewReport(dir, Meta{Seed: 1, Iterations: 1, Mode: "run"})
	if err != nil {
		t.Fatal(err)
	}
	defer rep.Close()
	stdout := strings.Repeat("PASS line\n", 100) + "# pkg\nfile.go:1: undefined: Foo\nFAIL\n"
	stderr := "DEBUG noise\n"
	rel, err := rep.WriteRunResult("proj-check-after-1", RunResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: 1,
		Args:     []string{"mise", "-v", "run", "test"},
		Err:      errStatus(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	full, err := os.ReadFile(filepath.Join(rep.LogPath(rel), "full.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(full), "undefined: Foo") || !strings.Contains(string(full), "DEBUG noise") {
		t.Fatalf("full.log incomplete: %q", full)
	}
	gotOut, _ := os.ReadFile(filepath.Join(rep.LogPath(rel), "stdout.log"))
	if string(gotOut) != stdout {
		t.Fatal("stdout.log mismatch")
	}
	gotErr, _ := os.ReadFile(filepath.Join(rep.LogPath(rel), "stderr.log"))
	if string(gotErr) != stderr {
		t.Fatal("stderr.log mismatch")
	}
	meta, err := os.ReadFile(filepath.Join(rep.LogPath(rel), "meta.json"))
	if err != nil {
		t.Fatalf("meta.json: %v", err)
	}
	if !strings.Contains(string(meta), `"exit_code": 1`) {
		t.Fatalf("meta.json missing exit_code: %q", meta)
	}
}

type errStatus int

func (e errStatus) Error() string { return "exit status 1" }
