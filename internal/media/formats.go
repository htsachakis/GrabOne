package media

import (
	"fmt"
	"sort"
	"strings"
)

// Classify fills in the derived fields of a format: which streams it carries,
// how it should be grouped, and how it is labelled in the interface. It is the
// single place where raw codec values are interpreted.
func Classify(format *MediaFormat) {
	format.HasVideo = HasVideoStream(format.RawVideoCodec)
	format.HasAudio = HasAudioStream(format.RawAudioCodec)
	inferMissingStreams(format)

	format.VideoCodec = FriendlyVideoCodec(format.RawVideoCodec)
	format.AudioCodec = FriendlyAudioCodec(format.RawAudioCodec)

	switch {
	case isStoryboard(*format):
		format.Kind = FormatKindOther
	case format.HasVideo && format.HasAudio:
		format.Kind = FormatKindCombined
	case format.HasVideo:
		format.Kind = FormatKindVideo
	case format.HasAudio:
		format.Kind = FormatKindAudio
	default:
		format.Kind = FormatKindOther
	}

	format.QualityLabel = QualityLabel(format.Width, format.Height)
	format.Resolution = ResolutionLabel(format.Width, format.Height)
	format.SizeLabel = sizeLabel(*format)
	format.Label = buildLabel(*format)
}

// imageExtensions are formats that carry a picture rather than a stream, such
// as the entries of an image post.
var imageExtensions = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "webp": true, "gif": true, "heic": true, "avif": true,
}

// inferMissingStreams fills in what an extractor left unsaid.
//
// Not every extractor reports codecs. A direct media link may carry no codec
// information at all, and some report only that one of the streams is absent.
// Dropping those formats would make perfectly downloadable media look empty, so
// the missing side is inferred instead.
func inferMissingStreams(format *MediaFormat) {
	if format.HasVideo || format.HasAudio {
		return
	}
	if isStoryboard(*format) || imageExtensions[strings.ToLower(format.Extension)] {
		return
	}

	videoStated := strings.TrimSpace(format.RawVideoCodec) != ""
	audioStated := strings.TrimSpace(format.RawAudioCodec) != ""

	switch {
	case videoStated && !audioStated:
		// Video is explicitly absent and audio was not reported: an audio file.
		format.HasAudio = true
	case audioStated && !videoStated:
		format.HasVideo = true
	case !videoStated && !audioStated:
		// Nothing was reported at all. The file still plays, so treat it as a
		// self-contained stream rather than hiding it.
		format.HasVideo = true
		format.HasAudio = true
	}
}

func isStoryboard(format MediaFormat) bool {
	if strings.EqualFold(format.Protocol, "mhtml") {
		return true
	}
	if strings.Contains(strings.ToLower(format.FormatNote), "storyboard") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(format.RawVideoCodec), "images")
}

func sizeLabel(format MediaFormat) string {
	size, approximate := format.EffectiveSize()
	if size <= 0 {
		return ""
	}
	if approximate {
		return "~" + FormatBytes(size)
	}
	return FormatBytes(size)
}

func buildLabel(format MediaFormat) string {
	switch format.Kind {
	case FormatKindAudio:
		return joinParts(" • ",
			audioName(format),
			FormatBitrate(audioBitrate(format)),
			ContainerLabel(format.Extension),
			format.SizeLabel,
		)
	case FormatKindCombined, FormatKindVideo:
		codecs := format.VideoCodec
		if format.Kind == FormatKindCombined && format.AudioCodec != "" {
			codecs = joinParts(" + ", format.VideoCodec, format.AudioCodec)
		}
		return joinParts(" • ",
			qualityWithFallback(format),
			codecs,
			fpsLabel(format.FPS),
			dynamicRangeLabel(format.DynamicRange),
			ContainerLabel(format.Extension),
			format.SizeLabel,
		)
	default:
		return joinParts(" • ", format.FormatID, format.FormatNote, ContainerLabel(format.Extension))
	}
}

// audioName names an audio stream. Not every extractor reports a codec, and a
// label of just the container would tell the user nothing.
func audioName(format MediaFormat) string {
	if format.AudioCodec != "" {
		return format.AudioCodec
	}
	if format.FormatNote != "" {
		return format.FormatNote
	}
	return "Audio " + format.FormatID
}

func qualityWithFallback(format MediaFormat) string {
	if format.QualityLabel != "" {
		return format.QualityLabel
	}
	if format.FormatNote != "" {
		return format.FormatNote
	}
	return format.FormatID
}

func fpsLabel(fps float64) string {
	if fps <= 0 {
		return ""
	}
	if fps == float64(int(fps)) {
		return fmt.Sprintf("%d FPS", int(fps))
	}
	return fmt.Sprintf("%.2f FPS", fps)
}

func dynamicRangeLabel(dynamicRange string) string {
	trimmed := strings.TrimSpace(dynamicRange)
	if trimmed == "" || strings.EqualFold(trimmed, "sdr") {
		return ""
	}
	return strings.ToUpper(trimmed)
}

func audioBitrate(format MediaFormat) float64 {
	if format.AudioBitrate > 0 {
		return format.AudioBitrate
	}
	return format.TotalBitrate
}

// DisambiguateLabels appends the format identifier to labels that would
// otherwise be identical.
//
// Sites publish streams that differ only in ways the label does not show — the
// delivery protocol, or a loudness-normalised variant — and a selector with two
// identical entries gives the user no way to tell them apart.
func DisambiguateLabels(formats []MediaFormat) {
	counts := map[string]int{}
	for _, format := range formats {
		counts[format.Label]++
	}
	for index, format := range formats {
		if counts[format.Label] > 1 && format.FormatID != "" {
			formats[index].Label = format.Label + " · " + format.FormatID
		}
	}
}

// SortFormats orders formats for presentation: video by resolution, then frame
// rate, then bitrate; audio by bitrate. Nothing is removed, so alternative
// codecs stay visible.
func SortFormats(formats []MediaFormat) {
	sort.SliceStable(formats, func(i, j int) bool {
		left, right := formats[i], formats[j]

		if left.Kind == FormatKindAudio && right.Kind == FormatKindAudio {
			if audioBitrate(left) != audioBitrate(right) {
				return audioBitrate(left) > audioBitrate(right)
			}
			return left.FormatID < right.FormatID
		}

		leftEdge, rightEdge := shortEdge(left), shortEdge(right)
		if leftEdge != rightEdge {
			return leftEdge > rightEdge
		}
		if left.FPS != right.FPS {
			return left.FPS > right.FPS
		}
		leftRate := preferredBitrate(left)
		rightRate := preferredBitrate(right)
		if leftRate != rightRate {
			return leftRate > rightRate
		}
		return left.FormatID < right.FormatID
	})
}

func preferredBitrate(format MediaFormat) float64 {
	if format.TotalBitrate > 0 {
		return format.TotalBitrate
	}
	if format.VideoBitrate > 0 {
		return format.VideoBitrate
	}
	return format.AudioBitrate
}

func shortEdge(format MediaFormat) int {
	if format.Width > 0 && format.Width < format.Height {
		return format.Width
	}
	return format.Height
}

// MarkRecommended flags the most broadly compatible choice in each group.
// On Windows, H.264 video with AAC audio in an MP4 container plays everywhere,
// which is a compatibility statement rather than a quality one.
func MarkRecommended(formats []MediaFormat) {
	best := map[FormatKind]int{}
	bestScore := map[FormatKind]int{}

	for index, format := range formats {
		score := compatibilityScore(format)
		if score <= 0 {
			continue
		}
		if current, ok := bestScore[format.Kind]; !ok || score > current {
			bestScore[format.Kind] = score
			best[format.Kind] = index
		}
	}
	for _, index := range best {
		formats[index].Recommended = true
	}
}

func compatibilityScore(format MediaFormat) int {
	switch format.Kind {
	case FormatKindCombined, FormatKindVideo:
		if format.VideoCodec != "H.264" {
			return 0
		}
		score := 1 + shortEdge(format)
		if strings.EqualFold(format.Extension, "mp4") {
			score += 10000
		}
		return score
	case FormatKindAudio:
		if !strings.HasPrefix(format.AudioCodec, "AAC") {
			return 0
		}
		return 1 + int(audioBitrate(format))
	default:
		return 0
	}
}

// ResolutionPreset is a convenience filter offered next to the full format
// list. Presets narrow the choice; they never replace the selected format ID.
type ResolutionPreset struct {
	Label     string `json:"label"`
	Height    int    `json:"height"`
	Available bool   `json:"available"`
}

var presetHeights = []struct {
	label  string
	height int
}{
	{"4K", 2160},
	{"1440p", 1440},
	{"1080p", 1080},
	{"720p", 720},
	{"480p", 480},
	{"360p", 360},
}

// ResolutionPresets reports which convenience resolutions the analyzed media
// can actually deliver. A stream is attributed to the highest preset it
// reaches, so a 900p stream counts as 720p rather than 1080p.
func ResolutionPresets(formats []MediaFormat) []ResolutionPreset {
	available := map[int]bool{}
	anyVideo := false

	for _, format := range formats {
		if !format.HasVideo || format.Kind == FormatKindOther {
			continue
		}
		anyVideo = true
		edge := shortEdge(format)
		for _, preset := range presetHeights {
			if edge >= preset.height {
				available[preset.height] = true
				break
			}
		}
	}

	presets := make([]ResolutionPreset, 0, len(presetHeights)+1)
	presets = append(presets, ResolutionPreset{Label: "Best", Height: 0, Available: anyVideo})
	for _, preset := range presetHeights {
		presets = append(presets, ResolutionPreset{
			Label:     preset.label,
			Height:    preset.height,
			Available: available[preset.height],
		})
	}
	return presets
}

// BestFormat returns the highest ranked format of a kind, or false when the
// media has none.
func BestFormat(formats []MediaFormat, kind FormatKind) (MediaFormat, bool) {
	candidates := filterKind(formats, kind)
	if len(candidates) == 0 {
		return MediaFormat{}, false
	}
	SortFormats(candidates)
	return candidates[0], true
}
