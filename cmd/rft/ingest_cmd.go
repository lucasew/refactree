package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func newIngestCmd() *cobra.Command {
	var output string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "ingest <dir>",
		Short: "Inspect ingest model for a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := renderExpectedIngestText(args[0])
			if asJSON {
				payload, err = renderExpectedIngestJSON(args[0])
			}
			if err != nil {
				return err
			}

			if output == "" {
				_, err := cmd.OutOrStdout().Write(payload)
				return err
			}
			return os.WriteFile(output, payload, 0o644)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write output to file instead of stdout")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON instead of human-readable text")
	return cmd
}

func collectExpectedIngest(dir string) (*ingest.Result, error) {
	result, err := ingest.Ingest(dir)
	if err != nil {
		return nil, err
	}
	ingest.SortResult(result)
	return result, nil
}

func renderExpectedIngestJSON(dir string) ([]byte, error) {
	result, err := collectExpectedIngest(dir)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderExpectedIngestText(dir string) ([]byte, error) {
	result, err := collectExpectedIngest(dir)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "Files (%d):\n", len(result.Files))
	for _, f := range result.Files {
		fmt.Fprintf(&buf, "- %s [%s]\n", f.Path, f.Language)
	}
	fmt.Fprintln(&buf)

	fmt.Fprintf(&buf, "Entities (%d):\n", len(result.Entities))
	for _, e := range result.Entities {
		fmt.Fprintf(&buf, "- %s [%d:%d]\n", e.Reference, e.StartByte, e.EndByte)
	}
	fmt.Fprintln(&buf)

	fmt.Fprintf(&buf, "Aliases (%d):\n", len(result.Aliases))
	for _, a := range result.Aliases {
		fmt.Fprintf(&buf, "- %s [%d:%d] -> %s\n", a.Reference, a.StartByte, a.EndByte, a.Target)
	}
	fmt.Fprintln(&buf)

	fmt.Fprintf(&buf, "Relations (%d):\n", len(result.Relations))
	for _, r := range result.Relations {
		via := ""
		if r.ViaImportAlias {
			via = " (via import alias)"
		}
		fmt.Fprintf(&buf, "- %s [%d:%d] -> %s%s\n", r.Reference, r.StartByte, r.EndByte, r.Target, via)
	}

	return buf.Bytes(), nil
}
