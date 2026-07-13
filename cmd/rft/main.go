package main

import (
	"errors"
	"fmt"
	"os"
)

type exitCoder interface {
	ExitCode() int
}

func main() {
	if err := Execute(); err != nil {
		code := 1
		var ec exitCoder
		if errors.As(err, &ec) {
			code = ec.ExitCode()
		}
		if msg := err.Error(); msg != "" {
			if _, printErr := fmt.Fprintln(os.Stderr, err); printErr != nil {
				os.Exit(1)
			}
		}
		os.Exit(code)
	}
}
