package util

import (
	"context"
	execdriver "example.com/m/pkg/driver/exec"
)

func Ensure(ctx context.Context) error {
	return install(ctx)
}
