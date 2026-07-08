package fuzzy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// containerHome is the HOME used inside isolated sessions so bind-mounted
// caches are writable by the host uid/gid (not root-only /root paths).
const containerHome = "/home/rft"

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
	// Offline disables package-manager network and requires local docker images.
	Offline bool
	// DataRoot holds persistent caches across projects (work-root/mise-data).
	DataRoot string
	// Verbose is reserved for extra harness noise; command stdout/stderr always
	// stream live when Stdout/Stderr/Log is set, and are still captured for reports.
	Verbose bool
	// Log receives progress lines always (session start/stop, exec labels).
	Log io.Writer
	// Stdout/Stderr receive live command output (also used as fallback for Log).
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

// liveStdout is where command stdout is teed in addition to capture buffers.
func (r Runner) liveStdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return r.Log
}

// liveStderr is where command stderr is teed in addition to capture buffers.
func (r Runner) liveStderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}
	if r.Stdout != nil {
		return r.Stdout
	}
	return r.Log
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
	hostEnv   []string // non-nil for host sessions: full env for exec.Cmd
	installed bool
}

// disableRyuk prevents testcontainers from pulling/running the reaper image.
func disableRyuk() {
	_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
}

// prepareDataDir creates work-root mise-data subdirs and an empty global mise config.
func (r Runner) prepareDataDir(imageKey string) (string, error) {
	dataDir := r.miseDataDir(imageKey)
	for _, sub := range []string{
		"mise",
		"mise-cache",
		"mise-state",
		"mise-config",
		"go-mod",
		"go-cache",
		"cache",
		"m2",
	} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return "", err
		}
	}
	emptyGlobal := filepath.Join(dataDir, "mise-config", "global.toml")
	if err := os.WriteFile(emptyGlobal, []byte("# refactree fuzzy: no global tools\n"), 0o644); err != nil {
		return "", err
	}
	return dataDir, nil
}

// sessionToolEnv builds HOME/mise/go/maven cache env for a session.
// home is the logical HOME path (containerHome in docker, or a host path under dataDir).
func sessionToolEnv(abs, home string, offline bool, extra map[string]string) map[string]string {
	env := map[string]string{}
	for k, v := range extra {
		env[k] = v
	}
	env["HOME"] = home
	env["USER"] = containerUserName()
	env["MISE_YES"] = "1"
	env["MISE_VERBOSE"] = "1"
	// Skip mise GPG (needs writable ~/.gnupg). Global gpg_verify plus node.gpg_verify:
	// core:node only checks node.gpg_verify (see mise node plugin).
	env["MISE_GPG_VERIFY"] = "false"
	env["MISE_NODE_GPG_VERIFY"] = "false"
	// npm:* tools: use node/npm backend, not bun/aqua (host mise often prefers bun).
	env["MISE_NPM_PACKAGE_MANAGER"] = "npm"
	env["MISE_TRUSTED_CONFIG_PATHS"] = strings.Join([]string{abs, home, home + "/.config/mise", "/mise"}, ":")
	env["MISE_DATA_DIR"] = home + "/.local/share/mise"
	env["MISE_CACHE_DIR"] = home + "/.cache/mise"
	env["MISE_STATE_DIR"] = home + "/.local/state/mise"
	env["MISE_CONFIG_DIR"] = home + "/.config/mise"
	env["MISE_GLOBAL_CONFIG_FILE"] = home + "/.config/mise/global.toml"
	env["GOPATH"] = home + "/go"
	env["GOMODCACHE"] = home + "/go/pkg/mod"
	env["GOCACHE"] = home + "/.cache/go-build"
	env["XDG_CACHE_HOME"] = home + "/.cache"
	env["XDG_STATE_HOME"] = home + "/.local/state"
	env["XDG_CONFIG_HOME"] = home + "/.config"
	env["MAVEN_USER_HOME"] = home + "/.m2"
	env["GRADLE_USER_HOME"] = home + "/.gradle"
	if offline {
		for k, v := range OfflineSessionEnv() {
			env[k] = v
		}
	}
	return env
}

// hostEnvFromDataDir builds exec env using work-root mise-data (no reliance on ambient HOME caches).
func hostEnvFromDataDir(abs, dataDir string, offline bool, projectEnv []string) []string {
	// Point cache env vars at dataDir subdirs directly (more reliable than symlink home).
	extra := envMap(projectEnv)
	envMap := map[string]string{
		"HOME":                      filepath.Join(dataDir, "host-home"),
		"USER":                      containerUserName(),
		"MISE_YES":                  "1",
		"MISE_VERBOSE":              "1",
		"MISE_GPG_VERIFY":           "false",
		"MISE_NODE_GPG_VERIFY":      "false",
		"MISE_NPM_PACKAGE_MANAGER":  "npm",
		"MISE_TRUSTED_CONFIG_PATHS": abs + ":" + filepath.Join(dataDir, "host-home") + ":" + filepath.Join(dataDir, "mise-config"),
		"MISE_DATA_DIR":             filepath.Join(dataDir, "mise"),
		"MISE_CACHE_DIR":            filepath.Join(dataDir, "mise-cache"),
		"MISE_STATE_DIR":            filepath.Join(dataDir, "mise-state"),
		"MISE_CONFIG_DIR":           filepath.Join(dataDir, "mise-config"),
		"MISE_GLOBAL_CONFIG_FILE":   filepath.Join(dataDir, "mise-config", "global.toml"),
		"GOPATH":                    filepath.Join(dataDir, "go"),
		"GOMODCACHE":                filepath.Join(dataDir, "go-mod"),
		"GOCACHE":                   filepath.Join(dataDir, "go-cache"),
		"XDG_CACHE_HOME":            filepath.Join(dataDir, "cache"),
		"XDG_STATE_HOME":            filepath.Join(dataDir, "mise-state"),
		"XDG_CONFIG_HOME":           filepath.Join(dataDir, "mise-config"),
		"MAVEN_USER_HOME":           filepath.Join(dataDir, "m2"),
		"GRADLE_USER_HOME":          filepath.Join(dataDir, "gradle"),
	}
	_ = os.MkdirAll(filepath.Join(dataDir, "host-home"), 0o755)
	_ = os.MkdirAll(filepath.Join(dataDir, "go"), 0o755)
	for k, v := range extra {
		envMap[k] = v
	}
	if offline {
		for k, v := range OfflineSessionEnv() {
			envMap[k] = v
		}
	}
	// Merge over ambient env so PATH and other host tools remain available.
	base := os.Environ()
	out := make([]string, 0, len(base)+len(envMap)+2)
	seen := map[string]struct{}{}
	for k := range envMap {
		seen[k] = struct{}{}
	}
	// Keys we always override from envMap.
	for _, e := range base {
		k, _, _ := strings.Cut(e, "=")
		if _, ok := seen[k]; ok {
			continue
		}
		// Strip ambient offline/mise cache vars that would fight work-root.
		switch k {
		case "MISE_OFFLINE", OfflineEnvKey, "GOPROXY", "GOSUMDB", "UV_OFFLINE", "NPM_CONFIG_OFFLINE", "PNPM_OFFLINE":
			if offline {
				continue
			}
		}
		out = append(out, e)
	}
	for k, v := range envMap {
		out = append(out, k+"="+v)
	}
	return appendVerboseEnv(out)
}

// StartSession starts a reusable container with the worktree and tool caches mounted.
// network follows setup_network (check reuses the same networked session).
func (r Runner) StartSession(ctx context.Context, cfg IsolateConfig, dir, imageKey string, network bool) (*Session, error) {
	disableRyuk()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dataDir, err := r.prepareDataDir(imageKey)
	if err != nil {
		return nil, err
	}

	if r.NoIsolate {
		r.logf("isolate: host session workdir=%s data=%s offline=%v\n", abs, dataDir, r.Offline)
		hostEnv := hostEnvFromDataDir(abs, dataDir, r.Offline, cfg.Env)
		return &Session{
			runner:  r,
			cfg:     cfg,
			abs:     abs,
			dataDir: dataDir,
			image:   "host",
			hostEnv: hostEnv,
		}, nil
	}
	if err := RequireDocker(ctx); err != nil {
		return nil, err
	}
	image := cfg.ImageOrDefault()
	if err := EnsureImages([]string{image}, !r.Offline); err != nil {
		return nil, err
	}
	// Online may still pull; offline already validated. AlwaysPullImage stays false.
	env := sessionToolEnv(abs, containerHome, r.Offline, envMap(cfg.Env))

	uid, gid := hostUserNamespace()
	userSpec := fmt.Sprintf("%d:%d", uid, gid)

	if network {
		r.logf("isolate: starting session image=%s user=%s network=on workdir=%s data=%s\n", image, userSpec, abs, dataDir)
	} else {
		r.logf("isolate: starting session image=%s user=%s network=none workdir=%s data=%s\n", image, userSpec, abs, dataDir)
	}

	home := containerHome
	req := testcontainers.ContainerRequest{
		Image:           image,
		AlwaysPullImage: false,
		Entrypoint:      []string{"/bin/bash", "-lc"},
		Cmd:             []string{"sleep infinity"},
		User:            userSpec,
		WorkingDir:      abs,
		Env:             env,
		WaitingFor:      wait.ForExec([]string{"/bin/true"}),
		HostConfigModifier: func(hc *container.HostConfig) {
			// Mount each cache path directly (no nested mounts under a HOME bind)
			// so host-side cleanup does not hit busy mountpoints.
			hc.Binds = []string{
				abs + ":" + abs,
				filepath.Join(dataDir, "mise") + ":" + home + "/.local/share/mise",
				filepath.Join(dataDir, "mise-cache") + ":" + home + "/.cache/mise",
				filepath.Join(dataDir, "mise-state") + ":" + home + "/.local/state/mise",
				filepath.Join(dataDir, "mise-config") + ":" + home + "/.config/mise",
				filepath.Join(dataDir, "mise-config", "global.toml") + ":/mise/config.toml:ro",
				filepath.Join(dataDir, "go-mod") + ":" + home + "/go/pkg/mod",
				filepath.Join(dataDir, "go-cache") + ":" + home + "/.cache/go-build",
				filepath.Join(dataDir, "cache") + ":" + home + "/.cache",
				filepath.Join(dataDir, "m2") + ":" + home + "/.m2",
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
		if !s.installed {
			if install := s.hostMiseInstall(ctx); !install.OK() {
				return install
			}
			s.installed = true
		}
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = s.abs
		if len(s.hostEnv) > 0 {
			cmd.Env = s.hostEnv
		} else {
			cmd.Env = appendVerboseEnv(os.Environ())
		}
		return finishHostCmd(cmd, &res, s.runner.liveStdout(), s.runner.liveStderr())
	}

	if !s.installed {
		// Offline relies on MISE_OFFLINE=1 in session env (no install --offline flag on all mise versions).
		install := s.execScript(ctx, "mise -v install")
		if !install.OK() {
			return install
		}
		s.installed = true
	}
	return s.execScript(ctx, joinCommand(argv))
}

func (s *Session) hostMiseInstall(ctx context.Context) RunResult {
	res := RunResult{Dir: s.abs, Isolated: false, Args: []string{"mise", "-v", "install"}}
	// Skip when no mise.toml (local fixtures may only run `true`).
	if _, err := os.Stat(filepath.Join(s.abs, "mise.toml")); err != nil {
		return res
	}
	s.runner.logf("isolate: host exec: mise -v install\n")
	cmd := exec.CommandContext(ctx, "mise", "-v", "install")
	cmd.Dir = s.abs
	if len(s.hostEnv) > 0 {
		cmd.Env = s.hostEnv
	} else {
		cmd.Env = appendVerboseEnv(os.Environ())
	}
	return finishHostCmd(cmd, &res, s.runner.liveStdout(), s.runner.liveStderr())
}

func (s *Session) execScript(ctx context.Context, scriptBody string) RunResult {
	script := "set -euo pipefail; set -x; " + scriptBody
	res := RunResult{
		Args:     []string{"exec", s.image, "/bin/bash", "-lc", script},
		Dir:      s.abs,
		Isolated: true,
	}
	s.runner.logf("isolate: exec %s\n", scriptBody)

	var outBuf bytes.Buffer
	// Multiplexed exec: capture + always passthrough to process stdout (and configured writers).
	live := passthroughOut(s.runner.liveStdout())
	if s.runner.Stderr != nil && s.runner.Stdout != nil && s.runner.Stdout != s.runner.Stderr {
		live = io.MultiWriter(passthroughOut(s.runner.Stdout), passthroughErr(s.runner.Stderr))
	}
	capture := io.MultiWriter(&outBuf, live)

	code, reader, err := s.ctr.Exec(ctx, []string{"/bin/bash", "-lc", script}, tcexec.Multiplexed())
	if err != nil {
		res.Err = fmt.Errorf("exec: %w", err)
		res.ExitCode = 1
		res.Stdout = outBuf.String()
		return res
	}
	if reader != nil {
		_, _ = io.Copy(capture, reader)
	}
	// Multiplexed output is combined; keep stderr field empty and put all in Stdout.
	res.Stdout = outBuf.String()
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
		// Same tree as work-root; only the work-root itself falls back to $TMPDIR/rft-fuzzy.
		root = filepath.Join(DefaultWorkRoot(), "mise-data")
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
	// Always passthrough live (default os.Stdout/Stderr) and capture for reports.
	outStr, errStr, err := runStreaming(cmd, stdout, stderr)
	res.Stdout = outStr
	res.Stderr = errStr
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

func hostUserNamespace() (uid, gid int) {
	uid = os.Getuid()
	gid = os.Getgid()
	if uid < 0 {
		uid = 0
	}
	if gid < 0 {
		gid = 0
	}
	return uid, gid
}

func containerUserName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "rft"
}

// ForceRemoveAll deletes path, using a privileged docker rm when the host
// user cannot remove root-owned artifacts left by older isolation runs.
func ForceRemoveAll(path string) error {
	if path == "" {
		return nil
	}
	if err := os.RemoveAll(path); err == nil {
		return nil
	}
	if _, statErr := os.Stat(path); statErr != nil && os.IsNotExist(statErr) {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
	}
	parent := filepath.Dir(abs)
	base := filepath.Base(abs)
	// Prefer CleanupImage (same pin as sessions). Never pull here — must be local.
	img := CleanupImage
	if !ImagePresent(img) {
		// Last resort: try plain rm again after privileged path unavailable.
		return fmt.Errorf("force remove %s: cleanup image %q not present locally (run: rft fuzzy prefetch)", abs, img)
	}
	cmd := exec.Command("docker", "run", "--rm", "-v", parent+":/work", img, "rm", "-rf", "/work/"+base)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("force remove %s: %w\n%s", abs, err, out)
	}
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("force remove %s: path still exists", abs)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
