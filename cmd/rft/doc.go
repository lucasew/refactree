package main

import (
	"fmt"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newDocCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doc <reference>",
		Short: "Show documentation for a reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := ingest.ResolveInputReferenceScope(".", args[0])
			reference := scope.Reference.String()

			doc, err := ingest.DocFor(scope.Dir, reference)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			if _, err := fmt.Fprintf(w, "# %s\n", doc.Name); err != nil {
				return err
			}
			if doc.Signature != "" {
				if _, err := fmt.Fprintf(w, "Signature: %s\n", doc.Signature); err != nil {
					return err
				}
			}
			if doc.DocString != "" {
				if _, err := fmt.Fprintln(w, doc.DocString); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
