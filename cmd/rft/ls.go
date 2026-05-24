package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLsCmd(root *rootOptions) *cobra.Command {
	var all bool
	var long bool

	cmd := &cobra.Command{
		Use:   "ls <reference>",
		Short: "List symbols in a reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reference := args[0]
			stubf(
				cmd.ErrOrStderr(),
				"ls stub: reference=%q all=%t long=%t verbose=%t\n",
				reference,
				all,
				long,
				root.verbose,
			)
			return fmt.Errorf("ls not implemented")
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "include normally hidden symbols")
	cmd.Flags().BoolVarP(&long, "long", "l", false, "use long listing format")

	return cmd
}
