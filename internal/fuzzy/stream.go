package fuzzy

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

// processLogPassthrough controls whether command streams are also written to the
// real process stdout/stderr. Must be off inside go test -fuzz workers: they use
// stdout for a binary protocol, and any harness/mise/git spam causes
// "fuzzing process hung or terminated unexpectedly: exit status 2".
var processLogPassthrough atomic.Bool

func init() {
	processLogPassthrough.Store(true)
}

// MuteProcessLogs disables teeing to os.Stdout/os.Stderr (fuzz workers).
// Returns a restore function.
func MuteProcessLogs() (restore func()) {
	prev := processLogPassthrough.Swap(false)
	return func() { processLogPassthrough.Store(prev) }
}

// flushWriter writes through and Syncs when possible so go test / pipes show
// output promptly instead of holding large blocks.
type flushWriter struct {
	w io.Writer
}

func (f flushWriter) Write(p []byte) (int, error) {
	if f.w == nil {
		return len(p), nil
	}
	n, err := f.w.Write(p)
	if s, ok := f.w.(interface{ Sync() error }); ok {
		_ = s.Sync()
	}
	return n, err
}

func liveOrStdout(w io.Writer) io.Writer {
	if w != nil {
		return flushWriter{w}
	}
	if !processLogPassthrough.Load() {
		return flushWriter{io.Discard}
	}
	return flushWriter{os.Stdout}
}

func liveOrStderr(w io.Writer) io.Writer {
	if w != nil {
		return flushWriter{w}
	}
	if !processLogPassthrough.Load() {
		return flushWriter{io.Discard}
	}
	return flushWriter{os.Stderr}
}

// passthroughOut tees to the process stdout when passthrough is enabled so
// normal go test / pipes show command logs even if the harness writer is a buffer.
// When muted (fuzz), only `w` receives bytes (use io.Discard to silence).
func passthroughOut(w io.Writer) io.Writer {
	fw := liveOrStdout(w)
	if !processLogPassthrough.Load() {
		return fw
	}
	if w == nil || w == os.Stdout {
		return fw
	}
	return io.MultiWriter(fw, flushWriter{os.Stdout})
}

func passthroughErr(w io.Writer) io.Writer {
	fw := liveOrStderr(w)
	if !processLogPassthrough.Load() {
		return fw
	}
	if w == nil || w == os.Stderr {
		return fw
	}
	return io.MultiWriter(fw, flushWriter{os.Stderr})
}

// runStreaming runs cmd, teeing stdout/stderr live while capturing for the return.
func runStreaming(cmd *exec.Cmd, stdout, stderr io.Writer) (stdoutStr, stderrStr string, err error) {
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&outBuf, passthroughOut(stdout))
	cmd.Stderr = io.MultiWriter(&errBuf, passthroughErr(stderr))
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// runStreamingCombined tees both streams live and into one buffer (for tools that
// mix progress on stdout/stderr).
func runStreamingCombined(cmd *exec.Cmd, live io.Writer) (combined string, err error) {
	var buf bytes.Buffer
	w := io.MultiWriter(&buf, passthroughOut(live))
	cmd.Stdout = w
	cmd.Stderr = w
	err = cmd.Run()
	return buf.String(), err
}

func logCmdLine(w io.Writer, argv ...string) {
	if w == nil {
		if !processLogPassthrough.Load() {
			return
		}
		w = os.Stdout
	}
	fmt.Fprintf(liveOrStdout(w), "+ %s\n", strings.Join(argv, " "))
}
