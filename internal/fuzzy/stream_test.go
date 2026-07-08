package fuzzy

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func TestMuteProcessLogsStopsStdoutTee(t *testing.T) {
	restore := MuteProcessLogs()
	t.Cleanup(restore)

	var capture bytes.Buffer
	// Even if we pass a capture writer, muted mode must not also require real stdout.
	cmd := exec.Command("sh", "-c", "echo hello-fuzz-mute")
	out, err := runStreamingCombined(cmd, &capture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello-fuzz-mute") {
		t.Fatalf("captured=%q", out)
	}
	if !strings.Contains(capture.String(), "hello-fuzz-mute") {
		t.Fatalf("writer=%q", capture.String())
	}

	// Discard path must not panic or write to process stdout.
	cmd2 := exec.Command("sh", "-c", "echo ignored")
	if _, err := runStreamingCombined(cmd2, io.Discard); err != nil {
		t.Fatal(err)
	}
}
