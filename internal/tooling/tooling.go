// Package tooling downloads the external programs GrabOne drives — yt-dlp,
// FFmpeg, FFprobe and aria2c — from their official releases, verifies them, and
// keeps them in a folder the application manages. A download is verified
// against the checksums published with its release, or, for a project that
// publishes none, against a checksum carried here for one exact file.
//
// This is offered, never automatic: a tool is fetched only when the user asks
// for it. Installing them with a package manager remains equally supported, and
// a tool found on PATH is used as it is.
package tooling

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"grabone/internal/dependencies"
	"grabone/internal/ghrelease"
)

// Source describes where a tool is published and how to recognise its files.
type Source struct {
	// Name is the dependency key this source provides.
	Names []string

	DisplayName string
	Owner       string
	Repo        string

	// ProjectURL is the page shown next to the download offer.
	ProjectURL string

	// ChecksumAsset names the file listing the SHA-256 of the release assets.
	// Without one the download is refused rather than trusted.
	ChecksumAsset func(release *ghrelease.Release) *ghrelease.Asset
	// Asset picks the file to download.
	Asset func(release *ghrelease.Release) *ghrelease.Asset
	// Extract turns the downloaded file into the executables, returning their
	// paths. A plain executable is simply moved into place.
	Extract func(downloaded, directory string) ([]string, error)

	// Licence is shown so the user knows what they are downloading.
	Licence string

	// Pinned names the one file that is downloaded, in place of Asset and
	// ChecksumAsset, for a project whose releases publish no checksums.
	Pinned *PinnedAsset
}

// PinnedAsset is one exact published file and its SHA-256.
//
// Looking up the latest release is only safe when the release itself says what
// its files should hash to. Without that, the newest file would have to be
// trusted blindly, so the version is fixed here and moves only with a new
// version of the application, after the new file has been checked.
type PinnedAsset struct {
	Version string
	Name    string
	URL     string
	Size    int64
	SHA256  string
}

// sources lists where each tool comes from.
//
// yt-dlp and FFmpeg are GitHub releases that publish a checksum file, which is
// what makes an automatic download verifiable. aria2 publishes none, so it is
// pinned to one file instead. Anything that could only be trusted blindly is
// not offered.
var sources = []Source{
	{
		Names:       []string{dependencies.YtDlp},
		DisplayName: "yt-dlp",
		Owner:       "yt-dlp",
		Repo:        "yt-dlp",
		ProjectURL:  "https://github.com/yt-dlp/yt-dlp",
		Licence:     "Unlicense",
		Asset: func(release *ghrelease.Release) *ghrelease.Asset {
			return release.FindNamed(executableName("yt-dlp"))
		},
		ChecksumAsset: func(release *ghrelease.Release) *ghrelease.Asset {
			return release.FindNamed("SHA2-256SUMS")
		},
		Extract: moveExecutable,
	},
	{
		Names:       []string{dependencies.FFmpeg, dependencies.FFprobe},
		DisplayName: "FFmpeg",
		Owner:       "BtbN",
		Repo:        "FFmpeg-Builds",
		ProjectURL:  "https://github.com/BtbN/FFmpeg-Builds",
		Licence:     "GPL",
		Asset: func(release *ghrelease.Release) *ghrelease.Asset {
			// The newest win64 GPL build: it carries the encoders the audio
			// conversion offers, which the LGPL build leaves out.
			var best *ghrelease.Asset
			for index := range release.Assets {
				name := strings.ToLower(release.Assets[index].Name)
				if !strings.HasSuffix(name, ".zip") {
					continue
				}
				if !strings.Contains(name, "win64") || !strings.Contains(name, "gpl") {
					continue
				}
				if strings.Contains(name, "shared") || strings.Contains(name, "lgpl") {
					continue
				}
				if best == nil || release.Assets[index].Name > best.Name {
					best = &release.Assets[index]
				}
			}
			return best
		},
		ChecksumAsset: func(release *ghrelease.Release) *ghrelease.Asset {
			return release.FindNamed("checksums.sha256")
		},
		Extract: extractFFmpeg,
	},
	{
		Names:       []string{dependencies.Aria2c},
		DisplayName: "aria2c",
		Owner:       "aria2",
		Repo:        "aria2",
		ProjectURL:  "https://github.com/aria2/aria2",
		Licence:     "GPL",
		// The checksum is that of the file the project serves, and matches the
		// one winget records for the same address.
		Pinned: &PinnedAsset{
			Version: "1.37.0",
			Name:    "aria2-1.37.0-win-64bit-build1.zip",
			URL:     "https://github.com/aria2/aria2/releases/download/release-1.37.0/aria2-1.37.0-win-64bit-build1.zip",
			Size:    2475379,
			SHA256:  "67d015301eef0b612191212d564c5bb0a14b5b9c4796b76454276a4d28d9b288",
		},
		Extract: extractAria2c,
	},
}

// SourceFor returns the source that provides a dependency.
func SourceFor(name string) (Source, bool) {
	for _, source := range sources {
		for _, provided := range source.Names {
			if provided == name {
				return source, true
			}
		}
	}
	return Source{}, false
}

// Sources returns every tool that can be downloaded.
func Sources() []Source { return append([]Source(nil), sources...) }

// Result describes what an install produced.
type Result struct {
	// Tool is the dependency that was asked for.
	Tool string `json:"tool"`
	// Version is the release tag the files came from.
	Version string `json:"version"`
	// Installed lists the executables now in the managed folder.
	Installed []string `json:"installed"`
	// Directory is where they were put.
	Directory string `json:"directory"`
}

// Stage reports what an install is doing, for the interface.
type Stage string

const (
	StageLookingUp   Stage = "looking-up"
	StageDownloading Stage = "downloading"
	StageExtracting  Stage = "extracting"
	StageFinished    Stage = "finished"
)

// Progress is emitted while a tool installs.
type Progress struct {
	Tool            string  `json:"tool"`
	Stage           Stage   `json:"stage"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Percent         float64 `json:"percent"`
	Message         string  `json:"message"`
}

// Installer downloads tools into a folder the application manages.
type Installer struct {
	// Directory is where the executables are kept, which is one of the places
	// dependency detection looks.
	Directory string

	client *ghrelease.Client
}

// NewInstaller builds an installer that keeps tools in directory.
func NewInstaller(directory, userAgent string) *Installer {
	return &Installer{Directory: directory, client: ghrelease.NewClient(userAgent)}
}

// Install fetches a tool and puts its executables in the managed folder.
func (i *Installer) Install(ctx context.Context, name string, onProgress func(Progress)) (*Result, error) {
	source, ok := SourceFor(name)
	if !ok {
		return nil, fmt.Errorf("%s cannot be downloaded automatically", dependencies.DisplayName(name))
	}

	report := func(stage Stage, message string, progress ghrelease.Progress) {
		if onProgress == nil {
			return
		}
		onProgress(Progress{
			Tool:            name,
			Stage:           stage,
			DownloadedBytes: progress.DownloadedBytes,
			TotalBytes:      progress.TotalBytes,
			Percent:         progress.Percent(),
			Message:         message,
		})
	}

	if err := os.MkdirAll(i.Directory, 0o755); err != nil {
		return nil, fmt.Errorf("prepare the tools folder: %w", err)
	}

	downloaded, version, err := i.fetch(ctx, source, report)
	if err != nil {
		return nil, err
	}

	report(StageExtracting, "Unpacking "+source.DisplayName, ghrelease.Progress{})

	installed, err := source.Extract(downloaded, i.Directory)
	if err != nil {
		return nil, err
	}

	report(StageFinished, source.DisplayName+" is ready", ghrelease.Progress{})

	return &Result{
		Tool:      name,
		Version:   version,
		Installed: installed,
		Directory: i.Directory,
	}, nil
}

// fetch downloads and verifies the file a tool is published as, returning
// where it was put and which version it is.
func (i *Installer) fetch(
	ctx context.Context,
	source Source,
	report func(Stage, string, ghrelease.Progress),
) (string, string, error) {
	if pinned := source.Pinned; pinned != nil {
		asset := &ghrelease.Asset{Name: pinned.Name, URL: pinned.URL, Size: pinned.Size}

		report(StageDownloading, "Downloading "+asset.Name, ghrelease.Progress{TotalBytes: asset.Size})

		downloaded, err := i.client.DownloadPinned(ctx, asset, pinned.SHA256, i.Directory, func(progress ghrelease.Progress) {
			report(StageDownloading, "Downloading "+asset.Name, progress)
		})
		return downloaded, pinned.Version, err
	}

	report(StageLookingUp, "Looking up the latest "+source.DisplayName+" release", ghrelease.Progress{})

	release, err := i.client.Latest(ctx, source.Owner, source.Repo)
	if err != nil {
		return "", "", fmt.Errorf("find the latest %s release: %w", source.DisplayName, err)
	}

	asset := source.Asset(release)
	if asset == nil {
		return "", "", fmt.Errorf("the latest %s release does not publish a Windows build", source.DisplayName)
	}
	checksums := source.ChecksumAsset(release)
	if checksums == nil {
		return "", "", fmt.Errorf("the latest %s release publishes no checksums, so it cannot be verified", source.DisplayName)
	}

	report(StageDownloading, "Downloading "+asset.Name, ghrelease.Progress{TotalBytes: asset.Size})

	downloaded, err := i.client.DownloadVerified(ctx, asset, checksums, i.Directory, func(progress ghrelease.Progress) {
		report(StageDownloading, "Downloading "+asset.Name, progress)
	})
	return downloaded, release.TagName, err
}

// Installed reports which managed executables are present.
func (i *Installer) Installed() map[string]string {
	found := map[string]string{}
	for _, name := range []string{dependencies.YtDlp, dependencies.FFmpeg, dependencies.FFprobe, dependencies.Aria2c} {
		path := filepath.Join(i.Directory, executableName(name))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			found[name] = path
		}
	}
	return found
}

// moveExecutable puts a downloaded executable in its final place.
func moveExecutable(downloaded, directory string) ([]string, error) {
	target := filepath.Join(directory, filepath.Base(downloaded))
	if downloaded != target {
		if err := os.Rename(downloaded, target); err != nil {
			return nil, fmt.Errorf("store the download: %w", err)
		}
	}
	return []string{target}, nil
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
