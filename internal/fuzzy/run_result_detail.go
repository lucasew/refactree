package fuzzy

import (
	"fmt"
	"strings"
)

// formatRunFailureDetail returns exit info plus a tail of stdout/stderr so
// campaign/CI failures are actionable without opening full report logs.
func formatRunFailureDetail(res RunResult, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	var b strings.Builder
	b.WriteString(shortRunErr(res))
	out := strings.TrimSpace(res.Stdout)
	err := strings.TrimSpace(res.Stderr)
	if out == "" && err == "" {
		return b.String()
	}
	// Prefer stderr for tool failures; fall back to combined.
	body := err
	if body == "" {
		body = out
	} else if out != "" {
		body = err + "\n" + out
	}
	body = tailBytes(body, maxBytes)
	b.WriteString("\n--- command output (tail) ---\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteByte('\n')
	}
	return b.String()
}

func tailBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return fmt.Sprintf("…(%d bytes omitted)…\n%s", len(s)-max, s[len(s)-max:])
}
