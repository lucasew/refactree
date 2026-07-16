package main

import (
	"github.com/lucasew/refactree/internal/lsp"
	"github.com/spf13/cobra"
)

func newLSPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lsp",
		Short: "Language server (stdio) for editor code intelligence",
		Long:  "Complement LSP: put after language-specific servers in Helix. stdout is protocol-only.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return lsp.RunStdio(cmd.Context())
		},
	}
}
