package ghrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// downloadTimeout bounds fetching one asset.
const downloadTimeout = 30 * time.Minute

// maxChecksumSize bounds a checksum file, which is a list of short lines.
const maxChecksumSize = 1 << 20

// maxAssetSize is a sanity bound on a single download.
const maxAssetSize = 1 << 30

// Progress reports how far a download has come.
type Progress struct {
	DownloadedBytes int64
	TotalBytes      int64
}

// Percent renders the progress as a percentage, or zero when the total is not
// known.
func (p Progress) Percent() float64 {
	if p.TotalBytes <= 0 {
		return 0
	}
	return float64(p.DownloadedBytes) / float64(p.TotalBytes) * 100
}

// DownloadVerified fetches an asset into directory and checks it against the
// checksum published with the release.
//
// A file that cannot be verified is deleted rather than returned. Callers act
// on these downloads by running or installing them, so handing back something
// unverified is the one outcome that must not happen.
func (c *Client) DownloadVerified(
	ctx context.Context,
	asset *Asset,
	checksums *Asset,
	directory string,
	onProgress func(Progress),
) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("no file was published to download")
	}
	if checksums == nil {
		return "", fmt.Errorf("no checksum file was published, so the download cannot be verified")
	}
	if err := c.ValidateURL(asset.URL); err != nil {
		return "", err
	}
	if err := c.ValidateURL(checksums.URL); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	expected, err := c.expectedChecksum(ctx, checksums, asset.Name)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("prepare the download folder: %w", err)
	}

	target := filepath.Join(directory, SafeFileName(asset.Name))
	partial := target + ".part"
	_ = os.Remove(partial)

	sum, err := c.fetchTo(ctx, asset.URL, partial, asset.Size, onProgress)
	if err != nil {
		_ = os.Remove(partial)
		return "", err
	}

	if !strings.EqualFold(sum, expected) {
		_ = os.Remove(partial)
		return "", fmt.Errorf("%s does not match the checksum published with the release, so it was discarded", asset.Name)
	}

	if err := os.Rename(partial, target); err != nil {
		_ = os.Remove(partial)
		return "", fmt.Errorf("store the download: %w", err)
	}
	return target, nil
}

// fetchTo downloads a URL to a file and returns its SHA-256.
func (c *Client) fetchTo(ctx context.Context, address, destination string, expectedSize int64, onProgress func(Progress)) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", fmt.Errorf("build the download request: %w", err)
	}
	request.Header.Set("User-Agent", c.userAgent())
	request.Header.Set("Accept", "application/octet-stream")

	response, err := c.downloadClient().Do(request)
	if err != nil {
		return "", fmt.Errorf("download the file: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the download failed with %s", response.Status)
	}
	// A redirect may have taken the request elsewhere, so the address it ended
	// at is checked too.
	if response.Request != nil && response.Request.URL != nil {
		if err := c.ValidateURL(response.Request.URL.String()); err != nil {
			return "", err
		}
	}

	file, err := os.Create(destination)
	if err != nil {
		return "", fmt.Errorf("create the download file: %w", err)
	}
	defer file.Close()

	total := expectedSize
	if total <= 0 {
		total = response.ContentLength
	}

	hasher := sha256.New()
	written, err := copyWithProgress(io.MultiWriter(file, hasher), response.Body, total, onProgress)
	if err != nil {
		return "", err
	}
	if expectedSize > 0 && written != expectedSize {
		return "", fmt.Errorf("the download stopped early (%d of %d bytes)", written, expectedSize)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func copyWithProgress(destination io.Writer, source io.Reader, total int64, onProgress func(Progress)) (int64, error) {
	buffer := make([]byte, 256<<10)
	var written int64
	lastReport := time.Now()

	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			if _, writeErr := destination.Write(buffer[:count]); writeErr != nil {
				return written, fmt.Errorf("write the download: %w", writeErr)
			}
			written += int64(count)

			if written > maxAssetSize {
				return written, fmt.Errorf("the download is larger than expected and was stopped")
			}
			if onProgress != nil && time.Since(lastReport) > 200*time.Millisecond {
				lastReport = time.Now()
				onProgress(Progress{DownloadedBytes: written, TotalBytes: total})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, fmt.Errorf("download the file: %w", readErr)
		}
	}

	if onProgress != nil {
		onProgress(Progress{DownloadedBytes: written, TotalBytes: total})
	}
	return written, nil
}

// expectedChecksum reads the SHA-256 published for one file.
func (c *Client) expectedChecksum(ctx context.Context, checksums *Asset, fileName string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksums.URL, nil)
	if err != nil {
		return "", fmt.Errorf("build the checksum request: %w", err)
	}
	request.Header.Set("User-Agent", c.userAgent())

	response, err := c.downloadClient().Do(request)
	if err != nil {
		return "", fmt.Errorf("download the checksum file: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the checksum file could not be read (%s)", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxChecksumSize))
	if err != nil {
		return "", fmt.Errorf("read the checksum file: %w", err)
	}

	sum, ok := FindChecksum(string(body), fileName)
	if !ok {
		return "", fmt.Errorf("the checksum file does not list %s", fileName)
	}
	return sum, nil
}

// FindChecksum reads a line of the usual "<sha256>  <filename>" form.
func FindChecksum(contents, fileName string) (string, bool) {
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}

		sum := fields[0]
		// The name may carry a "*" marker for binary mode, and a path prefix.
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))

		if strings.EqualFold(name, fileName) && isHexSum(sum) {
			return sum, true
		}
	}
	return "", false
}

func isHexSum(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// SafeFileName keeps a published asset name from escaping the download folder.
func SafeFileName(name string) string {
	cleaned := filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "download.bin"
	}
	return cleaned
}

func (c *Client) downloadClient() *http.Client {
	if c.HTTPClient != nil {
		// The shared client's timeout suits a metadata request, not a download
		// of a hundred megabytes.
		client := *c.HTTPClient
		client.Timeout = 0
		return &client
	}
	return &http.Client{}
}
