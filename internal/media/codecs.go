package media

import "strings"

// codecPrefix maps the start of a raw codec string to a friendly name. The
// extractor reports values such as "avc1.640028" or "mp4a.40.2", which are
// accurate but unhelpful in a selector.
var videoCodecNames = []struct {
	prefix string
	name   string
}{
	{"avc1", "H.264"},
	{"avc2", "H.264"},
	{"avc3", "H.264"},
	{"h264", "H.264"},
	{"h.264", "H.264"},
	{"hev1", "HEVC/H.265"},
	{"hvc1", "HEVC/H.265"},
	{"hevc", "HEVC/H.265"},
	{"h265", "HEVC/H.265"},
	{"av01", "AV1"},
	{"av1", "AV1"},
	{"vp09", "VP9"},
	{"vp9", "VP9"},
	{"vp08", "VP8"},
	{"vp8", "VP8"},
	{"vp6", "VP6"},
	{"theora", "Theora"},
	{"mp4v", "MPEG-4"},
	{"mpeg4", "MPEG-4"},
	{"mpeg2", "MPEG-2"},
	{"dvh1", "Dolby Vision"},
	{"dvhe", "Dolby Vision"},
	{"vvc1", "VVC/H.266"},
	{"images", "Images"},
}

var audioCodecNames = []struct {
	prefix string
	name   string
}{
	{"mp4a.40.2", "AAC"},
	{"mp4a.40.5", "AAC (HE)"},
	{"mp4a.40.29", "AAC (HE v2)"},
	{"mp4a", "AAC"},
	{"aac", "AAC"},
	{"opus", "Opus"},
	{"vorbis", "Vorbis"},
	{"mp3", "MP3"},
	{"mp2", "MP2"},
	{"flac", "FLAC"},
	{"alac", "ALAC"},
	{"ec-3", "Dolby Digital Plus"},
	{"eac3", "Dolby Digital Plus"},
	{"ac-3", "Dolby Digital"},
	{"ac3", "Dolby Digital"},
	{"dts", "DTS"},
	{"pcm", "PCM"},
	{"wav", "PCM"},
	{"vp9", ""}, // guard: never report a video codec as audio
}

// NoCodec is the value extractors use to say a stream is absent.
const NoCodec = "none"

// FriendlyVideoCodec converts a raw video codec value to a readable name. An
// unrecognised value is returned unchanged so nothing is ever hidden from the
// user.
func FriendlyVideoCodec(raw string) string {
	normalized := normalizeCodec(raw)
	if normalized == "" || normalized == NoCodec {
		return ""
	}
	for _, candidate := range videoCodecNames {
		if strings.HasPrefix(normalized, candidate.prefix) {
			return candidate.name
		}
	}
	return raw
}

// FriendlyAudioCodec converts a raw audio codec value to a readable name.
func FriendlyAudioCodec(raw string) string {
	normalized := normalizeCodec(raw)
	if normalized == "" || normalized == NoCodec {
		return ""
	}
	for _, candidate := range audioCodecNames {
		if candidate.name == "" {
			continue
		}
		if strings.HasPrefix(normalized, candidate.prefix) {
			return candidate.name
		}
	}
	return raw
}

// HasVideoStream reports whether a raw video codec value describes a real
// video stream.
func HasVideoStream(rawVideoCodec string) bool {
	normalized := normalizeCodec(rawVideoCodec)
	if normalized == "" || normalized == NoCodec {
		return false
	}
	// Storyboards are reported as an image sequence rather than a video track.
	return normalized != "images"
}

// HasAudioStream reports whether a raw audio codec value describes a real
// audio stream.
func HasAudioStream(rawAudioCodec string) bool {
	normalized := normalizeCodec(rawAudioCodec)
	return normalized != "" && normalized != NoCodec
}

func normalizeCodec(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// CodecFamily returns a coarse grouping used when choosing a container.
func CodecFamily(rawCodec string) string {
	switch FriendlyVideoCodec(rawCodec) {
	case "H.264":
		return "h264"
	case "HEVC/H.265":
		return "hevc"
	case "AV1":
		return "av1"
	case "VP9", "VP8":
		return "vpx"
	}
	switch FriendlyAudioCodec(rawCodec) {
	case "AAC", "AAC (HE)", "AAC (HE v2)":
		return "aac"
	case "Opus":
		return "opus"
	case "Vorbis":
		return "vorbis"
	case "MP3":
		return "mp3"
	case "FLAC":
		return "flac"
	}
	return ""
}
