// Package media holds the application's normalized media model. Nothing in
// this package knows about yt-dlp's JSON shape or about any specific website:
// it describes media in terms the user interface can present directly.
package media

// FormatKind classifies a format by the streams it carries.
type FormatKind string

const (
	// FormatKindCombined carries both video and audio.
	FormatKindCombined FormatKind = "combined"
	// FormatKindVideo carries video only and needs an audio stream merged in.
	FormatKindVideo FormatKind = "video"
	// FormatKindAudio carries audio only.
	FormatKindAudio FormatKind = "audio"
	// FormatKindOther is anything not directly useful, such as storyboards.
	FormatKindOther FormatKind = "other"
)

// Content types describe what a URL turned out to be. The set is open: new
// values can be added without changing the download pipeline.
const (
	ContentTypeVideo      = "Video"
	ContentTypeAudio      = "Audio"
	ContentTypeShort      = "Short"
	ContentTypeReel       = "Reel"
	ContentTypeStory      = "Story"
	ContentTypePost       = "Post"
	ContentTypeImage      = "Image"
	ContentTypeGallery    = "Gallery"
	ContentTypeCarousel   = "Carousel"
	ContentTypePlaylist   = "Playlist"
	ContentTypeCollection = "Collection"
	ContentTypeLive       = "Live"
	ContentTypeUnknown    = "Media"
)

// MediaFormat is one selectable stream.
type MediaFormat struct {
	FormatID string `json:"formatId"`

	Extension string `json:"extension"`

	Width  int     `json:"width"`
	Height int     `json:"height"`
	FPS    float64 `json:"fps"`

	// VideoCodec and AudioCodec are friendly names such as "H.264" or "AAC".
	VideoCodec string `json:"videoCodec"`
	AudioCodec string `json:"audioCodec"`

	// RawVideoCodec and RawAudioCodec keep the values reported by the
	// extractor, shown under advanced details.
	RawVideoCodec string `json:"rawVideoCodec"`
	RawAudioCodec string `json:"rawAudioCodec"`

	VideoBitrate float64 `json:"videoBitrate"`
	AudioBitrate float64 `json:"audioBitrate"`
	TotalBitrate float64 `json:"totalBitrate"`

	FileSize       int64 `json:"fileSize"`
	FileSizeApprox int64 `json:"fileSizeApprox"`

	HasVideo bool `json:"hasVideo"`
	HasAudio bool `json:"hasAudio"`

	DynamicRange string `json:"dynamicRange"`
	Protocol     string `json:"protocol"`

	FormatNote string `json:"formatNote"`
	Language   string `json:"language"`

	Kind FormatKind `json:"kind"`

	// QualityLabel is the short quality name, for example "1080p".
	QualityLabel string `json:"qualityLabel"`
	// Resolution is the full pixel size, useful for vertical media.
	Resolution string `json:"resolution"`
	// Label is the single line shown in a selector.
	Label string `json:"label"`
	// SizeLabel is the formatted size, prefixed with "~" when approximate.
	SizeLabel string `json:"sizeLabel"`

	// Recommended marks a broadly compatible choice. It is a hint about
	// compatibility, not a claim about quality.
	Recommended bool `json:"recommended"`
}

// EffectiveSize returns the exact size when known, otherwise the estimate,
// together with a flag reporting whether the value is approximate.
func (f MediaFormat) EffectiveSize() (int64, bool) {
	if f.FileSize > 0 {
		return f.FileSize, false
	}
	if f.FileSizeApprox > 0 {
		return f.FileSizeApprox, true
	}
	return 0, false
}

// SubtitleLanguage is one subtitle or caption track.
type SubtitleLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// Automatic is true for machine generated captions.
	Automatic bool `json:"automatic"`
	// Formats lists the available subtitle file extensions.
	Formats []string `json:"formats"`
}

// Chapter is a named section of the media.
type Chapter struct {
	Title     string  `json:"title"`
	StartTime float64 `json:"startTime"`
	EndTime   float64 `json:"endTime"`
}

// MediaEntry is one item of a collection: a playlist video, a carousel slide
// and so on. Formats are present only once the entry has been resolved.
type MediaEntry struct {
	Index        int           `json:"index"`
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	ThumbnailURL string        `json:"thumbnailUrl"`
	Duration     float64       `json:"duration"`
	DurationText string        `json:"durationText"`
	WebpageURL   string        `json:"webpageUrl"`
	ContentType  string        `json:"contentType"`
	Formats      []MediaFormat `json:"formats"`
	// Resolved reports whether formats were included in the analysis. When
	// false, the entry can be resolved on demand.
	Resolved bool `json:"resolved"`
	// Unavailable entries are listed but cannot be downloaded.
	Unavailable bool   `json:"unavailable"`
	Message     string `json:"message"`
}

// MediaCapabilities describes what the analyzed media supports. The user
// interface is built from these flags rather than from the platform name.
type MediaCapabilities struct {
	HasVideo             bool `json:"hasVideo"`
	HasAudio             bool `json:"hasAudio"`
	HasCombined          bool `json:"hasCombined"`
	HasSeparateVideo     bool `json:"hasSeparateVideo"`
	HasSeparateAudio     bool `json:"hasSeparateAudio"`
	HasSubtitles         bool `json:"hasSubtitles"`
	HasAutomaticCaptions bool `json:"hasAutomaticCaptions"`
	HasChapters          bool `json:"hasChapters"`
	HasThumbnail         bool `json:"hasThumbnail"`
	HasDescription       bool `json:"hasDescription"`
	HasMultipleFormats   bool `json:"hasMultipleFormats"`
	IsCollection         bool `json:"isCollection"`
	IsLive               bool `json:"isLive"`
}

// MediaInfo is the result of analyzing one URL.
type MediaInfo struct {
	Platform     string `json:"platform"`
	PlatformSlug string `json:"platformSlug"`
	Extractor    string `json:"extractor"`
	ExtractorKey string `json:"extractorKey"`

	ID string `json:"id"`

	Title       string `json:"title"`
	Uploader    string `json:"uploader"`
	Channel     string `json:"channel"`
	Description string `json:"description"`

	Duration     float64 `json:"duration"`
	DurationText string  `json:"durationText"`

	ThumbnailURL string `json:"thumbnailUrl"`
	WebpageURL   string `json:"webpageUrl"`
	// RequestedURL is the URL the user supplied. It is the URL handed back to
	// yt-dlp when downloading.
	RequestedURL string `json:"requestedUrl"`

	ContentType string `json:"contentType"`

	UploadDate string `json:"uploadDate"`
	ViewCount  int64  `json:"viewCount"`
	LiveStatus string `json:"liveStatus"`

	Formats   []MediaFormat      `json:"formats"`
	Subtitles []SubtitleLanguage `json:"subtitles"`
	Chapters  []Chapter          `json:"chapters"`

	IsCollection    bool         `json:"isCollection"`
	CollectionTitle string       `json:"collectionTitle"`
	Entries         []MediaEntry `json:"entries"`
	// EntriesResolved reports whether the entries already carry formats.
	EntriesResolved bool `json:"entriesResolved"`

	Capabilities MediaCapabilities `json:"capabilities"`

	// Warnings holds non-fatal messages produced during extraction.
	Warnings []string `json:"warnings"`
}

// CombinedFormats returns the formats carrying both video and audio.
func (m *MediaInfo) CombinedFormats() []MediaFormat {
	return filterKind(m.Formats, FormatKindCombined)
}

// VideoOnlyFormats returns the video-only formats.
func (m *MediaInfo) VideoOnlyFormats() []MediaFormat {
	return filterKind(m.Formats, FormatKindVideo)
}

// AudioOnlyFormats returns the audio-only formats.
func (m *MediaInfo) AudioOnlyFormats() []MediaFormat {
	return filterKind(m.Formats, FormatKindAudio)
}

// FormatByID looks up a format by its extractor supplied identifier, including
// the formats of resolved collection entries.
func (m *MediaInfo) FormatByID(id string) (MediaFormat, bool) {
	for _, f := range m.Formats {
		if f.FormatID == id {
			return f, true
		}
	}
	for _, entry := range m.Entries {
		for _, f := range entry.Formats {
			if f.FormatID == id {
				return f, true
			}
		}
	}
	return MediaFormat{}, false
}

func filterKind(formats []MediaFormat, kind FormatKind) []MediaFormat {
	out := make([]MediaFormat, 0, len(formats))
	for _, f := range formats {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}
