package dependencies

import (
	"path/filepath"
	"testing"
)

func TestUpdateCommandFollowsWhatInstalledTheTool(t *testing.T) {
	noFiles := func(string) bool { return false }

	cases := []struct {
		name    string
		tool    string
		path    string
		command string
		via     string
	}{
		{
			name:    "winget package folder",
			tool:    YtDlp,
			path:    `C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\yt-dlp.yt-dlp_Microsoft.Winget.Source_8wekyb3d8bbwe\yt-dlp.exe`,
			command: "winget upgrade --id yt-dlp.yt-dlp --exact",
			via:     "winget",
		},
		{
			// FFmpeg can come from a package other than the usual one, and the
			// folder says which.
			name:    "FFmpeg from another winget package",
			tool:    FFprobe,
			path:    `C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\yt-dlp.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-N-1-win64-gpl\bin\ffprobe.exe`,
			command: "winget upgrade --id yt-dlp.FFmpeg --exact",
			via:     "winget",
		},
		{
			name:    "machine-wide winget install",
			tool:    Aria2c,
			path:    `C:\Program Files\WinGet\Packages\aria2.aria2_Microsoft.Winget.Source_8wekyb3d8bbwe\aria2-1.37.0-win-64bit-build1\aria2c.exe`,
			command: "winget upgrade --id aria2.aria2 --exact",
			via:     "winget",
		},
		{
			name:    "winget link that could not be followed",
			tool:    FFmpeg,
			path:    `C:\Users\me\AppData\Local\Microsoft\WinGet\Links\ffmpeg.exe`,
			command: "winget upgrade --id Gyan.FFmpeg --exact",
			via:     "winget",
		},
		{
			name:    "Scoop",
			tool:    FFprobe,
			path:    `C:\Users\me\scoop\shims\ffprobe.exe`,
			command: "scoop update ffmpeg",
			via:     "Scoop",
		},
		{
			name:    "Chocolatey",
			tool:    YtDlp,
			path:    `C:\ProgramData\chocolatey\bin\yt-dlp.exe`,
			command: "choco upgrade yt-dlp -y",
			via:     "Chocolatey",
		},
		{
			// The Scripts folder is found by splitting the path on the host's
			// separator, so the path is written with it: on Windows this is
			// the backslash path pip creates.
			name:    "pip without a Python next to it",
			tool:    YtDlp,
			path:    filepath.FromSlash("C:/Users/me/AppData/Roaming/Python/Python312/Scripts/yt-dlp.exe"),
			command: "python -m pip install --upgrade yt-dlp",
			via:     "pip",
		},
		{
			name:    "standalone yt-dlp updates itself",
			tool:    YtDlp,
			path:    `D:\Tools\it's here\yt-dlp.exe`,
			command: `& 'D:\Tools\it''s here\yt-dlp.exe' -U`,
			via:     "yt-dlp",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			update, ok := updateCommandAt(tc.tool, tc.path, noFiles)
			if !ok {
				t.Fatalf("no update command for %s", tc.path)
			}
			if update.Command != tc.command {
				t.Errorf("command = %q, want %q", update.Command, tc.command)
			}
			if update.Via != tc.via {
				t.Errorf("via = %q, want %q", update.Via, tc.via)
			}
		})
	}
}

func TestPipUpdateUsesThePythonThatOwnsTheScriptsFolder(t *testing.T) {
	// Written with the host's separator, as above: on Windows these are
	// C:\Python312\python.exe and C:\Python312\Scripts\yt-dlp.exe.
	python := filepath.FromSlash("C:/Python312/python.exe")
	exists := func(path string) bool { return path == python }

	update, ok := updateCommandAt(YtDlp, filepath.FromSlash("C:/Python312/Scripts/yt-dlp.exe"), exists)
	if !ok {
		t.Fatal("no update command for a pip install")
	}
	if want := "& '" + python + "' -m pip install --upgrade yt-dlp"; update.Command != want {
		t.Errorf("command = %q, want %q", update.Command, want)
	}
}

func TestNoUpdateCommandForAToolCopiedByHand(t *testing.T) {
	for _, tool := range []string{FFmpeg, FFprobe, Aria2c} {
		if update, ok := updateCommandAt(tool, `D:\Tools\bin\`+tool+".exe", func(string) bool { return false }); ok {
			t.Errorf("%s copied by hand got update command %q", tool, update.Command)
		}
	}
}

func TestAnOddWingetFolderNameIsNotPutInACommand(t *testing.T) {
	path := `C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\x;calc_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg.exe`
	if update, ok := updateCommandAt(FFmpeg, path, func(string) bool { return false }); ok {
		t.Errorf("got update command %q from a folder name that is not a package identifier", update.Command)
	}
}
