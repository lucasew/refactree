package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/lint"
	"github.com/spf13/cobra"
)

func newLintCmd() *cobra.Command {
	var dir string
	var configPath string
	var lang string
	var format string
	var fix bool
	var dryRun bool
	var backup bool

	cmd := &cobra.Command{
		Use:   "lint [paths...]",
		Short: "Run the refactree.yaml codemod rulebook",
		Long: `Run all rules from refactree.yaml (project codemod catalog).

Config discovery: walk up from -C for refactree.yaml, or pass --config.
If no config is found, a built-in default catalog runs (currently unused
named imports via ImportHygiene).

Rules are structural patterns (same dialect as grep/rewrite) or builtins
(e.g. builtin: dead-imports). Optional replacement / builtin fixes make
findings fixable (SARIF result.fixes; --fix applies).

  rft lint
  rft lint --format sarif
  rft lint --fix
  rft lint -n --fix

Exit 1 if any finding remains (including report-only). Exit 0 when clean.
Tool/config errors use a non-zero exit with a message on stderr.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "text"
			}
			switch format {
			case "text", "sarif":
			default:
				return errExit{code: 2, err: fmt.Errorf("unknown format %q (want text or sarif)", format)}
			}

			root, err := filepath.Abs(dir)
			if err != nil {
				return errExit{code: 2, err: err}
			}
			_, rules, _, err := lint.LoadCatalog(root, configPath)
			if err != nil {
				return errExit{code: 2, err: err}
			}

			res, err := lint.Run(root, rules, lint.Options{
				Paths:      args,
				LangFilter: lang,
			})
			if err != nil {
				return errExit{code: 2, err: err}
			}

			out := cmd.OutOrStdout()
			applyNow := fix && !dryRun
			if dryRun && fix {
				if len(res.ApplyEdits) > 0 {
					if err := applyEditPlan(cmd, root, res.ApplyEdits, applyEditPlanOptions{
						DryRun: true,
					}); err != nil {
						return errExit{code: 2, err: err}
					}
				}
			}
			if applyNow && len(res.ApplyEdits) > 0 {
				if err := applyEditPlan(cmd, root, res.ApplyEdits, applyEditPlanOptions{
					Backup: backup,
				}); err != nil {
					return errExit{code: 2, err: err}
				}
				res, err = lint.Run(root, rules, lint.Options{
					Paths:      args,
					LangFilter: lang,
				})
				if err != nil {
					return errExit{code: 2, err: err}
				}
			}

			switch format {
			case "sarif":
				if err := lint.WriteSARIF(out, root, res); err != nil {
					return errExit{code: 2, err: err}
				}
			default:
				if err := lint.WriteText(out, res); err != nil {
					return errExit{code: 2, err: err}
				}
			}

			if len(res.Findings) > 0 {
				return errExit{code: 1, err: fmt.Errorf("")}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "C", ".", "project root")
	cmd.Flags().StringVar(&configPath, "config", "", "path to refactree.yaml (default: walk up from -C; built-in defaults if missing)")
	cmd.Flags().StringVarP(&lang, "lang", "l", "", "language or family filter for files")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or sarif")
	cmd.Flags().BoolVar(&fix, "fix", false, "apply non-conflicting fixes from rules with replacement")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "with --fix: show planned edits without writing")
	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "create .bak files before writing (--fix)")
	return cmd
}
