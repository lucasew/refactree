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
Each rule is a structural pattern (same dialect as grep/rewrite). Optional
replacement makes the finding fixable (SARIF result.fixes; --fix applies).

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
			cfgPath, err := lint.ResolveConfigPath(root, configPath)
			if err != nil {
				return errExit{code: 2, err: err}
			}
			_, rules, err := lint.LoadFile(cfgPath)
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

			// When --fix (and not dry-run only for write): apply first, then re-run
			// is expensive; SPEC says exit 1 if any finding remains after apply.
			// Report-only findings always remain. Conflict-skipped stay until next run.
			// For exit: after successful fix apply, findings that were fully fixed
			// (fixable and applied) should not count. Simplest accurate approach:
			// apply, then re-run match to compute remaining — expensive but correct.
			// Lighter: remaining = report-only + fix-skipped + (fixable not applied when !fix).
			// With --fix: apply ApplyEdits; remaining = report-only + FixSkipped (+ apply failure).
			// Without re-match after fix, we won't see if fix actually cleared the pattern.
			// Re-run after --fix is the truth for exit 0 "tree clean".

			out := cmd.OutOrStdout()
			applyNow := fix && !dryRun
			if dryRun && fix {
				// Show planned apply edits via applyEditPlan dry-run style.
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
				// Re-scan so exit reflects post-fix tree.
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
	cmd.Flags().StringVar(&configPath, "config", "", "path to refactree.yaml (default: walk up from -C)")
	cmd.Flags().StringVarP(&lang, "lang", "l", "", "language or family filter for files")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or sarif")
	cmd.Flags().BoolVar(&fix, "fix", false, "apply non-conflicting fixes from rules with replacement")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "with --fix: show planned edits without writing")
	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "create .bak files before writing (--fix)")
	return cmd
}
