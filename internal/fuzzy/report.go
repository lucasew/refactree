package fuzzy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Report is the on-disk run report.
type Report struct {
	Dir      string
	Meta     Meta
	eventsFD *os.File
}

// Meta is written to meta.json.
type Meta struct {
	StartedAt  string   `json:"started_at"`
	Seed       int64    `json:"seed"`
	Iterations int      `json:"iterations"`
	Mode       string   `json:"mode"`
	Projects   []string `json:"projects"`
	Commit     string   `json:"commit,omitempty"`
	WorkRoot   string   `json:"work_root"`
	Allow      bool     `json:"allow"`
	NoIsolate  bool     `json:"no_isolate"`
	Offline    bool     `json:"offline"`
	StrictRefs bool     `json:"strict_refs"`
}

// Event is one line in events.jsonl.
type Event struct {
	Time       string             `json:"time"`
	Project    string             `json:"project"`
	Iteration  int                `json:"iteration,omitempty"`
	Kind       string             `json:"kind"`
	Placement  string             `json:"placement,omitempty"`
	Source     string             `json:"source,omitempty"`
	Dest       string             `json:"destination,omitempty"`
	Outcome    string             `json:"outcome"`
	Class      string             `json:"class,omitempty"`
	Error      string             `json:"error,omitempty"`
	Log        string             `json:"log,omitempty"` // report-relative dir with full stdout/stderr
	ExitCode   int                `json:"exit_code,omitempty"`
	Failures   []InvariantFailure `json:"failures,omitempty"`
	DurationMs int64              `json:"duration_ms,omitempty"`
}

// NewReport creates base/<timestamp>-<seed>/.
// When base is empty, reports live under work-root/reports (meta.WorkRoot, or
// DefaultWorkRoot() from package init / SetDefaultWorkRoot).
func NewReport(base string, meta Meta) (*Report, error) {
	if base == "" {
		root := meta.WorkRoot
		if root == "" {
			root = DefaultWorkRoot()
		}
		base = filepath.Join(root, "reports")
	}
	stamp := time.Now().Format("20060102-150405")
	dir := filepath.Join(base, fmt.Sprintf("%s-%d", stamp, meta.Seed))
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "scaffold"), 0o755); err != nil {
		return nil, err
	}
	meta.StartedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644); err != nil {
		return nil, err
	}
	fd, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Report{Dir: dir, Meta: meta, eventsFD: fd}, nil
}

func (r *Report) Close() error {
	if r == nil || r.eventsFD == nil {
		return nil
	}
	return r.eventsFD.Close()
}

func (r *Report) LogEvent(ev Event) error {
	if ev.Time == "" {
		ev.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = r.eventsFD.Write(append(data, '\n'))
	return err
}

func (r *Report) WriteLog(name, content string) error {
	path := filepath.Join(r.Dir, "logs", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// WriteRunResult writes the full command output under logs/<name>/.
// Files: stdout.log, stderr.log, full.log. Returns the report-relative directory.
func (r *Report) WriteRunResult(name string, res RunResult) (string, error) {
	rel := filepath.ToSlash(filepath.Join("logs", name))
	dir := filepath.Join(r.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stdout := res.Stdout
	stderr := res.Stderr
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(stdout), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "stderr.log"), []byte(stderr), 0o644); err != nil {
		return "", err
	}
	full := mergeRunOutput(stdout, stderr)
	if err := os.WriteFile(filepath.Join(dir, "full.log"), []byte(full), 0o644); err != nil {
		return "", err
	}
	meta := map[string]any{
		"exit_code": res.ExitCode,
		"isolated":  res.Isolated,
		"args":      res.Args,
		"dir":       res.Dir,
	}
	if res.Err != nil {
		meta["error"] = res.Err.Error()
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return rel, err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), append(data, '\n'), 0o644); err != nil {
		return rel, err
	}
	return rel, nil
}

func mergeRunOutput(stdout, stderr string) string {
	switch {
	case stdout == "" && stderr == "":
		return ""
	case stderr == "":
		return stdout
	case stdout == "":
		return stderr
	default:
		out := stdout
		if out != "" && out[len(out)-1] != '\n' {
			out += "\n"
		}
		return out + stderr
	}
}

// LogPath returns the absolute path for a report-relative log dir from WriteRunResult.
func (r *Report) LogPath(rel string) string {
	if r == nil || rel == "" {
		return ""
	}
	return filepath.Join(r.Dir, filepath.FromSlash(rel))
}

func (r *Report) ScaffoldDir(projectID string, seed int64, iter int) string {
	return filepath.Join(r.Dir, "scaffold", fmt.Sprintf("%s-%d-%d", projectID, seed, iter))
}
