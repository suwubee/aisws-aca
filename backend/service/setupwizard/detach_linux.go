//go:build linux

package setupwizard

import (
	"os/exec"
	"syscall"
)

func applyDetachPlatform(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

