//go:build windows

// Package procutil holds the small platform specific bits of process handling.
package procutil

import (
	"os/exec"
	"syscall"
)

// Configure prepares a background tool for launching from a GUI application.
//
// It hides the console window these tools would otherwise flash on screen, so
// it must only be used for programs with no interface of their own. A program
// that is meant to show a window needs ConfigureVisible instead.
func Configure(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	// CREATE_NEW_PROCESS_GROUP lets the whole group be signalled, which is what
	// stops FFmpeg from surviving a cancelled download.
	cmd.SysProcAttr.CreationFlags |= 0x00000200
}

// ConfigureVisible prepares a command whose window the user is meant to see,
// such as opening a folder in Explorer. Hiding the window would start the
// program with nothing on screen, which looks exactly like nothing happening.
func ConfigureVisible(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = false
}

// SetCommandLine replaces the command line Go would build from the argument
// list with an exact string.
//
// Go quotes arguments the way the C runtime expects, which is right for almost
// everything. A few Windows programs — Explorer above all — parse their command
// line by hand and need a specific arrangement of quotes, and this is the only
// way to give them one.
func SetCommandLine(cmd *exec.Cmd, commandLine string) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CmdLine = commandLine
}
