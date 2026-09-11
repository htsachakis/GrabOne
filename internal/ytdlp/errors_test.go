package ytdlp

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClassifyError(t *testing.T) {
	// These messages are the real output of yt-dlp for each situation.
	tests := []struct {
		name     string
		output   string
		wantKind ErrorKind
		wantAuth bool
	}{
		{
			name: "vimeo requires a login",
			output: "ERROR: [vimeo] 76979871: The web client only works when logged-in. Use --cookies, " +
				"--cookies-from-browser, --username and --password, --netrc-cmd, or --netrc (vimeo) to provide account credentials",
			wantKind: KindAuthRequired,
			wantAuth: true,
		},
		{
			name: "instagram needs cookies",
			output: "ERROR: [Instagram] C5Y5V5YtJ5r: Instagram sent an empty media response. Check if this post is " +
				"accessible in your browser without being logged-in. If it is not, then use --cookies-from-browser or --cookies",
			wantKind: KindAuthRequired,
			wantAuth: true,
		},
		{
			name:     "age restricted",
			output:   "ERROR: [youtube] abc: Sign in to confirm your age. This video may be inappropriate for some users.",
			wantKind: KindAgeRestricted,
			wantAuth: true,
		},
		{
			name:     "private video",
			output:   "ERROR: [youtube] abc: Private video. Sign in if you've been granted access to this video",
			wantKind: KindPrivate,
			wantAuth: true,
		},
		{
			name:     "video removed",
			output:   "ERROR: [youtube] BaW_jenozKc: This video is unavailable",
			wantKind: KindUnavailable,
		},
		{
			name:     "page missing",
			output:   "ERROR: [vimeo] 148751763: Unable to download webpage: HTTP Error 404: Not Found",
			wantKind: KindUnavailable,
		},
		{
			name:     "no extractor",
			output:   "ERROR: Unsupported URL: https://example.com/not-media",
			wantKind: KindUnsupportedURL,
		},
		{
			name:     "format gone",
			output:   "ERROR: [youtube] abc: Requested format is not available. Use --list-formats for a list of available formats",
			wantKind: KindFormatUnavailable,
		},
		{
			name:     "ffmpeg missing",
			output:   "ERROR: You have requested merging of multiple formats but ffmpeg is not installed. Aborting due to --abort-on-error",
			wantKind: KindMissingDependency,
		},
		{
			name:     "disk full",
			output:   "ERROR: unable to write data: [Errno 28] No space left on device",
			wantKind: KindDiskFull,
		},
		{
			name:     "rate limited",
			output:   "ERROR: [tiktok] 123: Unable to download API page: HTTP Error 429: Too Many Requests",
			wantKind: KindNetwork,
		},
		{
			name:     "geo blocked",
			output:   "ERROR: [youtube] abc: The uploader has not made this video available in your country",
			wantKind: KindGeoBlocked,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classified := ClassifyError(test.output, errors.New("exit status 1"))

			if classified.Kind != test.wantKind {
				t.Errorf("kind = %q, want %q", classified.Kind, test.wantKind)
			}
			if classified.Message == "" {
				t.Error("no readable message was produced")
			}
			if classified.Details != strings.TrimSpace(test.output) {
				t.Error("the raw output must be preserved for troubleshooting")
			}
			if classified.NeedsAuthentication() != test.wantAuth {
				t.Errorf("NeedsAuthentication = %v, want %v", classified.NeedsAuthentication(), test.wantAuth)
			}
		})
	}
}

func TestClassifyErrorKeepsUnknownMessages(t *testing.T) {
	output := "[generic] Extracting URL: https://example.com\nERROR: [generic] Something entirely new went wrong"

	classified := ClassifyError(output, errors.New("exit status 1"))

	if classified.Kind != KindUnknown {
		t.Errorf("kind = %q, want unknown", classified.Kind)
	}
	if classified.Message != "Something entirely new went wrong" {
		t.Errorf("message = %q, want yt-dlp's own message without its prefixes", classified.Message)
	}
}

func TestClassifyErrorContextFailures(t *testing.T) {
	if classified := ClassifyError("", context.Canceled); classified.Kind != KindCancelled {
		t.Errorf("kind = %q, want cancelled", classified.Kind)
	}
	if classified := ClassifyError("", context.DeadlineExceeded); classified.Kind != KindTimeout {
		t.Errorf("kind = %q, want timeout", classified.Kind)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "2026.08.19\n", want: "2026.08.19"},
		{input: "2026.08.19.123456", want: "2026.08.19.123456"},
		{input: "  2025.01.15  ", want: "2025.01.15"},
		{input: "nightly build\n", want: "nightly build"},
		{input: "", want: ""},
	}

	for _, test := range tests {
		if got := ParseVersion(test.input); got != test.want {
			t.Errorf("ParseVersion(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}
