//go:build !windows

package procutil

import (
	"os/exec"
	"syscall"
)

// KillTree terminates a process and everything it started, so a cancelled
// download does not leave a post-processing tool behind.
func KillTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// A negative pid signals the whole process group created by Configure.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
