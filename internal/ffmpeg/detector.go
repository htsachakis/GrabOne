package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"grabone/internal/procutil"
)

const probeTimeout = 30 * time.Second

// Prober inspects finished files with FFprobe.
type Prober struct {
	// BinaryPath is the resolved path of ffprobe.exe. An empty value disables
	// inspection, which is optional: a download is complete without it.
	BinaryPath string
}

// NewProber builds a prober for the given executable.
func NewProber(binaryPath string) *Prober { return &Prober{BinaryPath: binaryPath} }

// Available reports whether inspection is possible.
func (p *Prober) Available() bool { return p != nil && p.BinaryPath != "" }

// rawProbe mirrors the machine readable output of ffprobe. Unknown fields are
// ignored, so a newer FFprobe cannot break parsing.
type rawProbe struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
	Chapters []struct {
		ID int `json:"id"`
	} `json:"chapters"`
}

// Inspect reads the streams of a finished file. It is best effort: callers treat
// a failure as "details unavailable" rather than as a failed download.
func (p *Prober) Inspect(ctx context.Context, path string) (*FileSummary, error) {
	if !p.Available() {
		return nil, fmt.Errorf("inspect %s: ffprobe is not available", path)
	}

	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		"-of", "json",
		path,
	}

	cmd := exec.CommandContext(ctx, p.BinaryPath, args...)
	procutil.Configure(cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	}

	var raw rawProbe
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("parse ffprobe output for %s: %w", path, err)
	}

	summary := &FileSummary{
		Path:         path,
		Container:    raw.Format.FormatName,
		ChapterCount: len(raw.Chapters),
	}
	summary.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	summary.SizeBytes, _ = strconv.ParseInt(raw.Format.Size, 10, 64)
	if summary.SizeBytes == 0 {
		if info, err := os.Stat(path); err == nil {
			summary.SizeBytes = info.Size()
		}
	}

	for _, stream := range raw.Streams {
		switch stream.CodecType {
		case "video":
			if summary.VideoCodec == "" {
				summary.VideoCodec = stream.CodecName
				summary.Width = stream.Width
				summary.Height = stream.Height
			}
		case "audio":
			if summary.AudioCodec == "" {
				summary.AudioCodec = stream.CodecName
			}
		case "subtitle":
			summary.SubtitleTracks++
		}
	}
	return summary, nil
}
