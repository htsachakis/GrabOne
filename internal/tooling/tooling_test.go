package tooling

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grabone/internal/dependencies"
	"grabone/internal/ghrelease"
)

func TestSourceForEveryDependency(t *testing.T) {
	for _, name := range []string{dependencies.YtDlp, dependencies.FFmpeg, dependencies.FFprobe} {
		source, ok := SourceFor(name)
		if !ok {
			t.Errorf("%s has no download source", name)
			continue
		}
		if source.Owner == "" || source.Repo == "" {
			t.Errorf("%s has an incomplete source: %+v", name, source)
		}
		if source.Asset == nil || source.ChecksumAsset == nil || source.Extract == nil {
			t.Errorf("%s is missing part of its download definition", name)
		}
		if source.Licence == "" || source.ProjectURL == "" {
			t.Errorf("%s should say where it comes from and under which licence", name)
		}
	}

	if _, ok := SourceFor("something-else"); ok {
		t.Error("an unknown tool must not claim to be downloadable")
	}
}

// ytDlpRelease mirrors what the yt-dlp project publishes.
func ytDlpRelease() *ghrelease.Release {
	return &ghrelease.Release{
		TagName: "2026.08.19",
		Assets: []ghrelease.Asset{
			{Name: "SHA2-256SUMS"},
			{Name: "SHA2-256SUMS.sig"},
			{Name: "yt-dlp", Size: 3_000_000},
			{Name: "yt-dlp.exe", Size: 17_800_000},
			{Name: "yt-dlp_macos"},
		},
	}
}

// ffmpegRelease mirrors what the FFmpeg build project publishes.
func ffmpegRelease() *ghrelease.Release {
	return &ghrelease.Release{
		TagName: "latest",
		Assets: []ghrelease.Asset{
			{Name: "checksums.sha256"},
			{Name: "ffmpeg-n8.1-latest-win64-gpl-8.1.zip", Size: 191_800_000},
			{Name: "ffmpeg-n8.1-latest-win64-lgpl-8.1.zip"},
			{Name: "ffmpeg-n9.0-latest-win64-gpl-9.0.zip", Size: 192_800_000},
			{Name: "ffmpeg-n9.0-latest-win64-lgpl-9.0.zip"},
			{Name: "ffmpeg-n9.0-latest-win64-gpl-shared-9.0.zip"},
			{Name: "ffmpeg-n9.0-latest-linux64-gpl-9.0.tar.xz"},
		},
	}
}

func TestYtDlpAssetSelection(t *testing.T) {
	source, _ := SourceFor(dependencies.YtDlp)
	release := ytDlpRelease()

	asset := source.Asset(release)
	if asset == nil || asset.Name != "yt-dlp.exe" {
		t.Fatalf("asset = %+v, want the Windows executable", asset)
	}

	checksums := source.ChecksumAsset(release)
	if checksums == nil || checksums.Name != "SHA2-256SUMS" {
		t.Fatalf("checksums = %+v, want SHA2-256SUMS", checksums)
	}
}

func TestFFmpegAssetSelection(t *testing.T) {
	source, _ := SourceFor(dependencies.FFmpeg)
	release := ffmpegRelease()

	asset := source.Asset(release)
	if asset == nil {
		t.Fatal("no FFmpeg build was chosen")
	}

	name := strings.ToLower(asset.Name)
	if !strings.Contains(name, "win64") {
		t.Errorf("asset = %q, want a Windows build", asset.Name)
	}
	// A shared build needs its DLLs alongside, which the application does not
	// unpack, so a static build is the only usable one.
	if strings.Contains(name, "shared") {
		t.Errorf("asset = %q, want a static build", asset.Name)
	}
	// The GPL build carries the encoders the audio conversion offers.
	if strings.Contains(name, "lgpl") {
		t.Errorf("asset = %q, want the GPL build", asset.Name)
	}
	// The newest of the offered versions.
	if !strings.Contains(name, "n9.0") {
		t.Errorf("asset = %q, want the newest version offered", asset.Name)
	}

	if checksums := source.ChecksumAsset(release); checksums == nil {
		t.Error("the checksum file was not recognised")
	}
}

func TestAssetSelectionOnAnEmptyRelease(t *testing.T) {
	empty := &ghrelease.Release{TagName: "none"}

	for _, name := range []string{dependencies.YtDlp, dependencies.FFmpeg} {
		source, _ := SourceFor(name)
		if asset := source.Asset(empty); asset != nil {
			t.Errorf("%s: asset = %+v, want nothing from a release with no files", name, asset)
		}
		if checksums := source.ChecksumAsset(empty); checksums != nil {
			t.Errorf("%s: checksums = %+v, want nothing", name, checksums)
		}
	}
}

// buildZip writes an archive with the given entries.
func buildZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create the archive: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, contents := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close the archive: %v", err)
	}
}

func TestExtractFFmpegTakesOnlyTheExecutables(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "ffmpeg.zip")

	buildZip(t, archive, map[string]string{
		"ffmpeg-n9.0-win64-gpl/bin/ffmpeg.exe":  "ffmpeg binary",
		"ffmpeg-n9.0-win64-gpl/bin/ffprobe.exe": "ffprobe binary",
		"ffmpeg-n9.0-win64-gpl/bin/ffplay.exe":  "ffplay binary",
		"ffmpeg-n9.0-win64-gpl/LICENSE.txt":     "licence text",
		"ffmpeg-n9.0-win64-gpl/doc/faq.html":    "docs",
	})

	extracted, err := extractFFmpeg(archive, directory)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(extracted) != 2 {
		t.Fatalf("extracted %v, want ffmpeg.exe and ffprobe.exe", extracted)
	}
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
		path := filepath.Join(directory, name)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s was not extracted: %v", name, err)
			continue
		}
		if !strings.Contains(string(contents), "binary") {
			t.Errorf("%s holds %q, want the archived contents", name, contents)
		}
	}

	// Files the application has no use for stay in the archive.
	for _, name := range []string{"ffplay.exe", "LICENSE.txt", "faq.html"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err == nil {
			t.Errorf("%s should not have been unpacked", name)
		}
	}

	// The archive itself is large and is removed once unpacked.
	if _, err := os.Stat(archive); err == nil {
		t.Error("the archive should be deleted after extraction")
	}
}

func TestExtractIgnoresPathsThatEscapeTheFolder(t *testing.T) {
	directory := t.TempDir()
	inner := filepath.Join(directory, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	archive := filepath.Join(directory, "hostile.zip")
	buildZip(t, archive, map[string]string{
		"../../ffmpeg.exe":            "escaped",
		"good/bin/ffprobe.exe":        "fine",
		"..\\..\\windows\\ffmpeg.exe": "escaped too",
	})

	extracted, err := ExtractFromZip(archive, inner, ffmpegExecutables)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// Everything lands inside the destination, whatever the entry claimed.
	for _, path := range extracted {
		if !strings.HasPrefix(path, inner) {
			t.Errorf("extracted %q, which is outside %q", path, inner)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "ffmpeg.exe")); err == nil {
		t.Error("an entry escaped the destination folder")
	}
}

func TestExtractReportsAnArchiveWithoutTheExecutables(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "empty.zip")
	buildZip(t, archive, map[string]string{"readme.txt": "nothing useful here"})

	if _, err := extractFFmpeg(archive, directory); err == nil {
		t.Error("an archive without ffmpeg.exe should be reported")
	}
}

func TestInstalledListsWhatIsPresent(t *testing.T) {
	directory := t.TempDir()
	installer := NewInstaller(directory, "test")

	if len(installer.Installed()) != 0 {
		t.Error("an empty folder holds no tools")
	}

	path := filepath.Join(directory, executableName(dependencies.YtDlp))
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	installed := installer.Installed()
	if installed[dependencies.YtDlp] != path {
		t.Errorf("installed = %v, want yt-dlp at %q", installed, path)
	}
	if _, ok := installed[dependencies.FFmpeg]; ok {
		t.Error("FFmpeg is not present and must not be reported as installed")
	}
}

func TestInstallRefusesAnUnknownTool(t *testing.T) {
	installer := NewInstaller(t.TempDir(), "test")

	if _, err := installer.Install(t.Context(), "notepad", nil); err == nil {
		t.Error("only the known tools may be downloaded")
	}
}
