package ytdlp

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"grabone/internal/logging"
	"grabone/internal/procutil"
)

// maxLineSize bounds a single output line. yt-dlp can print long paths and long
// JSON progress dictionaries, so the default scanner buffer is not enough.
const maxLineSize = 1 << 20

// stderrCaptureLimit bounds how much output is kept for error reporting.
const stderrCaptureLimit = 64 << 10

// DownloadResult describes a finished download.
type DownloadResult struct {
	// FinalPath is the file yt-dlp reported after moving it into place. It is
	// empty when yt-dlp did not report one, for example for a collection with
	// several files.
	FinalPath string
	// Args is the argument list that was executed, for the log and the preview.
	Args []string
}

// Download runs a download, reporting progress through onProgress. The call
// blocks until the process exits; cancelling ctx stops the download and any
// FFmpeg process it started.
func (c *Client) Download(ctx context.Context, options DownloadOptions, onProgress func(ProgressUpdate)) (*DownloadResult, *Error) {
	if !c.Available() {
		return nil, MissingDependencyError("yt-dlp", "no executable configured")
	}

	downloadArgs, err := BuildDownloadArgs(options)
	if err != nil {
		return nil, &Error{Kind: KindInvalidURL, Message: err.Error()}
	}
	args := append(ReportingArgs(), downloadArgs...)

	cmd := c.Command(ctx, args...)
	// exec kills only the process it started. yt-dlp launches FFmpeg for
	// merging and conversion, so cancelling has to take down the whole tree.
	cmd.Cancel = func() error { return procutil.KillTree(cmd) }
	cmd.WaitDelay = 5 * time.Second

	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return nil, &Error{Kind: KindUnknown, Message: "Could not start the download.", Details: pipeErr.Error()}
	}
	stderr, pipeErr := cmd.StderrPipe()
	if pipeErr != nil {
		return nil, &Error{Kind: KindUnknown, Message: "Could not start the download.", Details: pipeErr.Error()}
	}

	c.logger.Info("starting download",
		"url", options.URL,
		"type", options.DownloadType,
		"args", strings.Join(logging.RedactArgs(args), " "),
	)

	if startErr := cmd.Start(); startErr != nil {
		return nil, &Error{
			Kind:    KindMissingDependency,
			Message: "yt-dlp could not be started.",
			Details: startErr.Error(),
		}
	}

	parser := NewProgressParser(options.VideoFormatID, options.AudioFormatID, expectedStreams(options))

	var (
		mutex     sync.Mutex
		finalPath string
		captured  strings.Builder
		waitGroup sync.WaitGroup
	)

	emit := func(update ProgressUpdate) {
		mutex.Lock()
		if update.FinalPath != "" {
			finalPath = update.FinalPath
		}
		mutex.Unlock()

		if onProgress != nil {
			onProgress(update)
		}
	}

	consume := func(reader io.Reader, capture bool) {
		defer waitGroup.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64<<10), maxLineSize)
		for scanner.Scan() {
			line := scanner.Text()
			if capture {
				mutex.Lock()
				if captured.Len() < stderrCaptureLimit {
					captured.WriteString(line)
					captured.WriteByte('\n')
				}
				mutex.Unlock()
			}
			if update, ok := parser.Parse(line); ok {
				emit(update)
			}
		}
	}

	waitGroup.Add(2)
	go consume(stdout, false)
	go consume(stderr, true)
	waitGroup.Wait()

	waitErr := cmd.Wait()

	mutex.Lock()
	details := strings.TrimSpace(captured.String())
	path := finalPath
	mutex.Unlock()

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ClassifyError(details, ctxErr)
	}
	if waitErr != nil {
		if parsed := parser.ErrorText(); parsed != "" {
			details = strings.TrimSpace(parsed + "\n" + details)
		}
		classified := ClassifyError(details, waitErr)
		c.logger.Error("download failed", "url", options.URL, "kind", string(classified.Kind), "message", classified.Message)
		return nil, classified
	}

	c.logger.Info("download finished", "url", options.URL, "path", path)
	return &DownloadResult{FinalPath: path, Args: downloadArgs}, nil
}

// expectedStreams reports how many files the download is expected to transfer,
// which lets progress be reported across a merge of two streams.
func expectedStreams(options DownloadOptions) int {
	if options.DownloadType == DownloadTypeVideoAudio &&
		options.CombinedFormatID == "" &&
		(options.VideoFormatID != "" && options.AudioFormatID != "" || options.VideoFormatID == "" && options.AudioFormatID == "") {
		return 2
	}
	return 1
}
