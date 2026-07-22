package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/pattern"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	"github.com/spf13/cobra"
)

func newGrepCmd() *cobra.Command {
	var dir string
	var lang string

	cmd := &cobra.Command{
		Use:   "grep <pattern> [paths...]",
		Short: "Search for structural pattern matches",
		Long: `Search source files for matches of a structural pattern.

Patterns use a small code-shaped dialect:
  interface{}
  $F:@go:fmt::Errorf($MSG:/(?i)^failed to\s+(.*)/, $ERR)
  $F:@go:strings::SplitN($S, $SEP, 2)

@provider:path::Symbol is a hyperlink hole (same targets as the code view).

Exit status is 0 if any match, 1 if none, 2 on error.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pat := args[0]
			paths := args[1:]
			op, err := pattern.OpFromCLI("grep", lang, pat, "")
			if err != nil {
				return errExit{code: 2, err: err}
			}
			root, err := filepath.Abs(dir)
			if err != nil {
				return errExit{code: 2, err: err}
			}
			res, err := pattern.RunWithOptions(root, op, pattern.RunOptions{Paths: paths})
			if err != nil {
				return errExit{code: 2, err: err}
			}
			w := cmd.OutOrStdout()
			for _, m := range res.Matches {
				line, col, snippet, err := matchDisplay(root, m)
				if err != nil {
					return errExit{code: 2, err: err}
				}
				if _, err := fmt.Fprintf(w, "%s:%d:%d: %s\n", m.File, line, col, snippet); err != nil {
					return errExit{code: 2, err: err}
				}
			}
			if len(res.Matches) == 0 {
				return errExit{code: 1, err: fmt.Errorf("")}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "C", ".", "project root")
	cmd.Flags().StringVarP(&lang, "lang", "l", "go", "language (only go supported)")
	return cmd
}

func matchDisplay(root string, m pattern.Match) (line, col int, snippet string, err error) {
	abs := filepath.Join(root, filepath.FromSlash(m.File))
	src, err := os.ReadFile(abs)
	if err != nil {
		return 0, 0, "", err
	}
	if int(m.EndByte) > len(src) || m.StartByte > m.EndByte {
		return 0, 0, "", fmt.Errorf("match span out of range in %s", m.File)
	}
	li := grammar.NewLineIndexBytes(src)
	l, c0 := li.LineColumnAtU32(m.StartByte)
	text := string(src[m.StartByte:m.EndByte])
	// Single-line snippet for display.
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i] + "…"
	}
	return l, c0 + 1, text, nil
}

// errExit carries a process exit code for main.
type errExit struct {
	code int
	err  error
}

func (e errExit) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e errExit) ExitCode() int { return e.code }

func (e errExit) Unwrap() error { return e.err }
