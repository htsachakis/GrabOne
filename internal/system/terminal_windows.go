//go:build windows

package system

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

// createNewConsole gives the started program a console window of its own.
const createNewConsole = 0x00000010

// RunInTerminal opens a PowerShell window that runs a command where the user
// can watch it and answer any question it asks. The window stays open until
// the user presses Enter, and the returned command finishes when it closes.
func RunInTerminal(command string) (*exec.Cmd, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("there is no command to run")
	}

	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePowerShell(terminalScript(command)),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("open PowerShell: %w", err)
	}
	return cmd, nil
}

// terminalScript shows the command, runs it, and keeps the window open so the
// result can be read.
func terminalScript(command string) string {
	echo := "'> " + strings.ReplaceAll(command, "'", "''") + "'"
	return strings.Join([]string{
		"$Host.UI.RawUI.WindowTitle = 'GrabOne'",
		"Write-Host " + echo + " -ForegroundColor Cyan",
		command,
		"Write-Host ''",
		"Read-Host 'Finished. Press Enter to close this window'",
	}, "\n")
}

// encodePowerShell encodes a script for -EncodedCommand, which takes it as
// UTF-16 in Base64. Passing it this way avoids every question of how quotes
// inside the command survive the command line.
func encodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	bytes := make([]byte, 0, len(units)*2)
	for _, unit := range units {
		bytes = append(bytes, byte(unit), byte(unit>>8))
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
