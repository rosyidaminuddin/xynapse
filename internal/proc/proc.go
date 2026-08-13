// Package proc provides cross-platform process-group helpers so commands
// (and their children) can be killed together on context cancellation.
package proc

// ConfigureGroup places the command in its own process group and reports
// whether the platform supports it. When supported, KillGroup kills the
// whole tree of the command.
//
// KillGroup sends SIGKILL to the command's process group (its tree). No-op on
// platforms without process-group support; safe to call when the process has
// not started yet (Process is nil).