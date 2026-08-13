//go:build !darwin && !linux

package proc

import "os/exec"

func ConfigureGroup(_ *exec.Cmd) bool { return false }

func KillGroup(_ *exec.Cmd) {}