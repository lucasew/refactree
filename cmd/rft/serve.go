package main

import (
	"fmt"

	"github.com/lucasew/refactree/pkg/web"
	"github.com/spf13/cobra"
)

func newServeCmd(root *rootOptions) *cobra.Command {
	var (
		addr string
		dir  string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "HTTP code browser with symbol hyperlinks",
		Long: `Serve a local web UI that renders source with clickable symbols.

Each symbol usage links to /code/<reference>#anchor using the same
provider:path::symbol path system as the CLI.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := web.New(web.Options{RootDir: dir})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "refactree serve  root=%s  http://localhost%s/\n", dir, normalizeAddr(addr))
			return srv.ListenAndServe(addr)
		},
	}

	cmd.Flags().StringVarP(&addr, "addr", "l", ":8080", "listen address")
	cmd.Flags().StringVarP(&dir, "dir", "C", ".", "project root to browse")
	return cmd
}

func normalizeAddr(addr string) string {
	if addr == "" {
		return ":8080"
	}
	if addr[0] == ':' {
		return addr
	}
	return addr
}
