package dependencies

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// UpdateCommand is how to update a tool that was installed outside GrabOne.
// GrabOne does not replace such a tool itself, because the program that
// installed it keeps track of it; it says which command does the update.
type UpdateCommand struct {
	// Command is a PowerShell command line.
	Command string `json:"command"`
	// Via names what installed the tool, such as winget or Scoop.
	Via string `json:"via"`
	// Detail says anything the user needs to know before running it.
	Detail string `json:"detail,omitempty"`
}

// Package names the tools go by in each package manager. FFprobe has none of
// its own: it comes with FFmpeg, and updating FFmpeg updates both.
var (
	wingetIDs = map[string]string{
		YtDlp:   "yt-dlp.yt-dlp",
		FFmpeg:  "Gyan.FFmpeg",
		FFprobe: "Gyan.FFmpeg",
		Aria2c:  "aria2.aria2",
	}
	packageNames = map[string]string{
		YtDlp:   "yt-dlp",
		FFmpeg:  "ffmpeg",
		FFprobe: "ffmpeg",
		Aria2c:  "aria2",
	}
)

// UpdateCommandFor works out how to update the copy of a tool at path from
// where it is installed. ok is false when nothing installed it that can update
// it, such as an FFmpeg build copied into a folder by hand.
//
// A path through a link is followed first, because winget puts links on PATH
// and the package the tool came from is only visible in the real location.
func UpdateCommandFor(name, path string) (UpdateCommand, bool) {
	if strings.TrimSpace(path) == "" {
		return UpdateCommand{}, false
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return updateCommandAt(name, path, fileExists)
}

// updateCommandAt is UpdateCommandFor without touching links, with the file
// check passed in so the decision can be tested on made-up paths.
func updateCommandAt(name, path string, exists func(string) bool) (UpdateCommand, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(path, `\`, "/"))

	if id := wingetPackageID(path); id != "" {
		return UpdateCommand{
			Command: "winget upgrade --id " + id + " --exact",
			Via:     "winget",
		}, true
	}
	if strings.Contains(normalized, "/winget/links/") {
		if id, ok := wingetIDs[name]; ok {
			return UpdateCommand{
				Command: "winget upgrade --id " + id + " --exact",
				Via:     "winget",
			}, true
		}
	}

	if strings.Contains(normalized, "/scoop/") {
		if pkg, ok := packageNames[name]; ok {
			return UpdateCommand{Command: "scoop update " + pkg, Via: "Scoop"}, true
		}
	}

	if strings.Contains(normalized, "/chocolatey/") {
		if pkg, ok := packageNames[name]; ok {
			return UpdateCommand{
				Command: "choco upgrade " + pkg + " -y",
				Via:     "Chocolatey",
				Detail:  "Chocolatey needs a terminal opened as administrator.",
			}, true
		}
	}

	if name != YtDlp {
		return UpdateCommand{}, false
	}

	// yt-dlp installed with pip lives in a Python Scripts folder, and has to be
	// updated through pip: its self-update refuses to touch a pip install.
	directory := filepath.Dir(path)
	if strings.EqualFold(filepath.Base(directory), "Scripts") {
		python := filepath.Join(filepath.Dir(directory), "python.exe")
		if exists(python) {
			return UpdateCommand{
				Command: "& " + quote(python) + " -m pip install --upgrade yt-dlp",
				Via:     "pip",
			}, true
		}
		return UpdateCommand{Command: "python -m pip install --upgrade yt-dlp", Via: "pip"}, true
	}

	// The standalone yt-dlp.exe can replace itself.
	return UpdateCommand{
		Command: "& " + quote(path) + " -U",
		Via:     "yt-dlp",
		Detail:  "yt-dlp downloads its latest release and replaces itself.",
	}, true
}

// packageIDPattern is what a winget package identifier looks like. The
// identifier is read from a folder name and ends up in a command, so anything
// else is not used.
var packageIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]*$`)

// wingetPackageID reads the package a winget install came from out of its
// folder, which is named "<package id>_<source>_<hash>".
func wingetPackageID(path string) string {
	id := wingetFolderID(path)
	if !packageIDPattern.MatchString(id) {
		return ""
	}
	return id
}

func wingetFolderID(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '\\' || r == '/' })
	for index := 0; index+2 < len(parts); index++ {
		if !strings.EqualFold(parts[index], "WinGet") || !strings.EqualFold(parts[index+1], "Packages") {
			continue
		}
		folder := parts[index+2]
		if cut := strings.Index(strings.ToLower(folder), "_microsoft.winget.source_"); cut > 0 {
			return folder[:cut]
		}
		if id, _, found := strings.Cut(folder, "_"); found && id != "" {
			return id
		}
	}
	return ""
}

// quote wraps a path for PowerShell. Single quotes take everything literally,
// and a single quote inside is written twice.
func quote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
