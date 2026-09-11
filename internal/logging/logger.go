// Package logging provides the application's lightweight file logger.
//
// Sensitive material is never written: cookie files, cookie contents and
// authentication headers are redacted before an argument list is logged.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	logFileName    = "grabone.log"
	maxLogFileSize = 2 << 20 // rotate once the log passes 2 MiB
)

// Logger wraps slog with the file handle so it can be closed on shutdown.
type Logger struct {
	*slog.Logger
	file *os.File
}

// New creates a logger writing to dir/grabone.log and to stderr. It never
// fails: when the file cannot be opened, logging continues to stderr only.
func New(dir string) *Logger {
	writers := []io.Writer{os.Stderr}

	var file *os.File
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			path := filepath.Join(dir, logFileName)
			rotateIfLarge(path)
			if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
				file = f
				writers = append(writers, f)
			}
		}
	}

	handler := slog.NewTextHandler(resilientWriter{writers: writers}, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{Logger: slog.New(handler), file: file}
}

// resilientWriter writes to every destination and ignores one that fails.
//
// A windowed application has no console, so writing to stderr fails. An
// io.MultiWriter stops at the first failing writer, which would mean nothing
// ever reaches the log file in a normal build.
type resilientWriter struct {
	writers []io.Writer
}

func (w resilientWriter) Write(data []byte) (int, error) {
	for _, writer := range w.writers {
		_, _ = writer.Write(data)
	}
	return len(data), nil
}

// Discard returns a logger that writes nowhere, for use in tests.
func Discard() *Logger {
	return &Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// Close releases the log file.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func rotateIfLarge(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogFileSize {
		return
	}
	stamp := time.Now().Format("20060102-150405")
	_ = os.Rename(path, fmt.Sprintf("%s.%s", path, stamp))
}

// sensitiveFlags are command line switches whose values must not reach the log.
var sensitiveFlags = map[string]bool{
	"--cookies":              true,
	"--cookies-from-browser": true,
	"--username":             true,
	"--password":             true,
	"--video-password":       true,
	"--ap-username":          true,
	"--ap-password":          true,
	"--add-header":           true,
	"--netrc-cmd":            true,
}

// RedactArgs returns a copy of args safe to log: values belonging to
// authentication related switches are replaced with a placeholder.
func RedactArgs(args []string) []string {
	out := make([]string, 0, len(args))
	redactNext := false
	for _, arg := range args {
		switch {
		case redactNext:
			out = append(out, "<redacted>")
			redactNext = false
		case sensitiveFlags[arg]:
			out = append(out, arg)
			redactNext = true
		case strings.HasPrefix(arg, "--cookies=") || strings.HasPrefix(arg, "--cookies-from-browser="):
			out = append(out, strings.SplitN(arg, "=", 2)[0]+"=<redacted>")
		default:
			out = append(out, arg)
		}
	}
	return out
}
