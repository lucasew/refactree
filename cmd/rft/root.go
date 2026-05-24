package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type rootOptions struct {
	verbose bool
}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:           "rft",
		Short:         "Query symbols and plan refactorings",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "enable verbose logging")

	cmd.AddCommand(
		newLsCmd(opts),
		newMvCmd(opts),
		newDocCmd(opts),
	)

	return cmd
}

func stubf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
