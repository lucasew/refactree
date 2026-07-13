package main

import (
	"github.com/lucasew/refactree/pkg/ui/edit"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	var all bool
	var editor string

	cmd := &cobra.Command{
		Use:   "edit [reference]",
		Short: "Open a reference definition in an editor",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := ""
			if len(args) == 1 {
				input = args[0]
			}
			return edit.Run(edit.Options{
				BaseDir:       ".",
				Input:         input,
				IncludeHidden: all,
				EditorBin:     editor,
			})
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "include normally hidden symbols in the picker")
	cmd.Flags().StringVar(&editor, "editor", "", "editor binary (overrides RFT_EDITOR, VISUAL, EDITOR)")
	return cmd
}
