// Package partial keeps a download's in-progress files out of the download
// folder.
//
// While it works, yt-dlp writes partial files, the separate streams it is about
// to merge, and the thumbnail and subtitles it is about to embed. Written to the
// download folder they appear and vanish for the whole download and keep
// reordering it. Each download is given its own folder for them instead, inside
// one hidden folder in the download folder: on the same drive, moving the
// finished file into place is a rename however large it is.
package partial

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// DirName is the hidden folder inside the download folder that holds the
// folders of the downloads in progress.
const DirName = ".grabone-partial"

// StaleAfter is how long a download's folder can go untouched before it is
// taken to be left over from a download that never finished, such as when
// GrabOne was closed by force.
const StaleAfter = 24 * time.Hour

// Prepare creates the folder for one download's in-progress files and returns
// its path. The download folder must exist already.
func Prepare(outputDirectory, id string) (string, error) {
	if outputDirectory == "" || id == "" || filepath.Base(id) != id {
		return "", errors.New("prepare partial folder: a download folder and a plain identifier are needed")
	}

	parent := filepath.Join(outputDirectory, DirName)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	// Hiding is a courtesy: a folder that cannot be hidden still does its job.
	_ = hide(parent)

	directory := filepath.Join(parent, id)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	return directory, nil
}

// Release removes a download's folder with whatever is left in it, and the
// hidden folder too once no other download is using it.
//
// A stopped process can hold its files open for a moment after it ends, which
// Windows does not allow to be deleted, so a removal that fails is tried again
// briefly.
func Release(directory string) error {
	if directory == "" {
		return nil
	}

	var err error
	for attempt := 0; attempt < 10; attempt++ {
		if err = os.RemoveAll(directory); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Removing a folder that is not empty fails, which is exactly what keeps the
	// folder of another download in progress.
	_ = os.Remove(filepath.Dir(directory))
	return err
}

// CleanStale removes the folders of downloads that were left behind: those in
// which nothing has changed for StaleAfter. A folder still being written to is
// recent, so a download running in another GrabOne window is left alone.
func CleanStale(outputDirectory string, now time.Time) []string {
	if outputDirectory == "" {
		return nil
	}
	parent := filepath.Join(outputDirectory, DirName)

	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}

	var removed []string
	for _, entry := range entries {
		directory := filepath.Join(parent, entry.Name())
		if now.Sub(lastChange(directory)) < StaleAfter {
			continue
		}
		if err := os.RemoveAll(directory); err == nil {
			removed = append(removed, directory)
		}
	}

	_ = os.Remove(parent)
	return removed
}

// lastChange returns when anything at or under path last changed. A folder's
// own time does not move while a file inside it grows, so the files are looked
// at as well.
func lastChange(path string) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := entry.Info(); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}
