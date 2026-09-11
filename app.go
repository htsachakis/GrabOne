package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"grabone/internal/appinfo"
	"grabone/internal/config"
	"grabone/internal/dependencies"
	"grabone/internal/downloads"
	"grabone/internal/ffmpeg"
	"grabone/internal/logging"
	"grabone/internal/media"
	"grabone/internal/system"
	"grabone/internal/updater"
	"grabone/internal/ytdlp"
)

// analysisTimeout bounds one analysis. Extraction can be slow, but it must not
// hang the interface indefinitely.
const analysisTimeout = 3 * time.Minute

// App is the service bound to the frontend. Its methods are the whole API
// surface of the application.
type App struct {
	ctx context.Context

	logger   *logging.Logger
	store    *config.Store
	detector *dependencies.Detector

	mu       sync.RWMutex
	deps     dependencies.Set
	client   *ytdlp.Client
	prober   *ffmpeg.Prober
	analyses map[string]*media.MediaInfo

	manager *downloads.Manager

	// detected is closed once the executables have been probed for the first
	// time. Callers wait on it so the interface is never handed an empty
	// dependency picture that looks like "nothing was found".
	detected     chan struct{}
	detectedOnce sync.Once

	updates      updates
	toolInstalls toolInstalls
}

// firstDetectionTimeout bounds how long a caller waits for the initial probe.
// Three executables are run, each with its own timeout, so this only matters if
// something is badly stuck.
const firstDetectionTimeout = 60 * time.Second

// NewApp builds the application service.
func NewApp(logger *logging.Logger, store *config.Store) *App {
	app := &App{
		logger:   logger,
		store:    store,
		detector: dependencies.NewDetector(config.ExecutableDir(), config.ToolsDir()),
		analyses: map[string]*media.MediaInfo{},
		detected: make(chan struct{}),
	}
	app.manager = downloads.NewManager(nil, nil, logger, app.emit)
	return app
}

// startup runs once the window exists.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	settings := a.store.Get()
	a.logger.Info("application started",
		"name", appinfo.Name,
		"version", appinfo.Version,
		"settings", a.store.Path(),
		"output", settings.OutputDirectory,
	)

	a.refreshDependencies(ctx)
	a.ensureOutputDirectory(settings.OutputDirectory)
	updater.ClearDownloads(updateDirectory())
	a.startUpdateCheck(ctx)
}

// shutdown stops anything still running when the window closes.
func (a *App) beforeClose(context.Context) bool {
	if a.manager.ActiveCount() > 0 {
		a.logger.Info("cancelling downloads still running at shutdown")
		a.manager.CancelAll()
	}
	return false
}

func (a *App) emit(event string, payload any) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, event, payload)
}

// GetAppInfo returns the application identity.
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:           appinfo.Name,
		VersionLabel:   appinfo.VersionLabel(),
		Version:        appinfo.Version,
		Description:    appinfo.ShortDescription,
		LegalNotice:    appinfo.LegalNotice,
		ApplicationDir: config.ExecutableDir(),
		SettingsPath:   a.store.Path(),
		LogDirectory:   config.LogDir(),
	}
}

// GetDependencies returns the dependency status, waiting for the first probe to
// finish so a slow detection is never mistaken for a missing tool.
func (a *App) GetDependencies() dependencies.Set {
	a.awaitFirstDetection()

	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.deps
}

// awaitFirstDetection blocks until the executables have been probed once.
func (a *App) awaitFirstDetection() {
	select {
	case <-a.detected:
	case <-time.After(firstDetectionTimeout):
	}
}

// RefreshDependencies probes the executables again, which is what the Retry
// action in the interface calls.
func (a *App) RefreshDependencies() dependencies.Set {
	return a.refreshDependencies(a.context())
}

// refreshDependencies detects the tools and rebuilds the clients that use them.
func (a *App) refreshDependencies(ctx context.Context) dependencies.Set {
	settings := a.store.Get()

	set := a.detector.Detect(ctx, dependencies.Paths{
		YtDlp:   settings.YtDlpPath,
		FFmpeg:  settings.FFmpegPath,
		FFprobe: settings.FFprobePath,
	})

	client := ytdlp.NewClient(set.YtDlp.Path, set.FFmpeg.Path, a.logger)
	prober := ffmpeg.NewProber(set.FFprobe.Path)

	a.mu.Lock()
	a.deps = set
	a.client = client
	a.prober = prober
	a.mu.Unlock()

	a.manager.Configure(client, prober, settings.MaxConcurrentDownloads)

	a.detectedOnce.Do(func() { close(a.detected) })

	for _, status := range set.All() {
		a.logger.Info("dependency detected",
			"name", status.Name,
			"available", status.Available,
			"version", status.Version,
			"path", status.Path,
			"source", status.Source,
		)
	}
	return set
}

// LocateDependency asks the user where an executable is, stores the choice and
// probes it again. Dependencies are never downloaded automatically.
func (a *App) LocateDependency(name string) SettingsResponse {
	switch name {
	case dependencies.YtDlp, dependencies.FFmpeg, dependencies.FFprobe:
	default:
		return SettingsResponse{Settings: a.store.Get(), Dependencies: a.GetDependencies(), Error: "unknown dependency " + name}
	}

	selected, err := wailsruntime.OpenFileDialog(a.context(), wailsruntime.OpenDialogOptions{
		Title: "Select " + dependencies.DisplayName(name),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Executables (*.exe)", Pattern: "*.exe"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return SettingsResponse{Settings: a.store.Get(), Dependencies: a.GetDependencies(), Error: err.Error()}
	}
	if strings.TrimSpace(selected) == "" {
		// The dialog was dismissed.
		return SettingsResponse{Settings: a.store.Get(), Dependencies: a.GetDependencies()}
	}

	updated, err := a.store.Update(func(cfg *config.Config) {
		switch name {
		case dependencies.YtDlp:
			cfg.YtDlpPath = selected
		case dependencies.FFmpeg:
			cfg.FFmpegPath = selected
		case dependencies.FFprobe:
			cfg.FFprobePath = selected
		}
	})

	response := SettingsResponse{Settings: updated, Dependencies: a.refreshDependencies(a.context())}
	if err != nil {
		response.Error = err.Error()
	}
	return response
}

// GetSettings returns the stored settings with the dependency picture.
func (a *App) GetSettings() SettingsResponse {
	return SettingsResponse{Settings: a.store.Get(), Dependencies: a.GetDependencies()}
}

// SaveSettings persists the settings and applies them.
func (a *App) SaveSettings(settings config.Config) SettingsResponse {
	previous := a.store.Get()

	if err := a.store.Set(settings); err != nil {
		a.logger.Error("could not save settings", "error", err)
		return SettingsResponse{Settings: a.store.Get(), Dependencies: a.GetDependencies(), Error: err.Error()}
	}
	current := a.store.Get()

	a.ensureOutputDirectory(current.OutputDirectory)

	set := a.GetDependencies()
	if pathsChanged(previous, current) {
		set = a.refreshDependencies(a.context())
	} else {
		a.mu.RLock()
		client, prober := a.client, a.prober
		a.mu.RUnlock()
		a.manager.Configure(client, prober, current.MaxConcurrentDownloads)
	}

	a.logger.Info("settings saved", "output", current.OutputDirectory, "theme", current.Theme)
	return SettingsResponse{Settings: current, Dependencies: set}
}

func pathsChanged(previous, current config.Config) bool {
	return previous.YtDlpPath != current.YtDlpPath ||
		previous.FFmpegPath != current.FFmpegPath ||
		previous.FFprobePath != current.FFprobePath
}

// ChooseOutputDirectory opens the native folder picker.
func (a *App) ChooseOutputDirectory() (string, error) {
	settings := a.store.Get()
	selected, err := wailsruntime.OpenDirectoryDialog(a.context(), wailsruntime.OpenDialogOptions{
		Title:                "Choose where to save downloads",
		DefaultDirectory:     settings.OutputDirectory,
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("choose output folder: %w", err)
	}
	return selected, nil
}

// ChooseCookieFile opens a picker for a cookies.txt file. The file is only ever
// handed to yt-dlp; its contents are never read or logged by the application.
func (a *App) ChooseCookieFile() (string, error) {
	selected, err := wailsruntime.OpenFileDialog(a.context(), wailsruntime.OpenDialogOptions{
		Title: "Select a cookies.txt file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Cookie files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("choose cookie file: %w", err)
	}
	return selected, nil
}

// GetInstallGuidance returns how to install a dependency GrabOne could not
// find. The tools are never installed by the application itself.
func (a *App) GetInstallGuidance() []dependencies.Guidance {
	return dependencies.AllGuidance(config.ExecutableDir())
}

// OpenURL opens one of the documented download pages in the browser. Only the
// addresses the guidance itself offers are accepted, so this cannot become a
// way to open an arbitrary link.
func (a *App) OpenURL(url string) error {
	if !dependencies.IsKnownHelpURL(url) {
		return fmt.Errorf("refusing to open an unknown address")
	}
	wailsruntime.BrowserOpenURL(a.context(), url)
	return nil
}

// GetCookieBrowsers lists the browsers yt-dlp can read cookies from.
func (a *App) GetCookieBrowsers() []string {
	return append([]string(nil), ytdlp.SupportedCookieBrowsers...)
}

// AnalyzeURL inspects a link and returns what it holds. Any HTTP or HTTPS link
// is accepted: yt-dlp decides which extractor handles it, so the application
// gains support for new sites as yt-dlp evolves.
func (a *App) AnalyzeURL(request AnalyzeRequest) AnalyzeResponse {
	client, set := a.currentClient()
	if !set.YtDlp.Available {
		return AnalyzeResponse{Error: ytdlp.MissingDependencyError("yt-dlp", set.YtDlp.Error)}
	}

	settings := a.store.Get()
	mode := request.CollectionMode
	if mode == "" {
		mode = ytdlp.CollectionModeSingle
	}

	ctx, cancel := context.WithTimeout(a.context(), analysisTimeout)
	defer cancel()

	a.logger.Info("analyzing url", "url", request.URL, "mode", mode)

	info, failure := client.Analyze(ctx, ytdlp.AnalyzeOptions{
		URL:            request.URL,
		CollectionMode: mode,
		Cookies:        cookieOptions(settings),
	})
	if failure != nil && info == nil {
		a.logger.Info("analysis failed", "url", request.URL, "kind", string(failure.Kind), "message", failure.Message)
		return AnalyzeResponse{Error: failure, CanMerge: set.CanMerge(), CanConvertAudio: set.FFmpeg.Available}
	}

	a.rememberAnalysis(info)
	a.logger.Info("analysis finished",
		"url", request.URL,
		"extractor", info.ExtractorKey,
		"platform", info.Platform,
		"contentType", info.ContentType,
		"formats", len(info.Formats),
		"entries", len(info.Entries),
	)

	response := AnalyzeResponse{
		Success:           failure == nil,
		Media:             info,
		Error:             failure,
		ResolutionPresets: media.ResolutionPresets(info.Formats),
		Defaults:          selectionDefaults(info, settings),
		CanMerge:          set.CanMerge(),
		CanConvertAudio:   set.FFmpeg.Available,
	}
	return response
}

// ResolveEntry analyzes one item of a collection so its formats can be chosen.
func (a *App) ResolveEntry(entryURL string) AnalyzeResponse {
	return a.AnalyzeURL(AnalyzeRequest{URL: entryURL, CollectionMode: ytdlp.CollectionModeSingle})
}

// PreviewCommand renders the yt-dlp command the current selection produces.
func (a *App) PreviewCommand(request DownloadRequest) PreviewResponse {
	options, failure := a.buildOptions(request)
	if failure != nil {
		return PreviewResponse{Error: failure}
	}

	args, err := ytdlp.BuildDownloadArgs(options)
	if err != nil {
		return PreviewResponse{Error: &ytdlp.Error{Kind: ytdlp.KindUnknown, Message: err.Error()}}
	}

	client, _ := a.currentClient()
	return PreviewResponse{
		Success:           true,
		Command:           ytdlp.BuildCommandPreview(client.BinaryPath(), args),
		Args:              args,
		ResolvedContainer: resolvedContainer(options),
	}
}

// StartDownload queues a download and returns its job.
func (a *App) StartDownload(request DownloadRequest) StartResponse {
	options, failure := a.buildOptions(request)
	if failure != nil {
		return StartResponse{Error: failure}
	}

	view, err := a.manager.Enqueue(options, a.metadataFor(request, options))
	if err != nil {
		return StartResponse{Error: &ytdlp.Error{Kind: ytdlp.KindUnknown, Message: err.Error()}}
	}

	a.rememberPreferences(request)
	return StartResponse{Success: true, Download: &view}
}

// CancelDownload stops a running or queued download.
func (a *App) CancelDownload(id string) error { return a.manager.Cancel(id) }

// ListDownloads returns every job in this session.
func (a *App) ListDownloads() []downloads.View { return a.manager.List() }

// ClearFinishedDownloads removes finished jobs from the list.
func (a *App) ClearFinishedDownloads() []downloads.View {
	a.manager.ClearFinished()
	return a.manager.List()
}

// OpenFile opens a finished download with its default application.
func (a *App) OpenFile(path string) error { return system.OpenFile(path) }

// OpenContainingFolder reveals a finished download in Explorer.
func (a *App) OpenContainingFolder(path string) error { return system.RevealInFolder(path) }

// OpenOutputDirectory opens the configured download folder.
func (a *App) OpenOutputDirectory() error {
	directory := a.store.Get().OutputDirectory
	a.ensureOutputDirectory(directory)
	return system.OpenFolder(directory)
}

// OpenLogDirectory opens the folder holding the log files.
func (a *App) OpenLogDirectory() error {
	directory := config.LogDir()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("open log folder: %w", err)
	}
	return system.OpenFolder(directory)
}

// buildOptions turns a request from the interface into validated download
// options, filling in what the interface does not need to know: the stored
// settings, the codecs of the selected streams and the authentication choice.
func (a *App) buildOptions(request DownloadRequest) (ytdlp.DownloadOptions, *ytdlp.Error) {
	settings := a.store.Get()

	normalizedURL, urlErr := ytdlp.ValidateURL(request.URL)
	if urlErr != nil {
		return ytdlp.DownloadOptions{}, urlErr
	}

	outputDirectory := strings.TrimSpace(request.OutputDirectory)
	if outputDirectory == "" {
		outputDirectory = settings.OutputDirectory
	}
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return ytdlp.DownloadOptions{}, &ytdlp.Error{
			Kind:    ytdlp.KindUnknown,
			Message: "The download folder could not be created.",
			Hint:    "Choose a different folder in Settings.",
			Details: err.Error(),
		}
	}

	downloadType := request.DownloadType
	if downloadType == "" {
		downloadType = ytdlp.DownloadTypeVideoAudio
	}

	container := request.Container
	if container == "" {
		container = media.ContainerAuto
	}

	audioConversion := request.AudioConversionFormat
	if audioConversion == "" {
		audioConversion = media.AudioConversionOriginal
	}

	subtitleFormat := request.SubtitleFormat
	if subtitleFormat == "" {
		subtitleFormat = "srt"
	}

	options := ytdlp.DownloadOptions{
		URL:          normalizedURL,
		Title:        request.Title,
		DownloadType: downloadType,

		VideoFormatID:    request.VideoFormatID,
		AudioFormatID:    request.AudioFormatID,
		CombinedFormatID: request.CombinedFormatID,
		MaxHeight:        request.MaxHeight,

		Container: container,

		EmbedMetadata:  request.EmbedMetadata,
		EmbedChapters:  request.EmbedChapters,
		EmbedThumbnail: request.EmbedThumbnail,

		SaveThumbnail:   request.SaveThumbnail,
		SaveDescription: request.SaveDescription,
		SaveJSON:        request.SaveJSON,

		DownloadSubtitles: request.DownloadSubtitles,
		EmbedSubtitles:    request.EmbedSubtitles,
		KeepSubtitles:     request.KeepSubtitles,
		SubtitleLanguages: request.SubtitleLanguages,
		SubtitleFormat:    subtitleFormat,

		AudioConversionFormat: audioConversion,

		OutputDirectory:  outputDirectory,
		FilenameTemplate: settings.FilenameTemplate,

		CollectionMode: request.CollectionMode,
		PlaylistItems:  request.PlaylistItems,

		Cookies: cookieOptions(settings),
	}

	a.applyAnalysisContext(&options)

	if err := options.Validate(); err != nil {
		return ytdlp.DownloadOptions{}, &ytdlp.Error{
			Kind:    ytdlp.KindUnknown,
			Message: "That combination of options is not valid.",
			Details: err.Error(),
		}
	}
	if failure := a.checkFFmpegRequirement(options); failure != nil {
		return ytdlp.DownloadOptions{}, failure
	}
	return options, nil
}

// applyAnalysisContext fills in the details that come from the analysis rather
// than from the user: the codecs of the chosen streams, which decide the
// automatic container, and whether a chosen subtitle language is automatic.
func (a *App) applyAnalysisContext(options *ytdlp.DownloadOptions) {
	info := a.recallAnalysis(options.URL)
	if info == nil {
		return
	}

	if options.CombinedFormatID != "" {
		if format, ok := info.FormatByID(options.CombinedFormatID); ok {
			options.SelectedVideoCodec = format.RawVideoCodec
			options.SelectedAudioCodec = format.RawAudioCodec
		}
	}
	if options.VideoFormatID != "" {
		if format, ok := info.FormatByID(options.VideoFormatID); ok {
			options.SelectedVideoCodec = format.RawVideoCodec
		}
	}
	if options.AudioFormatID != "" {
		if format, ok := info.FormatByID(options.AudioFormatID); ok {
			options.SelectedAudioCodec = format.RawAudioCodec
		}
	}

	if options.DownloadSubtitles {
		for _, requested := range options.SubtitleLanguages {
			for _, track := range info.Subtitles {
				if track.Code == requested && track.Automatic {
					options.IncludeAutomaticCaptions = true
				}
			}
		}
	}
}

// checkFFmpegRequirement refuses a download that cannot succeed without FFmpeg,
// instead of letting it fail halfway through.
func (a *App) checkFFmpegRequirement(options ytdlp.DownloadOptions) *ytdlp.Error {
	_, set := a.currentClient()
	if set.FFmpeg.Available {
		return nil
	}

	reason := ""
	switch {
	case options.DownloadType == ytdlp.DownloadTypeVideoAudio && options.CombinedFormatID == "":
		reason = "merging separate video and audio streams"
	case options.DownloadType == ytdlp.DownloadTypeAudioOnly:
		reason = "extracting audio"
	case options.Container != "" && options.Container != media.ContainerAuto:
		reason = "changing the container"
	case options.EmbedMetadata || options.EmbedChapters || options.EmbedThumbnail:
		reason = "embedding metadata"
	case options.DownloadSubtitles && options.EmbedSubtitles:
		reason = "embedding subtitles"
	}
	if reason == "" {
		return nil
	}

	return &ytdlp.Error{
		Kind:    ytdlp.KindMissingDependency,
		Message: "FFmpeg is required for " + reason + ", but it was not found.",
		Hint:    "Set the FFmpeg location in Settings › Dependencies, or choose a single combined stream with no extra processing.",
		Details: set.FFmpeg.Error,
	}
}

// metadataFor describes the job in the download list.
func (a *App) metadataFor(request DownloadRequest, options ytdlp.DownloadOptions) downloads.Metadata {
	metadata := downloads.Metadata{Title: strings.TrimSpace(request.Title)}

	if info := a.recallAnalysis(options.URL); info != nil {
		metadata.Platform = info.Platform
		metadata.PlatformSlug = info.PlatformSlug
		metadata.ThumbnailURL = info.ThumbnailURL
		metadata.ContentType = info.ContentType
		metadata.DurationText = info.DurationText
		metadata.Uploader = firstNonEmptyString(info.Uploader, info.Channel)
		if metadata.Title == "" {
			metadata.Title = info.Title
		}
	}
	if metadata.Title == "" {
		metadata.Title = options.URL
	}
	return metadata
}

// rememberPreferences stores the option choices so the next download starts from
// the same settings.
func (a *App) rememberPreferences(request DownloadRequest) {
	_, err := a.store.Update(func(cfg *config.Config) {
		cfg.Preferences.DownloadType = request.DownloadType
		cfg.Preferences.Container = request.Container
		cfg.Preferences.EmbedMetadata = request.EmbedMetadata
		cfg.Preferences.EmbedChapters = request.EmbedChapters
		cfg.Preferences.EmbedThumbnail = request.EmbedThumbnail
		cfg.Preferences.SaveThumbnail = request.SaveThumbnail
		cfg.Preferences.SaveDescription = request.SaveDescription
		cfg.Preferences.SaveJSON = request.SaveJSON
		cfg.Preferences.DownloadSubtitles = request.DownloadSubtitles
		cfg.Preferences.EmbedSubtitles = request.EmbedSubtitles
		cfg.Preferences.KeepSubtitles = request.KeepSubtitles
		cfg.Preferences.SubtitleFormat = request.SubtitleFormat
		cfg.Preferences.SubtitleLanguages = append([]string(nil), request.SubtitleLanguages...)
		cfg.Preferences.AudioConversionFormat = request.AudioConversionFormat
		if directory := strings.TrimSpace(request.OutputDirectory); directory != "" {
			cfg.OutputDirectory = directory
		}
	})
	if err != nil {
		a.logger.Info("could not store preferences", "error", err)
	}
}

func (a *App) rememberAnalysis(info *media.MediaInfo) {
	if info == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	// The cache only needs the most recent analyses, keyed by the URL a download
	// will be started with.
	if len(a.analyses) > 32 {
		a.analyses = map[string]*media.MediaInfo{}
	}
	if info.RequestedURL != "" {
		a.analyses[info.RequestedURL] = info
	}
	if info.WebpageURL != "" {
		a.analyses[info.WebpageURL] = info
	}
}

func (a *App) recallAnalysis(url string) *media.MediaInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.analyses[url]
}

func (a *App) currentClient() (*ytdlp.Client, dependencies.Set) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	client := a.client
	if client == nil {
		client = ytdlp.NewClient("", "", a.logger)
	}
	return client, a.deps
}

// quit closes the window, used after an installer has been started.
func (a *App) quit() {
	if a.ctx == nil {
		return
	}
	wailsruntime.Quit(a.ctx)
}

// openReleaseURL opens a release page. Only addresses on the project's own
// repository are accepted.
func (a *App) openReleaseURL(url string) error {
	if !strings.HasPrefix(url, "https://github.com/"+appinfo.RepositoryOwner+"/"+appinfo.RepositoryName+"/") {
		return fmt.Errorf("refusing to open an unexpected address")
	}
	wailsruntime.BrowserOpenURL(a.context(), url)
	return nil
}

var (
	errNoInstaller = fmt.Errorf("no verified installer has been downloaded")
	errNoRelease   = fmt.Errorf("no release information is available")
)

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) ensureOutputDirectory(directory string) {
	if strings.TrimSpace(directory) == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		a.logger.Info("could not create output folder", "path", directory, "error", err)
	}
}

// cookieOptions maps the stored authentication settings onto the engine options.
// A source the user picked but has not finished configuring is treated as no
// authentication, so a half-made choice cannot fail a download.
func cookieOptions(settings config.Config) ytdlp.CookieOptions {
	return ytdlp.CookieOptions{
		Source:  settings.CookieSource,
		Browser: settings.CookieBrowser,
		File:    settings.CookieFile,
	}.Usable()
}

// selectionDefaults chooses the starting selection from the analysis, so the
// common case needs no decisions from the user.
func selectionDefaults(info *media.MediaInfo, settings config.Config) SelectionDefaults {
	defaults := SelectionDefaults{
		DownloadType:      settings.Preferences.DownloadType,
		Container:         settings.Preferences.Container,
		SubtitleLanguages: []string{},
	}
	if defaults.DownloadType == "" {
		defaults.DownloadType = ytdlp.DownloadTypeVideoAudio
	}
	if defaults.Container == "" {
		defaults.Container = media.ContainerAuto
	}

	capabilities := info.Capabilities
	if !capabilities.HasVideo && capabilities.HasAudio {
		defaults.DownloadType = ytdlp.DownloadTypeAudioOnly
	}

	videoCodec, audioCodec := "", ""

	if best, ok := media.BestFormat(info.Formats, media.FormatKindVideo); ok {
		defaults.VideoFormatID = best.FormatID
		videoCodec = best.RawVideoCodec
	}
	if best, ok := media.BestFormat(info.Formats, media.FormatKindAudio); ok {
		defaults.AudioFormatID = best.FormatID
		audioCodec = best.RawAudioCodec
	}
	if best, ok := media.BestFormat(info.Formats, media.FormatKindCombined); ok {
		defaults.CombinedFormatID = best.FormatID
		// A single combined stream needs no merging, so prefer it when the media
		// exposes nothing better.
		if !capabilities.HasSeparateVideo || !capabilities.HasSeparateAudio {
			defaults.VideoFormatID = ""
			defaults.AudioFormatID = ""
			videoCodec = best.RawVideoCodec
			audioCodec = best.RawAudioCodec
		} else {
			defaults.CombinedFormatID = ""
		}
	}

	if defaults.DownloadType == ytdlp.DownloadTypeAudioOnly {
		defaults.ResolvedContainer = media.AudioContainerFor(settings.Preferences.AudioConversionFormat)
	} else {
		defaults.ResolvedContainer = media.AutomaticContainer(videoCodec, audioCodec, false)
	}

	// Preselect the subtitle languages the user last used, when available.
	for _, code := range settings.Preferences.SubtitleLanguages {
		for _, track := range info.Subtitles {
			if track.Code == code {
				defaults.SubtitleLanguages = append(defaults.SubtitleLanguages, code)
				break
			}
		}
	}
	return defaults
}

// resolvedContainer reports the container the options will actually produce.
func resolvedContainer(options ytdlp.DownloadOptions) string {
	if options.DownloadType == ytdlp.DownloadTypeAudioOnly {
		if container := media.AudioContainerFor(options.AudioConversionFormat); container != "" {
			return container
		}
		return "source audio"
	}
	if options.Container != "" && options.Container != media.ContainerAuto {
		return options.Container
	}
	return media.AutomaticContainer(
		options.SelectedVideoCodec,
		options.SelectedAudioCodec,
		options.DownloadSubtitles && options.EmbedSubtitles,
	)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
