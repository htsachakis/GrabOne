package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"grabone/internal/procutil"
)

// Install starts a downloaded installer.
//
// The installer is a program the user is about to watch, so it is started with
// its window visible and detached from the application, which then closes to
// release the files being replaced.
func Install(installerPath string) error {
	absolute, err := filepath.Abs(strings.TrimSpace(installerPath))
	if err != nil {
		return fmt.Errorf("start the installer: %w", err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("the installer is no longer there: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("the installer path is a folder")
	}
	if !strings.EqualFold(filepath.Ext(absolute), ".exe") {
		// Only an installer is ever launched from here.
		return fmt.Errorf("refusing to start %s: it is not an installer", filepath.Base(absolute))
	}

	cmd := exec.Command(absolute)
	cmd.Dir = filepath.Dir(absolute)
	procutil.ConfigureVisible(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start the installer: %w", err)
	}
	return cmd.Process.Release()
}

// ClearDownloads removes every installer in the directory.
//
// A downloaded update is remembered only for as long as the application runs,
// so an installer still here at startup belongs to an update that was already
// applied or abandoned. Nothing can reach it again, and the one that carried
// out the last update cannot delete itself while it is running.
func ClearDownloads(directory string) {
	CleanDownloads(directory, "")
}

// CleanDownloads removes installers left behind by earlier updates, keeping the
// one just downloaded. An empty keep removes all of them.
func CleanDownloads(directory, keep string) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}

	// Not filepath.Base(keep): that turns "" into ".", which reads as though
	// some file might be spared when the caller asked for none to be.
	keepName := ""
	if keep != "" {
		keepName = filepath.Base(keep)
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == keepName {
			continue
		}
		_ = os.Remove(filepath.Join(directory, entry.Name()))
	}
}
