package ytdlp

import (
	"strings"
	"testing"

	"grabone/internal/media"
)

// baseOptions is a valid starting point each test narrows to its own case.
func baseOptions() DownloadOptions {
	return DownloadOptions{
		URL:              "https://www.youtube.com/watch?v=jNQXAC9IVRw",
		DownloadType:     DownloadTypeVideoAudio,
		Container:        media.ContainerAuto,
		OutputDirectory:  `C:\Users\example\Videos\GrabOne`,
		FilenameTemplate: "%(title)s.%(ext)s",
	}
}

// argValue returns the value following a switch, and whether the switch is present.
func argValue(args []string, flag string) (string, bool) {
	for index, arg := range args {
		if arg == flag && index+1 < len(args) {
			return args[index+1], true
		}
	}
	return "", false
}

func hasArg(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func TestBuildDownloadArgsFormatSelectors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*DownloadOptions)
		want    string
		wantNot []string
	}{
		{
			name: "separate video and audio are merged",
			mutate: func(options *DownloadOptions) {
				options.VideoFormatID = "137"
				options.AudioFormatID = "140"
			},
			want: "137+140/137+ba/137",
		},
		{
			name: "a combined stream is used as given",
			mutate: func(options *DownloadOptions) {
				options.CombinedFormatID = "22"
			},
			want: "22",
		},
		{
			name: "a video stream alone still gets audio",
			mutate: func(options *DownloadOptions) {
				options.VideoFormatID = "616"
			},
			want: "616+ba/616",
		},
		{
			name:   "nothing chosen falls back to the best streams",
			mutate: func(options *DownloadOptions) {},
			want:   "bv*+ba/b",
		},
		{
			name: "a resolution ceiling becomes a filter",
			mutate: func(options *DownloadOptions) {
				options.MaxHeight = 1080
			},
			want: "bv*[height<=1080]+ba/b[height<=1080]/bv*+ba/b",
		},
		{
			name: "video only",
			mutate: func(options *DownloadOptions) {
				options.DownloadType = DownloadTypeVideoOnly
				options.VideoFormatID = "137"
			},
			want: "137",
		},
		{
			name: "audio only",
			mutate: func(options *DownloadOptions) {
				options.DownloadType = DownloadTypeAudioOnly
				options.AudioFormatID = "251"
			},
			want: "251",
		},
		{
			name: "audio only with no choice",
			mutate: func(options *DownloadOptions) {
				options.DownloadType = DownloadTypeAudioOnly
			},
			want: "ba/b",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := baseOptions()
			test.mutate(&options)

			args, err := BuildDownloadArgs(options)
			if err != nil {
				t.Fatalf("BuildDownloadArgs: %v", err)
			}

			selector, ok := argValue(args, "-f")
			if !ok {
				t.Fatal("no format selector was produced")
			}
			if selector != test.want {
				t.Errorf("selector = %q, want %q", selector, test.want)
			}
		})
	}
}

func TestBuildDownloadArgsAutomaticContainer(t *testing.T) {
	tests := []struct {
		name       string
		videoCodec string
		audioCodec string
		embedSubs  bool
		want       string
	}{
		{name: "h264 and aac become mp4", videoCodec: "avc1.640028", audioCodec: "mp4a.40.2", want: media.ContainerMP4},
		{name: "vp9 and opus become webm", videoCodec: "vp9", audioCodec: "opus", want: media.ContainerWebM},
		{name: "mixed codecs become mkv", videoCodec: "vp9", audioCodec: "mp4a.40.2", want: media.ContainerMKV},
		{name: "av1 with aac stays mp4", videoCodec: "av01.0.05M.08", audioCodec: "mp4a.40.2", want: media.ContainerMP4},
		{name: "webm with embedded subtitles becomes mkv", videoCodec: "vp9", audioCodec: "opus", embedSubs: true, want: media.ContainerMKV},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := baseOptions()
			options.VideoFormatID = "1"
			options.AudioFormatID = "2"
			options.SelectedVideoCodec = test.videoCodec
			options.SelectedAudioCodec = test.audioCodec
			if test.embedSubs {
				options.DownloadSubtitles = true
				options.EmbedSubtitles = true
				options.SubtitleLanguages = []string{"en"}
				options.SubtitleFormat = "srt"
			}

			args, err := BuildDownloadArgs(options)
			if err != nil {
				t.Fatalf("BuildDownloadArgs: %v", err)
			}

			container, ok := argValue(args, "--merge-output-format")
			if !ok {
				t.Fatal("no merge container was chosen")
			}
			if container != test.want {
				t.Errorf("container = %q, want %q", container, test.want)
			}
			if hasArg(args, "--remux-video") {
				t.Error("automatic mode must not force a remux")
			}
		})
	}
}

func TestBuildDownloadArgsExplicitContainerRemuxes(t *testing.T) {
	options := baseOptions()
	options.Container = media.ContainerMKV
	options.CombinedFormatID = "22"

	args, err := BuildDownloadArgs(options)
	if err != nil {
		t.Fatalf("BuildDownloadArgs: %v", err)
	}

	if container, _ := argValue(args, "--remux-video"); container != media.ContainerMKV {
		t.Errorf("remux target = %q, want mkv", container)
	}
	// Remuxing rewrites the container; it must never become a re-encode.
	for _, arg := range args {
		if arg == "--recode-video" {
			t.Error("changing the container must not re-encode the video")
		}
	}
}

func TestBuildDownloadArgsAudioConversion(t *testing.T) {
	options := baseOptions()
	options.DownloadType = DownloadTypeAudioOnly
	options.AudioFormatID = "140"

	t.Run("original keeps the stream", func(t *testing.T) {
		options.AudioConversionFormat = media.AudioConversionOriginal

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if !hasArg(args, "--extract-audio") {
			t.Error("audio-only downloads must extract the audio stream")
		}
		if hasArg(args, "--audio-format") {
			t.Error("keeping the original stream must not request a conversion")
		}
	})

	t.Run("mp3 converts with ffmpeg", func(t *testing.T) {
		options.AudioConversionFormat = "mp3"

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if format, _ := argValue(args, "--audio-format"); format != "mp3" {
			t.Errorf("audio format = %q, want mp3", format)
		}
		if hasArg(args, "--merge-output-format") {
			t.Error("an audio-only download has no video container to choose")
		}
	})
}

func TestBuildDownloadArgsSubtitles(t *testing.T) {
	t.Run("embedded only leaves no file", func(t *testing.T) {
		options := baseOptions()
		options.DownloadSubtitles = true
		options.EmbedSubtitles = true
		options.KeepSubtitles = false
		options.SubtitleLanguages = []string{"en", "el"}
		options.SubtitleFormat = "srt"

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if hasArg(args, "--write-subs") {
			t.Error("embedding without keeping must not write a subtitle file")
		}
		if !hasArg(args, "--embed-subs") {
			t.Error("subtitles were not embedded")
		}
		if languages, _ := argValue(args, "--sub-langs"); languages != "en,el" {
			t.Errorf("languages = %q, want en,el", languages)
		}
		if format, _ := argValue(args, "--convert-subs"); format != "srt" {
			t.Errorf("converted format = %q, want srt", format)
		}
	})

	t.Run("keeping the file writes it too", func(t *testing.T) {
		options := baseOptions()
		options.DownloadSubtitles = true
		options.EmbedSubtitles = true
		options.KeepSubtitles = true
		options.SubtitleLanguages = []string{"en"}
		options.SubtitleFormat = "vtt"

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if !hasArg(args, "--write-subs") || !hasArg(args, "--embed-subs") {
			t.Error("both the file and the embedded track were requested, but not both were produced")
		}
	})

	t.Run("automatic captions are requested explicitly", func(t *testing.T) {
		options := baseOptions()
		options.DownloadSubtitles = true
		options.SubtitleLanguages = []string{"en"}
		options.SubtitleFormat = "srt"
		options.IncludeAutomaticCaptions = true

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if !hasArg(args, "--write-auto-subs") {
			t.Error("automatic captions need their own switch")
		}
	})

	t.Run("webm falls back to a compatible subtitle format", func(t *testing.T) {
		options := baseOptions()
		options.Container = media.ContainerWebM
		options.DownloadSubtitles = true
		options.EmbedSubtitles = true
		options.SubtitleLanguages = []string{"en"}
		options.SubtitleFormat = "srt"

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if format, _ := argValue(args, "--convert-subs"); format != "vtt" {
			t.Errorf("subtitle format = %q, want vtt for WebM", format)
		}
	})

	t.Run("no languages means no subtitle switches", func(t *testing.T) {
		options := baseOptions()
		options.DownloadSubtitles = true
		options.SubtitleLanguages = nil

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if hasArg(args, "--sub-langs") {
			t.Error("subtitles were requested with no language selected")
		}
	})
}

func TestBuildDownloadArgsThumbnailOnWebM(t *testing.T) {
	options := baseOptions()
	options.Container = media.ContainerWebM
	options.EmbedThumbnail = true

	args, err := BuildDownloadArgs(options)
	if err != nil {
		t.Fatalf("BuildDownloadArgs: %v", err)
	}

	// WebM cannot hold cover art, so the thumbnail is saved beside the file.
	if hasArg(args, "--embed-thumbnail") {
		t.Error("a thumbnail must not be embedded into WebM")
	}
	if !hasArg(args, "--write-thumbnail") {
		t.Error("the thumbnail should be saved separately instead")
	}
}

func TestBuildDownloadArgsCollectionModes(t *testing.T) {
	t.Run("single by default", func(t *testing.T) {
		args, err := BuildDownloadArgs(baseOptions())
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if !hasArg(args, "--no-playlist") {
			t.Error("a link belonging to a playlist must download one item by default")
		}
	})

	t.Run("selected items", func(t *testing.T) {
		options := baseOptions()
		options.CollectionMode = CollectionModeSelected
		options.PlaylistItems = "1,3,5-7"

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if !hasArg(args, "--yes-playlist") {
			t.Error("downloading several items requires opting into the playlist")
		}
		if items, _ := argValue(args, "--playlist-items"); items != "1,3,5-7" {
			t.Errorf("items = %q, want 1,3,5-7", items)
		}
		template, _ := argValue(args, "-o")
		if !strings.Contains(template, "playlist_index") {
			t.Errorf("template = %q, want collection items numbered on disk", template)
		}
	})

	t.Run("whole collection", func(t *testing.T) {
		options := baseOptions()
		options.CollectionMode = CollectionModeAll

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if !hasArg(args, "--yes-playlist") || hasArg(args, "--no-playlist") {
			t.Error("the whole collection was not requested")
		}
	})
}

func TestBuildDownloadArgsOutput(t *testing.T) {
	options := baseOptions()

	args, err := BuildDownloadArgs(options)
	if err != nil {
		t.Fatalf("BuildDownloadArgs: %v", err)
	}

	if template, _ := argValue(args, "-o"); template != "%(title)s.%(ext)s" {
		t.Errorf("template = %q, want the default", template)
	}
	if path, _ := argValue(args, "--paths"); path != options.OutputDirectory {
		t.Errorf("output folder = %q, want %q", path, options.OutputDirectory)
	}
	if args[len(args)-1] != options.URL || args[len(args)-2] != "--" {
		t.Error("the URL must come last, after --")
	}
}

func TestBuildDownloadArgsCookies(t *testing.T) {
	t.Run("browser", func(t *testing.T) {
		options := baseOptions()
		options.Cookies = CookieOptions{Source: CookieSourceBrowser, Browser: "firefox"}

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if browser, _ := argValue(args, "--cookies-from-browser"); browser != "firefox" {
			t.Errorf("browser = %q, want firefox", browser)
		}
	})

	t.Run("file", func(t *testing.T) {
		options := baseOptions()
		options.Cookies = CookieOptions{Source: CookieSourceFile, File: `C:\cookies.txt`}

		args, err := BuildDownloadArgs(options)
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if path, _ := argValue(args, "--cookies"); path != `C:\cookies.txt` {
			t.Errorf("cookie file = %q, want the selected file", path)
		}
	})

	t.Run("none", func(t *testing.T) {
		args, err := BuildDownloadArgs(baseOptions())
		if err != nil {
			t.Fatalf("BuildDownloadArgs: %v", err)
		}
		if hasArg(args, "--cookies") || hasArg(args, "--cookies-from-browser") {
			t.Error("no authentication was configured, so no cookie switches belong in the command")
		}
	})
}

func TestValidateRejectsUnknownValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DownloadOptions)
	}{
		{name: "unknown download type", mutate: func(o *DownloadOptions) { o.DownloadType = "everything" }},
		{name: "unknown container", mutate: func(o *DownloadOptions) { o.Container = "avi" }},
		{name: "unknown audio format", mutate: func(o *DownloadOptions) {
			o.DownloadType = DownloadTypeAudioOnly
			o.AudioConversionFormat = "aiff"
		}},
		{name: "unknown subtitle format", mutate: func(o *DownloadOptions) {
			o.DownloadSubtitles = true
			o.SubtitleFormat = "ass"
			o.SubtitleLanguages = []string{"en"}
		}},
		{name: "no url", mutate: func(o *DownloadOptions) { o.URL = "" }},
		{name: "unknown cookie browser", mutate: func(o *DownloadOptions) {
			o.Cookies = CookieOptions{Source: CookieSourceBrowser, Browser: "netscape"}
		}},
		{name: "invalid item selection", mutate: func(o *DownloadOptions) {
			o.CollectionMode = CollectionModeSelected
			o.PlaylistItems = "1; rm -rf"
		}},
		{
			// A format identifier always comes from the analysis, so anything
			// carrying selector syntax is a sign of tampering.
			name:   "format identifier carrying extra syntax",
			mutate: func(o *DownloadOptions) { o.VideoFormatID = "137+140/best[ext=mp4]" },
		},
		{name: "subtitle language with separators", mutate: func(o *DownloadOptions) {
			o.DownloadSubtitles = true
			o.SubtitleLanguages = []string{"en,--exec=calc"}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := baseOptions()
			test.mutate(&options)

			if _, err := BuildDownloadArgs(options); err == nil {
				t.Error("invalid options were accepted")
			}
		})
	}
}

func TestReportingArgsAreSeparateFromTheUserCommand(t *testing.T) {
	args, err := BuildDownloadArgs(baseOptions())
	if err != nil {
		t.Fatalf("BuildDownloadArgs: %v", err)
	}

	for _, plumbing := range []string{"--progress-template", "--newline", "--print", "--no-simulate"} {
		if hasArg(args, plumbing) {
			t.Errorf("%s is progress plumbing and does not belong in the shown command", plumbing)
		}
	}

	reporting := ReportingArgs()
	if !hasArg(reporting, "--newline") || !hasArg(reporting, "--no-quiet") {
		t.Error("progress reporting needs newline separated, non-quiet output")
	}
	if !hasArg(reporting, "--no-simulate") {
		t.Error("printing the final path must not turn the download into a simulation")
	}
}

// A title yt-dlp cannot encode in the locale code page loses those characters
// from the path it prints, leaving a path that matches no file on disk.
func TestOutputIsReadAsUTF8(t *testing.T) {
	for name, args := range map[string][]string{
		"reporting": ReportingArgs(),
		"analysis":  (&Client{}).AnalyzeArgs(AnalyzeOptions{URL: "https://example.com/watch?v=1"}),
	} {
		encoding, ok := argValue(args, "--encoding")
		if !ok {
			t.Errorf("%s: no --encoding, so a path with an emoji comes back missing it", name)
			continue
		}
		if encoding != "utf-8" {
			t.Errorf("%s: --encoding = %q, want utf-8", name, encoding)
		}
	}
}

func TestBuildCommandPreviewQuoting(t *testing.T) {
	options := baseOptions()
	options.VideoFormatID = "137"
	options.AudioFormatID = "140"
	options.Container = media.ContainerMKV
	options.OutputDirectory = `C:\Users\Some One\Videos\GrabOne`

	args, err := BuildDownloadArgs(options)
	if err != nil {
		t.Fatalf("BuildDownloadArgs: %v", err)
	}

	preview := BuildCommandPreview(`C:\tools\yt-dlp.exe`, args)

	if !strings.HasPrefix(preview, "yt-dlp ") {
		t.Errorf("preview = %q, want it to start with the command name", preview)
	}
	if !strings.Contains(preview, `"137+140/137+ba/137"`) {
		t.Errorf("preview = %q, want the format selector quoted", preview)
	}
	if !strings.Contains(preview, `"C:\Users\Some One\Videos\GrabOne"`) {
		t.Errorf("preview = %q, want the path with spaces quoted", preview)
	}
	if strings.Contains(preview, "\n") {
		t.Error("the preview should be a single line that can be pasted as is")
	}
}

func TestCookieOptionsCompleteness(t *testing.T) {
	// Choosing a source before its browser or file is a half-made choice. It is
	// kept as the user's intent, but it must not reach yt-dlp as authentication.
	tests := []struct {
		name         string
		options      CookieOptions
		wantComplete bool
		wantArgs     bool
	}{
		{name: "browser chosen", options: CookieOptions{Source: CookieSourceBrowser, Browser: "edge"}, wantComplete: true, wantArgs: true},
		{name: "browser not chosen yet", options: CookieOptions{Source: CookieSourceBrowser}},
		{name: "file chosen", options: CookieOptions{Source: CookieSourceFile, File: `C:\cookies.txt`}, wantComplete: true, wantArgs: true},
		{name: "file not chosen yet", options: CookieOptions{Source: CookieSourceFile}},
		{name: "none", options: CookieOptions{Source: CookieSourceNone}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.options.Complete(); got != test.wantComplete {
				t.Errorf("Complete() = %v, want %v", got, test.wantComplete)
			}

			usable := test.options.Usable()
			if gotArgs := len(usable.Args()) > 0; gotArgs != test.wantArgs {
				t.Errorf("usable args = %v, want %v", usable.Args(), test.wantArgs)
			}
			// An unusable choice must never make a download fail validation.
			if err := usable.Validate(); err != nil {
				t.Errorf("a usable cookie choice failed validation: %v", err)
			}
		})
	}
}

func TestBrowserProfileIsPassedThrough(t *testing.T) {
	options := CookieOptions{Source: CookieSourceBrowser, Browser: "chrome", Profile: "Profile 2"}

	args := options.Args()
	if len(args) != 2 || args[1] != "chrome:Profile 2" {
		t.Errorf("args = %v, want the profile appended to the browser", args)
	}
}
