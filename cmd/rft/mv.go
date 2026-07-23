package main

import (
	"log/slog"

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
			slog.Debug("mv resolve",
				"root", dir,
				"source_in", args[0],
				"dest_in", args[1],
				"source", source,
				"destination", destination,
				"interactive", interactive,
				"backup", backup,
			)

			edits, err := ingest.Rename(dir, source, destination)
			if err != nil {
				return err
			}
			slog.Debug("mv plan ready", "edits", len(edits), "files", editFileCount(edits))
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
