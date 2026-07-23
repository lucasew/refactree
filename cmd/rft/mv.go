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
			// Project root is always cwd ("."), then Abs'd by ResolveMoveArgs —
			// not the package being moved. Source/dest only name what to move;
			// materialize walks the whole root (skip list only).
			dir, source, destination := ingest.ResolveMoveArgs(".", args[0], args[1])
			slog.Debug("mv resolve",
				"root", dir,
				"root_note", "cwd Abs; full project walk for closed graph",
				"source_in", args[0],
				"dest_in", args[1],
				"source", source,
				"destination", destination,
				"interactive", interactive,
				"backup", backup,
			)

			plan, err := ingest.Rename(dir, source, destination)
			if err != nil {
				return err
			}
			slog.Debug("mv plan ready",
				"dir_moves", len(plan.DirMoves),
				"edits", len(plan.Edits),
				"files", editFileCount(plan.Edits),
			)
			return applyMovePlan(cmd, dir, plan, applyEditPlanOptions{
				Interactive: interactive,
				Backup:      backup,
			})
		},
	}

	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "create .bak files before writing")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "show edit plan and ask for confirmation")

	return cmd
}
