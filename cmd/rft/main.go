package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Execute(); err != nil {
		if _, err := fmt.Fprintln(os.Stderr, err); err != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}
