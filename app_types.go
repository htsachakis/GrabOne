package main

import (
	"grabone/internal/config"
	"grabone/internal/dependencies"
	"grabone/internal/downloads"
	"grabone/internal/media"
	"grabone/internal/ytdlp"
)

// AppInfo describes the application to the interface, so the name and version
// are never duplicated in the frontend.
type AppInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	VersionLabel string `json:"versionLabel"`
	Description  string `json:"description"`
	LegalNotice  string `json:"legalNotice"`
	// ApplicationDir is where GrabOne itself lives, which is one of the places
	// it looks for the external tools.
	ApplicationDir string `json:"applicationDir"`
	SettingsPath   string `json:"settingsPath"`
	LogDirectory   string `json:"logDirectory"`
}

// AnalyzeRequest is what the interface sends when the user analyzes a link.
type AnalyzeRequest struct {
	URL string `json:"url"`
	// CollectionMode decides whether a link belonging to a playlist is analyzed
	// as one item or as the whole collection.
	CollectionMode string `json:"collectionMode"`
}

// SelectionDefaults is the selection the interface starts from. It is computed
// from the analysis, so the sensible choice is already made for the common case.
type SelectionDefaults struct {
	DownloadType     string `json:"downloadType"`
	CombinedFormatID string `json:"combinedFormatId"`
	VideoFormatID    string `json:"videoFormatId"`
	AudioFormatID    string `json:"audioFormatId"`
	Container        string `json:"container"`
	// ResolvedContainer is the container the automatic choice results in, shown
	// next to the "Automatic" option.
	ResolvedContainer string   `json:"resolvedContainer"`
	SubtitleLanguages []string `json:"subtitleLanguages"`
}

// AnalyzeResponse carries either the analyzed media or a classified error.
// Errors are returned in the payload rather than as a rejected call so the
// interface can show a friendly message, a hint and the raw details together.
type AnalyzeResponse struct {
	Success bool             `json:"success"`
	Media   *media.MediaInfo `json:"media,omitempty"`
	Error   *ytdlp.Error     `json:"error,omitempty"`

	ResolutionPresets []media.ResolutionPreset `json:"resolutionPresets"`
	Defaults          SelectionDefaults        `json:"defaults"`

	// CanMerge and CanConvertAudio reflect FFmpeg availability, so the
	// interface can explain why an option is unavailable.
	CanMerge        bool `json:"canMerge"`
	CanConvertAudio bool `json:"canConvertAudio"`
}

// DownloadRequest is the selection the user made. Values are validated against
// known options before any argument is built.
type DownloadRequest struct {
	URL   string `json:"url"`
	Title string `json:"title"`

	DownloadType string `json:"downloadType"`

	VideoFormatID    string `json:"videoFormatId"`
	AudioFormatID    string `json:"audioFormatId"`
	CombinedFormatID string `json:"combinedFormatId"`
	MaxHeight        int    `json:"maxHeight"`

	Container string `json:"container"`

	EmbedMetadata  bool `json:"embedMetadata"`
	EmbedChapters  bool `json:"embedChapters"`
	EmbedThumbnail bool `json:"embedThumbnail"`

	SaveThumbnail   bool `json:"saveThumbnail"`
	SaveDescription bool `json:"saveDescription"`
	SaveJSON        bool `json:"saveJson"`

	DownloadSubtitles bool     `json:"downloadSubtitles"`
	EmbedSubtitles    bool     `json:"embedSubtitles"`
	KeepSubtitles     bool     `json:"keepSubtitles"`
	SubtitleLanguages []string `json:"subtitleLanguages"`
	SubtitleFormat    string   `json:"subtitleFormat"`

	AudioConversionFormat string `json:"audioConversionFormat"`

	OutputDirectory string `json:"outputDirectory"`

	CollectionMode string `json:"collectionMode"`
	PlaylistItems  string `json:"playlistItems"`
}

// PreviewResponse is the effective command, shown in the advanced section.
type PreviewResponse struct {
	Success bool         `json:"success"`
	Command string       `json:"command"`
	Args    []string     `json:"args"`
	Error   *ytdlp.Error `json:"error,omitempty"`
	// ResolvedContainer is the container the options actually result in.
	ResolvedContainer string `json:"resolvedContainer"`
}

// StartResponse reports whether a download was queued.
type StartResponse struct {
	Success  bool            `json:"success"`
	Download *downloads.View `json:"download,omitempty"`
	Error    *ytdlp.Error    `json:"error,omitempty"`
}

// SettingsResponse returns the stored settings together with the dependency
// picture, which changes when the executable paths change.
type SettingsResponse struct {
	Settings     config.Config    `json:"settings"`
	Dependencies dependencies.Set `json:"dependencies"`
	Error        string           `json:"error,omitempty"`
}
