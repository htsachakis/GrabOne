package ytdlp

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Download stages reported to the interface.
const (
	StageStarting           = "starting"
	StageAnalyzing          = "analyzing"
	StageDownloading        = "downloading"
	StageDownloadingVideo   = "downloading-video"
	StageDownloadingAudio   = "downloading-audio"
	StageMerging            = "merging"
	StageRemuxing           = "remuxing"
	StageConverting         = "converting"
	StageEmbeddingMetadata  = "embedding-metadata"
	StageEmbeddingSubtitles = "embedding-subtitles"
	StageEmbeddingThumbnail = "embedding-thumbnail"
	StagePostProcessing     = "post-processing"
	StageFinished           = "finished"
	StageFailed             = "failed"
	StageCancelled          = "cancelled"
)

// ProgressUpdate is one parsed progress report. Fields that yt-dlp did not
// provide stay zero: the interface degrades to showing less rather than showing
// something invented.
type ProgressUpdate struct {
	Stage string `json:"stage"`

	// Percent is the progress of the file currently transferring.
	Percent float64 `json:"percent"`
	// OverallPercent accounts for a download made of several streams or items.
	OverallPercent float64 `json:"overallPercent"`

	DownloadedBytes int64 `json:"downloadedBytes"`
	TotalBytes      int64 `json:"totalBytes"`
	// TotalIsEstimate marks a size that yt-dlp could only estimate.
	TotalIsEstimate bool `json:"totalIsEstimate"`

	SpeedBytesPerSecond float64 `json:"speedBytesPerSecond"`
	ETASeconds          int     `json:"etaSeconds"`

	Filename string `json:"filename"`
	Message  string `json:"message"`

	// StreamLabel says which part of the media is transferring.
	StreamLabel string `json:"streamLabel"`

	// ItemIndex and ItemCount track position within a collection download.
	ItemIndex int `json:"itemIndex"`
	ItemCount int `json:"itemCount"`

	// FinalPath is set once the finished file has been moved into place.
	FinalPath string `json:"finalPath"`
}

// rawProgress is the progress dictionary yt-dlp emits as JSON.
type rawProgress struct {
	Status             string    `json:"status"`
	DownloadedBytes    flexInt   `json:"downloaded_bytes"`
	TotalBytes         flexInt   `json:"total_bytes"`
	TotalBytesEstimate flexInt   `json:"total_bytes_estimate"`
	Speed              flexFloat `json:"speed"`
	ETA                flexFloat `json:"eta"`
	Elapsed            flexFloat `json:"elapsed"`
	Filename           string    `json:"filename"`
	TempFilename       string    `json:"tmpfilename"`
	Percent            flexFloat `json:"_percent"`
	FragmentIndex      flexInt   `json:"fragment_index"`
	FragmentCount      flexInt   `json:"fragment_count"`
}

type rawPostProgress struct {
	Status        string `json:"status"`
	PostProcessor string `json:"postprocessor"`
}

// ProgressParser turns yt-dlp's output lines into progress updates. It keeps the
// little state needed to tell one stream from another, and is the only place
// that reads yt-dlp's output text.
type ProgressParser struct {
	// VideoFormatID and AudioFormatID let the parser name the stream being
	// transferred. yt-dlp writes each stream to "<title>.f<format id>.<ext>".
	VideoFormatID string
	AudioFormatID string

	expectedStreams int
	seenFiles       []string
	currentFile     string
	itemIndex       int
	itemCount       int
	errorLines      []string
}

// NewProgressParser builds a parser for a download of the given streams.
func NewProgressParser(videoFormatID, audioFormatID string, expectedStreams int) *ProgressParser {
	if expectedStreams < 1 {
		expectedStreams = 1
	}
	return &ProgressParser{
		VideoFormatID:   videoFormatID,
		AudioFormatID:   audioFormatID,
		expectedStreams: expectedStreams,
	}
}

// ErrorText returns the error lines seen so far, for classification when the
// process exits non-zero.
func (p *ProgressParser) ErrorText() string { return strings.Join(p.errorLines, "\n") }

// Parse interprets one line of yt-dlp output. It reports false for lines that
// carry no progress information.
func (p *ProgressParser) Parse(line string) (ProgressUpdate, bool) {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return ProgressUpdate{}, false
	}

	switch {
	case strings.HasPrefix(trimmed, progressMarker):
		return p.parseDownload(strings.TrimPrefix(trimmed, progressMarker))
	case strings.HasPrefix(trimmed, postProcessMarker):
		return p.parsePostProcess(strings.TrimPrefix(trimmed, postProcessMarker))
	case strings.HasPrefix(trimmed, destinationMarker):
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, destinationMarker))
		return ProgressUpdate{Stage: StagePostProcessing, FinalPath: path, Filename: path}, path != ""
	}

	return p.parseTextLine(trimmed)
}

func (p *ProgressParser) parseDownload(payload string) (ProgressUpdate, bool) {
	var raw rawProgress
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return ProgressUpdate{}, false
	}

	filename := firstNonEmpty(raw.Filename, raw.TempFilename)
	p.noteFile(filename)

	update := ProgressUpdate{
		Stage:               p.downloadStage(filename),
		DownloadedBytes:     raw.DownloadedBytes.Int(),
		SpeedBytesPerSecond: raw.Speed.Float(),
		ETASeconds:          int(raw.ETA.Float()),
		Filename:            filename,
		StreamLabel:         p.streamLabel(filename),
		ItemIndex:           p.itemIndex,
		ItemCount:           p.itemCount,
	}

	switch {
	case raw.TotalBytes.Int() > 0:
		update.TotalBytes = raw.TotalBytes.Int()
	case raw.TotalBytesEstimate.Int() > 0:
		update.TotalBytes = raw.TotalBytesEstimate.Int()
		update.TotalIsEstimate = true
	}

	update.Percent = p.percentOf(raw, update)
	update.OverallPercent = p.overallPercent(update.Percent, filename)

	if raw.Status == "finished" {
		update.Percent = 100
		update.OverallPercent = p.overallPercent(100, filename)
	}
	return update, true
}

// percentOf prefers yt-dlp's own percentage, falling back to a byte or fragment
// ratio. A live stream reports none of these, and then no percentage is shown.
func (p *ProgressParser) percentOf(raw rawProgress, update ProgressUpdate) float64 {
	if value := raw.Percent.Float(); value > 0 {
		return clampPercent(value)
	}
	if update.TotalBytes > 0 && update.DownloadedBytes > 0 {
		return clampPercent(float64(update.DownloadedBytes) / float64(update.TotalBytes) * 100)
	}
	if raw.FragmentCount.Int() > 0 {
		return clampPercent(float64(raw.FragmentIndex.Int()) / float64(raw.FragmentCount.Int()) * 100)
	}
	return 0
}

// overallPercent spreads the per-file percentage across the streams and items
// the download is made of, so the progress bar advances once overall.
func (p *ProgressParser) overallPercent(filePercent float64, filename string) float64 {
	streamPosition := p.filePosition(filename)
	streams := p.expectedStreams
	if streamPosition > streams {
		streams = streamPosition
	}

	withinItem := (float64(streamPosition-1) + filePercent/100) / float64(streams) * 100

	if p.itemCount > 1 && p.itemIndex > 0 {
		return clampPercent((float64(p.itemIndex-1) + withinItem/100) / float64(p.itemCount) * 100)
	}
	return clampPercent(withinItem)
}

func (p *ProgressParser) parsePostProcess(payload string) (ProgressUpdate, bool) {
	var raw rawPostProgress
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return ProgressUpdate{}, false
	}
	if raw.Status != "started" {
		// Only the start of each step is interesting: the finish of one is
		// immediately followed by the start of the next, or by the final path.
		return ProgressUpdate{}, false
	}

	stage, message := postProcessorStage(raw.PostProcessor)
	return ProgressUpdate{
		Stage:          stage,
		Message:        message,
		Percent:        100,
		OverallPercent: 100,
		ItemIndex:      p.itemIndex,
		ItemCount:      p.itemCount,
	}, true
}

// postProcessorStage maps a yt-dlp post-processor name to a stage and a line of
// text for the interface.
func postProcessorStage(name string) (string, string) {
	switch {
	case strings.Contains(name, "Merger"):
		return StageMerging, "Merging video and audio with FFmpeg"
	case strings.Contains(name, "Remux"):
		return StageRemuxing, "Remuxing into the chosen container"
	case strings.Contains(name, "ExtractAudio"):
		return StageConverting, "Extracting audio with FFmpeg"
	case strings.Contains(name, "VideoConvertor"):
		return StageConverting, "Converting with FFmpeg"
	case strings.Contains(name, "Metadata"):
		return StageEmbeddingMetadata, "Embedding metadata"
	case strings.Contains(name, "EmbedSubtitle"), strings.Contains(name, "SubtitlesConvertor"):
		return StageEmbeddingSubtitles, "Embedding subtitles"
	case strings.Contains(name, "Thumbnail"):
		return StageEmbeddingThumbnail, "Embedding thumbnail"
	case strings.Contains(name, "MoveFiles"):
		return StagePostProcessing, "Finalising files"
	case name == "":
		return StagePostProcessing, "Post-processing"
	default:
		return StagePostProcessing, "Post-processing: " + name
	}
}

// parseTextLine reads the informational lines yt-dlp prints. These supplement
// the machine readable output: they say which formats were chosen and how far
// through a collection the download is.
func (p *ProgressParser) parseTextLine(line string) (ProgressUpdate, bool) {
	upper := strings.ToUpper(line)
	if strings.HasPrefix(upper, "ERROR:") || strings.Contains(upper, "ERROR: ") {
		p.errorLines = append(p.errorLines, line)
		return ProgressUpdate{}, false
	}
	if strings.HasPrefix(upper, "WARNING:") {
		return ProgressUpdate{}, false
	}

	switch {
	case strings.HasPrefix(line, "[download] Destination:"):
		filename := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
		p.noteFile(filename)
		return ProgressUpdate{
			Stage:       p.downloadStage(filename),
			Filename:    filename,
			StreamLabel: p.streamLabel(filename),
			ItemIndex:   p.itemIndex,
			ItemCount:   p.itemCount,
		}, true

	case strings.HasPrefix(line, "[download] Downloading item"):
		if index, count, ok := parseItemCounter(line); ok {
			p.itemIndex, p.itemCount = index, count
			p.seenFiles = nil
			return ProgressUpdate{
				Stage:     StageDownloading,
				Message:   "Item " + strconv.Itoa(index) + " of " + strconv.Itoa(count),
				ItemIndex: index,
				ItemCount: count,
			}, true
		}

	case strings.Contains(line, "Extracting URL"):
		return ProgressUpdate{Stage: StageAnalyzing, Message: "Reading the page"}, true

	case strings.HasPrefix(line, "[info] ") && strings.Contains(line, "format(s):"):
		// "[info] <id>: Downloading 1 format(s): 137+140" names the streams that
		// are about to be transferred, which is how progress is spread across a
		// merge of two of them.
		if index := strings.LastIndex(line, "format(s): "); index >= 0 {
			selection := strings.TrimSpace(line[index+len("format(s): "):])
			p.expectedStreams = strings.Count(selection, "+") + 1
		}
	}
	return ProgressUpdate{}, false
}

// parseItemCounter reads "[download] Downloading item 3 of 12".
func parseItemCounter(line string) (int, int, bool) {
	fields := strings.Fields(line)
	for position, field := range fields {
		if field != "item" || position+3 >= len(fields) {
			continue
		}
		index, err := strconv.Atoi(fields[position+1])
		if err != nil {
			return 0, 0, false
		}
		count, err := strconv.Atoi(fields[position+3])
		if err != nil {
			return 0, 0, false
		}
		return index, count, true
	}
	return 0, 0, false
}

// noteFile records each distinct file the download touches, which is how the
// parser knows whether it is on the first or second stream.
func (p *ProgressParser) noteFile(filename string) {
	if filename == "" {
		return
	}
	base := strings.TrimSuffix(filename, ".part")
	if p.currentFile == base {
		return
	}
	p.currentFile = base
	for _, seen := range p.seenFiles {
		if seen == base {
			return
		}
	}
	p.seenFiles = append(p.seenFiles, base)
}

func (p *ProgressParser) filePosition(filename string) int {
	base := strings.TrimSuffix(filename, ".part")
	for index, seen := range p.seenFiles {
		if seen == base {
			return index + 1
		}
	}
	if len(p.seenFiles) == 0 {
		return 1
	}
	return len(p.seenFiles)
}

// downloadStage names the stream being transferred, which lets the interface
// say "Downloading video" rather than just "Downloading".
func (p *ProgressParser) downloadStage(filename string) string {
	switch p.streamKind(filename) {
	case "video":
		return StageDownloadingVideo
	case "audio":
		return StageDownloadingAudio
	default:
		return StageDownloading
	}
}

func (p *ProgressParser) streamLabel(filename string) string {
	switch p.streamKind(filename) {
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	default:
		return ""
	}
}

// streamKind identifies a stream from the format identifier yt-dlp puts in the
// filename, falling back to the order the files appeared in.
func (p *ProgressParser) streamKind(filename string) string {
	if filename != "" {
		if p.VideoFormatID != "" && strings.Contains(filename, ".f"+p.VideoFormatID+".") {
			return "video"
		}
		if p.AudioFormatID != "" && strings.Contains(filename, ".f"+p.AudioFormatID+".") {
			return "audio"
		}
	}
	if p.expectedStreams < 2 {
		return ""
	}
	switch p.filePosition(filename) {
	case 1:
		return "video"
	case 2:
		return "audio"
	default:
		return ""
	}
}

func clampPercent(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}
