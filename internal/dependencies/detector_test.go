package dependencies

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDisplayNames(t *testing.T) {
	if DisplayName(FFmpeg) != "FFmpeg" || DisplayName(FFprobe) != "FFprobe" || DisplayName(YtDlp) != "yt-dlp" {
		t.Error("dependency names are shown with their own casing")
	}
}

func TestSetReadiness(t *testing.T) {
	set := Set{
		YtDlp:  DependencyStatus{Name: YtDlp, Available: true},
		FFmpeg: DependencyStatus{Name: FFmpeg, Available: false},
	}

	if !set.Ready() {
		t.Error("yt-dlp alone is enough to analyze and download a single stream")
	}
	if set.CanMerge() {
		t.Error("merging is not possible without FFmpeg")
	}
	if len(set.All()) != 4 {
		t.Errorf("All() returned %d statuses, want 4", len(set.All()))
	}

	missing := Set{YtDlp: DependencyStatus{Name: YtDlp, Available: false}}
	if missing.Ready() {
		t.Error("nothing works without yt-dlp")
	}
}

func TestResolveSearchOrder(t *testing.T) {
	applicationDir := t.TempDir()
	configuredDir := t.TempDir()

	// Place a stand-in executable in each location.
	appCopy := filepath.Join(applicationDir, executableName(YtDlp))
	if err := os.WriteFile(appCopy, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	configured := filepath.Join(configuredDir, "custom-yt-dlp.exe")
	if err := os.WriteFile(configured, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	detector := NewDetector(applicationDir, "")

	t.Run("configured path wins", func(t *testing.T) {
		path, source, err := detector.resolve(YtDlp, configured)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if path != configured {
			t.Errorf("path = %q, want the configured executable", path)
		}
		if source != sourceConfiguredLabel {
			t.Errorf("source = %q, want the configured path", source)
		}
	})

	t.Run("application folder is searched next", func(t *testing.T) {
		path, source, err := detector.resolve(YtDlp, "")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if path != appCopy {
			t.Errorf("path = %q, want the copy next to the application", path)
		}
		if source != SourceApplicationDir {
			t.Errorf("source = %q, want the application directory", source)
		}
	})

	t.Run("a configured path that does not exist is an error", func(t *testing.T) {
		_, _, err := detector.resolve(YtDlp, filepath.Join(configuredDir, "missing.exe"))
		if err == nil {
			t.Fatal("a missing configured executable should be reported")
		}
		if !strings.Contains(err.Error(), "configured path") {
			t.Errorf("error = %q, want it to mention the configured path", err)
		}
	})

	t.Run("a directory is not an executable", func(t *testing.T) {
		if _, _, err := detector.resolve(YtDlp, configuredDir); err == nil {
			t.Error("a folder should not be accepted as an executable")
		}
	})
}

func TestDetectOneReportsMissingExecutable(t *testing.T) {
	detector := NewDetector(t.TempDir(), "")

	status := detector.DetectOne(context.Background(), YtDlp, filepath.Join(t.TempDir(), "nothing-here.exe"))

	if status.Available {
		t.Error("a missing executable must never be reported as available")
	}
	if status.Error == "" {
		t.Error("the reason should be available to the settings screen")
	}
	if status.Source != SourceNotFound {
		t.Errorf("source = %q, want %q", status.Source, SourceNotFound)
	}
	if !status.Required {
		t.Error("yt-dlp is required")
	}
}

func TestDetectOneRunsTheExecutable(t *testing.T) {
	// A file that exists but cannot run must be reported as unavailable:
	// existence alone is not proof that a dependency is usable.
	directory := t.TempDir()
	fake := filepath.Join(directory, executableName(FFmpeg))

	contents := []byte("this is not a program")
	if err := os.WriteFile(fake, contents, 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	status := NewDetector(directory, "").DetectOne(context.Background(), FFmpeg, "")

	if status.Path != fake {
		t.Errorf("path = %q, want the file that was found", status.Path)
	}
	if status.Available {
		t.Error("a file that cannot be executed is not an available dependency")
	}
	if status.Required {
		t.Error("FFmpeg is not required to start the application")
	}
	if runtime.GOOS == "windows" && status.Error == "" {
		t.Error("the failure to run should be recorded")
	}
}

func TestAria2cIsAnOptionalExtra(t *testing.T) {
	if DisplayName(Aria2c) != "aria2c" {
		t.Errorf("display name = %q, want aria2c", DisplayName(Aria2c))
	}

	// Nothing but aria2c is missing here, and the application is as ready as it
	// is with it: aria2c only makes some downloads faster.
	set := Set{
		YtDlp:  DependencyStatus{Name: YtDlp, Available: true},
		FFmpeg: DependencyStatus{Name: FFmpeg, Available: true},
	}
	if !set.Ready() || !set.CanMerge() {
		t.Error("a missing aria2c must not hold anything back")
	}
}

func TestDetectLooksForAria2c(t *testing.T) {
	managed := t.TempDir()
	detector := NewDetector(t.TempDir(), managed)

	missing := detector.Detect(context.Background(), Paths{
		YtDlp:   filepath.Join(t.TempDir(), "nothing-here.exe"),
		FFmpeg:  filepath.Join(t.TempDir(), "nothing-here.exe"),
		FFprobe: filepath.Join(t.TempDir(), "nothing-here.exe"),
		Aria2c:  filepath.Join(t.TempDir(), "nothing-here.exe"),
	})
	if missing.Aria2c.Name != Aria2c {
		t.Fatalf("aria2c status = %+v, want it probed under its own name", missing.Aria2c)
	}
	if missing.Aria2c.Available || missing.Aria2c.Source != SourceNotFound {
		t.Errorf("aria2c status = %+v, want it reported as not found", missing.Aria2c)
	}
	if missing.Aria2c.Required {
		t.Error("aria2c is optional")
	}

	found := false
	for _, status := range missing.All() {
		if status.Name == Aria2c {
			found = true
		}
	}
	if !found {
		t.Error("aria2c is missing from the list of statuses")
	}

	// A copy in the folder GrabOne manages is the one that is found.
	copy := filepath.Join(managed, executableName(Aria2c))
	if err := os.WriteFile(copy, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	path, source, err := detector.resolve(Aria2c, "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if path != copy || source != SourceManaged {
		t.Errorf("resolved %q via %q, want the managed copy", path, source)
	}
}

func TestParseAria2cVersion(t *testing.T) {
	// The first lines of the real "aria2c --version" output.
	output := "aria2 version 1.37.0\nCopyright (C) 2006, 2019 Tatsuhiro Tsujikawa\n"

	if got := parseAria2cVersion(output); got != "1.37.0" {
		t.Errorf("version = %q, want 1.37.0", got)
	}
	if got := parseAria2cVersion("something else entirely\n"); got != "something else entirely" {
		t.Errorf("version = %q, want unexpected output kept as it is", got)
	}
}

func TestAria2cGuidanceSaysItIsOptional(t *testing.T) {
	guidance := GuidanceFor(Aria2c, `C:\Apps\GrabOne`)

	if guidance.Name != Aria2c || guidance.Summary == "" || len(guidance.Steps) == 0 {
		t.Fatalf("guidance = %+v, want a summary and at least one way to install it", guidance)
	}
	for _, step := range guidance.Steps {
		if step.URL != "" && !IsKnownHelpURL(step.URL) {
			t.Errorf("step %q links to %s, which the application would refuse to open", step.Title, step.URL)
		}
	}

	listed := false
	for _, entry := range AllGuidance("") {
		if entry.Name == Aria2c {
			listed = true
		}
	}
	if !listed {
		t.Error("aria2c is missing from the install guidance")
	}
}
