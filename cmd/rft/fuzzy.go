package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lucasew/refactree/internal/fuzzy"
	"github.com/spf13/cobra"
)

type fuzzyFlags struct {
	catalog         string
	projects        []string
	seed            int64
	iterations      int
	workRoot        string
	reportDir       string
	allow           bool
	noIsolate       bool
	offline         bool
	noVerifyOffline bool
	strictRefs      bool
	failFast        bool
	ops             []string
}

func newFuzzyCmd(root *rootOptions) *cobra.Command {
	flags := &fuzzyFlags{}

	cmd := &cobra.Command{
		Use:   "fuzzy",
		Short: "Real-world ingest/mv fuzzy harness on isolated workspaces",
	}

	addCommon := func(c *cobra.Command) {
		c.Flags().StringVar(&flags.catalog, "catalog", "", "path to projects.toml (default: testdata/fuzzy/projects.toml)")
		c.Flags().StringSliceVar(&flags.projects, "project", nil, "project slug (repeatable)")
		c.Flags().StringVar(&flags.workRoot, "work-root", os.Getenv("RFT_FUZZY_WORK_ROOT"), "workspace root for clones, mise-data, and preserve snapshots")
		c.Flags().StringVar(&flags.reportDir, "report-dir", "", "directory for reports")
		c.Flags().BoolVar(&flags.allow, "allow", false, "allow --no-isolate on a non-ephemeral host")
		c.Flags().BoolVar(&flags.noIsolate, "no-isolate", false, "opt out of Docker isolation; run setup/check on the host (Docker is the default)")
		c.Flags().BoolVar(&flags.failFast, "fail-fast", false, "stop on first bug-class failure")
	}
	addOffline := func(c *cobra.Command) {
		c.Flags().BoolVar(&flags.offline, "offline", false, "use work-root caches + local docker images only (no git fetch/pull; container network=none; package managers offline). Run prefetch online first")
	}
	addIngestFlags := func(c *cobra.Command) {
		addCommon(c)
		addOffline(c)
		c.Flags().BoolVar(&flags.strictRefs, "strict-refs", false, "fail on dangling path targets")
	}
	addMvFlags := func(c *cobra.Command) {
		addIngestFlags(c)
		c.Flags().Int64Var(&flags.seed, "seed", 0, "rng seed (default: time-based); also names the per-run worktree")
		c.Flags().IntVar(&flags.iterations, "iterations", 1, "mv iterations per project")
		c.Flags().StringSliceVar(&flags.ops, "ops", nil, "mv ops subset: rename,cross_file,package")
	}

	run := func(mode fuzzy.Mode) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			opts := fuzzy.Options{
				CatalogPath:     flags.catalog,
				ProjectIDs:      flags.projects,
				Mode:            mode,
				Seed:            flags.seed,
				Iterations:      flags.iterations,
				WorkRoot:        flags.workRoot,
				ReportDir:       flags.reportDir,
				Allow:           flags.allow,
				NoIsolate:       flags.noIsolate,
				Offline:         flags.offline,
				NoVerifyOffline: flags.noVerifyOffline,
				StrictRefs:      flags.strictRefs,
				FailFast:        flags.failFast,
				Verbose:         root != nil && root.verbose,
				Ops:             flags.ops,
				Stdout:          cmd.OutOrStdout(),
				Stderr:          cmd.ErrOrStderr(),
			}
			res, err := fuzzy.Run(context.Background(), opts)
			if res != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "summary: passed=%d bugs=%d unsupported=%d env_fails=%d report=%s\n",
					res.Passed, res.BugCount, res.Unsupported, res.EnvFails, res.ReportDir)
			}
			return err
		}
	}

	prefetchCmd := &cobra.Command{
		Use:   "prefetch",
		Short: "Fill work-root and pull docker images for later --offline runs",
		Long: strings.TrimSpace(`
Online-only pack step for airgapped fuzzy runs.

Fills --work-root with git bare caches, mise-data tool/package caches, preserve_globs
snapshots, and manifest.json. With Docker isolation (default), docker pull ensures
pinned session/cleanup images are on the local daemon (images are not stored in
work-root). After all projects succeed, writes the manifest and verifies offline
readiness (disable with --no-verify-offline).

Then unplug the network (or stay offline) and run:
  rft fuzzy run --work-root <same> --offline

Without Docker: pass --no-isolate --allow (or RFT_FUZZY_ALLOW=1). Host setup/check
use the same work-root/mise-data caches.
`),
		RunE: run(fuzzy.ModePrefetch),
	}
	addCommon(prefetchCmd)
	prefetchCmd.Flags().BoolVar(&flags.noVerifyOffline, "no-verify-offline", false, "skip post-prefetch offline readiness check")

	ingestCmd := &cobra.Command{
		Use:   "ingest",
		Short: "Run ingest invariant checks on catalog projects",
		RunE:  run(fuzzy.ModeIngest),
	}
	addIngestFlags(ingestCmd)

	mvCmd := &cobra.Command{
		Use:   "mv",
		Short: "Fuzz mv operations with isolated project checks",
		RunE:  run(fuzzy.ModeMv),
	}
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run ingest then mv fuzzing",
		RunE:  run(fuzzy.ModeRun),
	}
	for _, c := range []*cobra.Command{mvCmd, runCmd} {
		addMvFlags(c)
		c.PreRunE = func(cmd *cobra.Command, args []string) error {
			for _, op := range flags.ops {
				switch strings.TrimSpace(op) {
				case "", "rename", "cross_file", "package":
				default:
					return fmt.Errorf("unknown op %q", op)
				}
			}
			return nil
		}
	}
	cmd.AddCommand(ingestCmd, mvCmd, runCmd, prefetchCmd)

	return cmd
}
