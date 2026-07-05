package fuzzy_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/internal/fuzzy"
)

func TestRunnerNoIsolate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := fuzzy.Runner{NoIsolate: true, Log: io.Discard}
	s, err := r.StartSession(context.Background(), fuzzy.IsolateConfig{}, dir, "local", true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	res := s.Run(context.Background(), []string{"true"})
	if !res.OK() {
		t.Fatalf("true failed: %#v", res)
	}
	if res.Isolated {
		t.Fatal("expected non-isolated")
	}
}

func TestQuietModeBuffersUntilFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var live bytes.Buffer
	r := fuzzy.Runner{
		NoIsolate: true,
		Verbose:   false,
		Log:       io.Discard,
		Stdout:    &live,
		Stderr:    &live,
	}
	s, err := r.StartSession(context.Background(), fuzzy.IsolateConfig{}, dir, "quiet", true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	res := s.Run(context.Background(), []string{"sh", "-c", "echo secret-ok"})
	if !res.OK() {
		t.Fatal(res.Err)
	}
	if strings.Contains(live.String(), "secret-ok") {
		t.Fatalf("quiet mode leaked output: %q", live.String())
	}
	if !strings.Contains(res.Stdout, "secret-ok") {
		t.Fatalf("expected accumulated stdout, got %q", res.Stdout)
	}
	fail := s.Run(context.Background(), []string{"sh", "-c", "echo secret-fail >&2; exit 7"})
	if fail.OK() {
		t.Fatal("expected failure")
	}
	if !strings.Contains(fail.Stdout+fail.Stderr, "secret-fail") {
		t.Fatalf("failure should accumulate logs: stdout=%q stderr=%q", fail.Stdout, fail.Stderr)
	}
	if strings.Contains(live.String(), "secret-fail") {
		t.Fatalf("quiet mode leaked failure output live: %q", live.String())
	}
}

func TestSessionReusesContainerForSetupAndCheck(t *testing.T) {
	if err := fuzzy.RequireDocker(context.Background()); err != nil {
		t.Skip(err.Error())
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "session-marker")
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte(fmt.Sprintf(`
[tasks.setup]
run = "echo setup-marker > %s && echo did-setup"
[tasks.test]
run = "test -f %s && echo did-check"
`, marker, marker)), 0o644); err != nil {
		t.Fatal(err)
	}
	var live bytes.Buffer
	r := fuzzy.Runner{
		DataRoot: filepath.Join(t.TempDir(), "mise-data"),
		Verbose:  true,
		Log:      io.MultiWriter(os.Stdout, &live),
		Stdout:   io.MultiWriter(os.Stdout, &live),
		Stderr:   io.MultiWriter(os.Stderr, &live),
	}
	ctx := context.Background()
	s, err := r.StartSession(ctx, fuzzy.IsolateConfig{Image: fuzzy.DefaultMiseImage}, dir, "reuse", true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx)

	setup := s.Run(ctx, []string{"mise", "run", "setup"})
	if !setup.OK() {
		t.Fatalf("setup: %#v\n%s%s", setup.Err, setup.Stdout, setup.Stderr)
	}
	check := s.Run(ctx, []string{"mise", "run", "test"})
	if !check.OK() {
		t.Fatalf("check should see setup marker in same container: err=%v\n%s%s", check.Err, check.Stdout, check.Stderr)
	}
	combined := setup.Stdout + setup.Stderr + check.Stdout + check.Stderr + live.String()
	if !strings.Contains(combined, "did-setup") || !strings.Contains(combined, "did-check") {
		t.Fatalf("missing markers in output: %q", combined)
	}
	liveOut := live.String()
	if strings.Count(liveOut, "starting session") != 1 {
		t.Fatalf("expected one session start, output:\n%s", liveOut)
	}
	if strings.Count(liveOut, "isolate: exec mise -v install") != 1 {
		t.Fatalf("expected mise install once per session, got:\n%s", liveOut)
	}
}
