package dependencies

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"grabone/internal/ffmpeg"
	"grabone/internal/procutil"
	"grabone/internal/ytdlp"
)

// probeTimeout bounds a version check so a hung executable cannot delay
// startup.
const probeTimeout = 15 * time.Second

// Paths holds the executable locations configured by the user. An empty value
// means "detect automatically".
type Paths struct {
	YtDlp   string
	FFmpeg  string
	FFprobe string
}

// Detector probes the external executables.
type Detector struct {
	// ApplicationDir is searched after the configured path, so users can drop
	// the executables next to the application.
	ApplicationDir string
	// ManagedDir holds the tools GrabOne downloaded itself, which it can also
	// update.
	ManagedDir string
}

// NewDetector builds a detector that searches the application folder and the
// folder of tools the application manages.
func NewDetector(applicationDir, managedDir string) *Detector {
	return &Detector{ApplicationDir: applicationDir, ManagedDir: managedDir}
}

// Detect probes all three executables.
func (d *Detector) Detect(ctx context.Context, paths Paths) Set {
	return Set{
		YtDlp:   d.DetectOne(ctx, YtDlp, paths.YtDlp),
		FFmpeg:  d.DetectOne(ctx, FFmpeg, paths.FFmpeg),
		FFprobe: d.DetectOne(ctx, FFprobe, paths.FFprobe),
	}
}

// DetectOne resolves and probes a single executable. The search order is the
// configured path, then the application directory, then PATH.
func (d *Detector) DetectOne(ctx context.Context, name, configuredPath string) DependencyStatus {
	status := DependencyStatus{
		Name:        name,
		DisplayName: DisplayName(name),
		Required:    name == YtDlp,
	}

	path, source, err := d.resolve(name, configuredPath)
	if err != nil {
		status.Source = SourceNotFound
		status.Error = err.Error()
		return status
	}
	status.Path = path
	status.Source = source
	status.Managed = source == SourceManaged

	version, err := probeVersion(ctx, name, path)
	if err != nil {
		status.Error = err.Error()
		return status
	}

	status.Available = true
	status.Version = version
	return status
}

// resolve finds the executable without running it.
func (d *Detector) resolve(name, configuredPath string) (string, string, error) {
	if trimmed := strings.TrimSpace(configuredPath); trimmed != "" {
		absolute, err := filepath.Abs(trimmed)
		if err != nil {
			absolute = trimmed
		}
		if info, err := os.Stat(absolute); err != nil || info.IsDir() {
			return "", "", &NotFoundError{Name: name, ConfiguredPath: absolute}
		}
		return absolute, sourceConfiguredLabel, nil
	}

	if d.ApplicationDir != "" {
		candidate := filepath.Join(d.ApplicationDir, executableName(name))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, SourceApplicationDir, nil
		}
	}

	if d.ManagedDir != "" {
		candidate := filepath.Join(d.ManagedDir, executableName(name))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, SourceManaged, nil
		}
	}

	// PATH is what makes a tool reachable from a terminal, so anything the user
	// can run there is found here.
	if found, err := exec.LookPath(executableName(name)); err == nil {
		if absolute, err := filepath.Abs(found); err == nil {
			found = absolute
		}
		return found, SourcePath, nil
	}

	// Otherwise look where the common package managers install these tools.
	for _, candidate := range commonLocations(name) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, SourceCommonLocation, nil
		}
	}

	return "", "", &NotFoundError{Name: name}
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// probeVersion runs the executable and parses its reported version. Running it
// is the only way to know the file is usable.
func probeVersion(ctx context.Context, name, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	var args []string
	switch name {
	case YtDlp:
		args = []string{"--version"}
	default:
		args = []string{"-version"}
	}

	cmd := exec.CommandContext(ctx, path, args...)
	procutil.Configure(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", &ProbeError{Name: name, Path: path, Output: strings.TrimSpace(string(output)), Err: err}
	}

	text := string(output)
	switch name {
	case YtDlp:
		return ytdlp.ParseVersion(text), nil
	default:
		return ffmpeg.ParseVersion(text), nil
	}
}
