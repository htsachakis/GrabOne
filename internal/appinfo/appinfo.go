// Package appinfo holds the application's identity in one place so that
// branding can be changed without touching the rest of the codebase.
package appinfo

// Version is the application version, independent of the versions of yt-dlp,
// FFmpeg and FFprobe, which are updated separately.
//
// It is a variable rather than a constant so a release build can stamp the tag
// it was built from:
//
//	go build -ldflags "-X grabone/internal/appinfo.Version=1.2.3"
var Version = "1.0.0"

// Identity of the application. Nothing outside this package should hardcode
// the application name.
const (
	// Name is the user facing application name.
	Name = "GrabOne"

	// ShortDescription is shown in the about section and the window subtitle.
	ShortDescription = "Multi-platform media downloader"

	// DataDirName is the folder name used under %LOCALAPPDATA% for settings
	// and logs.
	DataDirName = "GrabOne"

	// OutputDirName is the folder name used for the default download location.
	OutputDirName = "GrabOne"

	// Repository is where releases are published, used by the update check.
	RepositoryOwner = "htsachakis"
	RepositoryName  = "GrabOne"

	// LegalNotice is displayed unobtrusively in the about section.
	LegalNotice = "Users are responsible for complying with applicable laws, copyright rules, and platform terms."
)

// VersionLabel is the version as it is shown to the user, such as "v1.0.0".
func VersionLabel() string { return "v" + Version }

// WindowTitle is the text shown in the title bar and the taskbar.
func WindowTitle() string { return Name + " " + VersionLabel() }
