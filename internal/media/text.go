package media

import (
	"fmt"
	"math"
	"strings"
)

// FormatDuration renders seconds as mm:ss, or hh:mm:ss for longer media.
func FormatDuration(seconds float64) string {
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return ""
	}
	total := int(math.Round(seconds))
	hours := total / 3600
	minutes := (total % 3600) / 60
	secs := total % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

// FormatBytes renders a byte count using binary multiples, with the precision
// reduced as the unit grows.
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return ""
	}
	const unit = 1024.0
	value := float64(bytes)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	index := 0
	for value >= unit && index < len(units)-1 {
		value /= unit
		index++
	}
	switch {
	case index == 0:
		return fmt.Sprintf("%d %s", int64(value), units[index])
	case value >= 100:
		return fmt.Sprintf("%.0f %s", value, units[index])
	case value >= 10:
		return fmt.Sprintf("%.1f %s", value, units[index])
	default:
		return fmt.Sprintf("%.2f %s", value, units[index])
	}
}

// FormatBitrate renders a bitrate given in kbit/s.
func FormatBitrate(kbitsPerSecond float64) string {
	if kbitsPerSecond <= 0 {
		return ""
	}
	if kbitsPerSecond >= 1000 {
		return fmt.Sprintf("%.1f Mbps", kbitsPerSecond/1000)
	}
	return fmt.Sprintf("%.0f kbps", kbitsPerSecond)
}

// FormatSpeed renders a transfer rate given in bytes per second.
func FormatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return ""
	}
	return FormatBytes(int64(bytesPerSecond)) + "/s"
}

// QualityLabel derives the short quality name from a pixel size. Vertical
// media is labelled by its smaller edge so a 1080x1920 reel reads as 1080p.
func QualityLabel(width, height int) string {
	shortEdge := height
	if width > 0 && width < height {
		shortEdge = width
	}
	if shortEdge <= 0 {
		return ""
	}
	if shortEdge >= 4320 {
		return "4320p"
	}
	return fmt.Sprintf("%dp", shortEdge)
}

// ResolutionLabel renders a pixel size for display.
func ResolutionLabel(width, height int) string {
	switch {
	case width > 0 && height > 0:
		return fmt.Sprintf("%d × %d", width, height)
	case height > 0:
		return fmt.Sprintf("%dp", height)
	default:
		return ""
	}
}

// ContainerLabel renders a file extension in the way users expect to read it.
func ContainerLabel(extension string) string {
	trimmed := strings.ToLower(strings.TrimSpace(extension))
	switch trimmed {
	case "":
		return ""
	case "mp4", "m4a", "mp3", "mkv", "webm", "ogg", "wav", "flac", "aac", "avi", "mov", "3gp", "ts", "opus":
		return strings.ToUpper(trimmed)
	default:
		return strings.ToUpper(trimmed)
	}
}

// joinParts assembles a display label from non-empty parts.
func joinParts(separator string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, separator)
}
