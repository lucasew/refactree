package git_fuzz


import (
	"context"
	"fmt"
	"os"
	execdriver "workspaced/pkg/driver/exec"
)

func SyncRepo(ctx context.Context, path string) error {
	hostname, _ := os.Hostname()
	_ = hostname
	if err := execdriver.MustRun(ctx, "git", "-C", path, "add", "-A").Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	return nil
}
