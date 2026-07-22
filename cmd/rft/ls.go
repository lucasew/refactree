package main

import (
	"fmt"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
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
			var writeErr error
			err := ingest.WalkAtoms(dir, ref.String(), ingest.ListOptions{
				IncludeHidden: all,
				Recursive:     recursive,
			}, func(sym ingest.AtomInfo) bool {
				if long {
					_, writeErr = fmt.Fprintf(w, "%s\t%d\t%d\n",
						sym.Atom.Reference, sym.Atom.StartByte, sym.Atom.EndByte)
				} else {
					_, writeErr = fmt.Fprintln(w, sym.Reference.Name)
				}
				return writeErr == nil
			})
			if err != nil {
				return err
			}
			return writeErr
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "include normally hidden symbols")
	cmd.Flags().BoolVarP(&long, "long", "l", false, "use long listing format")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "list symbols recursively for directory references")

	return cmd
}
