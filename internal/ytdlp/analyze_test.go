package ytdlp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"grabone/internal/media"
)

// loadFixture decodes a stored yt-dlp result. The fixtures are real output,
// trimmed, so parsing is tested against what yt-dlp actually emits. No test in
// this package needs network access.
func loadFixture(t *testing.T, name string) *rawInfo {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var raw rawInfo
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return &raw
}

func TestNormalizeYouTubeVideo(t *testing.T) {
	raw := loadFixture(t, "youtube.json")
	info := normalizeInfo(raw, "https://www.youtube.com/watch?v=jNQXAC9IVRw")

	if info.Platform != "YouTube" {
		t.Errorf("platform = %q, want YouTube", info.Platform)
	}
	if info.PlatformSlug != "youtube" {
		t.Errorf("platform slug = %q, want youtube", info.PlatformSlug)
	}
	if info.ContentType != media.ContentTypeVideo {
		t.Errorf("content type = %q, want %q", info.ContentType, media.ContentTypeVideo)
	}
	if info.Title == "" {
		t.Error("title is empty")
	}
	if info.Duration <= 0 {
		t.Errorf("duration = %v, want a positive value", info.Duration)
	}
	if info.DurationText == "" {
		t.Error("duration text was not derived")
	}
	if info.ThumbnailURL == "" {
		t.Error("thumbnail is empty")
	}
	if info.UploadDate != "2005-04-24" {
		t.Errorf("upload date = %q, want 2005-04-24", info.UploadDate)
	}

	if len(info.Formats) == 0 {
		t.Fatal("no formats were normalized")
	}
	for _, format := range info.Formats {
		if format.Kind == media.FormatKindOther {
			t.Errorf("format %s: storyboards should not reach the format list", format.FormatID)
		}
		if format.Label == "" {
			t.Errorf("format %s: label was not built", format.FormatID)
		}
	}

	capabilities := info.Capabilities
	if !capabilities.HasVideo || !capabilities.HasAudio {
		t.Errorf("capabilities = %+v, want video and audio", capabilities)
	}
	if !capabilities.HasSeparateVideo || !capabilities.HasSeparateAudio {
		t.Error("YouTube exposes separate video and audio streams, which was not detected")
	}
	if !capabilities.HasChapters || len(info.Chapters) != 3 {
		t.Errorf("chapters = %d, want 3", len(info.Chapters))
	}
	if !capabilities.HasSubtitles {
		t.Error("manual subtitles were not detected")
	}
	if !capabilities.HasAutomaticCaptions {
		t.Error("automatic captions were not detected")
	}
	if capabilities.IsCollection {
		t.Error("a single video must not be reported as a collection")
	}
}

func TestNormalizeSubtitlesSeparatesAutomaticCaptions(t *testing.T) {
	raw := loadFixture(t, "youtube.json")
	info := normalizeInfo(raw, "https://www.youtube.com/watch?v=jNQXAC9IVRw")

	manual, automatic := 0, 0
	seen := map[string]bool{}
	for _, track := range info.Subtitles {
		if seen[track.Code] {
			t.Errorf("language %s appears twice", track.Code)
		}
		seen[track.Code] = true

		if track.Automatic {
			automatic++
		} else {
			manual++
		}
		if track.Name == "" {
			t.Errorf("language %s has no display name", track.Code)
		}
		if len(track.Formats) == 0 {
			t.Errorf("language %s has no formats", track.Code)
		}
	}

	if manual == 0 {
		t.Error("no manual subtitles were kept")
	}
	if automatic != 0 && manual != 0 {
		// Automatic captions for a language that also has manual subtitles are
		// dropped, because the manual track is always preferable.
		for _, track := range info.Subtitles {
			if track.Automatic && seen[track.Code] && !track.Automatic {
				t.Errorf("language %s kept both a manual and an automatic track", track.Code)
			}
		}
	}

	// Manual tracks must sort ahead of automatic ones.
	sawAutomatic := false
	for _, track := range info.Subtitles {
		if track.Automatic {
			sawAutomatic = true
			continue
		}
		if sawAutomatic {
			t.Error("a manual track was sorted after an automatic one")
			break
		}
	}
}

func TestNormalizeTikTokSingleStream(t *testing.T) {
	raw := loadFixture(t, "tiktok.json")
	info := normalizeInfo(raw, "https://www.tiktok.com/@tiktok/video/7106594312292453675")

	if info.Platform != "TikTok" {
		t.Errorf("platform = %q, want TikTok", info.Platform)
	}
	if info.IsCollection {
		t.Error("a TikTok video must not be reported as a collection")
	}
	if !info.Capabilities.HasCombined {
		t.Error("TikTok publishes combined streams, which was not detected")
	}
	if info.Uploader == "" {
		t.Error("uploader is empty")
	}
	if len(info.Formats) == 0 {
		t.Fatal("no formats were normalized")
	}

	// Every TikTok format carries both streams, so nothing needs merging.
	for _, format := range info.Formats {
		if format.Kind == media.FormatKindCombined && (!format.HasVideo || !format.HasAudio) {
			t.Errorf("format %s classified as combined without both streams", format.FormatID)
		}
	}
}

func TestNormalizeGenericExtractor(t *testing.T) {
	raw := loadFixture(t, "generic.json")
	info := normalizeInfo(raw, "https://download.blender.org/peach/trailer/trailer_400p.ogg")

	if info.Platform != "Web" {
		t.Errorf("platform = %q, want Web for the generic extractor", info.Platform)
	}
	if len(info.Formats) == 0 {
		t.Fatal("the generic extractor's single format was dropped")
	}
	if info.Title == "" {
		t.Error("title is empty")
	}
	// A missing duration must not break anything.
	if info.DurationText != "" && info.Duration == 0 {
		t.Error("a duration label was invented for media with no duration")
	}
}

func TestNormalizePlaylistListsEntriesWithoutResolvingThem(t *testing.T) {
	raw := loadFixture(t, "playlist.json")
	info := normalizeInfo(raw, "https://www.youtube.com/playlist?list=PLbpi6ZahtOH6Blw3RGYpWkSByi_T7Rygb")

	if !info.IsCollection {
		t.Fatal("a playlist result was not reported as a collection")
	}
	if info.ContentType != media.ContentTypePlaylist {
		t.Errorf("content type = %q, want %q", info.ContentType, media.ContentTypePlaylist)
	}
	if len(info.Entries) == 0 {
		t.Fatal("no entries were normalized")
	}
	if info.EntriesResolved {
		t.Error("lazily listed entries must be reported as unresolved")
	}

	for index, entry := range info.Entries {
		if entry.Index != index+1 {
			t.Errorf("entry %d has index %d, want %d", index, entry.Index, index+1)
		}
		if entry.Title == "" {
			t.Errorf("entry %d has no title", entry.Index)
		}
		if entry.WebpageURL == "" {
			t.Errorf("entry %d has no URL to resolve later", entry.Index)
		}
	}
}

func TestNormalizeInstagramCarousel(t *testing.T) {
	raw := loadFixture(t, "instagram.json")
	info := normalizeInfo(raw, "https://www.instagram.com/p/C5Y5V5YtJ5r/")

	if info.Platform != "Instagram" {
		t.Errorf("platform = %q, want Instagram", info.Platform)
	}
	if !info.IsCollection {
		t.Fatal("a multi-item post was not reported as a collection")
	}
	if info.ContentType != media.ContentTypeCarousel {
		t.Errorf("content type = %q, want %q", info.ContentType, media.ContentTypeCarousel)
	}
	if len(info.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(info.Entries))
	}

	// The first two entries are videos published inline, so their formats are
	// already known.
	for _, index := range []int{0, 1} {
		entry := info.Entries[index]
		if !entry.Resolved || len(entry.Formats) == 0 {
			t.Errorf("entry %d should carry formats from the same extraction", entry.Index)
		}
	}

	// The third entry is an image, which carries no audio or video stream. It
	// must still be listed rather than dropped or treated as an error.
	image := info.Entries[2]
	if image.Title == "" {
		t.Error("the image entry was not listed")
	}

	if !info.Capabilities.IsCollection {
		t.Error("capabilities do not report the collection")
	}
	if !info.Capabilities.HasVideo {
		t.Error("capabilities should be the union of the resolved entries")
	}
}

func TestNormalizeToleratesUnexpectedJSON(t *testing.T) {
	// Unknown fields, values of the wrong type and nulls all appear as yt-dlp
	// changes and as extractors differ. None of them may fail an analysis.
	payload := `{
		"_type": "video",
		"id": "abc",
		"title": "Odd one",
		"duration": "42.5",
		"view_count": null,
		"brand_new_field": {"nested": [1, 2, 3]},
		"extractor": "example",
		"extractor_key": "ExampleSite",
		"thumbnails": [{"url": "https://example.com/t.jpg", "preference": -1, "width": "640", "height": 360}],
		"formats": [
			{"format_id": "1", "ext": "mp4", "vcodec": "avc1.4d401e", "acodec": "mp4a.40.2",
			 "width": "1280", "height": "720", "fps": null, "filesize": null, "tbr": "1500.5",
			 "unexpected": true}
		]
	}`

	var raw rawInfo
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	info := normalizeInfo(&raw, "https://example.com/watch")

	if info.Duration != 42.5 {
		t.Errorf("duration = %v, want 42.5 parsed from a string", info.Duration)
	}
	if info.ViewCount != 0 {
		t.Errorf("view count = %d, want 0 for a null value", info.ViewCount)
	}
	if info.Platform != "Example Site" {
		t.Errorf("platform = %q, want the friendly extractor name", info.Platform)
	}
	if len(info.Formats) != 1 {
		t.Fatalf("formats = %d, want 1", len(info.Formats))
	}

	format := info.Formats[0]
	if format.Width != 1280 || format.Height != 720 {
		t.Errorf("size = %dx%d, want 1280x720 parsed from strings", format.Width, format.Height)
	}
	if format.TotalBitrate != 1500.5 {
		t.Errorf("bitrate = %v, want 1500.5", format.TotalBitrate)
	}
	if format.Kind != media.FormatKindCombined {
		t.Errorf("kind = %q, want combined", format.Kind)
	}
	if format.SizeLabel != "" {
		t.Errorf("size label = %q, want empty when no size is known", format.SizeLabel)
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr ErrorKind
	}{
		{name: "https link", input: "https://www.youtube.com/watch?v=abc", want: "https://www.youtube.com/watch?v=abc"},
		{name: "surrounding space", input: "  https://vimeo.com/123  ", want: "https://vimeo.com/123"},
		{name: "missing scheme", input: "tiktok.com/@user/video/1", want: "https://tiktok.com/@user/video/1"},
		{name: "unknown site is accepted", input: "https://some-new-site.example/v/9", want: "https://some-new-site.example/v/9"},
		{name: "empty", input: "   ", wantErr: KindInvalidURL},
		{name: "not http", input: "ftp://example.com/file.mp4", wantErr: KindInvalidURL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidateURL(test.input)

			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("ValidateURL(%q) succeeded, want error %q", test.input, test.wantErr)
				}
				if err.Kind != test.wantErr {
					t.Errorf("error kind = %q, want %q", err.Kind, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateURL(%q) failed: %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("ValidateURL(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestAnalyzeArgsNeverDownloadAndDefaultToASingleItem(t *testing.T) {
	client := NewClient("yt-dlp", "", nil)

	args := client.AnalyzeArgs(AnalyzeOptions{URL: "https://example.com/watch?v=1&list=PL1"})

	if !contains(args, "--dump-single-json") {
		t.Error("analysis must ask for structured output")
	}
	if !contains(args, "--flat-playlist") {
		t.Error("analysis must not resolve every entry of a collection")
	}
	if !contains(args, "--no-playlist") {
		t.Error("analysis must default to the single item the link points at")
	}
	if last := args[len(args)-1]; last != "https://example.com/watch?v=1&list=PL1" {
		t.Errorf("URL = %q, want it last", last)
	}
	if args[len(args)-2] != "--" {
		t.Error("the URL must be preceded by -- so it cannot be read as a switch")
	}

	whole := client.AnalyzeArgs(AnalyzeOptions{URL: "https://example.com/list", CollectionMode: CollectionModeAll})
	if !contains(whole, "--yes-playlist") {
		t.Error("collection analysis must opt into the playlist explicitly")
	}
}

func TestCollectWarnings(t *testing.T) {
	stderr := "WARNING: Falling back on generic information extractor\n" +
		"[youtube] Extracting URL\n" +
		"WARNING: Falling back on generic information extractor\n" +
		"WARNING: nsig extraction failed\n"

	warnings := collectWarnings(stderr)

	if len(warnings) != 2 {
		t.Fatalf("warnings = %d (%v), want 2 unique warnings", len(warnings), warnings)
	}
	if warnings[0] != "Falling back on generic information extractor" {
		t.Errorf("warning = %q, want the message without its prefix", warnings[0])
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
