package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/ingest"
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
			ref := ingest.ParseReference(args[0])

			dir := "."
			if ref.Provider == "path" {
				p := strings.TrimPrefix(ref.Path, "./")
				if st, err := os.Stat(p); err == nil && st.IsDir() {
					dir = p
				} else if p != "" {
					dir = filepath.Dir(p)
				}
			}

			result, err := ingest.Ingest(dir)
			if err != nil {
				return err
			}

			// Build language lookup for visibility checks.
			langOf := map[string]string{}
			for _, f := range result.Files {
				langOf[f.Path] = f.Language
			}

			w := cmd.OutOrStdout()
			for _, ent := range result.Entities {
				entRef := ingest.ParseReference(ent.Reference)

				// Filter by reference scope.
				if ref.Symbol != "" && entRef.Symbol != ref.Symbol {
					continue
				}
				refPath := strings.TrimPrefix(ref.Path, "./")
				entPath := strings.TrimPrefix(entRef.Path, "./")
				if refPath != "" && !strings.HasPrefix(entPath, refPath) {
					continue
				}

				// Visibility filter (skip hidden unless -a).
				if !all && isHidden(entRef.Symbol, langOf[entPath]) {
					continue
				}

				if long {
					fmt.Fprintf(w, "%s\t%d\t%d\n",
						ent.Reference, ent.StartByte, ent.EndByte)
				} else {
					fmt.Fprintln(w, entRef.Symbol)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "include normally hidden symbols")
	cmd.Flags().BoolVarP(&long, "long", "l", false, "use long listing format")

	return cmd
}

func isHidden(name, language string) bool {
	switch language {
	case "go":
		return len(name) > 0 && name[0] >= 'a' && name[0] <= 'z'
	case "python":
		return strings.HasPrefix(name, "_")
	}
	return false
}
