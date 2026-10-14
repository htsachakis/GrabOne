//go:build !windows

package system

import (
	"fmt"
	"os/exec"
)

// RunInTerminal is not available on this platform: the update commands GrabOne
// offers are PowerShell commands for Windows.
func RunInTerminal(command string) (*exec.Cmd, error) {
	_ = command
	return nil, fmt.Errorf("running a command in a terminal is only supported on Windows")
}
