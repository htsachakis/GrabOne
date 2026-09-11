//go:build !windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenFile opens a file with the desktop's default application.
func OpenFile(path string) error {
	absolute, err := verify(path)
	if err != nil {
		return err
	}
	return launch(absolute)
}

// RevealInFolder opens the containing folder of a file.
func RevealInFolder(path string) error {
	absolute, err := verify(path)
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("open", "-R", absolute)
		return cmd.Start()
	}
	return OpenFolder(filepath.Dir(absolute))
}

// OpenFolder opens a directory in the file manager.
func OpenFolder(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("open folder %s: not a folder", absolute)
	}
	return launch(absolute)
}

func launch(target string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	cmd := exec.Command(opener, target)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", target, err)
	}
	return cmd.Process.Release()
}

func verify(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no file to open")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	if _, err := os.Stat(absolute); err != nil {
		return "", fmt.Errorf("open %s: the file is no longer there", absolute)
	}
	return absolute, nil
}
