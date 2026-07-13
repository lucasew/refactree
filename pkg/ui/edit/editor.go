package edit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Location is a file open target for an editor.
type Location struct {
	Path   string // filesystem path (absolute preferred)
	Line   int    // 1-based
	Column int    // 1-based (editor convention)
}

// Editor opens a location and blocks until the editor exits.
type Editor interface {
	Open(loc Location) error
}

// PathLineColumnEditor runs Bin with a single path:line:column argument.
type PathLineColumnEditor struct {
	Bin string

	// Optional process wiring (defaults to the process stdio).
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

func (e PathLineColumnEditor) Open(loc Location) error {
	if e.Bin == "" {
		return fmt.Errorf("editor binary is empty")
	}
	if loc.Path == "" {
		return fmt.Errorf("empty path")
	}
	line := loc.Line
	if line < 1 {
		line = 1
	}
	col := loc.Column
	if col < 1 {
		col = 1
	}
	arg := fmt.Sprintf("%s:%d:%d", loc.Path, line, col)
	cmd := exec.Command(e.Bin, arg)
	// Assign only non-nil *os.File values. A typed-nil *os.File in an
	// io.Reader/Writer interface is non-nil and breaks child stdio.
	if e.Stdin != nil {
		cmd.Stdin = e.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}
	if e.Stdout != nil {
		cmd.Stdout = e.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if e.Stderr != nil {
		cmd.Stderr = e.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return &ExitCodeError{Code: ee.ExitCode()}
	}
	return err
}

// ResolveEditorBin picks the editor binary.
// Order: flag, RFT_EDITOR, VISUAL, EDITOR.
func ResolveEditorBin(flag string, getenv func(string) string) (string, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, v := range []string{flag, getenv("RFT_EDITOR"), getenv("VISUAL"), getenv("EDITOR")} {
		v = strings.TrimSpace(v)
		if v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("no editor: set --editor, RFT_EDITOR, VISUAL, or EDITOR")
}
