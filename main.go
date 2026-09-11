// GrabOne is a desktop frontend for yt-dlp: it analyzes a media link, shows
// what the extractor offers, and downloads the chosen streams. All extraction,
// downloading and post-processing is performed by yt-dlp and FFmpeg, which are
// external executables the application drives and never bundles.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"grabone/internal/appinfo"
	"grabone/internal/config"
	"grabone/internal/logging"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logger := logging.New(config.LogDir())
	defer func() { _ = logger.Close() }()

	store, err := config.NewStore(config.SettingsFile())
	if err != nil {
		// A broken settings file must not stop the application: the defaults are
		// used and the problem is recorded.
		logger.Error("could not read settings, using defaults", "error", err)
	}

	app := NewApp(logger, store)

	err = wails.Run(&options.App{
		Title:     appinfo.WindowTitle(),
		Width:     1120,
		Height:    860,
		MinWidth:  880,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 20, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		logger.Error("application exited with an error", "error", err)
	}
}
