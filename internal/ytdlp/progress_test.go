package ytdlp

import (
	"testing"
)

// These lines are real yt-dlp output, produced with the switches ReportingArgs
// returns.
const (
	sampleProgressLine = progressMarker + `{"status": "downloading", "downloaded_bytes": 1024, "total_bytes": 223779, ` +
		`"tmpfilename": "dl\\Me at the zoo.f395.mp4.part", "filename": "dl\\Me at the zoo.f395.mp4", "eta": 12, ` +
		`"speed": 511610.16033353185, "elapsed": 0.045, "ctx_id": null, "_percent": 0.4575943229704306, ` +
		`"_percent_str": "  0.5%", "_default_template": "  0.5% of  218.53KiB at  499.62KiB/s ETA 00:00"}`

	sampleMergerLine = postProcessMarker + `{"status": "started", "postprocessor": "Merger", "_default_template": "Merger started"}`
)

func TestParseDownloadProgress(t *testing.T) {
	parser := NewProgressParser("395", "251", 2)

	update, ok := parser.Parse(sampleProgressLine)
	if !ok {
		t.Fatal("a progress line was not recognised")
	}

	if update.Stage != StageDownloadingVideo {
		t.Errorf("stage = %q, want the video stream to be named", update.Stage)
	}
	if update.StreamLabel != "Video" {
		t.Errorf("stream label = %q, want Video", update.StreamLabel)
	}
	if update.DownloadedBytes != 1024 {
		t.Errorf("downloaded = %d, want 1024", update.DownloadedBytes)
	}
	if update.TotalBytes != 223779 || update.TotalIsEstimate {
		t.Errorf("total = %d (estimate %v), want the exact size", update.TotalBytes, update.TotalIsEstimate)
	}
	if update.ETASeconds != 12 {
		t.Errorf("eta = %d, want 12", update.ETASeconds)
	}
	if update.SpeedBytesPerSecond < 511610 || update.SpeedBytesPerSecond > 511611 {
		t.Errorf("speed = %v, want the reported rate", update.SpeedBytesPerSecond)
	}

	// The first of two streams at half a percent is a quarter of a percent
	// overall.
	if update.OverallPercent >= update.Percent {
		t.Errorf("overall = %v, want less than the per-file %v while two streams are expected",
			update.OverallPercent, update.Percent)
	}
}

func TestProgressEstimatedSize(t *testing.T) {
	parser := NewProgressParser("", "", 1)
	line := progressMarker + `{"status": "downloading", "downloaded_bytes": 500, "total_bytes": null, ` +
		`"total_bytes_estimate": 4000, "filename": "clip.mp4"}`

	update, ok := parser.Parse(line)
	if !ok {
		t.Fatal("the line was not recognised")
	}
	if update.TotalBytes != 4000 || !update.TotalIsEstimate {
		t.Errorf("total = %d (estimate %v), want 4000 marked as an estimate", update.TotalBytes, update.TotalIsEstimate)
	}
	if update.Percent < 12 || update.Percent > 13 {
		t.Errorf("percent = %v, want it derived from the estimate", update.Percent)
	}
}

func TestProgressFragmentFallback(t *testing.T) {
	// A stream with no known size still reports fragments.
	parser := NewProgressParser("", "", 1)
	line := progressMarker + `{"status": "downloading", "downloaded_bytes": 900, "fragment_index": 3, "fragment_count": 12}`

	update, ok := parser.Parse(line)
	if !ok {
		t.Fatal("the line was not recognised")
	}
	if update.Percent < 24 || update.Percent > 26 {
		t.Errorf("percent = %v, want it derived from the fragment count", update.Percent)
	}
}

func TestProgressWithNothingKnownDoesNotInventAPercentage(t *testing.T) {
	parser := NewProgressParser("", "", 1)
	line := progressMarker + `{"status": "downloading", "downloaded_bytes": 900}`

	update, ok := parser.Parse(line)
	if !ok {
		t.Fatal("the line was not recognised")
	}
	if update.Percent != 0 {
		t.Errorf("percent = %v, want 0 when nothing is known about the total", update.Percent)
	}
	if update.DownloadedBytes != 900 {
		t.Error("what is known should still be reported")
	}
}

func TestParsePostProcessingStages(t *testing.T) {
	tests := []struct {
		processor string
		want      string
	}{
		{processor: "Merger", want: StageMerging},
		{processor: "VideoRemuxer", want: StageRemuxing},
		{processor: "ExtractAudio", want: StageConverting},
		{processor: "FFmpegMetadata", want: StageEmbeddingMetadata},
		{processor: "FFmpegEmbedSubtitle", want: StageEmbeddingSubtitles},
		{processor: "EmbedThumbnail", want: StageEmbeddingThumbnail},
		{processor: "MoveFiles", want: StagePostProcessing},
		{processor: "SomethingNew", want: StagePostProcessing},
	}

	for _, test := range tests {
		t.Run(test.processor, func(t *testing.T) {
			parser := NewProgressParser("", "", 1)
			line := postProcessMarker + `{"status": "started", "postprocessor": "` + test.processor + `"}`

			update, ok := parser.Parse(line)
			if !ok {
				t.Fatal("the post-processing line was not recognised")
			}
			if update.Stage != test.want {
				t.Errorf("stage = %q, want %q", update.Stage, test.want)
			}
			if update.Message == "" {
				t.Error("no message was produced for the stage")
			}
		})
	}
}

func TestPostProcessingFinishIsNotReported(t *testing.T) {
	parser := NewProgressParser("", "", 1)
	line := postProcessMarker + `{"status": "finished", "postprocessor": "Merger"}`

	if _, ok := parser.Parse(line); ok {
		t.Error("only the start of a post-processing step is interesting")
	}
}

func TestParseFinalPath(t *testing.T) {
	parser := NewProgressParser("", "", 1)
	path := `C:\Users\example\Videos\GrabOne\Me at the zoo.mkv`

	update, ok := parser.Parse(destinationMarker + path)
	if !ok {
		t.Fatal("the final path line was not recognised")
	}
	if update.FinalPath != path {
		t.Errorf("final path = %q, want %q", update.FinalPath, path)
	}
}

func TestParseStreamOrderWithoutFormatIdentifiers(t *testing.T) {
	// When no identifiers were chosen, the order of the files is what tells the
	// video stream from the audio one.
	parser := NewProgressParser("", "", 2)

	video, _ := parser.Parse("[download] Destination: clip.f399.mp4")
	if video.Stage != StageDownloadingVideo {
		t.Errorf("first stream stage = %q, want the video stage", video.Stage)
	}

	audio, _ := parser.Parse("[download] Destination: clip.f251.webm")
	if audio.Stage != StageDownloadingAudio {
		t.Errorf("second stream stage = %q, want the audio stage", audio.Stage)
	}
}

func TestParseCollectionCounter(t *testing.T) {
	parser := NewProgressParser("", "", 1)

	update, ok := parser.Parse("[download] Downloading item 3 of 12")
	if !ok {
		t.Fatal("the item counter was not recognised")
	}
	if update.ItemIndex != 3 || update.ItemCount != 12 {
		t.Errorf("item = %d of %d, want 3 of 12", update.ItemIndex, update.ItemCount)
	}

	// Progress within an item is now part of the whole collection.
	progress, _ := parser.Parse(progressMarker + `{"status": "downloading", "downloaded_bytes": 50, "total_bytes": 100, "filename": "item3.mp4"}`)
	if progress.OverallPercent < 20 || progress.OverallPercent > 22 {
		t.Errorf("overall = %v, want the position within the collection", progress.OverallPercent)
	}
}

func TestParseExpectedStreamsFromInfoLine(t *testing.T) {
	parser := NewProgressParser("", "", 1)
	parser.Parse("[info] jNQXAC9IVRw: Downloading 1 format(s): 395+251")

	if parser.expectedStreams != 2 {
		t.Errorf("expected streams = %d, want 2 from the merge expression", parser.expectedStreams)
	}
}

func TestParserCollectsErrorLines(t *testing.T) {
	parser := NewProgressParser("", "", 1)

	parser.Parse("[download] Destination: clip.mp4")
	parser.Parse("ERROR: [youtube] abc: Video unavailable")

	if parser.ErrorText() == "" {
		t.Error("error lines must be kept so a failure can be classified")
	}
}

func TestParserIgnoresNoise(t *testing.T) {
	parser := NewProgressParser("", "", 1)

	for _, line := range []string{"", "   ", "[youtube] abc: Downloading webpage", "WARNING: something minor"} {
		if _, ok := parser.Parse(line); ok {
			t.Errorf("line %q should not produce a progress update", line)
		}
	}
}
