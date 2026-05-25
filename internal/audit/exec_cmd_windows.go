//go:build windows

package audit

import (
	"os/exec"
	"syscall"
)

func configureCommandForPlatform(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
