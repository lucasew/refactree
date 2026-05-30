package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/ingest"
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
			ref := ingest.ParseReference(args[0])
			ref = coerceLocalPathRef(ref)
			refIsDir := false

			dir := "."
			if ref.Provider == "path" {
				p := strings.TrimPrefix(ref.Path, "./")
				if st, err := os.Stat(p); err == nil && st.IsDir() {
					dir = p
					refIsDir = true
				} else if p != "" {
					dir = filepath.Dir(p)
				}
			}
			ref = normalizeRefForIngestDir(dir, ref)

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
				if refPath == "." {
					refPath = ""
				}
				entPath := strings.TrimPrefix(entRef.Path, "./")
				if !matchesPathScope(entPath, refPath, refIsDir, recursive) {
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
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "list symbols recursively for directory references")

	return cmd
}

func matchesPathScope(entPath, refPath string, refIsDir, recursive bool) bool {
	entPath = filepath.ToSlash(entPath)
	refPath = filepath.ToSlash(refPath)
	refPath = strings.TrimPrefix(refPath, "./")
	if refPath == "." {
		refPath = ""
	}

	if refIsDir {
		if recursive {
			if refPath == "" {
				return true
			}
			return entPath == refPath || strings.HasPrefix(entPath, refPath+"/")
		}
		parent := filepath.ToSlash(path.Dir(entPath))
		if parent == "." {
			parent = ""
		}
		return parent == refPath
	}

	if refPath == "" {
		return true
	}
	return entPath == refPath
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
