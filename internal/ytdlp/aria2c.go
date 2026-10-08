package ytdlp

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
)

// yt-dlp reports nothing while aria2c transfers a file: it starts aria2c, waits,
// and announces the file as finished. aria2c's own status line is all there is,
// and it reaches the application because aria2c inherits yt-dlp's output.
//
// The Windows build of aria2c always behaves as if it were writing to a
// terminal. Once a second it blanks the line with carriage returns and redraws
// it, and it never ends it with a line break:
//
//	\r<spaces>\r[#c7f6bb 1476815B/4360399B(33%) CN:4 DL:674502B ETA:4s]
//
// So the output has to be split at carriage returns as well as line breaks, and
// whatever yt-dlp prints after the last status line arrives stuck to the end of
// it.

// aria2cStatus matches a status line at the start of a line. Sizes are exact
// byte counts when aria2c is run with --human-readable=false, which is how the
// application runs it, and abbreviated otherwise.
var aria2cStatus = regexp.MustCompile(
	`^\[#[0-9a-f]+ ` +
		`([0-9.,]+)(Ki|Mi|Gi)?B/([0-9.,]+)(Ki|Mi|Gi)?B(?:\(\d+%\))?` +
		` CN:\d+(?: SD:\d+)?` +
		`(?: DL:([0-9.,]+)(Ki|Mi|Gi)?B)?` +
		`(?: UL:[^\s\]]+)?` +
		`(?: ETA:([^\s\]]+))?` +
		`\]`,
)

// splitOutputLines is a bufio.SplitFunc that ends a line at a carriage return
// as well as at a line break.
func splitOutputLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if index := bytes.IndexAny(data, "\r\n"); index >= 0 {
		return index + 1, data[:index], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// cutAria2cStatus separates an aria2c status line from whatever follows it on
// the same line.
func cutAria2cStatus(line string) (status []string, rest string, ok bool) {
	match := aria2cStatus.FindStringSubmatch(line)
	if match == nil {
		return nil, line, false
	}
	return match, line[len(match[0]):], true
}

// parseAria2cStatus turns a matched status line into a progress update for the
// file yt-dlp last announced.
func (p *ProgressParser) parseAria2cStatus(status []string) ProgressUpdate {
	update := ProgressUpdate{
		Stage:               p.downloadStage(p.currentFile),
		DownloadedBytes:     aria2cSize(status[1], status[2]),
		TotalBytes:          aria2cSize(status[3], status[4]),
		SpeedBytesPerSecond: float64(aria2cSize(status[5], status[6])),
		ETASeconds:          aria2cDuration(status[7]),
		Filename:            p.currentFile,
		StreamLabel:         p.streamLabel(p.currentFile),
		ItemIndex:           p.itemIndex,
		ItemCount:           p.itemCount,
	}

	if update.TotalBytes > 0 {
		update.Percent = clampPercent(float64(update.DownloadedBytes) / float64(update.TotalBytes) * 100)
	}
	update.OverallPercent = p.overallPercent(update.Percent, p.currentFile)
	return update
}

// aria2cSize reads a size such as "4360399", "1.3" with the unit "Mi", or
// "1,234" with the unit "Gi".
func aria2cSize(number, unit string) int64 {
	value, err := strconv.ParseFloat(strings.ReplaceAll(number, ",", ""), 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "Ki":
		value *= 1 << 10
	case "Mi":
		value *= 1 << 20
	case "Gi":
		value *= 1 << 30
	}
	return int64(value)
}

// aria2cDuration reads a countdown such as "43s", "4m51s" or "1h2m3s". Anything
// else is no countdown.
func aria2cDuration(text string) int {
	seconds, digits := 0, ""
	for _, current := range text {
		switch {
		case current >= '0' && current <= '9':
			digits += string(current)
			continue
		case digits == "":
			return 0
		}

		amount, _ := strconv.Atoi(digits)
		digits = ""
		switch current {
		case 'h':
			seconds += amount * 3600
		case 'm':
			seconds += amount * 60
		case 's':
			seconds += amount
		default:
			return 0
		}
	}
	if digits != "" {
		return 0
	}
	return seconds
}
