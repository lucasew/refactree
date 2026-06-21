package main

import (
	"github.com/lucasew/refactree/pkg/version"
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
		Version:       version.GetBuildID(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "enable verbose logging")

	cmd.AddCommand(
		newASTDumpCmd(opts),
		newBrowseCmd(opts),
		newLsCmd(opts),
		newMvCmd(opts),
		newDocCmd(opts),
		newIngestCmd(opts),
	)

	return cmd
}
