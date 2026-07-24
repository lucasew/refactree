package git

import (
	"context"

	execdriver "workspaced/pkg/driver/exec"
)

func Other(ctx context.Context, path string) error {
	return execdriver.MustRun(ctx, "git", "-C", path, "status").Run()
}
