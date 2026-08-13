//go:build darwin || linux

package proc

import (
	"os/exec"
	"syscall"
)

func ConfigureGroup(cmd *exec.Cmd) bool {
	if cmd.Process != nil {
		return false // already started
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return true
}

func KillGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}