package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/ingest"
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
			dir := "."
			if srcRef.Provider == "path" {
				p := strings.TrimPrefix(srcRef.Path, "./")
				if st, err := os.Stat(p); err == nil && st.IsDir() {
					dir = p
				} else if p != "" {
					dir = filepath.Dir(p)
				}
			}
			source = normalizeRefForIngestDir(dir, srcRef).String()
			destination = normalizeRefForIngestDir(dir, ingest.ParseReference(destination)).String()

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

func normalizeRefForIngestDir(dir string, ref ingest.Reference) ingest.Reference {
	if ref.Provider != "path" || ref.Path == "" {
		return ref
	}

	rootAbs, err := filepath.Abs(dir)
	if err != nil {
		return ref
	}

	var refAbs string
	if filepath.IsAbs(ref.Path) {
		refAbs = ref.Path
	} else {
		refAbs, err = filepath.Abs(ref.Path)
		if err != nil {
			return ref
		}
	}

	rel, err := filepath.Rel(rootAbs, refAbs)
	if err != nil {
		return ref
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ref
	}

	rel = filepath.ToSlash(rel)
	if rel == "." {
		ref.Path = "./"
		return ref
	}
	ref.Path = "./" + rel
	return ref
}
