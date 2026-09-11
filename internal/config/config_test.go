package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAreUsable(t *testing.T) {
	cfg := Default()

	if cfg.Theme != ThemeDark {
		t.Errorf("theme = %q, want dark", cfg.Theme)
	}
	if cfg.FilenameTemplate != DefaultFilenameTemplate {
		t.Errorf("template = %q, want the yt-dlp default", cfg.FilenameTemplate)
	}
	if cfg.OutputDirectory == "" {
		t.Error("a fresh installation needs somewhere to save downloads")
	}
	if cfg.MaxConcurrentDownloads != 1 {
		t.Errorf("concurrency = %d, want 1", cfg.MaxConcurrentDownloads)
	}
	if cfg.CookieSource != CookieSourceNone {
		t.Errorf("cookie source = %q, want none", cfg.CookieSource)
	}
	// Empty executable paths mean automatic detection.
	if cfg.YtDlpPath != "" || cfg.FFmpegPath != "" || cfg.FFprobePath != "" {
		t.Error("executable paths should start empty so detection runs")
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	original := Default()
	original.OutputDirectory = `C:\Users\example\Videos\GrabOne`
	original.YtDlpPath = `C:\tools\yt-dlp.exe`
	original.Theme = ThemeLight
	original.MaxConcurrentDownloads = 3
	original.CookieSource = CookieSourceBrowser
	original.CookieBrowser = "firefox"
	original.Preferences.SubtitleLanguages = []string{"en", "el"}
	original.Preferences.DownloadSubtitles = true

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var restored Config
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if restored.OutputDirectory != original.OutputDirectory ||
		restored.YtDlpPath != original.YtDlpPath ||
		restored.Theme != original.Theme ||
		restored.MaxConcurrentDownloads != original.MaxConcurrentDownloads ||
		restored.CookieBrowser != original.CookieBrowser {
		t.Errorf("round trip changed the settings:\n got %+v\nwant %+v", restored, original)
	}
	if len(restored.Preferences.SubtitleLanguages) != 2 {
		t.Errorf("subtitle languages = %v, want both to survive", restored.Preferences.SubtitleLanguages)
	}

	// The JSON keys are part of the contract with the frontend.
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	for _, key := range []string{"ytDlpPath", "ffmpegPath", "ffprobePath", "outputDirectory", "theme", "preferences"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("key %q is missing from the encoded settings", key)
		}
	}
}

func TestNormalizeRepairsInvalidValues(t *testing.T) {
	cfg := Config{
		Theme:                  "solarized",
		MaxConcurrentDownloads: 99,
		CookieSource:           "telepathy",
	}
	cfg.Normalize()

	if cfg.Theme != ThemeDark {
		t.Errorf("theme = %q, want it repaired to the default", cfg.Theme)
	}
	if cfg.MaxConcurrentDownloads != MaxConcurrentDownloadsLimit {
		t.Errorf("concurrency = %d, want it clamped to %d", cfg.MaxConcurrentDownloads, MaxConcurrentDownloadsLimit)
	}
	if cfg.CookieSource != CookieSourceNone {
		t.Errorf("cookie source = %q, want none", cfg.CookieSource)
	}
	if cfg.OutputDirectory == "" || cfg.FilenameTemplate == "" {
		t.Error("missing values should fall back to the defaults")
	}

	// A source chosen before its browser is a half-made choice, and must be kept
	// so the user can finish making it.
	incomplete := Config{CookieSource: CookieSourceBrowser}
	incomplete.Normalize()
	if incomplete.CookieSource != CookieSourceBrowser {
		t.Error("choosing a cookie source before its browser should not erase the choice")
	}

	zero := Config{MaxConcurrentDownloads: 0}
	zero.Normalize()
	if zero.MaxConcurrentDownloads != 1 {
		t.Errorf("concurrency = %d, want at least 1", zero.MaxConcurrentDownloads)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("a missing settings file is not an error: %v", err)
	}
	if cfg.Theme != ThemeDark {
		t.Errorf("theme = %q, want the default", cfg.Theme)
	}
}

func TestLoadIgnoresUnknownFieldsAndKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// An older or newer settings file: one known key, one unknown key, and
	// several keys missing entirely.
	contents := `{"outputDirectory": "D:\\Media", "somethingFromTheFuture": {"a": 1}}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.OutputDirectory != `D:\Media` {
		t.Errorf("output = %q, want the stored value", cfg.OutputDirectory)
	}
	if cfg.FilenameTemplate != DefaultFilenameTemplate {
		t.Errorf("template = %q, want the default for a missing key", cfg.FilenameTemplate)
	}
	if cfg.MaxConcurrentDownloads < 1 {
		t.Error("a missing concurrency value should fall back to a usable one")
	}
}

func TestLoadBrokenFileReturnsDefaultsAndAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err == nil {
		t.Error("a broken settings file should be reported")
	}
	if cfg.Theme != ThemeDark {
		t.Error("the application must still start with usable settings")
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")

	cfg := Default()
	cfg.OutputDirectory = `E:\Downloads`
	cfg.Preferences.EmbedThumbnail = true

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if reloaded.OutputDirectory != `E:\Downloads` || !reloaded.Preferences.EmbedThumbnail {
		t.Errorf("reloaded = %+v, want the saved values", reloaded)
	}

	// No temporary file should be left behind by the atomic write.
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("the temporary file from the atomic write was not removed")
	}
}

func TestStoreUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	updated, err := store.Update(func(cfg *Config) {
		cfg.OutputDirectory = `F:\Clips`
		cfg.Preferences.SubtitleLanguages = []string{"de"}
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.OutputDirectory != `F:\Clips` {
		t.Errorf("output = %q, want the updated value", updated.OutputDirectory)
	}

	// The change must have reached disk, not only memory.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.OutputDirectory != `F:\Clips` {
		t.Errorf("stored output = %q, want the updated value", reloaded.OutputDirectory)
	}

	// Get returns a copy: mutating it must not affect the store.
	snapshot := store.Get()
	snapshot.Preferences.SubtitleLanguages[0] = "fr"
	if store.Get().Preferences.SubtitleLanguages[0] != "de" {
		t.Error("Get returned a view into the store's own state")
	}
}
