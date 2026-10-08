package ytdlp

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

// These are real status lines, captured from aria2c 1.37.0 running under yt-dlp
// with its output piped the way the application pipes it.
const (
	sampleAria2cLine        = "[#c7f6bb 1476815B/4360399B(33%) CN:4 DL:674502B ETA:4s]"
	sampleAria2cLastLine    = "[#c7f6bb 3880239B/4360399B(88%) CN:3 DL:912301B]"
	sampleAria2cAbbreviated = "[#3626d1 1.3MiB/4.1MiB(32%) CN:4 DL:419KiB ETA:6s]"
)

func TestParseAria2cStatusLine(t *testing.T) {
	parser := NewProgressParser("", "", 1)
	parser.Parse(`[download] Destination: C:\Downloads\trailer_400p.ogg`)

	update, ok := parser.Parse(sampleAria2cLine)
	if !ok {
		t.Fatal("an aria2c status line was not recognised")
	}

	if update.Stage != StageDownloading {
		t.Errorf("stage = %q, want downloading", update.Stage)
	}
	if update.DownloadedBytes != 1476815 || update.TotalBytes != 4360399 {
		t.Errorf("bytes = %d of %d, want 1476815 of 4360399", update.DownloadedBytes, update.TotalBytes)
	}
	if update.TotalIsEstimate {
		t.Error("aria2c reports the exact size, so it is not an estimate")
	}
	if update.Percent < 33 || update.Percent >= 34 {
		t.Errorf("percent = %v, want it between 33 and 34", update.Percent)
	}
	if update.OverallPercent != update.Percent {
		t.Errorf("overall = %v, want the per-file %v for a download of one stream", update.OverallPercent, update.Percent)
	}
	if update.SpeedBytesPerSecond != 674502 {
		t.Errorf("speed = %v, want 674502", update.SpeedBytesPerSecond)
	}
	if update.ETASeconds != 4 {
		t.Errorf("eta = %d, want 4", update.ETASeconds)
	}
	if update.Filename != `C:\Downloads\trailer_400p.ogg` {
		t.Errorf("filename = %q, want the destination yt-dlp announced", update.Filename)
	}
}

func TestParseAria2cStatusLineWithoutAnEta(t *testing.T) {
	parser := NewProgressParser("", "", 1)

	// aria2c leaves the countdown out of its last lines.
	update, ok := parser.Parse(sampleAria2cLastLine)
	if !ok {
		t.Fatal("the status line was not recognised")
	}
	if update.ETASeconds != 0 {
		t.Errorf("eta = %d, want none", update.ETASeconds)
	}
	if update.SpeedBytesPerSecond != 912301 {
		t.Errorf("speed = %v, want 912301", update.SpeedBytesPerSecond)
	}
}

func TestParseAria2cAbbreviatedSizes(t *testing.T) {
	parser := NewProgressParser("", "", 1)

	// The application asks for exact byte counts, but a status line in aria2c's
	// default form must still read correctly.
	update, ok := parser.Parse(sampleAria2cAbbreviated)
	if !ok {
		t.Fatal("the status line was not recognised")
	}
	if want := int64(1363148); update.DownloadedBytes != want {
		t.Errorf("downloaded = %d, want %d", update.DownloadedBytes, want)
	}
	if want := int64(4299161); update.TotalBytes != want {
		t.Errorf("total = %d, want %d", update.TotalBytes, want)
	}
	if update.SpeedBytesPerSecond != 419*1024 {
		t.Errorf("speed = %v, want %d", update.SpeedBytesPerSecond, 419*1024)
	}
}

func TestParseAria2cEtaForms(t *testing.T) {
	tests := map[string]int{
		"43s":     43,
		"4m51s":   4*60 + 51,
		"2m":      2 * 60,
		"1h2m3s":  3600 + 2*60 + 3,
		"3h":      3 * 3600,
		"1h5s":    3600 + 5,
		"garbage": 0,
	}

	for eta, want := range tests {
		parser := NewProgressParser("", "", 1)
		update, ok := parser.Parse("[#c7f6bb 100B/1000B(10%) CN:1 DL:10B ETA:" + eta + "]")
		if !ok {
			t.Errorf("a status line with ETA %s was not recognised", eta)
			continue
		}
		if update.ETASeconds != want {
			t.Errorf("ETA %s = %d seconds, want %d", eta, update.ETASeconds, want)
		}
	}
}

func TestAria2cProgressIsSpreadAcrossStreams(t *testing.T) {
	parser := NewProgressParser("137", "140", 2)

	parser.Parse(`[download] Destination: C:\Downloads\Clip.f137.mp4`)
	video, _ := parser.Parse("[#aaaaaa 500B/1000B(50%) CN:4 DL:10B ETA:1s]")
	if video.Stage != StageDownloadingVideo {
		t.Errorf("stage = %q, want the video stream to be named", video.Stage)
	}
	if video.OverallPercent != 25 {
		t.Errorf("overall = %v, want 25: half of the first of two streams", video.OverallPercent)
	}

	parser.Parse(`[download] Destination: C:\Downloads\Clip.f140.m4a`)
	audio, _ := parser.Parse("[#bbbbbb 500B/1000B(50%) CN:4 DL:10B ETA:1s]")
	if audio.Stage != StageDownloadingAudio {
		t.Errorf("stage = %q, want the audio stream to be named", audio.Stage)
	}
	if audio.OverallPercent != 75 {
		t.Errorf("overall = %v, want 75: half of the second of two streams", audio.OverallPercent)
	}
}

func TestLineStuckBehindAnAria2cStatusLineIsStillRead(t *testing.T) {
	// aria2c ends its status line without a line break, so whatever yt-dlp
	// prints next arrives on the same line. That next line is the one saying
	// the file is finished, and it must not be lost.
	parser := NewProgressParser("", "", 1)
	finished := progressMarker + `{"filename": "clip.ogg", "status": "finished", "downloaded_bytes": 4360399, "total_bytes": 4360399}`

	update, ok := parser.Parse(sampleAria2cLastLine + finished)
	if !ok {
		t.Fatal("the line behind the status line was not recognised")
	}
	if update.Percent != 100 {
		t.Errorf("percent = %v, want the finished line to be the one that counts", update.Percent)
	}
	if update.DownloadedBytes != 4360399 {
		t.Errorf("downloaded = %d, want the finished size", update.DownloadedBytes)
	}

	// The final path can be stuck there in the same way.
	parser = NewProgressParser("", "", 1)
	update, ok = parser.Parse(sampleAria2cLastLine + destinationMarker + `C:\Downloads\clip.ogg`)
	if !ok || update.FinalPath != `C:\Downloads\clip.ogg` {
		t.Errorf("final path = %q (recognised %v), want it read from behind the status line", update.FinalPath, ok)
	}
}

func TestBracketedLinesThatAreNotStatusLinesAreLeftAlone(t *testing.T) {
	parser := NewProgressParser("", "", 1)

	for _, line := range []string{
		"[#notastatusline]",
		"[#c7f6bb waiting]",
		"[generic] Extracting something else",
		"[#c7f6bb 100B/1000B(10%) CN:1 DL:10B",
	} {
		if update, ok := parser.Parse(line); ok && update.DownloadedBytes > 0 {
			t.Errorf("%q was read as progress: %+v", line, update)
		}
	}
}

func TestOutputIsSplitAtCarriageReturnsToo(t *testing.T) {
	// What actually arrives on the pipe: yt-dlp ends its lines with a line
	// break, and aria2c redraws its status line with carriage returns, the way
	// a terminal would want it.
	blank := strings.Repeat(" ", 79)
	output := "[download] Destination: clip.ogg\r\n" +
		"\r" + blank + "\r" + sampleAria2cLine +
		"\r" + blank + "\r" + sampleAria2cLastLine +
		progressMarker + "{}\n" +
		destinationMarker + "clip.ogg\n"

	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Split(splitOutputLines)

	var lines []string
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}

	want := []string{
		"[download] Destination: clip.ogg",
		sampleAria2cLine,
		sampleAria2cLastLine + progressMarker + "{}",
		destinationMarker + "clip.ogg",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("lines:\n got %q\nwant %q", lines, want)
	}
}

func TestOutputWithoutATrailingLineBreakIsNotLost(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("first\nlast"))
	scanner.Split(splitOutputLines)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if want := []string{"first", "last"}; !reflect.DeepEqual(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
}

func TestAria2cTakesSingleFileStreams(t *testing.T) {
	options := baseOptions()
	options.Speed = SpeedOptions{Connections: 8, Aria2cPath: `C:\Program Files\aria2\aria2c.exe`}

	args, err := BuildDownloadArgs(options)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Naming the protocols keeps aria2c to single-file streams. A stream
	// delivered in fragments stays with yt-dlp, which reports its progress.
	if got, _ := argValue(args, "--downloader"); got != `http,ftp:C:\Program Files\aria2\aria2c.exe` {
		t.Errorf("--downloader = %q, want aria2c for http and ftp only", got)
	}
	if got, _ := argValue(args, "--downloader-args"); got != "aria2c:-x 8 -s 8 --human-readable=false" {
		t.Errorf("--downloader-args = %q, want the connections and exact byte counts", got)
	}
	if got, _ := argValue(args, "--concurrent-fragments"); got != "8" {
		t.Errorf("--concurrent-fragments = %q, want fragmented streams to keep their connections", got)
	}
}

func TestAria2cIsLeftOutWhenThereIsNothingToGain(t *testing.T) {
	tests := []struct {
		name  string
		speed SpeedOptions
	}{
		{name: "one connection", speed: SpeedOptions{Connections: 1, Aria2cPath: `C:\tools\aria2c.exe`}},
		{name: "no connections set", speed: SpeedOptions{Aria2cPath: `C:\tools\aria2c.exe`}},
		{name: "no aria2c", speed: SpeedOptions{Connections: 8}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := baseOptions()
			options.Speed = test.speed

			args, err := BuildDownloadArgs(options)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if hasArg(args, "--downloader") || hasArg(args, "--downloader-args") {
				t.Errorf("args = %v, want yt-dlp's own downloader", args)
			}
		})
	}
}

func TestPlainRetryDropsAria2c(t *testing.T) {
	options := baseOptions()
	options.Speed = SpeedOptions{Connections: 8, Aria2cPath: `C:\tools\aria2c.exe`}

	args, err := BuildDownloadArgs(options.PlainRetry())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if hasArg(args, "--downloader") {
		t.Errorf("a plain retry still uses aria2c: %v", args)
	}
}
