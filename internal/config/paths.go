package config

import (
	"os"
	"path/filepath"

	"grabone/internal/appinfo"
)

// DataDir returns the directory holding settings and logs. On Windows this
// resolves to %LOCALAPPDATA%\<app>, with a home directory fallback so the
// package stays usable on other platforms and in tests.
func DataDir() string {
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		return filepath.Join(base, appinfo.DataDirName)
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, appinfo.DataDirName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "."+appinfo.DataDirName)
	}
	return filepath.Join(home, "."+appinfo.DataDirName)
}

// SettingsFile returns the full path of the settings file.
func SettingsFile() string {
	return filepath.Join(DataDir(), "settings.json")
}

// ToolsDir returns the folder holding the copies of yt-dlp and FFmpeg that the
// application downloaded itself. It is part of the dependency search path.
func ToolsDir() string {
	return filepath.Join(DataDir(), "tools")
}

// LogDir returns the directory where log files are written.
func LogDir() string {
	return filepath.Join(DataDir(), "logs")
}

// DefaultOutputDir returns the download location used until the user picks
// another one: %USERPROFILE%\Videos\<app>, falling back to the user home
// directory when the Videos folder cannot be determined.
func DefaultOutputDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		videos := filepath.Join(home, "Videos")
		if info, err := os.Stat(videos); err == nil && info.IsDir() {
			return filepath.Join(videos, appinfo.OutputDirName)
		}
		return filepath.Join(home, "Downloads", appinfo.OutputDirName)
	}
	return filepath.Join(".", appinfo.OutputDirName)
}

// ExecutableDir returns the directory containing the running binary. Users may
// drop yt-dlp.exe, ffmpeg.exe and ffprobe.exe next to the application, so this
// is part of the dependency search path.
func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	return filepath.Dir(resolved)
}
