package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMvCmd(root *rootOptions) *cobra.Command {
	var backup bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "mv <source-reference> <destination-reference>",
		Short: "Move or rename a symbol reference",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			destination := args[1]
			stubf(
				cmd.ErrOrStderr(),
				"mv stub: source=%q destination=%q backup=%t interactive=%t verbose=%t\n",
				source,
				destination,
				backup,
				interactive,
				root.verbose,
			)
			return fmt.Errorf("mv not implemented")
		},
	}

	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "create .bak files before writing")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "show edit plan and ask for confirmation")

	return cmd
}
