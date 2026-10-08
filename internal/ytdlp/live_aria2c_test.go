//go:build integration

package ytdlp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// largeSingleFile is a single-file stream big enough to still be transferring
// when aria2c prints its first status lines, a second apart.
const largeSingleFile = "https://archive.org/download/BigBuckBunny_124/Content/big_buck_bunny_720p_surround.mp4"

// liveAria2c finds aria2c for the live tests: the copy named by
// GRABONE_TEST_ARIA2C, or one on PATH.
func liveAria2c(t *testing.T) string {
	t.Helper()

	if path := os.Getenv("GRABONE_TEST_ARIA2C"); path != "" {
		return path
	}
	path, err := exec.LookPath("aria2c")
	if err != nil {
		t.Skip("aria2c is not on PATH and GRABONE_TEST_ARIA2C is not set")
	}
	return path
}

func aria2cOptions(t *testing.T, aria2c string) DownloadOptions {
	return DownloadOptions{
		URL:              largeSingleFile,
		DownloadType:     DownloadTypeVideoOnly,
		OutputDirectory:  t.TempDir(),
		FilenameTemplate: "%(title)s.%(ext)s",
		Speed:            SpeedOptions{Connections: 8, ChunkedTransfer: true, Aria2cPath: aria2c},
	}
}

func TestLiveDownloadWithAria2cReportsProgress(t *testing.T) {
	client := liveClient(t)
	options := aria2cOptions(t, liveAria2c(t))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var (
		mutex     sync.Mutex
		during    []ProgressUpdate
		lastTotal float64
	)
	result, failure := client.Download(ctx, options, func(update ProgressUpdate) {
		mutex.Lock()
		defer mutex.Unlock()
		if update.OverallPercent > 0 {
			lastTotal = update.OverallPercent
		}
		if update.Percent > 0 && update.Percent < 100 && update.SpeedBytesPerSecond > 0 {
			during = append(during, update)
		}
	})
	if failure != nil {
		t.Fatalf("download: [%s] %s\n%s", failure.Kind, failure.Message, failure.Details)
	}

	info, err := os.Stat(result.FinalPath)
	if err != nil {
		t.Fatalf("the reported file is not there: %v", err)
	}
	t.Logf("downloaded %s (%d bytes) with %d progress reports during the transfer", result.FinalPath, info.Size(), len(during))
	for _, update := range during {
		t.Logf("  %5.1f%%  %d of %d bytes  %.0f B/s  eta %ds", update.Percent, update.DownloadedBytes, update.TotalBytes, update.SpeedBytesPerSecond, update.ETASeconds)
	}

	// yt-dlp reports nothing while aria2c transfers, so every one of these was
	// read from aria2c's own status line.
	if len(during) == 0 {
		t.Error("no progress was reported while aria2c transferred the file")
	}
	for _, update := range during {
		if update.TotalBytes != info.Size() {
			t.Errorf("a status line gave the size as %d, want the %d that was saved", update.TotalBytes, info.Size())
			break
		}
	}
	if lastTotal < 99 {
		t.Errorf("last progress = %.1f%%, want the finished line read even though it arrives behind a status line", lastTotal)
	}
	if leftover, _ := filepath.Glob(filepath.Join(options.OutputDirectory, "*.aria2")); len(leftover) > 0 {
		t.Errorf("aria2c left its control file behind: %v", leftover)
	}
}

func TestLiveCancelStopsAria2c(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the process check is written for Windows")
	}
	client := liveClient(t)
	aria2c := liveAria2c(t)
	options := aria2cOptions(t, aria2c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel as soon as aria2c has shown it is transferring.
	var once sync.Once
	_, failure := client.Download(ctx, options, func(update ProgressUpdate) {
		if update.SpeedBytesPerSecond > 0 {
			once.Do(cancel)
		}
	})
	if failure == nil {
		t.Skip("the download finished before it could be cancelled")
	}
	if failure.Kind != KindCancelled {
		t.Fatalf("failure = [%s] %s, want the cancellation", failure.Kind, failure.Message)
	}

	// yt-dlp started aria2c, and a cancelled download must not leave it running.
	deadline := time.Now().Add(10 * time.Second)
	for {
		output, err := exec.Command("wmic", "process", "where", "name='aria2c.exe'", "get", "ExecutablePath").Output()
		if err != nil {
			output, _ = exec.Command("powershell", "-NoProfile", "-Command",
				"Get-CimInstance Win32_Process -Filter \"Name='aria2c.exe'\" | ForEach-Object ExecutablePath").Output()
		}
		if !strings.Contains(strings.ToLower(string(output)), strings.ToLower(aria2c)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("aria2c is still running after the download was cancelled:\n%s", output)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
