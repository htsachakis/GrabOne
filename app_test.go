package main

import (
	"path/filepath"
	"testing"

	"grabone/internal/config"
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
