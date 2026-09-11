// Package tooling downloads the external programs GrabOne drives — yt-dlp,
// FFmpeg and FFprobe — from their official releases, verifies them against the
// checksums published with those releases, and keeps them in a folder the
// application manages.
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
}

// sources lists where each tool comes from.
//
// Both are GitHub releases that publish a checksum file, which is what makes an
// automatic download verifiable. Anything without one would have to be trusted
// blindly, and is not offered.
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

	report(StageLookingUp, "Looking up the latest "+source.DisplayName+" release", ghrelease.Progress{})

	release, err := i.client.Latest(ctx, source.Owner, source.Repo)
	if err != nil {
		return nil, fmt.Errorf("find the latest %s release: %w", source.DisplayName, err)
	}

	asset := source.Asset(release)
	if asset == nil {
		return nil, fmt.Errorf("the latest %s release does not publish a Windows build", source.DisplayName)
	}
	checksums := source.ChecksumAsset(release)
	if checksums == nil {
		return nil, fmt.Errorf("the latest %s release publishes no checksums, so it cannot be verified", source.DisplayName)
	}

	if err := os.MkdirAll(i.Directory, 0o755); err != nil {
		return nil, fmt.Errorf("prepare the tools folder: %w", err)
	}

	report(StageDownloading, "Downloading "+asset.Name, ghrelease.Progress{TotalBytes: asset.Size})

	downloaded, err := i.client.DownloadVerified(ctx, asset, checksums, i.Directory, func(progress ghrelease.Progress) {
		report(StageDownloading, "Downloading "+asset.Name, progress)
	})
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
		Version:   release.TagName,
		Installed: installed,
		Directory: i.Directory,
	}, nil
}

// Installed reports which managed executables are present.
func (i *Installer) Installed() map[string]string {
	found := map[string]string{}
	for _, name := range []string{dependencies.YtDlp, dependencies.FFmpeg, dependencies.FFprobe} {
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
