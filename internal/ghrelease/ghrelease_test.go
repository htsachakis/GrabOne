package ghrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const samplePayload = `{
  "tag_name": "v1.4.0",
  "name": "GrabOne 1.4.0",
  "body": "Adds subtitle conversion.",
  "draft": false,
  "published_at": "2026-09-01T10:00:00Z",
  "html_url": "https://github.com/owner/repo/releases/tag/v1.4.0",
  "assets": [
    {"name": "GrabOne-1.4.0-installer.exe", "browser_download_url": "https://github.com/owner/repo/releases/download/v1.4.0/GrabOne-1.4.0-installer.exe", "size": 12345},
    {"name": "checksums.txt", "browser_download_url": "https://github.com/owner/repo/releases/download/v1.4.0/checksums.txt", "size": 200}
  ]
}`

func releaseServer(t *testing.T, payload string, status int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if status != http.StatusOK {
			writer.WriteHeader(status)
			return
		}
		if !strings.HasSuffix(request.URL.Path, "/releases/latest") {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestLatestReadsARelease(t *testing.T) {
	server := releaseServer(t, samplePayload, http.StatusOK)

	client := NewClient("test")
	client.APIBaseURL = server.URL

	release, err := client.Latest(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}

	if release.TagName != "v1.4.0" {
		t.Errorf("tag = %q, want v1.4.0", release.TagName)
	}
	if release.Notes == "" || release.PageURL == "" {
		t.Error("the notes and page link should be carried through")
	}
	if len(release.Assets) != 2 {
		t.Fatalf("assets = %d, want 2", len(release.Assets))
	}

	if asset := release.FindNamed("checksums.txt"); asset == nil {
		t.Error("FindNamed did not locate the checksum file")
	}
	if asset := release.FindSuffix("installer.exe"); asset == nil || asset.Size != 12345 {
		t.Errorf("FindSuffix returned %+v, want the installer", asset)
	}
	if asset := release.FindNamed("not-published.zip"); asset != nil {
		t.Error("an asset that does not exist must not be found")
	}
}

func TestLatestReportsFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{name: "rate limited", status: http.StatusForbidden, want: "rate limiting"},
		{name: "missing", status: http.StatusNotFound, want: "no published release"},
		{name: "server error", status: http.StatusInternalServerError, want: "replied with"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := releaseServer(t, "", test.status)

			client := NewClient("test")
			client.APIBaseURL = server.URL

			_, err := client.Latest(context.Background(), "owner", "repo")
			if err == nil {
				t.Fatal("the failure should be reported")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestDraftReleasesAreNotOffered(t *testing.T) {
	server := releaseServer(t, strings.Replace(samplePayload, `"draft": false`, `"draft": true`, 1), http.StatusOK)

	client := NewClient("test")
	client.APIBaseURL = server.URL

	if _, err := client.Latest(context.Background(), "owner", "repo"); err == nil {
		t.Error("a draft release must not be treated as published")
	}
}

// assetServer serves one asset and a checksum file.
func assetServer(t *testing.T, payload []byte, checksums string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "checksums.txt"):
			_, _ = writer.Write([]byte(checksums))
		case strings.HasSuffix(request.URL.Path, ".exe"), strings.HasSuffix(request.URL.Path, ".zip"):
			writer.Header().Set("Content-Length", fmt.Sprint(len(payload)))
			_, _ = writer.Write(payload)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// localClient points a client at a test server, relaxing the address policy for
// that host only through a field nothing outside this package can set.
func localClient(server *httptest.Server) (*Client, *Asset, *Asset) {
	client := NewClient("test")
	client.allowPlainHTTP = true

	host := strings.Split(strings.TrimPrefix(server.URL, "http://"), ":")[0]
	allowedHosts[host] = true

	asset := &Asset{Name: "GrabOne-1.4.0-installer.exe", URL: server.URL + "/GrabOne-1.4.0-installer.exe"}
	checksums := &Asset{Name: "checksums.txt", URL: server.URL + "/checksums.txt"}
	return client, asset, checksums
}

func TestDownloadVerifiedAcceptsAMatchingFile(t *testing.T) {
	payload := []byte("this stands in for a published asset")
	sum := sha256.Sum256(payload)
	server := assetServer(t, payload, hex.EncodeToString(sum[:])+"  GrabOne-1.4.0-installer.exe\n")

	client, asset, checksums := localClient(server)
	asset.Size = int64(len(payload))

	var last Progress
	path, err := client.DownloadVerified(context.Background(), asset, checksums, t.TempDir(), func(p Progress) {
		last = p
	})
	if err != nil {
		t.Fatalf("download: %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(contents) != string(payload) {
		t.Error("the stored file does not match what was served")
	}
	if last.DownloadedBytes != int64(len(payload)) {
		t.Errorf("progress = %+v, want the full size reported", last)
	}
	if last.Percent() < 99 {
		t.Errorf("percent = %v, want the download to read as complete", last.Percent())
	}
	if _, err := os.Stat(path + ".part"); err == nil {
		t.Error("the partial file was not cleaned up")
	}
}

func TestDownloadVerifiedRejectsATamperedFile(t *testing.T) {
	served := []byte("this is not what was published")
	otherSum := sha256.Sum256([]byte("the expected payload"))
	server := assetServer(t, served, hex.EncodeToString(otherSum[:])+"  GrabOne-1.4.0-installer.exe\n")

	client, asset, checksums := localClient(server)
	directory := t.TempDir()

	path, err := client.DownloadVerified(context.Background(), asset, checksums, directory, nil)
	if err == nil {
		t.Fatal("a file that fails its checksum must be refused")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %q, want it to name the checksum as the reason", err)
	}
	if path != "" {
		t.Errorf("path = %q, want nothing returned for a failed verification", path)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read the folder: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the folder still holds %d file(s); an unverified download must be discarded", len(entries))
	}
}

func TestDownloadVerifiedRejectsATruncatedFile(t *testing.T) {
	payload := []byte("short")
	sum := sha256.Sum256(payload)
	server := assetServer(t, payload, hex.EncodeToString(sum[:])+"  GrabOne-1.4.0-installer.exe\n")

	client, asset, checksums := localClient(server)
	asset.Size = int64(len(payload)) + 100

	if _, err := client.DownloadVerified(context.Background(), asset, checksums, t.TempDir(), nil); err == nil {
		t.Error("a download that stops early must be refused")
	}
}

func TestDownloadVerifiedNeedsAChecksum(t *testing.T) {
	server := assetServer(t, []byte("payload"), "")
	client, asset, _ := localClient(server)

	if _, err := client.DownloadVerified(context.Background(), asset, nil, t.TempDir(), nil); err == nil {
		t.Error("a download with no checksum file must be refused")
	}
}

func TestDownloadVerifiedNeedsTheFileListed(t *testing.T) {
	server := assetServer(t, []byte("payload"), strings.Repeat("b", 64)+"  something-else.exe\n")
	client, asset, checksums := localClient(server)

	if _, err := client.DownloadVerified(context.Background(), asset, checksums, t.TempDir(), nil); err == nil {
		t.Error("a file missing from the checksum list must be refused")
	}
}

func TestFindChecksum(t *testing.T) {
	body := "d41d8cd98f00b204e9800998ecf8427e  other.exe\n" +
		strings.Repeat("a", 64) + " *GrabOne-installer.exe\n" +
		"not-a-sum GrabOne-installer.exe\n"

	sum, ok := FindChecksum(body, "GrabOne-installer.exe")
	if !ok {
		t.Fatal("the checksum line was not found")
	}
	if sum != strings.Repeat("a", 64) {
		t.Errorf("sum = %q, want the 64 character hash", sum)
	}

	if _, ok := FindChecksum(body, "missing.exe"); ok {
		t.Error("a file that is not listed must not appear to have a checksum")
	}
	if _, ok := FindChecksum("d41d8cd98f00b204e9800998ecf8427e  short.exe", "short.exe"); ok {
		t.Error("a hash of the wrong length is not a SHA-256")
	}
}

func TestValidateURLPolicy(t *testing.T) {
	valid := []string{
		"https://github.com/owner/repo/releases/download/v1/GrabOne.exe",
		"https://objects.githubusercontent.com/whatever",
		"https://release-assets.githubusercontent.com/whatever",
	}
	for _, address := range valid {
		if err := validateURL(address, false); err != nil {
			t.Errorf("validateURL(%q) = %v, want it accepted", address, err)
		}
	}

	invalid := []string{
		"http://github.com/owner/repo/releases/download/v1/GrabOne.exe",
		"https://example.com/GrabOne.exe",
		"https://github.com.evil.test/GrabOne.exe",
		"ftp://github.com/GrabOne.exe",
		"not a url at all",
	}
	for _, address := range invalid {
		if err := validateURL(address, false); err == nil {
			t.Errorf("validateURL(%q) was accepted, want it refused", address)
		}
	}
}

func TestSafeFileName(t *testing.T) {
	tests := []struct{ input, want string }{
		{input: "GrabOne-1.4.0-installer.exe", want: "GrabOne-1.4.0-installer.exe"},
		{input: "../../evil.exe", want: "evil.exe"},
		{input: `C:\Windows\System32\evil.exe`, want: "evil.exe"},
		{input: "..", want: "download.bin"},
		{input: "  ", want: "download.bin"},
	}
	for _, test := range tests {
		if got := SafeFileName(test.input); got != test.want {
			t.Errorf("SafeFileName(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestProgressPercent(t *testing.T) {
	if got := (Progress{DownloadedBytes: 50, TotalBytes: 200}).Percent(); got != 25 {
		t.Errorf("percent = %v, want 25", got)
	}
	// A server that reports no length must not produce a nonsense percentage.
	if got := (Progress{DownloadedBytes: 50}).Percent(); got != 0 {
		t.Errorf("percent = %v, want 0 when the total is unknown", got)
	}
}

func TestDownloadedFileLandsInTheChosenFolder(t *testing.T) {
	payload := []byte("payload")
	sum := sha256.Sum256(payload)
	server := assetServer(t, payload, hex.EncodeToString(sum[:])+"  GrabOne-1.4.0-installer.exe\n")

	client, asset, checksums := localClient(server)
	directory := t.TempDir()

	path, err := client.DownloadVerified(context.Background(), asset, checksums, directory, nil)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if filepath.Dir(path) != directory {
		t.Errorf("path = %q, want it inside %q", path, directory)
	}
}
