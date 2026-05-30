package main

import (
	"fmt"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newDocCmd(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doc <reference>",
		Short: "Show documentation for a reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reference := args[0]
			ref := ingest.ParseReference(reference)
			ref = coerceLocalPathRef(ref)
			dir, ref := normalizeRefForCommandScope(ref)
			reference = ref.String()

			doc, err := ingest.DocFor(dir, reference)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "# %s\n", doc.Name)
			if doc.Signature != "" {
				fmt.Fprintf(w, "Signature: %s\n", doc.Signature)
			}
			if doc.DocString != "" {
				fmt.Fprintln(w, doc.DocString)
			}

			return nil
		},
	}
}
