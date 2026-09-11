//go:build windows

// Package system holds the small desktop integrations: opening a finished file
// and revealing it in the file manager.
package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"grabone/internal/procutil"
)

// OpenFile opens a file with the application Windows associates with it.
//
// The path is passed as a separate argument and no shell is involved, so a
// filename coming from extracted metadata cannot be interpreted as a command.
func OpenFile(path string) error {
	absolute, err := verify(path)
	if err != nil {
		return err
	}

	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", absolute)
	procutil.ConfigureVisible(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", absolute, err)
	}
	return cmd.Process.Release()
}

// revealPlan is what revealing a path should do, decided before anything is
// launched so the decision can be tested without opening windows.
type revealPlan struct {
	// SelectFile is the file to show selected in its folder.
	SelectFile string
	// OpenFolder is the folder to open when the file itself is gone.
	OpenFolder string
}

// planReveal decides how to reveal a path.
//
// A file that is no longer there is not a failure worth reporting when its
// folder still exists: the user asked to see where it was, and the folder
// answers that.
func planReveal(path string) (revealPlan, error) {
	absolute, err := verify(path)
	if err == nil {
		return revealPlan{SelectFile: absolute}, nil
	}

	directory := filepath.Dir(strings.TrimSpace(path))
	if directory == "" || directory == "." {
		return revealPlan{}, err
	}
	if info, statErr := os.Stat(directory); statErr != nil || !info.IsDir() {
		return revealPlan{}, err
	}
	if absoluteDir, absErr := filepath.Abs(directory); absErr == nil {
		directory = absoluteDir
	}
	return revealPlan{OpenFolder: directory}, nil
}

// RevealInFolder opens the containing folder with the file selected.
func RevealInFolder(path string) error {
	plan, err := planReveal(path)
	if err != nil {
		return err
	}
	if plan.OpenFolder != "" {
		return OpenFolder(plan.OpenFolder)
	}

	cmd := exec.Command(explorerPath())
	// The command line is built by hand: see revealCommandLine.
	procutil.SetCommandLine(cmd, revealCommandLine(plan.SelectFile))
	procutil.ConfigureVisible(cmd)

	// explorer returns a non-zero exit code even when it succeeds, so its
	// result is deliberately not checked.
	_ = cmd.Start()
	if cmd.Process != nil {
		return cmd.Process.Release()
	}
	return nil
}

// revealCommandLine builds the exact command line that selects a file in
// Explorer.
//
// Explorer does not parse its command line the way everything else does. The
// quotes have to surround the path alone:
//
//	explorer.exe /select,"C:\Users\me\Videos\A clip.mp4"     opens the folder
//	explorer.exe "/select,C:\Users\me\Videos\A clip.mp4"     opens Documents
//
// Building the arguments normally produces the second form as soon as the path
// contains a space, because the whole argument is quoted as one unit. Explorer
// then fails to parse it and silently falls back to the default folder, which
// looks exactly like the button doing nothing useful.
func revealCommandLine(absolute string) string {
	return `explorer.exe /select,"` + absolute + `"`
}

// explorerPath returns the full path of Explorer, so the right program is
// started even when PATH is unusual.
func explorerPath() string {
	if windows := os.Getenv("WINDIR"); windows != "" {
		candidate := filepath.Join(windows, "explorer.exe")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "explorer.exe"
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

	cmd := exec.Command("explorer.exe", absolute)
	procutil.ConfigureVisible(cmd)
	_ = cmd.Start()
	if cmd.Process != nil {
		return cmd.Process.Release()
	}
	return nil
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
