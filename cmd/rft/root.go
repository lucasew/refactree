package main

import (
	"github.com/lucasew/refactree/pkg/pprof"
	"github.com/lucasew/refactree/pkg/version"
	"github.com/spf13/cobra"
)

type rootOptions struct {
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
		// Profiling runs only when --pprof-dir is set; hooks are no-ops otherwise.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			profiler.Dir = opts.pprofDir
			return profiler.Start()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			profiler.Stop()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.pprofDir, "pprof-dir", "", "optional; when set, write cpu/heap/memory/goroutine/allocs pprof profiles into this directory")

	cmd.AddCommand(
		newASTDumpCmd(),
		newBrowseCmd(),
		newLsCmd(),
		newMvCmd(),
		newDocCmd(),
		newIngestCmd(),
		newServeCmd(),
	)

	return cmd, profiler
}
