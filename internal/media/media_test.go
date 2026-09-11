package media

import (
	"strings"
	"testing"
)

func TestFriendlyCodecNames(t *testing.T) {
	videoTests := []struct{ raw, want string }{
		{raw: "avc1.640028", want: "H.264"},
		{raw: "avc1.4d401e", want: "H.264"},
		{raw: "h264", want: "H.264"},
		{raw: "vp9", want: "VP9"},
		{raw: "vp09.00.50.08", want: "VP9"},
		{raw: "av01.0.05M.08", want: "AV1"},
		{raw: "hev1.1.6.L93.B0", want: "HEVC/H.265"},
		{raw: "hvc1.2.4.L120", want: "HEVC/H.265"},
		{raw: "none", want: ""},
		{raw: "", want: ""},
		{raw: "brandnewcodec1", want: "brandnewcodec1"},
	}
	for _, test := range videoTests {
		if got := FriendlyVideoCodec(test.raw); got != test.want {
			t.Errorf("FriendlyVideoCodec(%q) = %q, want %q", test.raw, got, test.want)
		}
	}

	audioTests := []struct{ raw, want string }{
		{raw: "mp4a.40.2", want: "AAC"},
		{raw: "mp4a.40.5", want: "AAC (HE)"},
		{raw: "opus", want: "Opus"},
		{raw: "vorbis", want: "Vorbis"},
		{raw: "ec-3", want: "Dolby Digital Plus"},
		{raw: "flac", want: "FLAC"},
		{raw: "none", want: ""},
		{raw: "somethingelse", want: "somethingelse"},
	}
	for _, test := range audioTests {
		if got := FriendlyAudioCodec(test.raw); got != test.want {
			t.Errorf("FriendlyAudioCodec(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestClassifyFormats(t *testing.T) {
	tests := []struct {
		name      string
		format    MediaFormat
		wantKind  FormatKind
		wantVideo bool
		wantAudio bool
	}{
		{
			name:      "combined stream",
			format:    MediaFormat{FormatID: "22", Extension: "mp4", RawVideoCodec: "avc1.64001F", RawAudioCodec: "mp4a.40.2"},
			wantKind:  FormatKindCombined,
			wantVideo: true,
			wantAudio: true,
		},
		{
			name:      "video only",
			format:    MediaFormat{FormatID: "137", Extension: "mp4", RawVideoCodec: "avc1.640028", RawAudioCodec: "none"},
			wantKind:  FormatKindVideo,
			wantVideo: true,
		},
		{
			name:      "audio only",
			format:    MediaFormat{FormatID: "140", Extension: "m4a", RawVideoCodec: "none", RawAudioCodec: "mp4a.40.2"},
			wantKind:  FormatKindAudio,
			wantAudio: true,
		},
		{
			name:     "storyboard",
			format:   MediaFormat{FormatID: "sb0", Extension: "mhtml", RawVideoCodec: "images", RawAudioCodec: "none", Protocol: "mhtml"},
			wantKind: FormatKindOther,
		},
		{
			name:     "image entry",
			format:   MediaFormat{FormatID: "0", Extension: "jpg"},
			wantKind: FormatKindOther,
		},
		{
			// A direct audio link reports that video is absent and says nothing
			// about the audio codec.
			name:      "audio file with no codec reported",
			format:    MediaFormat{FormatID: "ogg", Extension: "ogg", RawVideoCodec: "none"},
			wantKind:  FormatKindAudio,
			wantAudio: true,
		},
		{
			// A direct media link with nothing reported at all still plays.
			name:      "media file with nothing reported",
			format:    MediaFormat{FormatID: "mp4", Extension: "mp4"},
			wantKind:  FormatKindCombined,
			wantVideo: true,
			wantAudio: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			format := test.format
			Classify(&format)

			if format.Kind != test.wantKind {
				t.Errorf("kind = %q, want %q", format.Kind, test.wantKind)
			}
			if format.HasVideo != test.wantVideo {
				t.Errorf("HasVideo = %v, want %v", format.HasVideo, test.wantVideo)
			}
			if format.HasAudio != test.wantAudio {
				t.Errorf("HasAudio = %v, want %v", format.HasAudio, test.wantAudio)
			}
		})
	}
}

func TestFormatLabels(t *testing.T) {
	video := MediaFormat{
		FormatID: "137", Extension: "mp4", Width: 1920, Height: 1080, FPS: 60,
		RawVideoCodec: "avc1.640028", RawAudioCodec: "none", FileSize: 440401920,
	}
	Classify(&video)

	if video.QualityLabel != "1080p" {
		t.Errorf("quality = %q, want 1080p", video.QualityLabel)
	}
	for _, part := range []string{"1080p", "H.264", "60 FPS", "MP4", "420 MB"} {
		if !strings.Contains(video.Label, part) {
			t.Errorf("label %q is missing %q", video.Label, part)
		}
	}

	approximate := MediaFormat{
		FormatID: "571", Extension: "mp4", Width: 3840, Height: 2160, FPS: 60,
		RawVideoCodec: "av01.0.13M.08", RawAudioCodec: "none", FileSizeApprox: 1932735283,
	}
	Classify(&approximate)
	if !strings.Contains(approximate.Label, "~1.80 GB") {
		t.Errorf("label %q should mark an estimated size with a tilde", approximate.Label)
	}
	if approximate.QualityLabel != "2160p" {
		t.Errorf("quality = %q, want 2160p", approximate.QualityLabel)
	}

	unknownSize := MediaFormat{FormatID: "9", Extension: "mp4", Height: 720, RawVideoCodec: "avc1.4d401f", RawAudioCodec: "none"}
	Classify(&unknownSize)
	if unknownSize.SizeLabel != "" {
		t.Errorf("size label = %q, want it empty so the interface can say the size is unknown", unknownSize.SizeLabel)
	}

	audio := MediaFormat{FormatID: "140", Extension: "m4a", RawVideoCodec: "none", RawAudioCodec: "mp4a.40.2", AudioBitrate: 129.5}
	Classify(&audio)
	for _, part := range []string{"AAC", "130 kbps", "M4A"} {
		if !strings.Contains(audio.Label, part) {
			t.Errorf("audio label %q is missing %q", audio.Label, part)
		}
	}
}

func TestVerticalMediaIsLabelledByItsShortEdge(t *testing.T) {
	reel := MediaFormat{FormatID: "dash-hd", Extension: "mp4", Width: 1080, Height: 1920, RawVideoCodec: "avc1.64001f", RawAudioCodec: "mp4a.40.2"}
	Classify(&reel)

	if reel.QualityLabel != "1080p" {
		t.Errorf("quality = %q, want 1080p for a vertical video", reel.QualityLabel)
	}
	if reel.Resolution != "1080 × 1920" {
		t.Errorf("resolution = %q, want the full pixel size", reel.Resolution)
	}
}

func TestSortFormats(t *testing.T) {
	formats := []MediaFormat{
		{FormatID: "a", Height: 720, FPS: 30, TotalBitrate: 1000, RawVideoCodec: "avc1", RawAudioCodec: "none"},
		{FormatID: "b", Height: 1080, FPS: 30, TotalBitrate: 3000, RawVideoCodec: "avc1", RawAudioCodec: "none"},
		{FormatID: "c", Height: 1080, FPS: 60, TotalBitrate: 4000, RawVideoCodec: "vp9", RawAudioCodec: "none"},
		{FormatID: "d", Height: 2160, FPS: 30, TotalBitrate: 12000, RawVideoCodec: "av01", RawAudioCodec: "none"},
	}
	for index := range formats {
		Classify(&formats[index])
	}

	SortFormats(formats)

	want := []string{"d", "c", "b", "a"}
	for index, id := range want {
		if formats[index].FormatID != id {
			t.Fatalf("order = %v, want %v", ids(formats), want)
		}
	}

	audio := []MediaFormat{
		{FormatID: "low", RawVideoCodec: "none", RawAudioCodec: "opus", AudioBitrate: 70},
		{FormatID: "high", RawVideoCodec: "none", RawAudioCodec: "mp4a.40.2", AudioBitrate: 130},
	}
	for index := range audio {
		Classify(&audio[index])
	}
	SortFormats(audio)
	if audio[0].FormatID != "high" {
		t.Errorf("audio order = %v, want the higher bitrate first", ids(audio))
	}
}

func TestMarkRecommendedPrefersBroadlyCompatibleStreams(t *testing.T) {
	formats := []MediaFormat{
		{FormatID: "av1-4k", Extension: "mp4", Height: 2160, RawVideoCodec: "av01.0.13M.08", RawAudioCodec: "none"},
		{FormatID: "h264-1080", Extension: "mp4", Height: 1080, RawVideoCodec: "avc1.640028", RawAudioCodec: "none"},
		{FormatID: "h264-720", Extension: "mp4", Height: 720, RawVideoCodec: "avc1.4d401f", RawAudioCodec: "none"},
		{FormatID: "opus", RawVideoCodec: "none", RawAudioCodec: "opus", AudioBitrate: 160},
		{FormatID: "aac", Extension: "m4a", RawVideoCodec: "none", RawAudioCodec: "mp4a.40.2", AudioBitrate: 130},
	}
	for index := range formats {
		Classify(&formats[index])
	}

	MarkRecommended(formats)

	recommended := map[string]bool{}
	for _, format := range formats {
		if format.Recommended {
			recommended[format.FormatID] = true
		}
	}

	if !recommended["h264-1080"] {
		t.Error("the highest H.264 stream should be marked as broadly compatible")
	}
	if !recommended["aac"] {
		t.Error("the AAC stream should be marked as broadly compatible")
	}
	if recommended["av1-4k"] {
		t.Error("a recommendation is about compatibility, and AV1 is not the compatible choice")
	}
	if len(recommended) != 2 {
		t.Errorf("recommended = %v, want one per group", recommended)
	}
}

func TestResolutionPresets(t *testing.T) {
	formats := []MediaFormat{
		{FormatID: "a", Height: 2160, RawVideoCodec: "av01", RawAudioCodec: "none"},
		{FormatID: "b", Height: 1080, RawVideoCodec: "avc1", RawAudioCodec: "none"},
		{FormatID: "c", Height: 900, RawVideoCodec: "avc1", RawAudioCodec: "none"},
		{FormatID: "d", RawVideoCodec: "none", RawAudioCodec: "opus"},
	}
	for index := range formats {
		Classify(&formats[index])
	}

	presets := ResolutionPresets(formats)
	available := map[string]bool{}
	for _, preset := range presets {
		available[preset.Label] = preset.Available
	}

	if !available["Best"] || !available["4K"] || !available["1080p"] {
		t.Errorf("presets = %v, want the resolutions the media offers", available)
	}
	if !available["720p"] {
		t.Error("a 900p stream should count towards the 720p preset")
	}
	if available["1440p"] || available["480p"] || available["360p"] {
		t.Errorf("presets = %v, want unavailable resolutions disabled", available)
	}

	if len(ResolutionPresets(nil)) == 0 {
		t.Error("presets should still be listed, all unavailable, when there are no formats")
	}
	for _, preset := range ResolutionPresets(nil) {
		if preset.Available {
			t.Errorf("preset %s is available with no formats", preset.Label)
		}
	}
}

func TestNormalizePlatform(t *testing.T) {
	tests := []struct {
		extractorKey string
		extractor    string
		wantName     string
		wantSlug     string
	}{
		{extractorKey: "Youtube", extractor: "youtube", wantName: "YouTube", wantSlug: "youtube"},
		{extractorKey: "youtube:tab", extractor: "youtube:tab", wantName: "YouTube", wantSlug: "youtube"},
		{extractorKey: "Instagram", extractor: "Instagram", wantName: "Instagram", wantSlug: "instagram"},
		{extractorKey: "TikTok", extractor: "TikTok", wantName: "TikTok", wantSlug: "tiktok"},
		{extractorKey: "FacebookPluginsVideo", extractor: "facebook", wantName: "Facebook", wantSlug: "facebook"},
		{extractorKey: "twitter", extractor: "twitter", wantName: "Twitter/X", wantSlug: "twitter"},
		{extractorKey: "Vimeo", extractor: "vimeo", wantName: "Vimeo", wantSlug: "vimeo"},
		{extractorKey: "Generic", extractor: "generic", wantName: "Web", wantSlug: "web"},
		// An extractor with no rule keeps its own name rather than being refused.
		{extractorKey: "ARDBetaMediathek", extractor: "ard:mediathek", wantName: "ARD Beta Mediathek", wantSlug: "ard-beta-mediathek"},
		{extractorKey: "", extractor: "", wantName: "Web", wantSlug: "web"},
	}

	for _, test := range tests {
		t.Run(test.extractorKey, func(t *testing.T) {
			platform := NormalizePlatform(test.extractorKey, test.extractor)

			if platform.Name != test.wantName {
				t.Errorf("name = %q, want %q", platform.Name, test.wantName)
			}
			if platform.Slug != test.wantSlug {
				t.Errorf("slug = %q, want %q", platform.Slug, test.wantSlug)
			}
		})
	}
}

func TestAutomaticContainer(t *testing.T) {
	tests := []struct {
		name      string
		video     string
		audio     string
		embedSubs bool
		want      string
	}{
		{name: "h264 and aac", video: "avc1.640028", audio: "mp4a.40.2", want: ContainerMP4},
		{name: "hevc and aac", video: "hev1.1.6.L93.B0", audio: "mp4a.40.2", want: ContainerMP4},
		{name: "av1 and aac", video: "av01.0.05M.08", audio: "mp4a.40.2", want: ContainerMP4},
		{name: "vp9 and opus", video: "vp9", audio: "opus", want: ContainerWebM},
		{name: "vp9 and aac", video: "vp9", audio: "mp4a.40.2", want: ContainerMKV},
		{name: "h264 and opus", video: "avc1.640028", audio: "opus", want: ContainerMKV},
		{name: "vp9, opus and subtitles", video: "vp9", audio: "opus", embedSubs: true, want: ContainerMKV},
		{name: "no video", video: "", audio: "mp4a.40.2", want: ""},
		{name: "unknown codecs", video: "brandnew", audio: "brandnew", want: ContainerMKV},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := AutomaticContainer(test.video, test.audio, test.embedSubs); got != test.want {
				t.Errorf("AutomaticContainer(%q, %q, %v) = %q, want %q",
					test.video, test.audio, test.embedSubs, got, test.want)
			}
		})
	}
}

func TestCanEmbedSubtitles(t *testing.T) {
	if !CanEmbedSubtitles(ContainerMKV, "srt") || !CanEmbedSubtitles(ContainerMKV, "vtt") {
		t.Error("MKV accepts any subtitle format")
	}
	if !CanEmbedSubtitles(ContainerMP4, "srt") {
		t.Error("MP4 carries text subtitles as mov_text")
	}
	if CanEmbedSubtitles(ContainerWebM, "srt") {
		t.Error("WebM carries only WebVTT")
	}
	if !CanEmbedSubtitles(ContainerWebM, "vtt") {
		t.Error("WebM accepts WebVTT")
	}
}

func TestOptionValidation(t *testing.T) {
	if !IsValidContainer(ContainerAuto) || !IsValidContainer(ContainerMKV) || IsValidContainer("avi") {
		t.Error("container validation accepts only known values")
	}
	if !IsValidAudioConversion("mp3") || !IsValidAudioConversion(AudioConversionOriginal) || IsValidAudioConversion("aiff") {
		t.Error("audio conversion validation accepts only known values")
	}
	if !IsValidSubtitleFormat("srt") || IsValidSubtitleFormat("ass") {
		t.Error("subtitle format validation accepts only known values")
	}
	if AudioContainerFor("mp3") != "mp3" || AudioContainerFor(AudioConversionOriginal) != "" {
		t.Error("keeping the original stream has no fixed extension")
	}
}

func TestDetectCapabilities(t *testing.T) {
	info := &MediaInfo{
		ThumbnailURL: "https://example.com/thumb.jpg",
		Description:  "text",
		Chapters:     []Chapter{{Title: "Intro"}},
		Subtitles: []SubtitleLanguage{
			{Code: "en", Automatic: false},
			{Code: "de", Automatic: true},
		},
		Formats: []MediaFormat{
			{FormatID: "137", Kind: FormatKindVideo},
			{FormatID: "140", Kind: FormatKindAudio},
		},
	}

	capabilities := DetectCapabilities(info)

	if !capabilities.HasVideo || !capabilities.HasAudio {
		t.Error("separate streams should still report video and audio")
	}
	if !capabilities.HasSeparateVideo || !capabilities.HasSeparateAudio {
		t.Error("separate streams were not detected")
	}
	if capabilities.HasCombined {
		t.Error("no combined stream exists")
	}
	if !capabilities.HasSubtitles || !capabilities.HasAutomaticCaptions {
		t.Error("both subtitle kinds should be detected")
	}
	if !capabilities.HasChapters || !capabilities.HasThumbnail || !capabilities.HasDescription {
		t.Error("chapters, thumbnail and description were not detected")
	}
	if !capabilities.HasMultipleFormats {
		t.Error("two selectable formats is more than one")
	}

	single := &MediaInfo{Formats: []MediaFormat{{FormatID: "0", Kind: FormatKindCombined}}}
	if DetectCapabilities(single).HasMultipleFormats {
		t.Error("a single stream must not offer a choice of formats")
	}
	if !DetectCapabilities(single).HasCombined {
		t.Error("a combined stream was not detected")
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		info     MediaInfo
		playlist bool
		entries  int
		want     string
	}{
		{
			name: "youtube video",
			info: MediaInfo{ExtractorKey: "Youtube", WebpageURL: "https://www.youtube.com/watch?v=abc",
				Formats: []MediaFormat{{HasVideo: true, HasAudio: true}}},
			want: ContentTypeVideo,
		},
		{
			name: "youtube short",
			info: MediaInfo{ExtractorKey: "Youtube", WebpageURL: "https://www.youtube.com/shorts/abc",
				Formats: []MediaFormat{{HasVideo: true, HasAudio: true}}},
			want: ContentTypeShort,
		},
		{
			name: "instagram reel",
			info: MediaInfo{ExtractorKey: "Instagram", WebpageURL: "https://www.instagram.com/reel/abc/",
				Formats: []MediaFormat{{HasVideo: true, HasAudio: true}}},
			want: ContentTypeReel,
		},
		{
			name: "instagram story",
			info: MediaInfo{ExtractorKey: "InstagramStory", WebpageURL: "https://www.instagram.com/stories/user/123/",
				Formats: []MediaFormat{{HasVideo: true, HasAudio: true}}},
			want: ContentTypeStory,
		},
		{
			name: "audio only media",
			info: MediaInfo{ExtractorKey: "Generic", WebpageURL: "https://example.com/song.ogg",
				Formats: []MediaFormat{{HasAudio: true}}},
			want: ContentTypeAudio,
		},
		{
			name:     "instagram carousel",
			info:     MediaInfo{ExtractorKey: "Instagram", WebpageURL: "https://www.instagram.com/p/abc/"},
			playlist: true,
			entries:  4,
			want:     ContentTypeCarousel,
		},
		{
			name:     "youtube playlist",
			info:     MediaInfo{ExtractorKey: "youtube:tab", WebpageURL: "https://www.youtube.com/playlist?list=PL1"},
			playlist: true,
			entries:  12,
			want:     ContentTypePlaylist,
		},
		{
			name: "live stream",
			info: MediaInfo{ExtractorKey: "Twitch", LiveStatus: "is_live",
				Formats: []MediaFormat{{HasVideo: true, HasAudio: true}}},
			want: ContentTypeLive,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := test.info
			if got := DetectContentType(&info, test.playlist, test.entries); got != test.want {
				t.Errorf("content type = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLanguageNames(t *testing.T) {
	tests := []struct{ code, want string }{
		{code: "en", want: "English"},
		{code: "el", want: "Greek"},
		{code: "pt-BR", want: "Portuguese (Brazil)"},
		{code: "zh-Hans", want: "Chinese (Simplified)"},
		{code: "en-US", want: "English (US)"},
		{code: "xx", want: "xx"},
		{code: "", want: ""},
	}

	for _, test := range tests {
		if got := LanguageName(test.code); got != test.want {
			t.Errorf("LanguageName(%q) = %q, want %q", test.code, got, test.want)
		}
	}
}

func TestFormattingHelpers(t *testing.T) {
	sizes := []struct {
		bytes int64
		want  string
	}{
		{bytes: 913408, want: "892 KB"},
		{bytes: 15204352, want: "14.5 MB"},
		{bytes: 1363148898, want: "1.27 GB"},
		{bytes: 0, want: ""},
	}
	for _, test := range sizes {
		if got := FormatBytes(test.bytes); got != test.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", test.bytes, got, test.want)
		}
	}

	durations := []struct {
		seconds float64
		want    string
	}{
		{seconds: 222, want: "03:42"},
		{seconds: 4996, want: "01:23:16"},
		{seconds: 0, want: ""},
		{seconds: -5, want: ""},
	}
	for _, test := range durations {
		if got := FormatDuration(test.seconds); got != test.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", test.seconds, got, test.want)
		}
	}

	if got := FormatSpeed(13316915); got != "12.7 MB/s" {
		t.Errorf("FormatSpeed = %q, want 12.7 MB/s", got)
	}
	if got := FormatBitrate(4800); got != "4.8 Mbps" {
		t.Errorf("FormatBitrate = %q, want 4.8 Mbps", got)
	}
	if got := FormatBitrate(128); got != "128 kbps" {
		t.Errorf("FormatBitrate = %q, want 128 kbps", got)
	}
}

func ids(formats []MediaFormat) []string {
	out := make([]string, 0, len(formats))
	for _, format := range formats {
		out = append(out, format.FormatID)
	}
	return out
}
