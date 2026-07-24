package util

import (
	"context"
	execdriver "example.com/m/pkg/driver/exec"
)

func install(ctx context.Context) error {
	cmd, err := execdriver.Run(ctx, "bash", "-s")
	if err != nil {
		return err
	}
	return cmd.Run()
}

func Ensure(ctx context.Context) error {
	return install(ctx)
}
