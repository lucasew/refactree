package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

// applyEditPlanOptions controls plan preview and write behavior for a full
// edit list (mv; rewrite -i/-n path). Stream apply reuses backup/ensure only.
type applyEditPlanOptions struct {
	Interactive bool
	DryRun      bool
	Backup      bool
}

// applyEditPlan prints the plan when Interactive or DryRun, optionally asks
// for confirmation, then backups / creates missing files / ApplyEdits.
// DryRun never writes. Empty edits is a no-op success (callers may error first).
func applyEditPlan(cmd *cobra.Command, root string, edits []ingest.Edit, opts applyEditPlanOptions) error {
	return applyMovePlan(cmd, root, ingest.Plan{Edits: edits}, opts)
}

// applyMovePlan is applyEditPlan for a full Rename plan (DirMoves + Edits).
func applyMovePlan(cmd *cobra.Command, root string, plan ingest.Plan, opts applyEditPlanOptions) error {
	if plan.Empty() {
		slog.Debug("apply plan: empty")
		return nil
	}
	slog.Debug("apply plan",
		"root", root,
		"dir_moves", len(plan.DirMoves),
		"edits", len(plan.Edits),
		"files", editFileCount(plan.Edits),
		"interactive", opts.Interactive,
		"dry_run", opts.DryRun,
		"backup", opts.Backup,
	)
	if opts.Interactive || opts.DryRun {
		w := cmd.ErrOrStderr()
		if err := printMovePlan(w, plan); err != nil {
			return err
		}
		if opts.DryRun {
			slog.Debug("apply plan: dry-run, skip write")
			return nil
		}
		ok, err := confirmApply(w, cmd.InOrStdin())
		if err != nil {
			return err
		}
		if !ok {
			slog.Debug("apply plan: cancelled by user")
			return fmt.Errorf("cancelled")
		}
	}
	return writeMovePlan(root, plan, opts.Backup)
}

func printMovePlan(w io.Writer, plan ingest.Plan) error {
	if len(plan.DirMoves) > 0 {
		if _, err := fmt.Fprintf(w, "Dir moves (%d):\n", len(plan.DirMoves)); err != nil {
			return err
		}
		for _, m := range plan.DirMoves {
			if _, err := fmt.Fprintf(w, "  %s → %s\n", m.From, m.To); err != nil {
				return err
			}
		}
	}
	return printEditPlan(w, plan.Edits)
}

// writeMovePlan optionally backups text-edit files, ensures create targets, then ApplyPlan.
// DirMoves run before ensureEditFiles so post-rename paths are not pre-created
// as empty files (that would block os.Rename of the package tree).
func writeMovePlan(root string, plan ingest.Plan, backup bool) error {
	slog.Debug("write plan", "root", root, "dir_moves", len(plan.DirMoves), "edits", len(plan.Edits), "backup", backup)
	if backup {
		if err := createBackups(root, plan.Edits); err != nil {
			return err
		}
	}
	// Apply directory renames first; then ensure/create only remaining edit targets.
	if err := ingest.ApplyPlan(root, ingest.Plan{DirMoves: plan.DirMoves}); err != nil {
		return err
	}
	if err := ensureEditFiles(root, plan.Edits); err != nil {
		return err
	}
	if err := ingest.ApplyEdits(root, plan.Edits); err != nil {
		return err
	}
	slog.Debug("write plan: done")
	return nil
}

// writeEdits optionally backups, ensures files exist, then ApplyEdits.
func writeEdits(root string, edits []ingest.Edit, backup bool) error {
	slog.Debug("write edits", "root", root, "edits", len(edits), "backup", backup)
	if backup {
		if err := createBackups(root, edits); err != nil {
			return err
		}
	}
	if err := ensureEditFiles(root, edits); err != nil {
		return err
	}
	if err := ingest.ApplyEdits(root, edits); err != nil {
		return err
	}
	slog.Debug("write edits: done")
	return nil
}

func editFileCount(edits []ingest.Edit) int {
	seen := map[string]bool{}
	for _, e := range edits {
		seen[e.File] = true
	}
	return len(seen)
}

func printEditPlan(w io.Writer, edits []ingest.Edit) error {
	if _, err := fmt.Fprintf(w, "Edit plan (%d edits):\n", len(edits)); err != nil {
		return err
	}
	for _, e := range edits {
		if _, err := fmt.Fprintf(w, "  %s [%d:%d] → %q\n", e.File, e.StartByte, e.EndByte, e.NewText); err != nil {
			return err
		}
	}
	return nil
}

func confirmApply(w io.Writer, r io.Reader) (bool, error) {
	if _, err := fmt.Fprint(w, "Apply? [y/N] "); err != nil {
		return false, err
	}
	var answer string
	if _, err := fmt.Fscan(r, &answer); err != nil {
		return false, err
	}
	return answer == "y" || answer == "Y", nil
}

func createBackups(dir string, edits []ingest.Edit) error {
	seen := map[string]bool{}
	for _, e := range edits {
		if seen[e.File] {
			continue
		}
		seen[e.File] = true
		src := filepath.Join(dir, e.File)
		dst := src + ".bak"
		in, err := os.Open(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if cerr := in.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureEditFiles(dir string, edits []ingest.Edit) error {
	seen := map[string]bool{}
	for _, e := range edits {
		if seen[e.File] {
			continue
		}
		seen[e.File] = true

		p := filepath.Join(dir, e.File)
		_, err := os.Stat(p)
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
			return err
		}
	}
	return nil
}
