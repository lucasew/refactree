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
	catalog    string
	projects   []string
	seed       int64
	iterations int
	workRoot   string
	reportDir  string
	allow      bool
	noIsolate  bool
	offline    bool
	strictRefs bool
	failFast   bool
	ops        []string
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
	addFuzz := func(c *cobra.Command) {
		addCommon(c)
		c.Flags().BoolVar(&flags.offline, "offline", false, "use work-root caches only (no git fetch/clone; container network disabled). Run prefetch online first")
		c.Flags().Int64Var(&flags.seed, "seed", 0, "rng seed (default: time-based)")
		c.Flags().IntVar(&flags.iterations, "iterations", 1, "mv iterations per project")
		c.Flags().BoolVar(&flags.strictRefs, "strict-refs", false, "fail on dangling path targets")
		c.Flags().StringSliceVar(&flags.ops, "ops", nil, "mv ops subset: rename,cross_file,package")
	}

	run := func(mode fuzzy.Mode) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			opts := fuzzy.Options{
				CatalogPath: flags.catalog,
				ProjectIDs:  flags.projects,
				Mode:        mode,
				Seed:        flags.seed,
				Iterations:  flags.iterations,
				WorkRoot:    flags.workRoot,
				ReportDir:   flags.reportDir,
				Allow:       flags.allow,
				NoIsolate:   flags.noIsolate,
				Offline:     flags.offline,
				StrictRefs:  flags.strictRefs,
				FailFast:    flags.failFast,
				Verbose:     root != nil && root.verbose,
				Ops:         flags.ops,
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
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
		Short: "Clone catalog repos into work-root and run setup (for later --offline runs)",
		Long:  "Downloads git pins into --work-root/cache, runs mise install + setup (Docker by default), and saves preserve_globs snapshots. Use the same --work-root with ingest/mv/run --offline on an airgapped host. Pass --no-isolate to run setup on the host instead of Docker.",
		RunE:  run(fuzzy.ModePrefetch),
	}
	addCommon(prefetchCmd)

	ingestCmd := &cobra.Command{
		Use:   "ingest",
		Short: "Run ingest invariant checks on catalog projects",
		RunE:  run(fuzzy.ModeIngest),
	}
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
	for _, c := range []*cobra.Command{ingestCmd, mvCmd, runCmd} {
		addFuzz(c)
		cmd.AddCommand(c)
	}
	cmd.AddCommand(prefetchCmd)

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		for _, op := range flags.ops {
			switch strings.TrimSpace(op) {
			case "", "rename", "cross_file", "package":
			default:
				return fmt.Errorf("unknown op %q", op)
			}
		}
		return nil
	}

	return cmd
}
