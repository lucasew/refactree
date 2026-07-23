package main

import (
	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newMvCmd() *cobra.Command {
	var backup bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "mv <source-reference> <destination-reference>",
		Short: "Move or rename a symbol reference",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Project root is cwd. Do not narrow to the source file's parent —
			// that rebased cross-dir destinations under the source package.
			dir, source, destination := ingest.ResolveMoveArgs(".", args[0], args[1])

			edits, err := ingest.Rename(dir, source, destination)
			if err != nil {
				return err
			}
			return applyEditPlan(cmd, dir, edits, applyEditPlanOptions{
				Interactive: interactive,
				Backup:      backup,
			})
		},
	}

	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "create .bak files before writing")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "show edit plan and ask for confirmation")

	return cmd
}
