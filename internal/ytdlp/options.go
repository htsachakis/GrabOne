package ytdlp

import (
	"fmt"
	"strings"

	"grabone/internal/media"
)

// Download types offered in the interface.
const (
	DownloadTypeVideoAudio = "video+audio"
	DownloadTypeVideoOnly  = "video"
	DownloadTypeAudioOnly  = "audio"
)

// Collection modes decide how a multi-item URL is treated. The default is
// deliberately the safest one: a playlist link never downloads everything
// unless the user asks for it.
const (
	CollectionModeSingle   = "single"
	CollectionModeSelected = "selected"
	CollectionModeAll      = "all"
)

// Cookie sources, mirroring the settings values.
const (
	CookieSourceNone    = "none"
	CookieSourceBrowser = "browser"
	CookieSourceFile    = "file"
)

// SupportedCookieBrowsers lists the browsers yt-dlp can read cookies from. The
// application never touches the cookie store itself: the name is handed to
// yt-dlp, which does the reading.
var SupportedCookieBrowsers = []string{"chrome", "edge", "firefox", "brave", "opera", "vivaldi", "chromium", "safari", "whale"}

// CookieOptions describes how to authenticate. Cookie contents are never read
// or logged by the application.
type CookieOptions struct {
	Source  string `json:"source"`
	Browser string `json:"browser"`
	File    string `json:"file"`
	// Profile is the optional browser profile, passed through to yt-dlp.
	Profile string `json:"profile"`
}

// Validate checks the cookie configuration against known values.
func (c CookieOptions) Validate() error {
	switch c.Source {
	case "", CookieSourceNone:
		return nil
	case CookieSourceBrowser:
		if !isKnownBrowser(c.Browser) {
			return fmt.Errorf("unsupported cookie browser %q", c.Browser)
		}
		return nil
	case CookieSourceFile:
		if strings.TrimSpace(c.File) == "" {
			return fmt.Errorf("no cookie file selected")
		}
		return nil
	default:
		return fmt.Errorf("unsupported cookie source %q", c.Source)
	}
}

// Args renders the cookie switches for yt-dlp.
func (c CookieOptions) Args() []string {
	switch c.Source {
	case CookieSourceBrowser:
		if !isKnownBrowser(c.Browser) {
			return nil
		}
		specifier := strings.ToLower(c.Browser)
		if profile := strings.TrimSpace(c.Profile); profile != "" {
			specifier += ":" + profile
		}
		return []string{"--cookies-from-browser", specifier}
	case CookieSourceFile:
		if strings.TrimSpace(c.File) == "" {
			return nil
		}
		return []string{"--cookies", c.File}
	default:
		return nil
	}
}

// Enabled reports whether any authentication is configured.
func (c CookieOptions) Enabled() bool {
	return c.Source == CookieSourceBrowser || c.Source == CookieSourceFile
}

// Complete reports whether the chosen source has everything it needs. A source
// picked before its browser or file is a half-made choice, not a usable one.
func (c CookieOptions) Complete() bool {
	switch c.Source {
	case CookieSourceBrowser:
		return isKnownBrowser(c.Browser)
	case CookieSourceFile:
		return strings.TrimSpace(c.File) != ""
	default:
		return false
	}
}

// Usable returns the options to hand to yt-dlp: an incomplete choice becomes no
// authentication rather than a failed download.
func (c CookieOptions) Usable() CookieOptions {
	if c.Enabled() && !c.Complete() {
		return CookieOptions{Source: CookieSourceNone}
	}
	return c
}

func isKnownBrowser(name string) bool {
	lowered := strings.ToLower(strings.TrimSpace(name))
	for _, candidate := range SupportedCookieBrowsers {
		if candidate == lowered {
			return true
		}
	}
	return false
}

// DownloadOptions is the complete description of a requested download. It is
// validated before any argument is built: values arriving from the interface are
// checked against known options rather than passed through to the command line.
type DownloadOptions struct {
	URL string `json:"url"`

	// Title is used for display only.
	Title string `json:"title"`

	DownloadType string `json:"downloadType"`

	VideoFormatID    string `json:"videoFormatId"`
	AudioFormatID    string `json:"audioFormatId"`
	CombinedFormatID string `json:"combinedFormatId"`

	// MaxHeight applies a resolution ceiling when no exact format is chosen.
	MaxHeight int `json:"maxHeight"`

	Container string `json:"container"`

	EmbedMetadata  bool `json:"embedMetadata"`
	EmbedChapters  bool `json:"embedChapters"`
	EmbedThumbnail bool `json:"embedThumbnail"`

	SaveThumbnail   bool `json:"saveThumbnail"`
	SaveDescription bool `json:"saveDescription"`
	SaveJSON        bool `json:"saveJson"`

	DownloadSubtitles bool     `json:"downloadSubtitles"`
	EmbedSubtitles    bool     `json:"embedSubtitles"`
	KeepSubtitles     bool     `json:"keepSubtitles"`
	SubtitleLanguages []string `json:"subtitleLanguages"`
	SubtitleFormat    string   `json:"subtitleFormat"`
	// IncludeAutomaticCaptions is set when at least one selected language is
	// only available as machine generated captions.
	IncludeAutomaticCaptions bool `json:"includeAutomaticCaptions"`

	AudioConversionFormat string `json:"audioConversionFormat"`

	OutputDirectory  string `json:"outputDirectory"`
	FilenameTemplate string `json:"filenameTemplate"`

	// CollectionMode and PlaylistItems control multi-item URLs.
	CollectionMode string `json:"collectionMode"`
	PlaylistItems  string `json:"playlistItems"`

	Cookies CookieOptions `json:"cookies"`

	// SelectedVideoCodec and SelectedAudioCodec are the raw codecs of the
	// chosen streams, used to decide the automatic container. They are filled
	// in by the caller from the analysis result.
	SelectedVideoCodec string `json:"selectedVideoCodec"`
	SelectedAudioCodec string `json:"selectedAudioCodec"`
}

// Validate checks the options against the known value sets.
func (o DownloadOptions) Validate() error {
	if strings.TrimSpace(o.URL) == "" {
		return fmt.Errorf("no URL given")
	}
	switch o.DownloadType {
	case DownloadTypeVideoAudio, DownloadTypeVideoOnly, DownloadTypeAudioOnly:
	default:
		return fmt.Errorf("unsupported download type %q", o.DownloadType)
	}
	if o.Container != "" && !media.IsValidContainer(o.Container) {
		return fmt.Errorf("unsupported container %q", o.Container)
	}
	if o.AudioConversionFormat != "" && !media.IsValidAudioConversion(o.AudioConversionFormat) {
		return fmt.Errorf("unsupported audio format %q", o.AudioConversionFormat)
	}
	if o.DownloadSubtitles && o.SubtitleFormat != "" && !media.IsValidSubtitleFormat(o.SubtitleFormat) {
		return fmt.Errorf("unsupported subtitle format %q", o.SubtitleFormat)
	}
	for _, language := range o.SubtitleLanguages {
		if !isSafeLanguageCode(language) {
			return fmt.Errorf("unsupported subtitle language %q", language)
		}
	}
	switch o.CollectionMode {
	case "", CollectionModeSingle, CollectionModeSelected, CollectionModeAll:
	default:
		return fmt.Errorf("unsupported collection mode %q", o.CollectionMode)
	}
	if o.CollectionMode == CollectionModeSelected && !isSafePlaylistItems(o.PlaylistItems) {
		return fmt.Errorf("invalid item selection %q", o.PlaylistItems)
	}
	if err := o.Cookies.Validate(); err != nil {
		return err
	}
	if !isSafeFormatID(o.VideoFormatID) || !isSafeFormatID(o.AudioFormatID) || !isSafeFormatID(o.CombinedFormatID) {
		return fmt.Errorf("invalid format identifier")
	}
	return nil
}

// isSafeFormatID guards against a format identifier being used to smuggle extra
// format selector syntax into the command line. Identifiers come from the
// analysis result, so they are always plain tokens.
func isSafeFormatID(id string) bool {
	if id == "" {
		return true
	}
	for _, current := range id {
		switch {
		case current >= 'a' && current <= 'z',
			current >= 'A' && current <= 'Z',
			current >= '0' && current <= '9',
			current == '-', current == '_', current == '.', current == '~':
		default:
			return false
		}
	}
	return true
}

func isSafeLanguageCode(code string) bool {
	if code == "" {
		return false
	}
	if code == "all" {
		return true
	}
	for _, current := range code {
		switch {
		case current >= 'a' && current <= 'z',
			current >= 'A' && current <= 'Z',
			current >= '0' && current <= '9',
			current == '-', current == '_':
		default:
			return false
		}
	}
	return true
}

// isSafePlaylistItems accepts the digit, comma, dash and colon syntax yt-dlp
// uses for item ranges, and nothing else.
func isSafePlaylistItems(items string) bool {
	trimmed := strings.TrimSpace(items)
	if trimmed == "" {
		return false
	}
	for _, current := range trimmed {
		switch {
		case current >= '0' && current <= '9', current == ',', current == '-', current == ':':
		default:
			return false
		}
	}
	return true
}
