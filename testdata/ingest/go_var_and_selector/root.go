package main

import "github.com/spf13/cobra"

var Registry int

func GetCommand() *cobra.Command {
	_ = Registry
	return &cobra.Command{}
}
