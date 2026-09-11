package updater

import (
	"context"
	"fmt"
	"strings"

	"grabone/internal/ghrelease"
)

// Asset is one file published with a release.
type Asset = ghrelease.Asset

// Progress reports how far the installer download has come.
type Progress = ghrelease.Progress

// Release describes a published version of the application.
type Release struct {
	Version     string `json:"version"`
	TagName     string `json:"tagName"`
	Name        string `json:"name"`
	Notes       string `json:"notes"`
	PublishedAt string `json:"publishedAt"`
	PageURL     string `json:"pageUrl"`

	// Installer is the Windows installer asset, when the release has one.
	Installer *Asset `json:"installer,omitempty"`
	// Checksums lists the SHA-256 of each asset. Without it an installer cannot
	// be verified, and an unverified installer is never run.
	Checksums *Asset `json:"checksums,omitempty"`
}

// Verifiable reports whether the release can be installed automatically: it
// needs both an installer and its checksum.
func (r *Release) Verifiable() bool {
	return r != nil && r.Installer != nil && r.Checksums != nil
}

// Checker asks GitHub what the latest release of the application is.
type Checker struct {
	Owner          string
	Repo           string
	CurrentVersion string

	client *ghrelease.Client
}

// NewChecker builds a checker for a repository.
func NewChecker(owner, repo, currentVersion string) *Checker {
	version := NormalizeVersion(currentVersion)
	return &Checker{
		Owner:          owner,
		Repo:           repo,
		CurrentVersion: version,
		client:         ghrelease.NewClient("GrabOne/" + version),
	}
}

// Latest returns the newest published release.
func (c *Checker) Latest(ctx context.Context) (*Release, error) {
	raw, err := c.client.Latest(ctx, c.Owner, c.Repo)
	if err != nil {
		return nil, err
	}
	return toRelease(raw), nil
}

// Check reports whether a newer release exists.
func (c *Checker) Check(ctx context.Context) (*Release, bool, error) {
	release, err := c.Latest(ctx)
	if err != nil {
		return nil, false, err
	}
	return release, IsNewer(release.Version, c.CurrentVersion), nil
}

// Download fetches the installer of a release and verifies it against the
// checksum published with it.
func (c *Checker) Download(ctx context.Context, release *Release, directory string, onProgress func(Progress)) (string, error) {
	if !release.Verifiable() {
		return "", fmt.Errorf("this release does not publish an installer with a checksum, so it cannot be installed automatically")
	}
	return c.client.DownloadVerified(ctx, release.Installer, release.Checksums, directory, onProgress)
}

// toRelease picks the installer and checksum assets out of a release.
func toRelease(raw *ghrelease.Release) *Release {
	release := &Release{
		Version:     NormalizeVersion(raw.TagName),
		TagName:     raw.TagName,
		Name:        raw.Name,
		Notes:       raw.Notes,
		PublishedAt: raw.PublishedAt,
		PageURL:     raw.PageURL,
	}

	release.Installer = raw.Find(func(name string) bool {
		lowered := strings.ToLower(name)
		return strings.Contains(lowered, "installer") && strings.HasSuffix(lowered, ".exe")
	})
	release.Checksums = raw.Find(func(name string) bool {
		lowered := strings.ToLower(name)
		return lowered == "checksums.txt" || strings.HasSuffix(lowered, "sha256.txt")
	})
	return release
}
