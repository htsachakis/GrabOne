package downloads

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grabone/internal/partial"
)

func TestInProgressFilesGoToTheJobsOwnHiddenFolder(t *testing.T) {
	options := testOptions()
	options.OutputDirectory = t.TempDir()

	view, _, attempts := runWithStandIn(t, rejectsSpeed, options)

	if view.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed", view.Status)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d, want 1", len(attempts))
	}
	want := "--paths temp:" + filepath.Join(options.OutputDirectory, partial.DirName, view.ID)
	if !strings.Contains(attempts[0], want) {
		t.Errorf("yt-dlp was run with\n%s\nwant it to contain %q", attempts[0], want)
	}
}

func TestPartialFilesAreRemovedWhenTheJobEnds(t *testing.T) {
	for _, behaviour := range []string{rejectsSpeed, privateMedia} {
		options := testOptions()
		options.OutputDirectory = t.TempDir()

		view, _, _ := runWithStandIn(t, behaviour, options)

		if _, err := os.Stat(filepath.Join(options.OutputDirectory, partial.DirName)); !os.IsNotExist(err) {
			t.Errorf("job that ended %s left its partial files behind: %v", view.Status, err)
		}
	}
}

func TestTheCommandShownLeavesOutThePartialFolder(t *testing.T) {
	options := testOptions()
	options.OutputDirectory = t.TempDir()

	view, _, _ := runWithStandIn(t, rejectsSpeed, options)

	if strings.Contains(view.Command, "temp:") {
		t.Errorf("the command shown includes the partial folder: %s", view.Command)
	}
}

func TestADownloadFolderThatRefusesThePartialFolderStillDownloads(t *testing.T) {
	options := testOptions()
	// A file where the download folder should be: nothing can be made in it.
	blocked := filepath.Join(t.TempDir(), "not-a-folder")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	options.OutputDirectory = blocked

	view, _, attempts := runWithStandIn(t, rejectsSpeed, options)

	if view.Status != StatusCompleted {
		t.Errorf("status = %q, want the download to go ahead without the partial folder", view.Status)
	}
	if len(attempts) == 1 && strings.Contains(attempts[0], "temp:") {
		t.Errorf("yt-dlp was given a partial folder that could not be made: %s", attempts[0])
	}
}
