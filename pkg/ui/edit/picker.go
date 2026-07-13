package edit

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Picker selects one candidate string (typically a full reference).
// stream emits candidates incrementally; emit may return a non-nil error
// when the consumer stops early (e.g. fzf closed stdin).
type Picker interface {
	Pick(stream func(emit func(string) error) error) (string, error)
}

// FZFPicker shells out to fzf and streams candidates on stdin, one per line.
type FZFPicker struct {
	// LookPath defaults to exec.LookPath.
	LookPath func(file string) (string, error)
	// Stderr is attached to the process stderr so the TUI renders.
	Stderr *os.File
}

func (p FZFPicker) Pick(stream func(emit func(string) error) error) (string, error) {
	if stream == nil {
		return "", fmt.Errorf("no symbols to edit")
	}
	look := p.LookPath
	if look == nil {
		look = exec.LookPath
	}
	bin, err := look("fzf")
	if err != nil {
		return "", fmt.Errorf("fzf not found on PATH")
	}

	cmd := exec.Command(bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	// Avoid typed-nil *os.File in the io.Writer interface (breaks fzf TUI).
	if p.Stderr != nil {
		cmd.Stderr = p.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return "", err
	}

	var streamErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdin.Close()
		streamErr = stream(func(line string) error {
			_, err := io.WriteString(stdin, line+"\n")
			return err
		})
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	sel := strings.TrimSpace(stdout.String())
	if waitErr != nil {
		// Prefer source errors (e.g. empty scope) over raw fzf exit noise.
		if streamErr != nil && sel == "" {
			return "", streamErr
		}
		return "", fmt.Errorf("fzf: %w", waitErr)
	}
	if sel == "" {
		if streamErr != nil {
			return "", streamErr
		}
		return "", fmt.Errorf("no selection")
	}
	// Broken pipe / early close while streaming after a selection is fine.
	return sel, nil
}
