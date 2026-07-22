package main

import (
	"fmt"
	"path/filepath"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/pattern"
	"github.com/spf13/cobra"
)

func newRewriteCmd() *cobra.Command {
	var dir string
	var lang string
	var backup bool
	var interactive bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "rewrite <pattern> <replacement> [paths...]",
		Short: "Rewrite structural pattern matches",
		Long: `Find matches of a structural pattern and replace each match root with the
replacement template.

Example:
  rft rewrite 'interface{}' 'any'
  rft rewrite \
    '$F:@go:fmt::Errorf($MSG:/(?i)^failed to\s+(.*)/, $ERR)' \
    '$F($MSG, $ERR)'

$Name holes in the replacement are filled from the match. String holes bound
via regex with a capture group re-emit the group text as a string literal.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pat, repl := args[0], args[1]
			paths := args[2:]
			op, err := pattern.OpFromCLI("rewrite", lang, pat, repl)
			if err != nil {
				return err
			}
			root, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			res, err := pattern.RunWithOptions(root, op, pattern.RunOptions{Paths: paths})
			if err != nil {
				return err
			}
			if len(res.Edits) == 0 {
				return fmt.Errorf("no matches")
			}

			w := cmd.ErrOrStderr()
			if interactive || dryRun {
				if _, err := fmt.Fprintf(w, "Edit plan (%d edits):\n", len(res.Edits)); err != nil {
					return err
				}
				for _, e := range res.Edits {
					if _, err := fmt.Fprintf(w, "  %s [%d:%d] → %q\n", e.File, e.StartByte, e.EndByte, e.NewText); err != nil {
						return err
					}
				}
			}
			if dryRun {
				return nil
			}
			if interactive {
				if _, err := fmt.Fprint(w, "Apply? [y/N] "); err != nil {
					return err
				}
				var answer string
				if _, err := fmt.Fscan(cmd.InOrStdin(), &answer); err != nil {
					return err
				}
				if answer != "y" && answer != "Y" {
					return fmt.Errorf("cancelled")
				}
			}
			if backup {
				if err := createBackups(root, res.Edits); err != nil {
					return err
				}
			}
			if err := ensureEditFiles(root, res.Edits); err != nil {
				return err
			}
			return ingest.ApplyEdits(root, res.Edits)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "C", ".", "project root")
	cmd.Flags().StringVarP(&lang, "lang", "l", "go", "language (only go supported)")
	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "create .bak files before writing")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "show edit plan and ask for confirmation")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show edit plan without writing")
	return cmd
}
