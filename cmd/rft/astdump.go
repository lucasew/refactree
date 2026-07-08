package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	"github.com/spf13/cobra"
)

func newASTDumpCmd(root *rootOptions) *cobra.Command {
	var querySource string
	var queryFile string

	cmd := &cobra.Command{
		Use:   "astdump <language> <file>",
		Short: "Dump the tree-sitter AST or query matches for a file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runASTDump(args[0], args[1], querySource, queryFile, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(&querySource, "query", "", "tree-sitter query source")
	cmd.Flags().StringVar(&queryFile, "query-file", "", "path to a file containing a tree-sitter query")

	return cmd
}

func runASTDump(languageName, filename, querySource, queryFile string, output io.Writer) error {
	if querySource != "" && queryFile != "" {
		return fmt.Errorf("use either --query or --query-file, not both")
	}

	source, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	lang, ok := grammar.Get(strings.ToLower(languageName))
	if !ok {
		return fmt.Errorf("unsupported language: %s", languageName)
	}

	parser := grammar.NewParser()
	defer parser.Delete()

	if !parser.SetLanguage(lang) {
		return fmt.Errorf("failed to set language")
	}

	tree := parser.ParseString(string(source))
	defer tree.Delete()

	query := querySource
	if queryFile != "" {
		queryBytes, err := os.ReadFile(queryFile)
		if err != nil {
			return fmt.Errorf("error reading query file: %w", err)
		}
		query = string(queryBytes)
	}

	root := tree.RootNode()
	enc := json.NewEncoder(output)

	if query != "" {
		compiledQuery, err := grammar.NewQuery(lang, query)
		if err != nil {
			return err
		}
		defer compiledQuery.Delete()

		for _, match := range compiledQuery.ExecuteMatches(root, source) {
			if err := enc.Encode(match); err != nil {
				return fmt.Errorf("failed to encode match output: %w", err)
			}
		}
		return nil
	}

	enc.SetIndent("", "  ")
	if err := enc.Encode(grammar.ParseOutput{
		Language: strings.ToLower(languageName),
		File:     filename,
		Root:     grammar.BuildParseNode(root, source, ""),
	}); err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}

	return nil
}
