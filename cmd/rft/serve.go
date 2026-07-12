package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/lucasew/refactree/pkg/web"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var (
		addr string
		dir  string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "HTTP code browser with symbol hyperlinks",
		Long: `Serve a local web UI that renders source with clickable symbols.

Each symbol usage links to /code/<reference>#anchor using the same
provider:path::symbol path system as the CLI.

Listens on loopback by default (127.0.0.1:8080). Use --addr to bind
elsewhere. Does not open a browser.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := web.New(web.Options{RootDir: dir})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "refactree serve  root=%s  addr=%s  %s\n", dir, addr, listenURL(addr))
			return srv.ListenAndServe(addr)
		},
	}

	cmd.Flags().StringVarP(&addr, "addr", "l", "127.0.0.1:8080", "listen address (default loopback only)")
	cmd.Flags().StringVarP(&dir, "dir", "C", ".", "project root to browse")
	return cmd
}

// listenURL builds a browser-oriented URL from the listen address.
// Port-only forms (":8080") and wildcard hosts use localhost so the link is openable;
// explicit hosts (including 127.0.0.1) are kept as given.
func listenURL(addr string) string {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if strings.Contains(addr, "://") {
		if strings.HasSuffix(addr, "/") {
			return addr
		}
		return addr + "/"
	}
	host, port, err := splitHostPort(addr)
	if err != nil {
		return "http://" + addr + "/"
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return "http://" + host + ":" + port + "/"
}

func splitHostPort(addr string) (host, port string, err error) {
	// net.SplitHostPort rejects bare ":8080" on some paths; handle explicitly.
	if strings.HasPrefix(addr, ":") && !strings.HasPrefix(addr, "::") {
		return "", strings.TrimPrefix(addr, ":"), nil
	}
	return net.SplitHostPort(addr)
}
