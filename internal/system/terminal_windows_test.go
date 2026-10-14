//go:build windows

package system

import (
	"os/exec"
	"strings"
	"testing"
)

// The command reaches PowerShell intact, quotes and all, which is the reason
// it is passed encoded rather than on the command line.
func TestEncodedCommandReachesPowerShellIntact(t *testing.T) {
	script := `Write-Output 'it''s "quoted" & fine'`

	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-EncodedCommand", encodePowerShell(script)).Output()
	if err != nil {
		t.Skipf("PowerShell is not available: %v", err)
	}
	if got, want := strings.TrimSpace(string(output)), `it's "quoted" & fine`; got != want {
		t.Errorf("PowerShell printed %q, want %q", got, want)
	}
}

func TestTerminalScriptShowsTheCommandBeforeRunningIt(t *testing.T) {
	script := terminalScript(`& 'D:\it''s\yt-dlp.exe' -U`)

	if !strings.Contains(script, `Write-Host '> & ''D:\it''''s\yt-dlp.exe'' -U'`) {
		t.Errorf("script does not echo the command safely:\n%s", script)
	}
	if !strings.Contains(script, "\n& 'D:\\it''s\\yt-dlp.exe' -U\n") {
		t.Errorf("script does not run the command as given:\n%s", script)
	}
}
