package main

import (
	"log/slog"
	"os"

	"github.com/lucasew/refactree/pkg/pprof"
	"github.com/lucasew/refactree/pkg/version"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	verbose  bool
	pprofDir string
}

func Execute() error {
	cmd, profiler := newRootCmd()
	// Always finalize profiles even when the command exits via error or signal
	// handling that skips cobra PostRun in some edge cases.
	defer profiler.Stop()
	return cmd.Execute()
}

func newRootCmd() (*cobra.Command, *pprof.Profiler) {
	opts := &rootOptions{}
	profiler := &pprof.Profiler{}

	cmd := &cobra.Command{
		Use:           "rft",
		Short:         "Query symbols and plan refactorings",
		Version:       version.GetBuildID(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			configureLogging(opts.verbose)
			profiler.Dir = opts.pprofDir
			return profiler.Start()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			profiler.Stop()
		},
	}

	cmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "enable debug logging (slog)")
	cmd.PersistentFlags().StringVar(&opts.pprofDir, "pprof-dir", "", "optional; when set, write cpu/heap/memory/goroutine/allocs pprof profiles into this directory")

	cmd.AddCommand(
		newASTDumpCmd(),
		newBrowseCmd(),
		newDesktopCmd(),
		newEditCmd(),
		newLsCmd(),
		newLSPCmd(),
		newMvCmd(),
		newDocCmd(),
		newIngestCmd(),
		newServeCmd(),
	)

	return cmd, profiler
}

// configureLogging sets the process slog default: Info normally, Debug with --verbose.
func configureLogging(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}
