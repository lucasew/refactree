package edit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathLineColumnEditor_NilFileFieldsUseProcessStdio(t *testing.T) {
	// Zero-value PathLineColumnEditor has nil *os.File fields; assigning those
	// into cmd.Std* as interfaces must not produce typed nils.
	dir := t.TempDir()
	script := filepath.Join(dir, "ed.sh")
	out := filepath.Join(dir, "args.txt")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + out + "\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ed := PathLineColumnEditor{Bin: script} // Stdin/Stdout/Stderr intentionally unset
	if err := ed.Open(Location{Path: "/tmp/x.go", Line: 2, Column: 3}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "/tmp/x.go:2:3\n" {
		t.Fatalf("got %q", got)
	}
}

func TestPathLineColumnEditor_EchoExitZero(t *testing.T) {
	ed := PathLineColumnEditor{Bin: "/run/current-system/sw/bin/echo"}
	if err := ed.Open(Location{Path: "/tmp/x.go", Line: 1, Column: 1}); err != nil {
		t.Fatalf("echo editor should exit 0, got %v", err)
	}
}
