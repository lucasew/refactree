package main

import (
	"github.com/lucasew/refactree/pkg/ingest"
	browseui "github.com/lucasew/refactree/pkg/ui/browse"
	"github.com/spf13/cobra"
)

func newBrowseCmd(root *rootOptions) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "browse [reference]",
		Short: "Interactive symbol browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refInput := "path:./"
			if len(args) == 1 {
				refInput = args[0]
			}

			scope := ingest.ResolveInputReferenceScope(".", refInput)
			ref := ingest.AbsolutePathReferenceForScope(scope)
			ui, err := browseui.New(browseui.Options{
				Reference:     ref,
				IncludeHidden: all,
			})
			if err != nil {
				return err
			}
			return ui.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "show hidden packages and symbols")
	return cmd
}
