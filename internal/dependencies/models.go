// Package dependencies locates and verifies the external executables the
// application drives. The application is a controller around these tools, so
// they are detected at runtime and can be updated independently.
package dependencies

// Dependency identifiers. These are stable keys used by the frontend and by
// the settings file.
const (
	YtDlp   = "yt-dlp"
	FFmpeg  = "ffmpeg"
	FFprobe = "ffprobe"
)

// Source describes where an executable was found, which is what the settings
// screen shows next to the path.
const (
	SourceConfigured      = "configured"
	SourceApplicationDir  = "application directory"
	SourceManaged         = "downloaded by GrabOne"
	SourcePath            = "PATH"
	SourceCommonLocation  = "a standard install location"
	SourceNotFound        = "not found"
	sourceConfiguredLabel = "configured path"
)

// DependencyStatus is the result of probing one executable. Availability is
// established by running the executable, not by checking that a file exists:
// a present but broken binary must not be reported as usable.
type DependencyStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Available   bool   `json:"available"`
	Path        string `json:"path"`
	Version     string `json:"version"`
	Source      string `json:"source"`
	Error       string `json:"error,omitempty"`
	// Required reports whether the application can function without it.
	Required bool `json:"required"`
	// Managed reports that GrabOne downloaded this copy and can update it.
	Managed bool `json:"managed"`
}

// Set is the full dependency picture handed to the frontend.
type Set struct {
	YtDlp   DependencyStatus `json:"ytDlp"`
	FFmpeg  DependencyStatus `json:"ffmpeg"`
	FFprobe DependencyStatus `json:"ffprobe"`
}

// All returns the statuses in display order.
func (s Set) All() []DependencyStatus {
	return []DependencyStatus{s.YtDlp, s.FFmpeg, s.FFprobe}
}

// Ready reports whether the application can analyze and download. yt-dlp is
// mandatory; FFmpeg is needed for merging, remuxing and conversion, and
// FFprobe only for inspecting finished files.
func (s Set) Ready() bool { return s.YtDlp.Available }

// CanMerge reports whether separate video and audio streams can be combined.
func (s Set) CanMerge() bool { return s.FFmpeg.Available }

// DisplayName maps a dependency key to its cased name.
func DisplayName(name string) string {
	switch name {
	case YtDlp:
		return "yt-dlp"
	case FFmpeg:
		return "FFmpeg"
	case FFprobe:
		return "FFprobe"
	default:
		return name
	}
}
