package main

import (
	"fmt"
	"io"
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
	if len(edits) == 0 {
		return nil
	}
	if opts.Interactive || opts.DryRun {
		w := cmd.ErrOrStderr()
		if err := printEditPlan(w, edits); err != nil {
			return err
		}
		if opts.DryRun {
			return nil
		}
		ok, err := confirmApply(w, cmd.InOrStdin())
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("cancelled")
		}
	}
	return writeEdits(root, edits, opts.Backup)
}

// writeEdits optionally backups, ensures files exist, then ApplyEdits.
func writeEdits(root string, edits []ingest.Edit, backup bool) error {
	if backup {
		if err := createBackups(root, edits); err != nil {
			return err
		}
	}
	if err := ensureEditFiles(root, edits); err != nil {
		return err
	}
	return ingest.ApplyEdits(root, edits)
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
