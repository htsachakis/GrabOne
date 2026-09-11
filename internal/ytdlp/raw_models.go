package ytdlp

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// rawInfo mirrors the machine readable output of yt-dlp. Only the fields the
// application uses are declared: unknown fields are ignored by the decoder, so
// a newer yt-dlp adding keys cannot break analysis. Every field is treated as
// optional, because which keys an extractor fills in varies per site.
type rawInfo struct {
	Type string `json:"_type"`

	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`

	Uploader   string `json:"uploader"`
	UploaderID string `json:"uploader_id"`
	Channel    string `json:"channel"`

	Duration flexFloat `json:"duration"`

	Thumbnail   string         `json:"thumbnail"`
	Thumbnails  []rawThumbnail `json:"thumbnails"`
	WebpageURL  string         `json:"webpage_url"`
	OriginalURL string         `json:"original_url"`
	URL         string         `json:"url"`

	Extractor    string `json:"extractor"`
	ExtractorKey string `json:"extractor_key"`
	IEKey        string `json:"ie_key"`

	UploadDate   string  `json:"upload_date"`
	ViewCount    flexInt `json:"view_count"`
	LiveStatus   string  `json:"live_status"`
	IsLive       bool    `json:"is_live"`
	MediaType    string  `json:"media_type"`
	Availability string  `json:"availability"`

	Formats []rawFormat `json:"formats"`

	Subtitles         map[string][]rawSubtitle `json:"subtitles"`
	AutomaticCaptions map[string][]rawSubtitle `json:"automatic_captions"`

	Chapters []rawChapter `json:"chapters"`

	// Playlist fields.
	PlaylistTitle string    `json:"playlist_title"`
	PlaylistCount flexInt   `json:"playlist_count"`
	Entries       []rawInfo `json:"entries"`

	// Single format fields, present when the extractor returns one stream
	// directly instead of a format list.
	FormatID   string    `json:"format_id"`
	Extension  string    `json:"ext"`
	VideoCodec string    `json:"vcodec"`
	AudioCodec string    `json:"acodec"`
	Width      flexInt   `json:"width"`
	Height     flexInt   `json:"height"`
	FPS        flexFloat `json:"fps"`
	Protocol   string    `json:"protocol"`
}

type rawFormat struct {
	FormatID   string `json:"format_id"`
	FormatNote string `json:"format_note"`
	Format     string `json:"format"`

	Extension string `json:"ext"`
	Protocol  string `json:"protocol"`
	Language  string `json:"language"`

	Width  flexInt   `json:"width"`
	Height flexInt   `json:"height"`
	FPS    flexFloat `json:"fps"`

	VideoCodec string `json:"vcodec"`
	AudioCodec string `json:"acodec"`

	VideoBitrate flexFloat `json:"vbr"`
	AudioBitrate flexFloat `json:"abr"`
	TotalBitrate flexFloat `json:"tbr"`

	Filesize       flexInt `json:"filesize"`
	FilesizeApprox flexInt `json:"filesize_approx"`

	DynamicRange string `json:"dynamic_range"`
	Resolution   string `json:"resolution"`

	AudioChannels flexInt `json:"audio_channels"`
}

type rawThumbnail struct {
	URL        string  `json:"url"`
	Preference flexInt `json:"preference"`
	Width      flexInt `json:"width"`
	Height     flexInt `json:"height"`
}

type rawSubtitle struct {
	Extension string `json:"ext"`
	URL       string `json:"url"`
	Name      string `json:"name"`
}

type rawChapter struct {
	Title     string    `json:"title"`
	StartTime flexFloat `json:"start_time"`
	EndTime   flexFloat `json:"end_time"`
}

// flexFloat accepts a JSON number, a numeric string or null. Extractors are not
// consistent about which of those they emit.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil
		}
		*f = flexFloat(value)
		return nil
	}
	var value float64
	if err := json.Unmarshal(trimmed, &value); err != nil {
		// A malformed number must not fail the whole analysis.
		return nil
	}
	*f = flexFloat(value)
	return nil
}

// Float returns the value as a plain float64.
func (f flexFloat) Float() float64 { return float64(f) }

// flexInt accepts a JSON number, a numeric string or null, rounding floats.
type flexInt int64

func (i *flexInt) UnmarshalJSON(data []byte) error {
	var value flexFloat
	if err := value.UnmarshalJSON(data); err != nil {
		return err
	}
	*i = flexInt(int64(value))
	return nil
}

// Int returns the value as a plain int64.
func (i flexInt) Int() int64 { return int64(i) }

// IntValue returns the value as an int, for pixel sizes.
func (i flexInt) IntValue() int { return int(i) }
