package fuzzy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RunResult captures command output and status.
type RunResult struct {
	Args     []string
	Dir      string
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
	Isolated bool
}

func (r RunResult) OK() bool {
	return r.Err == nil && r.ExitCode == 0
}

// Runner creates isolation sessions backed by testcontainers.
type Runner struct {
	NoIsolate bool
	// DataRoot holds persistent caches across projects (work-root/mise-data).
	DataRoot string
	// Verbose streams command stdout/stderr live; otherwise output is accumulated
	// and only surfaced on failure.
	Verbose bool
	// Log receives progress lines always (session start/stop, exec labels).
	Log io.Writer
	// Stdout/Stderr receive live command output only when Verbose is set.
	Stdout io.Writer
	Stderr io.Writer
}

func (r Runner) logf(format string, args ...any) {
	w := r.Log
	if w == nil {
		w = r.Stdout
	}
	if w == nil {
		return
	}
	fmt.Fprintf(w, format, args...)
}

func (r Runner) liveStdout() io.Writer {
	if r.Verbose {
		return r.Stdout
	}
	return nil
}

func (r Runner) liveStderr() io.Writer {
	if !r.Verbose {
		return nil
	}
	if r.Stderr != nil {
		return r.Stderr
	}
	return r.Stdout
}

// RequireDocker checks that the Docker API is reachable (Jules includes docker).
func RequireDocker(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found on PATH (required for fuzzy isolation, or pass --no-isolate): %w", err)
	}
	cmd := exec.CommandContext(ctx, "docker", "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker is not usable: %w\n%s", err, out)
	}
	return nil
}

// Session is one long-lived mise container reused for setup and checks.
type Session struct {
	runner    Runner
	cfg       IsolateConfig
	ctr       testcontainers.Container
	abs       string
	dataDir   string
	image     string
	installed bool
}

// StartSession starts a reusable container with the worktree and tool caches mounted.
// network follows setup_network (check reuses the same networked session).
func (r Runner) StartSession(ctx context.Context, cfg IsolateConfig, dir, imageKey string, network bool) (*Session, error) {
	if r.NoIsolate {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		return &Session{runner: r, cfg: cfg, abs: abs, image: "host"}, nil
	}
	if err := RequireDocker(ctx); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dataDir := r.miseDataDir(imageKey)
	for _, sub := range []string{"mise", "go-mod", "go-cache", "cache"} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	image := cfg.ImageOrDefault()
	env := envMap(cfg.Env)
	if env == nil {
		env = map[string]string{}
	}
	env["MISE_YES"] = "1"
	env["MISE_VERBOSE"] = "1"
	env["MISE_TRUSTED_CONFIG_PATHS"] = abs
	env["GOPATH"] = "/root/go"
	env["GOMODCACHE"] = "/root/go/pkg/mod"
	env["GOCACHE"] = "/root/.cache/go-build"
	env["XDG_CACHE_HOME"] = "/root/.cache"

	if network {
		r.logf("isolate: starting session image=%s network=on workdir=%s data=%s\n", image, abs, dataDir)
	} else {
		r.logf("isolate: starting session image=%s network=none workdir=%s data=%s\n", image, abs, dataDir)
	}

	req := testcontainers.ContainerRequest{
		Image:      image,
		Entrypoint: []string{"/bin/bash", "-lc"},
		Cmd:        []string{"sleep infinity"},
		WorkingDir: abs,
		Env:        env,
		WaitingFor: wait.ForExec([]string{"/bin/true"}),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = []string{
				abs + ":" + abs,
				filepath.Join(dataDir, "mise") + ":/root/.local/share/mise",
				filepath.Join(dataDir, "go-mod") + ":/root/go/pkg/mod",
				filepath.Join(dataDir, "go-cache") + ":/root/.cache/go-build",
				filepath.Join(dataDir, "cache") + ":/root/.cache",
			}
			if !network {
				hc.NetworkMode = container.NetworkMode("none")
			}
		},
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start session container: %w", err)
	}
	s := &Session{runner: r, cfg: cfg, ctr: ctr, abs: abs, dataDir: dataDir, image: image}
	r.logf("isolate: session ready (%s)\n", image)
	return s, nil
}

// Close terminates the session container.
func (s *Session) Close(ctx context.Context) error {
	if s == nil || s.ctr == nil {
		return nil
	}
	s.runner.logf("isolate: stopping session\n")
	return s.ctr.Terminate(ctx)
}

// Run executes argv inside the session (or on the host when isolation is disabled).
func (s *Session) Run(ctx context.Context, argv []string) RunResult {
	res := RunResult{Dir: s.abs, Isolated: s.ctr != nil}
	if len(argv) == 0 {
		res.Err = fmt.Errorf("empty command")
		res.ExitCode = 1
		return res
	}
	argv = withVerbose(argv)
	res.Args = append([]string(nil), argv...)

	if s.ctr == nil {
		s.runner.logf("isolate: host exec: %s\n", strings.Join(argv, " "))
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = s.abs
		cmd.Env = appendVerboseEnv(os.Environ())
		return finishHostCmd(cmd, &res, s.runner.liveStdout(), s.runner.liveStderr())
	}

	if !s.installed {
		install := s.execScript(ctx, "mise -v install")
		if !install.OK() {
			return install
		}
		s.installed = true
	}
	return s.execScript(ctx, joinCommand(argv))
}

func (s *Session) execScript(ctx context.Context, scriptBody string) RunResult {
	script := "set -euo pipefail; set -x; " + scriptBody
	res := RunResult{
		Args:     []string{"exec", s.image, "/bin/bash", "-lc", script},
		Dir:      s.abs,
		Isolated: true,
	}
	s.runner.logf("isolate: exec %s\n", scriptBody)

	var outBuf, errBuf bytes.Buffer
	// Multiplexed exec combines streams; split live passthrough vs capture.
	var live io.Writer
	if s.runner.Verbose {
		if s.runner.Stdout != nil && s.runner.Stderr != nil && s.runner.Stdout != s.runner.Stderr {
			live = io.MultiWriter(s.runner.Stdout, s.runner.Stderr)
		} else if s.runner.Stdout != nil {
			live = s.runner.Stdout
		} else {
			live = s.runner.Stderr
		}
	}
	capture := io.Writer(&outBuf)
	if live != nil {
		capture = io.MultiWriter(&outBuf, live)
	}

	code, reader, err := s.ctr.Exec(ctx, []string{"/bin/bash", "-lc", script}, tcexec.Multiplexed())
	if err != nil {
		res.Err = fmt.Errorf("exec: %w", err)
		res.ExitCode = 1
		res.Stdout = outBuf.String()
		res.Stderr = errBuf.String()
		return res
	}
	if reader != nil {
		_, _ = io.Copy(capture, reader)
	}
	// Multiplexed output is combined; keep stderr field empty and put all in Stdout.
	res.Stdout = outBuf.String()
	res.Stderr = errBuf.String()
	res.ExitCode = code
	if code != 0 {
		res.Err = fmt.Errorf("exit status %d", code)
	}
	return res
}

// Run is a one-shot helper that starts a session, runs argv, and closes it.
// Prefer StartSession when setup and check must share state.
func (r Runner) Run(ctx context.Context, cfg IsolateConfig, dir, imageKey string, argv []string, network bool) RunResult {
	s, err := r.StartSession(ctx, cfg, dir, imageKey, network)
	if err != nil {
		return RunResult{Err: err, ExitCode: 1, Args: append([]string(nil), argv...)}
	}
	defer func() { _ = s.Close(ctx) }()
	return s.Run(ctx, argv)
}

func (r Runner) miseDataDir(imageKey string) string {
	root := r.DataRoot
	if root == "" {
		root = filepath.Join(os.TempDir(), "rft-fuzzy-mise-data")
	}
	key := imageKey
	if key == "" {
		key = "default"
	}
	return filepath.Join(root, sanitizeKey(key))
}

func joinCommand(argv []string) string {
	var b strings.Builder
	for i, a := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellQuote(a))
	}
	return b.String()
}

func appendVerboseEnv(base []string) []string {
	out := make([]string, 0, len(base)+2)
	for _, e := range base {
		k, _, _ := strings.Cut(e, "=")
		if k == "MISE_VERBOSE" || k == "MISE_YES" {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "MISE_VERBOSE=1", "MISE_YES=1")
	return out
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func envMap(entries []string) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			out[k] = v
		}
	}
	return out
}

func finishHostCmd(cmd *exec.Cmd, res *RunResult, stdout, stderr io.Writer) RunResult {
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if stdout != nil {
		cmd.Stdout = io.MultiWriter(&outBuf, stdout)
	}
	if stderr != nil {
		cmd.Stderr = io.MultiWriter(&errBuf, stderr)
	}
	err := cmd.Run()
	res.Stdout = outBuf.String()
	res.Stderr = errBuf.String()
	if err != nil {
		res.Err = err
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = 1
		}
		return *res
	}
	res.ExitCode = 0
	return *res
}

func sanitizeKey(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, s)
	if s == "" {
		return "default"
	}
	return s
}

// ImageKey builds a stable cache key for persistent mise data.
func ImageKey(p Project, commit string) string {
	ref := commit
	if ref == "" {
		ref = p.Ref
	}
	if len(ref) > 12 {
		ref = ref[:12]
	}
	return p.ID + "-" + ref
}
