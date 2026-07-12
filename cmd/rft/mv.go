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
			source := args[0]
			destination := args[1]

			srcScope := ingest.ResolveInputReferenceScope(".", source)
			dir := srcScope.Dir
			source = srcScope.Reference.String()
			dstRef := ingest.CoerceLocalPathReference(".", ingest.ParseReference(destination))
			destination = ingest.NormalizeReferenceForScope(".", dir, dstRef).String()

			// If the source was a bare directory (no ::symbol), ResolveInputReferenceScope
			// + NormalizeReferenceForScope collapses the source ref to Path="./" (scope root).
			// This makes planPackageMove see an empty srcDir after TrimPrefix and refuse with
			// "package move requires non-empty directory paths".
			// When this happens for a package move (both sides end up without symbol), adjust
			// so that the ingest root becomes the parent directory and the refs keep the
			// sub-directory segment (e.g. source becomes "path:./ingest", dir becomes "pkg").
			// This lets package moves via bare dir paths on the CLI work while preserving the
			// original scope logic for symbol operations.
			srcParsed := ingest.ParseReference(source)
			dstParsed := ingest.ParseReference(destination)
			if srcParsed.Symbol == "" && dstParsed.Symbol == "" && (srcParsed.Path == "./" || srcParsed.Path == ".") {
				parent := filepath.Dir(dir)
				if parent == "" || parent == "." || parent == dir {
					parent = "."
				}
				base := filepath.Base(dir)
				if base != "" && base != "." {
					source = "path:./" + base
				} else {
					source = "path:./"
				}
				destination = ingest.NormalizeReferenceForScope(".", parent, dstRef).String()
				dir = parent
			}

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
