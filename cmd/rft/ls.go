package main

import (
	"fmt"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newLsCmd(root *rootOptions) *cobra.Command {
	var all bool
	var long bool
	var recursive bool

	cmd := &cobra.Command{
		Use:   "ls <reference>",
		Short: "List symbols in a reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := ingest.ResolveInputReferenceScope(".", args[0])
			dir, ref := scope.Dir, scope.Reference

			w := cmd.OutOrStdout()
			err := ingest.WalkSymbols(dir, ref.String(), ingest.ListOptions{
				IncludeHidden: all,
				Recursive:     recursive,
			}, func(sym ingest.SymbolInfo) bool {
				if long {
					fmt.Fprintf(w, "%s\t%d\t%d\n",
						sym.Entity.Reference, sym.Entity.StartByte, sym.Entity.EndByte)
				} else {
					fmt.Fprintln(w, sym.Reference.Symbol)
				}
				return true
			})
			return err
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "include normally hidden symbols")
	cmd.Flags().BoolVarP(&long, "long", "l", false, "use long listing format")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "list symbols recursively for directory references")

	return cmd
}
