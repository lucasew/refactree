package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newMvCmd() *cobra.Command {
	var backup bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "mv <source-reference> <destination-reference>",
		Short: "Move or rename a symbol reference",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Project root is cwd. Do not narrow to the source file's parent —
			// that rebased cross-dir destinations under the source package.
			dir, source, destination := ingest.ResolveMoveArgs(".", args[0], args[1])

			edits, err := ingest.Rename(dir, source, destination)
			if err != nil {
				return err
			}

			if interactive {
				w := cmd.ErrOrStderr()
				if _, err := fmt.Fprintf(w, "Edit plan (%d edits):\n", len(edits)); err != nil {
					return err
				}
				for _, e := range edits {
					if _, err := fmt.Fprintf(w, "  %s [%d:%d] → %q\n", e.File, e.StartByte, e.EndByte, e.NewText); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprint(w, "Apply? [y/N] "); err != nil {
					return err
				}
				var answer string
				if _, err := fmt.Fscan(cmd.InOrStdin(), &answer); err != nil {
					return err
				}
				if answer != "y" && answer != "Y" {
					return fmt.Errorf("cancelled")
				}
			}

			if backup {
				if err := createBackups(dir, edits); err != nil {
					return err
				}
			}

			if err := ensureEditFiles(dir, edits); err != nil {
				return err
			}

			return ingest.ApplyEdits(dir, edits)
		},
	}

	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "create .bak files before writing")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "show edit plan and ask for confirmation")

	return cmd
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
