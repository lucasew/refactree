package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newMvCmd(root *rootOptions) *cobra.Command {
	var backup bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "mv <source-reference> <destination-reference>",
		Short: "Move or rename a symbol reference",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			destination := args[1]

			srcRef := ingest.ParseReference(source)
			srcRef = coerceLocalPathRef(srcRef)
			dir, srcRef := normalizeRefForCommandScope(srcRef)
			source = srcRef.String()
			dstRef := ingest.ParseReference(destination)
			dstRef = coerceLocalPathRef(dstRef)
			destination = normalizeRefForIngestDir(dir, dstRef).String()

			edits, err := ingest.Rename(dir, source, destination)
			if err != nil {
				return err
			}

			if interactive {
				w := cmd.ErrOrStderr()
				fmt.Fprintf(w, "Edit plan (%d edits):\n", len(edits))
				for _, e := range edits {
					fmt.Fprintf(w, "  %s [%d:%d] → %q\n", e.File, e.StartByte, e.EndByte, e.NewText)
				}
				fmt.Fprint(w, "Apply? [y/N] ")
				var answer string
				fmt.Fscan(cmd.InOrStdin(), &answer)
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
		in.Close()
		out.Close()
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
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte{}, 0644); err != nil {
			return err
		}
	}
	return nil
}
