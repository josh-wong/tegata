//go:build !windows

package audit

import "os/exec"

func configureCommandForPlatform(_ *exec.Cmd) {}
