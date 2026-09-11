package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grabone/internal/ghrelease"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "1.0.0", right: "1.0.0", want: 0},
		{left: "v1.0.0", right: "1.0.0", want: 0},
		{left: "1.0.1", right: "1.0.0", want: 1},
		{left: "1.0.0", right: "1.0.1", want: -1},
		// A plain string comparison would put 1.9.0 ahead of 1.10.0.
		{left: "1.10.0", right: "1.9.0", want: 1},
		{left: "2.0.0", right: "1.99.99", want: 1},
		{left: "1.2", right: "1.2.0", want: 0},
		{left: "1.2.3.4", right: "1.2.3", want: 1},
		// A release is newer than its own pre-releases.
		{left: "1.2.0", right: "1.2.0-rc1", want: 1},
		{left: "1.2.0-rc1", right: "1.2.0-rc2", want: -1},
		{left: "1.2.0+build7", right: "1.2.0", want: 0},
		// An unparsable component must not panic or claim an upgrade.
		{left: "nightly", right: "1.0.0", want: -1},
	}

	for _, test := range tests {
		t.Run(test.left+" vs "+test.right, func(t *testing.T) {
			if got := CompareVersions(test.left, test.right); got != test.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v1.1.0", "1.0.0") {
		t.Error("a later tag should be reported as newer")
	}
	if IsNewer("1.0.0", "1.0.0") {
		t.Error("the same version is not an update")
	}
	if IsNewer("0.9.0", "1.0.0") {
		t.Error("an older release must never be offered as an update")
	}
}

const samplePayload = `{
  "tag_name": "v1.4.0",
  "name": "GrabOne 1.4.0",
  "body": "Adds subtitle conversion.",
  "draft": false,
  "published_at": "2026-09-01T10:00:00Z",
  "html_url": "https://github.com/owner/repo/releases/tag/v1.4.0",
  "assets": [
    {"name": "GrabOne-1.4.0-installer.exe", "browser_download_url": "https://github.com/owner/repo/releases/download/v1.4.0/GrabOne-1.4.0-installer.exe", "size": 12345},
    {"name": "GrabOne-1.4.0-portable.exe", "browser_download_url": "https://github.com/owner/repo/releases/download/v1.4.0/GrabOne-1.4.0-portable.exe", "size": 12000},
    {"name": "checksums.txt", "browser_download_url": "https://github.com/owner/repo/releases/download/v1.4.0/checksums.txt", "size": 200}
  ]
}`

// checkerFor builds a checker pointed at a test server serving payload.
func checkerFor(t *testing.T, payload, currentVersion string) *Checker {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasSuffix(request.URL.Path, "/releases/latest") {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)

	checker := NewChecker("owner", "repo", currentVersion)
	checker.client = ghrelease.NewClient("test")
	checker.client.APIBaseURL = server.URL
	return checker
}

func TestCheckFindsANewerRelease(t *testing.T) {
	checker := checkerFor(t, samplePayload, "1.0.0")

	release, available, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	if !available {
		t.Error("1.4.0 should be offered as an update over 1.0.0")
	}
	if release.Version != "1.4.0" {
		t.Errorf("version = %q, want the tag without its prefix", release.Version)
	}
	if release.Notes == "" || release.PageURL == "" {
		t.Error("the notes and page link should be carried through")
	}
	if release.Installer == nil || release.Installer.Name != "GrabOne-1.4.0-installer.exe" {
		t.Errorf("installer = %+v, want the installer asset, not the portable build", release.Installer)
	}
	if release.Checksums == nil {
		t.Error("the checksum file was not recognised")
	}
	if !release.Verifiable() {
		t.Error("a release with an installer and checksums is verifiable")
	}
}

func TestCheckIgnoresAnOlderOrEqualRelease(t *testing.T) {
	checker := checkerFor(t, samplePayload, "1.4.0")

	_, available, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if available {
		t.Error("the current version must not be offered as an update")
	}
}

func TestReleaseWithoutChecksumsCannotBeInstalled(t *testing.T) {
	payload := strings.Replace(samplePayload, `,
    {"name": "checksums.txt", "browser_download_url": "https://github.com/owner/repo/releases/download/v1.4.0/checksums.txt", "size": 200}`, "", 1)
	checker := checkerFor(t, payload, "1.0.0")

	release, _, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if release.Verifiable() {
		t.Fatal("a release with no checksum file is not verifiable")
	}

	if _, err := checker.Download(context.Background(), release, t.TempDir(), nil); err == nil {
		t.Error("an unverifiable release must not be downloaded for installation")
	}
}

func TestInstallRefusesAnythingButAnInstaller(t *testing.T) {
	directory := t.TempDir()
	notAnInstaller := filepath.Join(directory, "notes.txt")
	if err := os.WriteFile(notAnInstaller, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := Install(notAnInstaller); err == nil {
		t.Error("only an installer may be started")
	}
	if err := Install(filepath.Join(directory, "missing.exe")); err == nil {
		t.Error("a missing installer should be reported")
	}
}

func TestCleanDownloadsKeepsTheCurrentInstaller(t *testing.T) {
	directory := t.TempDir()
	keep := filepath.Join(directory, "GrabOne-1.4.0-installer.exe")
	old := filepath.Join(directory, "GrabOne-1.3.0-installer.exe")

	for _, path := range []string{keep, old} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	CleanDownloads(directory, keep)

	if _, err := os.Stat(keep); err != nil {
		t.Error("the current installer should be kept")
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("an installer from an earlier update should be removed")
	}
}
