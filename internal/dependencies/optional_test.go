package dependencies

import (
	"context"
	"path/filepath"
	"testing"
)

// isolateSearch points every place the detector looks at empty folders, so a
// tool installed on the machine running the tests is not found.
func isolateSearch(t *testing.T) {
	t.Helper()

	for _, variable := range []string{
		"PATH", "LOCALAPPDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)", "USERPROFILE", "HOME",
	} {
		t.Setenv(variable, t.TempDir())
	}
}

func TestAbsentOptionalExtraIsNotAnError(t *testing.T) {
	isolateSearch(t)
	detector := NewDetector(t.TempDir(), t.TempDir())

	// aria2c that was never installed is a fact about the machine, not a fault
	// to report: nothing is lost without it.
	absent := detector.DetectOne(context.Background(), Aria2c, "")
	if absent.Available || absent.Source != SourceNotFound {
		t.Fatalf("status = %+v, want aria2c reported as not found", absent)
	}
	if absent.Error != "" {
		t.Errorf("error = %q, want none for an optional extra that is simply not installed", absent.Error)
	}

	// A location the user set and that holds nothing is their mistake to hear
	// about, optional or not.
	misplaced := detector.DetectOne(context.Background(), Aria2c, filepath.Join(t.TempDir(), "aria2c.exe"))
	if misplaced.Error == "" {
		t.Error("a configured location with no aria2c in it should be reported")
	}

	// yt-dlp and FFmpeg are missed when absent, so they keep their message.
	for _, name := range []string{YtDlp, FFmpeg} {
		if status := detector.DetectOne(context.Background(), name, ""); status.Error == "" {
			t.Errorf("%s is missing and says nothing about it", name)
		}
	}
}
