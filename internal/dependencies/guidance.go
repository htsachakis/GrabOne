package dependencies

import "path/filepath"

// InstallStep is one way to install a missing dependency. A step carries a
// command to copy, a page to open, or simply an instruction.
type InstallStep struct {
	Title string `json:"title"`
	// Command is a shell command the user can copy and run.
	Command string `json:"command,omitempty"`
	// URL is a page to open in the browser.
	URL string `json:"url,omitempty"`
	// Detail explains what the step does.
	Detail string `json:"detail"`
}

// Guidance tells the user how to install a dependency GrabOne could not find.
// The application never installs these tools itself: they are separate programs
// with their own release cycles, and the user stays in control of them.
type Guidance struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"displayName"`
	Summary     string        `json:"summary"`
	Steps       []InstallStep `json:"steps"`
	// AfterInstall says what to do once the tool is installed.
	AfterInstall string `json:"afterInstall"`
}

// Known download pages. These are the only addresses the application will open,
// which is what keeps the link buttons from becoming a way to open anything.
const (
	ytDlpReleasesURL   = "https://github.com/yt-dlp/yt-dlp/releases/latest"
	ytDlpHomeURL       = "https://github.com/yt-dlp/yt-dlp#installation"
	ffmpegBuildsURL    = "https://www.gyan.dev/ffmpeg/builds/"
	ffmpegDownloadURL  = "https://ffmpeg.org/download.html"
	wingetLearnMoreURL = "https://learn.microsoft.com/windows/package-manager/winget/"
)

// GuidanceFor returns the installation help for one dependency. applicationDir
// is the folder holding GrabOne, which is one of the places it looks.
func GuidanceFor(name, applicationDir string) Guidance {
	folder := applicationDir
	if folder == "" {
		folder = "the folder containing GrabOne.exe"
	} else {
		folder = filepath.Clean(folder)
	}

	switch name {
	case YtDlp:
		return Guidance{
			Name:        YtDlp,
			DisplayName: DisplayName(YtDlp),
			Summary:     "yt-dlp does the extracting and downloading. GrabOne cannot analyze or download anything without it.",
			Steps: []InstallStep{
				{
					Title:   "Install with winget",
					Command: "winget install yt-dlp.yt-dlp",
					Detail:  "Paste this into PowerShell. winget ships with Windows 10 and 11.",
				},
				{
					Title:   "Or install with Scoop or Chocolatey",
					Command: "scoop install yt-dlp",
					Detail:  "Chocolatey users can run choco install yt-dlp instead.",
				},
				{
					Title:  "Or download the executable",
					URL:    ytDlpReleasesURL,
					Detail: "Download yt-dlp.exe from the latest release and put it in " + folder + ", or anywhere on your PATH.",
				},
			},
			AfterInstall: "Then press Retry. GrabOne looks in its own folder, on your PATH, and in the usual install locations.",
		}

	case FFmpeg, FFprobe:
		return Guidance{
			Name:        name,
			DisplayName: DisplayName(name),
			Summary:     "FFmpeg merges separate video and audio streams, converts audio, and embeds subtitles, chapters and metadata. FFprobe comes with it and reports what the finished file contains. Downloading a single combined stream works without them.",
			Steps: []InstallStep{
				{
					Title:   "Install with winget",
					Command: "winget install Gyan.FFmpeg",
					Detail:  "Paste this into PowerShell. It installs ffmpeg.exe and ffprobe.exe together.",
				},
				{
					Title:   "Or install with Scoop or Chocolatey",
					Command: "scoop install ffmpeg",
					Detail:  "Chocolatey users can run choco install ffmpeg instead.",
				},
				{
					Title:  "Or download a build",
					URL:    ffmpegBuildsURL,
					Detail: "Take a release build, then copy ffmpeg.exe and ffprobe.exe from its bin folder into " + folder + ", or anywhere on your PATH.",
				},
			},
			AfterInstall: "Then press Retry. If you installed while GrabOne was open, Retry is all that is needed.",
		}

	default:
		return Guidance{Name: name, DisplayName: DisplayName(name)}
	}
}

// AllGuidance returns the help for every dependency.
func AllGuidance(applicationDir string) []Guidance {
	return []Guidance{
		GuidanceFor(YtDlp, applicationDir),
		GuidanceFor(FFmpeg, applicationDir),
		GuidanceFor(FFprobe, applicationDir),
	}
}

// IsKnownHelpURL reports whether a URL is one the application offers to open.
// The frontend can only ask for these, so a link button cannot be turned into a
// way to open an arbitrary address.
func IsKnownHelpURL(url string) bool {
	switch url {
	case ytDlpReleasesURL, ytDlpHomeURL, ffmpegBuildsURL, ffmpegDownloadURL, wingetLearnMoreURL:
		return true
	default:
		return false
	}
}
