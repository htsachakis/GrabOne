package tooling

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// maxExtractedSize bounds a single extracted file, so a malformed or hostile
// archive cannot fill the disk.
const maxExtractedSize = 512 << 20

// ffmpegExecutables are the files taken out of an FFmpeg build. The archive
// also holds ffplay and documentation, which the application has no use for.
var ffmpegExecutables = []string{"ffmpeg.exe", "ffprobe.exe"}

// extractFFmpeg pulls the executables out of a downloaded FFmpeg archive and
// removes the archive afterwards.
func extractFFmpeg(downloaded, directory string) ([]string, error) {
	extracted, err := ExtractFromZip(downloaded, directory, ffmpegExecutables)
	if err != nil {
		return nil, err
	}
	if len(extracted) == 0 {
		return nil, fmt.Errorf("the downloaded archive did not contain ffmpeg.exe")
	}

	// The archive is large and serves no further purpose.
	_ = os.Remove(downloaded)
	return extracted, nil
}

// ExtractFromZip copies the named files out of an archive into directory,
// ignoring the folder structure inside it.
//
// Only the wanted names are taken, and each one is written to a path built from
// its base name, so an entry such as "../../evil.exe" cannot write outside the
// destination.
func ExtractFromZip(archivePath, directory string, wanted []string) ([]string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open the downloaded archive: %w", err)
	}
	defer reader.Close()

	wantedSet := map[string]bool{}
	for _, name := range wanted {
		wantedSet[strings.ToLower(name)] = true
	}

	extracted := make([]string, 0, len(wanted))
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}

		// Archive entries always use forward slashes, whatever built them.
		base := path.Base(strings.ReplaceAll(entry.Name, "\\", "/"))
		if !wantedSet[strings.ToLower(base)] {
			continue
		}

		target := filepath.Join(directory, base)
		if err := writeEntry(entry, target); err != nil {
			return nil, err
		}
		extracted = append(extracted, target)
	}
	return extracted, nil
}

func writeEntry(entry *zip.File, target string) error {
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("read %s from the archive: %w", entry.Name, err)
	}
	defer source.Close()

	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(target), err)
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(source, maxExtractedSize+1))
	if err != nil {
		return fmt.Errorf("unpack %s: %w", filepath.Base(target), err)
	}
	if written > maxExtractedSize {
		_ = os.Remove(target)
		return fmt.Errorf("%s is larger than expected and was not unpacked", filepath.Base(target))
	}
	return nil
}
