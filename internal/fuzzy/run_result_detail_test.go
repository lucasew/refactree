package fuzzy

import (
	"strings"
	"testing"
)

func TestFormatRunFailureDetailIncludesTail(t *testing.T) {
	res := RunResult{ExitCode: 1, Err: errExit(1), Stderr: strings.Repeat("x", 100) + "COMPILE_FAIL_HERE\n"}
	got := formatRunFailureDetail(res, 50)
	if !strings.Contains(got, "COMPILE_FAIL_HERE") {
		t.Fatalf("missing tail content: %q", got)
	}
	if !strings.Contains(got, "command output") {
		t.Fatalf("missing section header: %q", got)
	}
}

type errExit int

func (e errExit) Error() string { return "exit status 1" }
