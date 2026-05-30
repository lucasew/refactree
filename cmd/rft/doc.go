package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/ingest"
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

			dir := "."
			if ref.Provider == "path" {
				p := strings.TrimPrefix(ref.Path, "./")
				if st, err := os.Stat(p); err == nil && st.IsDir() {
					dir = p
				} else if p != "" {
					dir = filepath.Dir(p)
				}
			}
			reference = normalizeRefForIngestDir(dir, ref).String()

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
