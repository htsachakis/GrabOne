//go:build !windows

package procutil

import (
	"os/exec"
	"syscall"
)

// Configure prepares a background tool for launching from a GUI application.
// Placing the child in its own process group means cancelling a download can
// also stop any post-processing tool it started.
func Configure(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// ConfigureVisible prepares a command whose window the user is meant to see.
// On this platform nothing special is needed, and the child is deliberately not
// placed in its own process group: it outlives the application.
func ConfigureVisible(cmd *exec.Cmd) {
	_ = cmd
}

// SetCommandLine has no effect on this platform: argument lists are passed to
// the kernel as a list, so there is no command line string to arrange.
func SetCommandLine(cmd *exec.Cmd, commandLine string) {
	_, _ = cmd, commandLine
}
