package edit

import "fmt"

// ExitCodeError is a process exit status that main should propagate without
// treating it as a generic failure message (editor non-zero exits).
type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	if e == nil {
		return "exit"
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitCodeError) ExitCode() int {
	if e == nil {
		return 1
	}
	return e.Code
}
