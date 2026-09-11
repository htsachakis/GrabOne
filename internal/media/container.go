package media

import "strings"

// Container values offered in the interface. "auto" lets the application pick
// the container that holds the selected streams without re-encoding them.
const (
	ContainerAuto = "auto"
	ContainerMP4  = "mp4"
	ContainerMKV  = "mkv"
	ContainerWebM = "webm"
)

// SupportedContainers lists the containers a user may choose for video output.
var SupportedContainers = []string{ContainerAuto, ContainerMKV, ContainerMP4, ContainerWebM}

// AudioConversionOriginal keeps the downloaded audio stream untouched.
const AudioConversionOriginal = "original"

// SupportedAudioConversions lists the audio targets FFmpeg can produce.
// "original" is not a conversion: the source stream is kept as downloaded.
var SupportedAudioConversions = []string{AudioConversionOriginal, "mp3", "m4a", "flac", "wav", "opus"}

// SupportedSubtitleFormats lists the subtitle file formats offered.
var SupportedSubtitleFormats = []string{"srt", "vtt"}

var mp4VideoCodecs = map[string]bool{"h264": true, "hevc": true, "av1": true}
var mp4AudioCodecs = map[string]bool{"aac": true, "mp3": true, "flac": true}
var webmVideoCodecs = map[string]bool{"vpx": true, "av1": true}
var webmAudioCodecs = map[string]bool{"opus": true, "vorbis": true}

// AutomaticContainer picks the container that can hold the selected streams
// without transcoding them. Remuxing rewrites the container only; changing the
// container is never a reason to re-encode video.
//
// H.264 with AAC becomes MP4, VP9 with Opus becomes WebM, and anything mixed
// falls back to MKV, which accepts practically any combination.
func AutomaticContainer(videoCodec, audioCodec string, embedSubtitles bool) string {
	if !HasVideoStream(videoCodec) {
		// No video stream: the container is decided by the audio pipeline.
		return ""
	}

	// An empty family means the codec is real but unrecognised, which is a
	// reason to choose MKV rather than to assume the stream is absent.
	video := CodecFamily(videoCodec)

	audio := ""
	if HasAudioStream(audioCodec) {
		audio = CodecFamily(audioCodec)
		if audio == "" {
			audio = "unknown"
		}
	}

	switch {
	case mp4VideoCodecs[video] && (audio == "" || mp4AudioCodecs[audio]):
		return ContainerMP4
	case webmVideoCodecs[video] && (audio == "" || webmAudioCodecs[audio]):
		if embedSubtitles {
			// WebM only accepts WebVTT subtitle tracks, and support for them is
			// patchy in players. MKV embeds any subtitle format reliably.
			return ContainerMKV
		}
		return ContainerWebM
	default:
		return ContainerMKV
	}
}

// CanEmbedSubtitles reports whether a container can carry an embedded subtitle
// track of the given format.
func CanEmbedSubtitles(container, subtitleFormat string) bool {
	switch strings.ToLower(container) {
	case ContainerMKV, "":
		return true
	case ContainerMP4:
		// MP4 carries text subtitles as mov_text, which FFmpeg converts to.
		return subtitleFormat == "srt" || subtitleFormat == "vtt"
	case ContainerWebM:
		return subtitleFormat == "vtt"
	default:
		return true
	}
}

// AudioContainerFor reports the file extension produced by an audio conversion
// target, which is what the interface shows as the resulting file type.
func AudioContainerFor(conversion string) string {
	switch strings.ToLower(conversion) {
	case "mp3":
		return "mp3"
	case "m4a":
		return "m4a"
	case "flac":
		return "flac"
	case "wav":
		return "wav"
	case "opus":
		return "opus"
	default:
		return ""
	}
}

// IsValidContainer reports whether a value may be sent to the command builder.
// Options are validated against known values rather than passed through.
func IsValidContainer(container string) bool {
	for _, candidate := range SupportedContainers {
		if container == candidate {
			return true
		}
	}
	return false
}

// IsValidAudioConversion reports whether an audio target is known.
func IsValidAudioConversion(conversion string) bool {
	for _, candidate := range SupportedAudioConversions {
		if conversion == candidate {
			return true
		}
	}
	return false
}

// IsValidSubtitleFormat reports whether a subtitle format is known.
func IsValidSubtitleFormat(format string) bool {
	for _, candidate := range SupportedSubtitleFormats {
		if format == candidate {
			return true
		}
	}
	return false
}
