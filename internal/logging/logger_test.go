package logging

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactArgsHidesCredentials(t *testing.T) {
	args := []string{
		"-f", "137+140",
		"--cookies-from-browser", "firefox:Default",
		"--cookies", `C:\Users\example\cookies.txt`,
		"--username", "someone@example.com",
		"--password", "hunter2",
		"--add-header", "Authorization:Bearer abc123",
		"-o", "%(title)s.%(ext)s",
		"--", "https://example.com/watch",
	}

	redacted := RedactArgs(args)
	joined := strings.Join(redacted, " ")

	for _, secret := range []string{"firefox:Default", "cookies.txt", "someone@example.com", "hunter2", "Bearer abc123"} {
		if strings.Contains(joined, secret) {
			t.Errorf("redacted arguments still contain %q: %s", secret, joined)
		}
	}

	// The switches themselves stay, so the log still shows what was requested.
	for _, flag := range []string{"--cookies-from-browser", "--cookies", "--username", "--password"} {
		if !strings.Contains(joined, flag) {
			t.Errorf("switch %q should remain visible in the log", flag)
		}
	}
	// Everything unrelated is untouched.
	for _, kept := range []string{"137+140", "%(title)s.%(ext)s", "https://example.com/watch"} {
		if !strings.Contains(joined, kept) {
			t.Errorf("%q should not have been redacted", kept)
		}
	}
	if len(redacted) != len(args) {
		t.Errorf("redaction changed the argument count: %d, want %d", len(redacted), len(args))
	}
}

func TestRedactArgsHandlesInlineValues(t *testing.T) {
	redacted := RedactArgs([]string{"--cookies=C:\\secret\\cookies.txt", "--cookies-from-browser=chrome"})

	for _, arg := range redacted {
		if strings.Contains(arg, "cookies.txt") || strings.Contains(arg, "chrome") {
			t.Errorf("inline value was not redacted: %s", arg)
		}
		if !strings.Contains(arg, "<redacted>") {
			t.Errorf("argument %q should be marked as redacted", arg)
		}
	}
}

func TestLoggerWritesToFile(t *testing.T) {
	directory := t.TempDir()

	logger := New(directory)
	logger.Info("analysis finished", "extractor", "Youtube", "formats", 24)
	if err := logger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(directory, logFileName))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(contents), "analysis finished") {
		t.Errorf("log does not contain the message: %s", contents)
	}
	if !strings.Contains(string(contents), "Youtube") {
		t.Errorf("log does not contain the attributes: %s", contents)
	}
}

func TestLoggerSurvivesAnUnusableDirectory(t *testing.T) {
	// A logger that cannot open its file must still be usable.
	logger := New(string([]byte{0}))
	logger.Info("still works")
	if err := logger.Close(); err != nil {
		t.Errorf("close: %v", err)
	}

	discard := Discard()
	discard.Info("goes nowhere")
	if err := discard.Close(); err != nil {
		t.Errorf("close discard logger: %v", err)
	}
}

// failingWriter stands in for stderr in a windowed application, where the
// handle is invalid and every write fails.
type failingWriter struct{ attempts int }

func (w *failingWriter) Write(data []byte) (int, error) {
	w.attempts++
	return 0, errors.New("no console attached")
}

func TestLoggingSurvivesAFailingDestination(t *testing.T) {
	// The real case this guards: a normal build has no console, so writing to
	// stderr fails. That must not stop the log file from being written.
	var buffer bytes.Buffer
	stderr := &failingWriter{}

	writer := resilientWriter{writers: []io.Writer{stderr, &buffer}}
	logger := slog.New(slog.NewTextHandler(writer, nil))

	logger.Info("download finished", "path", "clip.mkv")

	if stderr.attempts == 0 {
		t.Error("the failing destination should still have been attempted")
	}
	if !strings.Contains(buffer.String(), "download finished") {
		t.Errorf("the working destination received %q, want the log line", buffer.String())
	}
}
