package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDocCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doc <reference>",
		Short: "Show documentation for a reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reference := args[0]
			stubf(
				cmd.ErrOrStderr(),
				"doc stub: reference=%q verbose=%t\n",
				reference,
				root.verbose,
			)
			return fmt.Errorf("doc not implemented")
		},
	}
}
