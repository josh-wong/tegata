package audit

import (
	"context"
	"os/exec"
)

func newCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	configureCommandForPlatform(cmd)
	return cmd
}

func newCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	configureCommandForPlatform(cmd)
	return cmd
}
