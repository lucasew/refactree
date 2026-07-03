package fuzzy

import "testing"

func TestExtractFailureHighlights(t *testing.T) {
	log := "ok  pkg/text (cached)\n=== RUN TestFoo\n--- PASS: TestFoo\n# workspaced/pkg/taskgroup\npkg/taskgroup/taskgroup.go:480:20: undefined: logging.FormatPrepend\nFAIL\tworkspaced/pkg/taskgroup [build failed]\n[test] ERROR task failed\n"
	got := extractFailureHighlights(log)
	for _, want := range []string{"undefined: logging.FormatPrepend", "# workspaced/pkg/taskgroup", "[build failed]", "ERROR task failed"} {
		if !containsStr(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if containsStr(got, "=== RUN") || containsStr(got, "--- PASS") {
		t.Fatalf("highlights should omit pass noise:\n%s", got)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
