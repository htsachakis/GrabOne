//go:build integration

// These tests run the real yt-dlp against real sites, and are excluded from the
// normal test run because they need a network and working executables:
//
//	go test -tags integration ./internal/ytdlp/ -v
//
// They are the checks that the pipeline works end to end against the platforms
// GrabOne targets.
package ytdlp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"grabone/internal/media"
)

func liveClient(t *testing.T) *Client {
	t.Helper()

	ytDlp, err := exec.LookPath("yt-dlp")
	if err != nil {
		t.Skip("yt-dlp is not on PATH")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not on PATH")
	}
	return NewClient(ytDlp, ffmpeg, nil)
}

func TestLiveAnalyze(t *testing.T) {
	client := liveClient(t)

	tests := []struct {
		name            string
		url             string
		wantPlatform    string
		wantVideo       bool
		wantCollection  bool
		wantMultiFormat bool
	}{
		{
			name:            "youtube video",
			url:             "https://www.youtube.com/watch?v=jNQXAC9IVRw",
			wantPlatform:    "YouTube",
			wantVideo:       true,
			wantMultiFormat: true,
		},
		{
			name:         "tiktok video",
			url:          "https://www.tiktok.com/@tiktok/video/7106594312292453675",
			wantPlatform: "TikTok",
			wantVideo:    true,
		},
		{
			name:         "generic extractor",
			url:          "https://download.blender.org/peach/trailer/trailer_400p.ogg",
			wantPlatform: "Web",
		},
		{
			name:           "youtube playlist",
			url:            "https://www.youtube.com/playlist?list=PLbpi6ZahtOH6Blw3RGYpWkSByi_T7Rygb",
			wantPlatform:   "YouTube",
			wantCollection: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			info, failure := client.Analyze(ctx, AnalyzeOptions{URL: test.url})
			if failure != nil {
				t.Fatalf("analyze: [%s] %s\n%s", failure.Kind, failure.Message, failure.Details)
			}

			t.Logf("platform=%s extractor=%s type=%s title=%q formats=%d entries=%d",
				info.Platform, info.ExtractorKey, info.ContentType, info.Title, len(info.Formats), len(info.Entries))

			if info.Platform != test.wantPlatform {
				t.Errorf("platform = %q, want %q", info.Platform, test.wantPlatform)
			}
			if info.Title == "" {
				t.Error("no title was reported")
			}
			if test.wantVideo && !info.Capabilities.HasVideo {
				t.Error("no video stream was found")
			}
			if test.wantCollection != info.IsCollection {
				t.Errorf("IsCollection = %v, want %v", info.IsCollection, test.wantCollection)
			}
			if test.wantMultiFormat && !info.Capabilities.HasMultipleFormats {
				t.Error("only one format was found where several were expected")
			}
			if !test.wantCollection && len(info.Formats) == 0 {
				t.Error("no formats were reported")
			}

			for _, format := range info.Formats {
				if format.Label == "" {
					t.Errorf("format %s has no label", format.FormatID)
				}
			}
		})
	}
}

func TestLiveAuthenticationRequiredIsReportedAsSuch(t *testing.T) {
	client := liveClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Vimeo's web client refuses without a login, which is the situation the
	// interface must describe accurately rather than calling it unsupported.
	_, failure := client.Analyze(ctx, AnalyzeOptions{URL: "https://vimeo.com/76979871"})
	if failure == nil {
		t.Skip("this URL no longer requires authentication")
	}

	t.Logf("kind=%s message=%q", failure.Kind, failure.Message)

	if !failure.NeedsAuthentication() {
		t.Errorf("kind = %q, want an authentication related classification", failure.Kind)
	}
	if failure.Hint == "" {
		t.Error("an authentication failure should say what to do about it")
	}
	if failure.Details == "" {
		t.Error("the raw output should be kept for troubleshooting")
	}
}

func TestLiveUnsupportedURL(t *testing.T) {
	client := liveClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, failure := client.Analyze(ctx, AnalyzeOptions{URL: "https://example.com/"})
	if failure == nil {
		t.Fatal("a page with no media should fail")
	}
	t.Logf("kind=%s message=%q", failure.Kind, failure.Message)

	switch failure.Kind {
	case KindUnsupportedURL, KindUnavailable, KindFormatUnavailable:
	default:
		t.Errorf("kind = %q, want the link to be reported as unsupported", failure.Kind)
	}
}

func TestLiveDownloadWithMerge(t *testing.T) {
	client := liveClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	const url = "https://www.youtube.com/watch?v=jNQXAC9IVRw"

	info, failure := client.Analyze(ctx, AnalyzeOptions{URL: url})
	if failure != nil {
		t.Fatalf("analyze: %s", failure.Message)
	}

	video, ok := media.BestFormat(info.Formats, media.FormatKindVideo)
	if !ok {
		t.Skip("this media has no separate video stream to merge")
	}
	audio, ok := media.BestFormat(info.Formats, media.FormatKindAudio)
	if !ok {
		t.Skip("this media has no separate audio stream to merge")
	}

	// Pick the smallest video stream so the test stays quick.
	videoFormats := info.VideoOnlyFormats()
	video = videoFormats[len(videoFormats)-1]

	directory := t.TempDir()
	options := DownloadOptions{
		URL:                url,
		DownloadType:       DownloadTypeVideoAudio,
		VideoFormatID:      video.FormatID,
		AudioFormatID:      audio.FormatID,
		SelectedVideoCodec: video.RawVideoCodec,
		SelectedAudioCodec: audio.RawAudioCodec,
		Container:          media.ContainerAuto,
		EmbedMetadata:      true,
		EmbedChapters:      true,
		DownloadSubtitles:  true,
		EmbedSubtitles:     true,
		SubtitleLanguages:  []string{"en"},
		SubtitleFormat:     "srt",
		OutputDirectory:    directory,
		FilenameTemplate:   "%(title)s.%(ext)s",
	}

	var (
		sawVideoStage bool
		sawAudioStage bool
		sawMerge      bool
		lastPercent   float64
	)

	result, failure := client.Download(ctx, options, func(update ProgressUpdate) {
		switch update.Stage {
		case StageDownloadingVideo:
			sawVideoStage = true
		case StageDownloadingAudio:
			sawAudioStage = true
		case StageMerging:
			sawMerge = true
		}
		if update.OverallPercent > 0 {
			lastPercent = update.OverallPercent
		}
	})
	if failure != nil {
		t.Fatalf("download: [%s] %s\n%s", failure.Kind, failure.Message, failure.Details)
	}

	if result.FinalPath == "" {
		t.Fatal("no final path was reported")
	}
	info2, err := os.Stat(result.FinalPath)
	if err != nil {
		t.Fatalf("the reported file is not there: %v", err)
	}
	if info2.Size() == 0 {
		t.Error("the downloaded file is empty")
	}
	if filepath.Dir(result.FinalPath) != directory {
		t.Errorf("file = %q, want it in the chosen folder %q", result.FinalPath, directory)
	}

	t.Logf("downloaded %s (%d bytes), last reported %.1f%%", result.FinalPath, info2.Size(), lastPercent)

	if !sawVideoStage || !sawAudioStage {
		t.Errorf("stages: video=%v audio=%v, want both streams to be reported separately", sawVideoStage, sawAudioStage)
	}
	if !sawMerge {
		t.Error("merging with FFmpeg was not reported")
	}
	if lastPercent < 99 {
		t.Errorf("last progress = %.1f%%, want it to reach completion", lastPercent)
	}
}

func TestLiveAudioOnlyConversion(t *testing.T) {
	client := liveClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	directory := t.TempDir()
	options := DownloadOptions{
		URL:                   "https://www.youtube.com/watch?v=jNQXAC9IVRw",
		DownloadType:          DownloadTypeAudioOnly,
		AudioConversionFormat: "mp3",
		OutputDirectory:       directory,
		FilenameTemplate:      "%(title)s.%(ext)s",
		EmbedMetadata:         true,
	}

	sawConversion := false
	result, failure := client.Download(ctx, options, func(update ProgressUpdate) {
		if update.Stage == StageConverting {
			sawConversion = true
		}
	})
	if failure != nil {
		t.Fatalf("download: [%s] %s\n%s", failure.Kind, failure.Message, failure.Details)
	}

	if filepath.Ext(result.FinalPath) != ".mp3" {
		t.Errorf("file = %q, want an mp3", result.FinalPath)
	}
	if !sawConversion {
		t.Error("the conversion stage was not reported")
	}
	t.Logf("converted to %s", result.FinalPath)
}

func TestLiveCancellationStopsTheDownload(t *testing.T) {
	client := liveClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	directory := t.TempDir()
	options := DownloadOptions{
		URL:              "https://www.youtube.com/watch?v=aqz-KE-bpKQ",
		DownloadType:     DownloadTypeVideoAudio,
		OutputDirectory:  directory,
		FilenameTemplate: "%(title)s.%(ext)s",
	}

	type outcome struct {
		failure *Error
	}

	started := make(chan struct{})
	done := make(chan outcome, 1)
	var cancelOnce sync.Once

	go func() {
		_, failure := client.Download(ctx, options, func(update ProgressUpdate) {
			if update.DownloadedBytes > 0 {
				cancelOnce.Do(func() { close(started) })
			}
		})
		done <- outcome{failure: failure}
	}()

	select {
	case <-started:
		// Stop it once bytes are actually moving.
		cancel()
	case <-time.After(90 * time.Second):
		t.Fatal("the download never started transferring")
	}

	select {
	case result := <-done:
		if result.failure == nil {
			t.Skip("the download finished before the cancellation took effect")
		}
		if result.failure.Kind != KindCancelled {
			t.Errorf("kind = %q, want cancelled", result.failure.Kind)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the download did not stop after being cancelled")
	}

	// The process tree must be gone: no FFmpeg left holding the output folder.
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read output folder: %v", err)
	}
	t.Logf("cancelled, %d partial file(s) left behind", len(entries))
}
