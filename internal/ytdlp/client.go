// Package ytdlp drives the yt-dlp executable. All extraction and downloading
// is performed by yt-dlp: this package builds argument lists, runs the process
// and interprets its machine readable output. It contains no site specific
// logic of its own.
package ytdlp

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"grabone/internal/logging"
	"grabone/internal/procutil"
)

// Client runs yt-dlp.
type Client struct {
	binaryPath string
	ffmpegPath string
	logger     *logging.Logger
}

// NewClient builds a client for the given executable. ffmpegPath may be empty,
// in which case yt-dlp looks for FFmpeg itself.
func NewClient(binaryPath, ffmpegPath string, logger *logging.Logger) *Client {
	if logger == nil {
		logger = logging.Discard()
	}
	return &Client{binaryPath: binaryPath, ffmpegPath: ffmpegPath, logger: logger}
}

// Available reports whether a yt-dlp executable is configured.
func (c *Client) Available() bool { return c != nil && strings.TrimSpace(c.binaryPath) != "" }

// BinaryPath returns the executable this client runs.
func (c *Client) BinaryPath() string {
	if c == nil {
		return ""
	}
	return c.binaryPath
}

// FFmpegPath returns the FFmpeg location passed to yt-dlp.
func (c *Client) FFmpegPath() string {
	if c == nil {
		return ""
	}
	return c.ffmpegPath
}

// BaseArgs returns the switches used for every invocation.
//
// --ignore-config matters: without it a yt-dlp.conf somewhere on the system
// could silently change formats, output paths or post-processing, and the
// application would be reporting something other than what it ran.
func (c *Client) BaseArgs() []string {
	args := []string{"--ignore-config", "--no-config-locations", "--color", "no_color"}
	if path := strings.TrimSpace(c.ffmpegPath); path != "" {
		args = append(args, "--ffmpeg-location", path)
	}
	return args
}

// Command builds a yt-dlp command with the base switches applied. Arguments are
// always passed as an array: no shell is involved, so a URL or a title can
// never be interpreted as a command.
func (c *Client) Command(ctx context.Context, args ...string) *exec.Cmd {
	full := append(c.BaseArgs(), args...)
	cmd := exec.CommandContext(ctx, c.binaryPath, full...)
	procutil.Configure(cmd)
	return cmd
}

// Run executes yt-dlp and collects its output. The returned string is the
// combined stderr text, which is what the error classifier inspects.
func (c *Client) Run(ctx context.Context, args ...string) (stdout []byte, stderr string, err error) {
	if !c.Available() {
		return nil, "", MissingDependencyError("yt-dlp", "no executable configured")
	}

	cmd := c.Command(ctx, args...)

	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	c.logger.Info("running yt-dlp", "args", strings.Join(logging.RedactArgs(args), " "))

	runErr := cmd.Run()
	if ctx.Err() != nil {
		runErr = ctx.Err()
	}
	return outBuffer.Bytes(), errBuffer.String(), runErr
}

// versionPattern matches a yt-dlp version such as "2026.08.19".
var versionPattern = regexp.MustCompile(`\d{4}\.\d{2}\.\d{2}(\.\d+)?`)

// ParseVersion extracts the version from "yt-dlp --version" output. Anything
// unexpected is returned as-is rather than discarded.
func ParseVersion(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	if match := versionPattern.FindString(trimmed); match != "" {
		return match
	}
	if index := strings.IndexAny(trimmed, "\r\n"); index >= 0 {
		return strings.TrimSpace(trimmed[:index])
	}
	return trimmed
}

// Version runs the executable and returns its version.
func (c *Client) Version(ctx context.Context) (string, error) {
	stdout, stderr, err := c.Run(ctx, "--version")
	if err != nil {
		return "", fmt.Errorf("read yt-dlp version: %w", ClassifyError(stderr, err))
	}
	return ParseVersion(string(stdout)), nil
}
