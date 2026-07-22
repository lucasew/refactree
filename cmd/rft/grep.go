package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/pattern"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	"github.com/spf13/cobra"
)

func newGrepCmd() *cobra.Command {
	var dir string
	var lang string
	var showVars bool
	var format string

	cmd := &cobra.Command{
		Use:   "grep <pattern> [paths...]",
		Short: "Search for structural pattern matches",
		Long: `Search source files for matches of a structural pattern.

Processing is per-file (map): each file is hop-parsed and matched independently,
and matches are printed as soon as that file is done (streaming; no full-tree
barrier before output).

Patterns use a small code-shaped dialect:
  interface{}
  $F:@go:fmt::Errorf($MSG:/(?i)^failed to\s+(.*)/, $ERR)
  $F:@go:strings::SplitN($S, $SEP, 2)
  $F:@go:testing::T
  func $name:{/^Test(?P<rest>.*)/}

@provider:path::Symbol is a hyperlink hole (same targets as the code view).

Output (--format):
  text   (default)  file:line:col: snippet
                    --vars: tab-indented name=value under each match
  csv               header file,line,col,match,<capture…> then one row per hit
                    capture columns are derived statically from the pattern
  jsonl             one JSON object per match (captures map uses the same names)

Formats are pluggable (pattern.GrepFormatter); more can be added later.

Exit status is 0 if any match, 1 if none, 2 on error.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pat := args[0]
			paths := args[1:]
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "text"
			}

			op, err := pattern.OpFromCLI("grep", lang, pat, "")
			if err != nil {
				return errExit{code: 2, err: err}
			}
			root, err := filepath.Abs(dir)
			if err != nil {
				return errExit{code: 2, err: err}
			}

			var enc pattern.GrepFormatter
			switch format {
			case "text":
				enc = pattern.NewTextGrepFormatter(showVars)
			default:
				enc, err = pattern.NewGrepFormatter(format)
				if err != nil {
					return errExit{code: 2, err: err}
				}
			}

			varNames := pattern.CaptureNames(op.PatternIR)
			w := cmd.OutOrStdout()
			if err := enc.Begin(w, varNames); err != nil {
				return errExit{code: 2, err: err}
			}

			matchCount := 0
			err = pattern.Stream(root, op, pattern.StreamOptions{
				Paths: paths,
				OnMatch: func(m pattern.Match, source []byte) bool {
					line, col, snippet, err := matchDisplay(source, m)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "display %s: %v\n", m.File, err)
						return true
					}
					hit := pattern.GrepHit{
						File:     m.File,
						Line:     line,
						Col:      col,
						Match:    snippet,
						Captures: pattern.PublicCaptures(m, source),
					}
					if err := enc.Format(w, hit, varNames); err != nil {
						return false
					}
					matchCount++
					return true
				},
			})
			if endErr := enc.End(w); endErr != nil && err == nil {
				err = endErr
			}
			if err != nil {
				return errExit{code: 2, err: err}
			}
			if matchCount == 0 {
				return errExit{code: 1, err: fmt.Errorf("")}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "C", ".", "project root")
	cmd.Flags().StringVarP(&lang, "lang", "l", "", "language filter (empty = all registered languages)")
	cmd.Flags().BoolVar(&showVars, "vars", false, "with --format=text: print captures under each match as tab-indented name=value")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text, csv, or jsonl")
	return cmd
}

func matchDisplay(src []byte, m pattern.Match) (line, col int, snippet string, err error) {
	if int(m.EndByte) > len(src) || m.StartByte > m.EndByte {
		return 0, 0, "", fmt.Errorf("match span out of range in %s", m.File)
	}
	li := grammar.NewLineIndexBytes(src)
	l, c0 := li.LineColumnAtU32(m.StartByte)
	text := string(src[m.StartByte:m.EndByte])
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
