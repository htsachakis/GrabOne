package partial

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrepareGivesEachDownloadItsOwnFolderInsideTheHiddenOne(t *testing.T) {
	output := t.TempDir()

	directory, err := Prepare(output, "job-1")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if want := filepath.Join(output, DirName, "job-1"); directory != want {
		t.Errorf("folder = %q, want %q", directory, want)
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		t.Errorf("the folder was not created: %v", err)
	}
}

func TestPrepareRefusesAnIdentifierThatIsAPath(t *testing.T) {
	if _, err := Prepare(t.TempDir(), "../elsewhere"); err == nil {
		t.Error("an identifier that leads out of the hidden folder was accepted")
	}
}

func TestReleaseRemovesTheDownloadsFolderAndTheEmptyHiddenOne(t *testing.T) {
	output := t.TempDir()
	directory, err := Prepare(output, "job-1")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	writeFile(t, filepath.Join(directory, "clip.f137.mp4.part"))

	if err := Release(directory); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, DirName)); !os.IsNotExist(err) {
		t.Errorf("the hidden folder is still there once no download uses it: %v", err)
	}
}

func TestReleaseLeavesAnotherDownloadInProgressAlone(t *testing.T) {
	output := t.TempDir()
	first, _ := Prepare(output, "job-1")
	second, _ := Prepare(output, "job-2")
	writeFile(t, filepath.Join(second, "other.part"))

	if err := Release(first); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(filepath.Join(second, "other.part")); err != nil {
		t.Errorf("the other download's files were touched: %v", err)
	}
}

func TestCleanStaleRemovesOnlyFoldersLeftUntouched(t *testing.T) {
	output := t.TempDir()
	now := time.Now()

	old, _ := Prepare(output, "left-behind")
	oldFile := filepath.Join(old, "clip.part")
	writeFile(t, oldFile)
	setTime(t, old, now.Add(-48*time.Hour))
	setTime(t, oldFile, now.Add(-48*time.Hour))

	// The folder itself is old, but a file in it is still being written to:
	// this is a download running in another window.
	running, _ := Prepare(output, "running")
	runningFile := filepath.Join(running, "clip.part")
	writeFile(t, runningFile)
	setTime(t, running, now.Add(-48*time.Hour))

	removed := CleanStale(output, now)

	if len(removed) != 1 || removed[0] != old {
		t.Errorf("removed %v, want only %s", removed, old)
	}
	if _, err := os.Stat(runningFile); err != nil {
		t.Errorf("a download still being written to was removed: %v", err)
	}
}

func TestCleanStaleRemovesTheHiddenFolderOnceEmpty(t *testing.T) {
	output := t.TempDir()
	old, _ := Prepare(output, "left-behind")
	setTime(t, old, time.Now().Add(-48*time.Hour))

	CleanStale(output, time.Now())

	if _, err := os.Stat(filepath.Join(output, DirName)); !os.IsNotExist(err) {
		t.Errorf("the empty hidden folder was kept: %v", err)
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setTime(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}
