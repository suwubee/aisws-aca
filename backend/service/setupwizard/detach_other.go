//go:build !linux

package setupwizard

import "os/exec"

func applyDetachPlatform(cmd *exec.Cmd) {
	// No-op on non-Linux platforms.
	_ = cmd
}

