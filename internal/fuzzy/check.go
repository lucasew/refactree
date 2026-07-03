package fuzzy

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// RunSetup executes setup in the shared session (installs tools once per session).
func RunSetup(ctx context.Context, session *Session, p Project) RunResult {
	argv := SetupArgv(p)
	if len(argv) == 0 {
		return RunResult{ExitCode: 0}
	}
	if session == nil {
		return RunResult{Err: fmt.Errorf("nil session"), ExitCode: 1}
	}
	if err := ApplyProjectMise(p, session.abs); err != nil {
		return RunResult{Err: err, ExitCode: 1}
	}
	return session.Run(ctx, argv)
}

// RunCheck executes the check task in the shared session.
func RunCheck(ctx context.Context, session *Session, p Project) RunResult {
	argv := CheckArgv(p)
	if len(argv) == 0 {
		return RunResult{ExitCode: 0}
	}
	if session == nil {
		return RunResult{Err: fmt.Errorf("nil session"), ExitCode: 1}
	}
	if err := ApplyProjectMise(p, session.abs); err != nil {
		return RunResult{Err: err, ExitCode: 1}
	}
	return session.Run(ctx, argv)
}

// formatRunOutput returns accumulated command logs for display.
// Go test dumps are noisy; compiler/test failure lines are lifted to the top.
func formatRunOutput(res RunResult) string {
	combined := res.Stdout
	if res.Stderr != "" {
		if combined != "" && !strings.HasSuffix(combined, "\n") {
			combined += "\n"
		}
		combined += res.Stderr
	}
	highlights := extractFailureHighlights(combined)

	var b strings.Builder
	if highlights != "" {
		b.WriteString("--- failure highlights ---\n")
		b.WriteString(highlights)
		if !strings.HasSuffix(highlights, "\n") {
			b.WriteByte('\n')
		}
	}
	if res.Stdout != "" {
		b.WriteString("--- stdout ---\n")
		b.WriteString(res.Stdout)
		if !strings.HasSuffix(res.Stdout, "\n") {
			b.WriteByte('\n')
		}
	}
	if res.Stderr != "" {
		b.WriteString("--- stderr ---\n")
		b.WriteString(res.Stderr)
		if !strings.HasSuffix(res.Stderr, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func extractFailureHighlights(log string) string {
	if log == "" {
		return ""
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(log, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || seen[trim] {
			continue
		}
		keep := strings.HasPrefix(trim, "# ") ||
			strings.Contains(trim, "undefined:") ||
			strings.Contains(trim, "ERROR task failed") ||
			strings.Contains(trim, "[build failed]") ||
			strings.Contains(trim, "[setup failed]") ||
			strings.HasPrefix(trim, "FAIL\t") ||
			strings.HasPrefix(trim, "--- FAIL:") ||
			strings.Contains(trim, "Error:") ||
			strings.Contains(trim, "panic:") ||
			strings.Contains(trim, "is not in std") ||
			strings.Contains(trim, "imported and not used")
		if !keep {
			continue
		}
		// Skip the huge PASS noise lists.
		if strings.HasPrefix(trim, "ok  ") || strings.HasPrefix(trim, "=== RUN") || strings.HasPrefix(trim, "--- PASS:") {
			continue
		}
		seen[trim] = true
		out = append(out, line)
		if len(out) >= 40 {
			break
		}
	}
	return strings.Join(out, "\n")
}

// formatRunFailure builds a failure message including accumulated logs.
func formatRunFailure(label string, res RunResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s failed (exit %d): %v\n", label, res.ExitCode, res.Err)
	b.WriteString(formatRunOutput(res))
	return b.String()
}

// printRunFailure writes accumulated command logs unless they were already
// streamed live under --verbose. logRel is the report-relative log directory.
func printRunFailure(w io.Writer, verbose bool, label string, res RunResult, logAbs string) {
	if w == nil || res.OK() {
		return
	}
	if verbose {
		fmt.Fprintf(w, "%s failed (exit %d): %v\n", label, res.ExitCode, res.Err)
	} else {
		_, _ = io.WriteString(w, formatRunFailure(label, res))
	}
	if logAbs != "" {
		fmt.Fprintf(w, "full check log: %s\n", filepath.Join(logAbs, "full.log"))
	}
}

// requireOK returns nil on success. On failure it always includes accumulated
// stdout/stderr in the error (shown even when live passthrough was off).
func requireOK(label string, res RunResult) error {
	if res.OK() {
		return nil
	}
	return fmt.Errorf("%s", strings.TrimSuffix(formatRunFailure(label, res), "\n"))
}
