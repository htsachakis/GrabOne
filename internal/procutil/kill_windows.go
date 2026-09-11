//go:build windows

package procutil

import (
	"os/exec"
	"strconv"
)

// KillTree terminates a process and everything it started. yt-dlp launches
// FFmpeg for merging and conversion, and a cancelled download must not leave
// those running.
func KillTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid

	// taskkill walks the process tree, which os.Process.Kill cannot do.
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	Configure(kill)
	if err := kill.Run(); err != nil {
		// Fall back to terminating just the process we started.
		return cmd.Process.Kill()
	}
	return nil
}
