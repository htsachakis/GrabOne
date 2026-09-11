// Package ffmpeg wraps the parts of FFmpeg and FFprobe the application needs:
// reading their versions, and inspecting a finished file. All merging,
// remuxing and conversion is performed by yt-dlp, which invokes FFmpeg itself.
package ffmpeg

import (
	"regexp"
	"strings"
)

// FileSummary describes a media file as reported by FFprobe. It is used to show
// what was actually produced once a download finishes.
type FileSummary struct {
	Path      string  `json:"path"`
	Container string  `json:"container"`
	Duration  float64 `json:"duration"`
	SizeBytes int64   `json:"sizeBytes"`

	VideoCodec string `json:"videoCodec"`
	AudioCodec string `json:"audioCodec"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`

	SubtitleTracks int `json:"subtitleTracks"`
	ChapterCount   int `json:"chapterCount"`
}

// versionPattern matches the version token in "ffmpeg version 8.0 Copyright..."
// and in git build strings such as "ffmpeg version N-125875-g5d4d3bdc61".
var versionPattern = regexp.MustCompile(`version\s+(\S+)`)

// ParseVersion extracts the version from FFmpeg or FFprobe output. An
// unexpected format is not an error: the first line is returned instead, so the
// settings screen always shows something truthful.
func ParseVersion(output string) string {
	line := strings.TrimSpace(firstLine(output))
	if line == "" {
		return ""
	}
	if match := versionPattern.FindStringSubmatch(line); len(match) == 2 {
		return strings.TrimSuffix(match[1], ",")
	}
	return line
}

func firstLine(text string) string {
	if index := strings.IndexAny(text, "\r\n"); index >= 0 {
		return text[:index]
	}
	return text
}
