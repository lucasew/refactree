package main

import (
	"github.com/lewtec/eletrocromo"
	"github.com/lucasew/refactree/pkg/web"
	"github.com/spf13/cobra"
)

// eletrocromoAppID is the reverse-domain Helium profile for refactree desktop.
const eletrocromoAppID = "br.tec.lew.refactree"

func newDesktopCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Code browser in a Helium window (eletrocromo)",
		Long: `Open the same local web UI as "rft serve" inside a Helium
--app window via eletrocromo. The library owns loopback bind and token auth;
there is no --addr flag.

Project root is -C/--dir (default .), same as serve — no project-marker walk-up.
Fails hard if Helium/eletrocromo cannot start; never falls back to a system
browser or to headless serve.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := web.New(web.Options{RootDir: dir})
			if err != nil {
				return err
			}
			app := eletrocromo.App{
				ID:      eletrocromoAppID,
				Handler: srv.Handler(),
				Context: cmd.Context(),
			}
			return app.Run()
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "C", ".", "project root to browse")
	return cmd
}
