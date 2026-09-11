// Package config stores user settings on disk and provides safe defaults.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Theme values supported by the frontend.
const (
	ThemeDark  = "dark"
	ThemeLight = "light"
)

// Cookie sources for authenticated extraction. Cookie contents are never read
// or logged by the application: the values here are only passed to yt-dlp.
const (
	CookieSourceNone    = "none"
	CookieSourceBrowser = "browser"
	CookieSourceFile    = "file"
)

// DefaultFilenameTemplate is handed to yt-dlp as-is; yt-dlp owns filename
// sanitisation.
const DefaultFilenameTemplate = "%(title)s.%(ext)s"

// MaxConcurrentDownloadsLimit caps how many downloads may run at once.
const MaxConcurrentDownloadsLimit = 5

// Preferences remembers the last used download options so the user does not
// have to re-tick the same boxes on every download.
type Preferences struct {
	DownloadType          string   `json:"downloadType"`
	Container             string   `json:"container"`
	EmbedMetadata         bool     `json:"embedMetadata"`
	EmbedChapters         bool     `json:"embedChapters"`
	EmbedThumbnail        bool     `json:"embedThumbnail"`
	SaveThumbnail         bool     `json:"saveThumbnail"`
	SaveDescription       bool     `json:"saveDescription"`
	SaveJSON              bool     `json:"saveJson"`
	DownloadSubtitles     bool     `json:"downloadSubtitles"`
	EmbedSubtitles        bool     `json:"embedSubtitles"`
	KeepSubtitles         bool     `json:"keepSubtitles"`
	SubtitleFormat        string   `json:"subtitleFormat"`
	SubtitleLanguages     []string `json:"subtitleLanguages"`
	AudioConversionFormat string   `json:"audioConversionFormat"`
}

// Config is the persisted application configuration. Empty executable paths
// mean "detect automatically".
type Config struct {
	YtDlpPath   string `json:"ytDlpPath"`
	FFmpegPath  string `json:"ffmpegPath"`
	FFprobePath string `json:"ffprobePath"`

	OutputDirectory  string `json:"outputDirectory"`
	FilenameTemplate string `json:"filenameTemplate"`

	Theme string `json:"theme"`

	MaxConcurrentDownloads int `json:"maxConcurrentDownloads"`

	// AutoCheckUpdates asks GitHub for a newer release shortly after startup.
	AutoCheckUpdates bool `json:"autoCheckUpdates"`
	// SkippedUpdateVersion is a version the user chose not to be reminded about.
	SkippedUpdateVersion string `json:"skippedUpdateVersion"`

	CookieSource  string `json:"cookieSource"`
	CookieBrowser string `json:"cookieBrowser"`
	CookieFile    string `json:"cookieFile"`

	Preferences Preferences `json:"preferences"`
}

// Default returns the configuration used on a fresh installation.
func Default() Config {
	return Config{
		OutputDirectory:        DefaultOutputDir(),
		FilenameTemplate:       DefaultFilenameTemplate,
		Theme:                  ThemeDark,
		MaxConcurrentDownloads: 1,
		AutoCheckUpdates:       true,
		CookieSource:           CookieSourceNone,
		Preferences: Preferences{
			DownloadType:          "video+audio",
			Container:             "auto",
			EmbedMetadata:         true,
			EmbedChapters:         true,
			EmbedSubtitles:        true,
			SubtitleFormat:        "srt",
			AudioConversionFormat: "original",
			SubtitleLanguages:     []string{},
		},
	}
}

// Normalize repairs missing or invalid values in place, so a hand-edited or
// outdated settings file can never leave the application in a broken state.
func (c *Config) Normalize() {
	defaults := Default()

	if c.OutputDirectory == "" {
		c.OutputDirectory = defaults.OutputDirectory
	}
	if c.FilenameTemplate == "" {
		c.FilenameTemplate = defaults.FilenameTemplate
	}
	if c.Theme != ThemeDark && c.Theme != ThemeLight {
		c.Theme = defaults.Theme
	}
	if c.MaxConcurrentDownloads < 1 {
		c.MaxConcurrentDownloads = 1
	}
	if c.MaxConcurrentDownloads > MaxConcurrentDownloadsLimit {
		c.MaxConcurrentDownloads = MaxConcurrentDownloadsLimit
	}
	switch c.CookieSource {
	case CookieSourceNone, CookieSourceBrowser, CookieSourceFile:
	default:
		c.CookieSource = CookieSourceNone
	}
	// A source chosen without a browser or file yet is kept as the user's
	// intent: they picked the method before picking the value, and erasing it
	// here would make the choice impossible to make. Whether it is usable is
	// decided by CookieOptions.Complete when a download is built.
	if c.Preferences.DownloadType == "" {
		c.Preferences.DownloadType = defaults.Preferences.DownloadType
	}
	if c.Preferences.Container == "" {
		c.Preferences.Container = defaults.Preferences.Container
	}
	if c.Preferences.SubtitleFormat == "" {
		c.Preferences.SubtitleFormat = defaults.Preferences.SubtitleFormat
	}
	if c.Preferences.AudioConversionFormat == "" {
		c.Preferences.AudioConversionFormat = defaults.Preferences.AudioConversionFormat
	}
	if c.Preferences.SubtitleLanguages == nil {
		c.Preferences.SubtitleLanguages = []string{}
	}
}

// Load reads the configuration from path. A missing file is not an error: the
// defaults are returned instead.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read settings %s: %w", path, err)
	}

	// Decode on top of the defaults so fields absent from an older settings
	// file keep their default value, and unknown fields are ignored.
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse settings %s: %w", path, err)
	}
	cfg.Normalize()
	return cfg, nil
}

// Save writes the configuration to path atomically.
func Save(path string, cfg Config) error {
	cfg.Normalize()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')

	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}

// Store keeps the active configuration in memory and persists changes. It is
// safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// NewStore loads the configuration from path, returning a usable store even
// when loading failed, in which case the error is also returned.
func NewStore(path string) (*Store, error) {
	cfg, err := Load(path)
	return &Store{path: path, cfg: cfg}, err
}

// Get returns a copy of the current configuration.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.cfg
	cfg.Preferences.SubtitleLanguages = append([]string(nil), s.cfg.Preferences.SubtitleLanguages...)
	return cfg
}

// Set replaces and persists the configuration.
func (s *Store) Set(cfg Config) error {
	cfg.Normalize()

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	return Save(s.path, cfg)
}

// Update applies mutate to a copy of the configuration and persists the result.
func (s *Store) Update(mutate func(*Config)) (Config, error) {
	s.mu.Lock()
	cfg := s.cfg
	mutate(&cfg)
	cfg.Normalize()
	s.cfg = cfg
	s.mu.Unlock()

	return cfg, Save(s.path, cfg)
}

// Path returns the settings file location.
func (s *Store) Path() string { return s.path }
