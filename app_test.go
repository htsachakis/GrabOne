package main

import (
	"path/filepath"
	"testing"

	"grabone/internal/config"
	"grabone/internal/dependencies"
	"grabone/internal/logging"
	"grabone/internal/ytdlp"
)

// testApp builds the service on a settings file in a temporary folder. Nothing
// is detected, so no executable is ever run.
func testApp(t *testing.T, mutate func(*config.Config)) *App {
	t.Helper()

	store, err := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if _, err := store.Update(func(cfg *config.Config) {
		cfg.OutputDirectory = t.TempDir()
		mutate(cfg)
	}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	return NewApp(logging.Discard(), store)
}

// plainRequest is a download that needs nothing but yt-dlp: one combined
// stream and no processing.
func plainRequest() DownloadRequest {
	return DownloadRequest{
		URL:              "https://example.com/watch?v=1",
		DownloadType:     ytdlp.DownloadTypeVideoAudio,
		CombinedFormatID: "22",
	}
}

func TestDownloadIsBuiltWithTheSpeedSettings(t *testing.T) {
	app := testApp(t, func(cfg *config.Config) {
		cfg.Connections = 6
		cfg.ChunkedTransfer = true
	})

	options, failure := app.buildOptions(plainRequest())
	if failure != nil {
		t.Fatalf("build: %+v", failure)
	}

	if options.Speed.Connections != 6 {
		t.Errorf("connections = %d, want the stored 6", options.Speed.Connections)
	}
	if !options.Speed.ChunkedTransfer {
		t.Error("chunked transfer is on in the settings, want it on for the download")
	}
	if options.Restart {
		t.Error("a first attempt should not start over")
	}
}

func TestDownloadBuiltAfterTheSpeedSettingsAreSwitchedOff(t *testing.T) {
	app := testApp(t, func(cfg *config.Config) {
		cfg.Connections = 1
		cfg.ChunkedTransfer = false
	})

	options, failure := app.buildOptions(plainRequest())
	if failure != nil {
		t.Fatalf("build: %+v", failure)
	}

	if options.Speed.Active() {
		t.Errorf("speed = %+v, want nothing active when the settings are off", options.Speed)
	}
}

func TestCommandPreviewShowsTheSpeedSettings(t *testing.T) {
	app := testApp(t, func(cfg *config.Config) {
		cfg.Connections = 6
		cfg.ChunkedTransfer = true
	})

	preview := app.PreviewCommand(plainRequest())
	if !preview.Success {
		t.Fatalf("preview: %+v", preview.Error)
	}

	want := map[string]string{"--concurrent-fragments": "6", "--http-chunk-size": "10M"}
	for flag, value := range want {
		found := false
		for index, arg := range preview.Args {
			if arg == flag && index+1 < len(preview.Args) && preview.Args[index+1] == value {
				found = true
			}
		}
		if !found {
			t.Errorf("preview %v is missing %s %s: they are choices the user made", preview.Args, flag, value)
		}
	}
}

// withAria2c reports aria2c as found, or as looked for and not found.
func withAria2c(app *App, available bool) {
	status := dependencies.DependencyStatus{Name: dependencies.Aria2c}
	if available {
		status.Available = true
		status.Path = filepath.Join("tools", "aria2c.exe")
	}
	app.mu.Lock()
	app.deps.Aria2c = status
	app.mu.Unlock()
}

func TestAria2cIsUsedWhenSwitchedOnAndFound(t *testing.T) {
	app := testApp(t, func(cfg *config.Config) {
		cfg.Connections = 8
		cfg.UseAria2c = true
	})
	withAria2c(app, true)

	options, failure := app.buildOptions(plainRequest())
	if failure != nil {
		t.Fatalf("build: %+v", failure)
	}

	if options.Speed.Aria2cPath != filepath.Join("tools", "aria2c.exe") {
		t.Errorf("aria2c path = %q, want the copy that was found", options.Speed.Aria2cPath)
	}
	if options.Speed.Aria2cMissing {
		t.Error("aria2c was found, so the download should not report it missing")
	}
}

func TestMissingAria2cDoesNotCostTheDownload(t *testing.T) {
	app := testApp(t, func(cfg *config.Config) {
		cfg.Connections = 8
		cfg.UseAria2c = true
	})
	withAria2c(app, false)

	options, failure := app.buildOptions(plainRequest())
	if failure != nil {
		t.Fatalf("a missing extra failed the download: %+v", failure)
	}

	if options.Speed.Aria2cPath != "" {
		t.Errorf("aria2c path = %q, want none when it was not found", options.Speed.Aria2cPath)
	}
	if !options.Speed.Aria2cMissing {
		t.Error("the download should record that aria2c was asked for and not found")
	}
	if options.Speed.Connections != 8 {
		t.Errorf("connections = %d, want the rest of the speed settings kept", options.Speed.Connections)
	}
}

func TestAria2cIsLeftAloneWhenSwitchedOff(t *testing.T) {
	for _, available := range []bool{true, false} {
		app := testApp(t, func(cfg *config.Config) {
			cfg.Connections = 8
			cfg.UseAria2c = false
		})
		withAria2c(app, available)

		options, failure := app.buildOptions(plainRequest())
		if failure != nil {
			t.Fatalf("build: %+v", failure)
		}
		if options.Speed.Aria2cPath != "" || options.Speed.Aria2cMissing {
			t.Errorf("available=%v: speed = %+v, want aria2c neither used nor reported missing", available, options.Speed)
		}
	}
}

func TestAria2cIsNotMissedWithOneConnection(t *testing.T) {
	// With one connection aria2c would not be used even if it were there, so
	// there is nothing to tell the user about.
	app := testApp(t, func(cfg *config.Config) {
		cfg.Connections = 1
		cfg.UseAria2c = true
	})
	withAria2c(app, false)

	options, failure := app.buildOptions(plainRequest())
	if failure != nil {
		t.Fatalf("build: %+v", failure)
	}
	if options.Speed.Aria2cMissing {
		t.Error("aria2c reported missing for a download that would not have used it")
	}
}

func TestToolSourcesSayWhichDownloadsArePinned(t *testing.T) {
	app := testApp(t, func(*config.Config) {})

	pinned := map[string]string{}
	for _, source := range app.GetToolSources() {
		pinned[source.Name] = source.PinnedVersion
	}

	// A pinned tool has no newer version to fetch, so the interface must be
	// able to tell it from one that follows the latest release.
	if pinned[dependencies.Aria2c] != "1.37.0" {
		t.Errorf("aria2c pinned version = %q, want 1.37.0", pinned[dependencies.Aria2c])
	}
	if version, listed := pinned[dependencies.YtDlp]; !listed || version != "" {
		t.Errorf("yt-dlp pinned version = %q (listed %v), want it listed and following the latest release", version, listed)
	}
}
