package main

import (
	"context"
	"sync"
	"time"

	"grabone/internal/appinfo"
	"grabone/internal/config"
	"grabone/internal/dependencies"
	"grabone/internal/tooling"
)

// eventToolProgress reports the progress of a tool download.
const eventToolProgress = "tool:progress"

// toolInstallTimeout bounds one tool download. FFmpeg is a large archive on a
// slow connection, so this is generous.
const toolInstallTimeout = 30 * time.Minute

// ToolInstallResponse is the result of downloading a dependency.
type ToolInstallResponse struct {
	Success bool   `json:"success"`
	Tool    string `json:"tool"`
	Version string `json:"version"`
	// Directory is where the executables were put.
	Directory string `json:"directory"`

	Dependencies dependencies.Set `json:"dependencies"`
	Error        string           `json:"error,omitempty"`
}

// ToolSource describes a tool the application can download for the user.
type ToolSource struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ProjectURL  string `json:"projectUrl"`
	Licence     string `json:"licence"`
	// Provides lists the dependencies one download supplies, since an FFmpeg
	// build carries FFprobe as well.
	Provides []string `json:"provides"`
}

// toolInstalls guards against two downloads of the same tool at once.
type toolInstalls struct {
	mu     sync.Mutex
	active map[string]bool
}

func (t *toolInstalls) begin(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active == nil {
		t.active = map[string]bool{}
	}
	if t.active[name] {
		return false
	}
	t.active[name] = true
	return true
}

func (t *toolInstalls) end(name string) {
	t.mu.Lock()
	delete(t.active, name)
	t.mu.Unlock()
}

// GetToolSources lists the dependencies GrabOne can download, so the interface
// can offer it only where it is possible.
func (a *App) GetToolSources() []ToolSource {
	sources := tooling.Sources()

	out := make([]ToolSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, ToolSource{
			Name:        source.Names[0],
			DisplayName: source.DisplayName,
			ProjectURL:  source.ProjectURL,
			Licence:     source.Licence,
			Provides:    source.Names,
		})
	}
	return out
}

// InstallTool downloads a dependency from its official release, verifies it
// against the published checksum and puts it in the folder GrabOne manages.
//
// Nothing here runs on its own: this is called because the user asked for it.
func (a *App) InstallTool(name string) ToolInstallResponse {
	response := ToolInstallResponse{Tool: name, Dependencies: a.GetDependencies()}

	if _, ok := tooling.SourceFor(name); !ok {
		response.Error = dependencies.DisplayName(name) + " cannot be downloaded automatically."
		return response
	}
	if !a.toolInstalls.begin(name) {
		response.Error = dependencies.DisplayName(name) + " is already downloading."
		return response
	}
	defer a.toolInstalls.end(name)

	ctx, cancel := context.WithTimeout(a.context(), toolInstallTimeout)
	defer cancel()

	a.logger.Info("downloading tool", "tool", name)

	installer := tooling.NewInstaller(config.ToolsDir(), "GrabOne/"+appinfo.Version)
	result, err := installer.Install(ctx, name, func(progress tooling.Progress) {
		a.emit(eventToolProgress, progress)
	})
	if err != nil {
		a.logger.Error("tool download failed", "tool", name, "error", err)
		response.Error = err.Error()
		return response
	}

	a.logger.Info("tool downloaded", "tool", name, "version", result.Version, "files", result.Installed)

	// A configured path takes priority over everything else, so a stale one
	// would hide the copy just downloaded. Asking GrabOne to provide the tool
	// supersedes a location set by hand.
	a.clearConfiguredPaths(result.Tool)

	// The new copy has to be found before it can be used.
	response.Success = true
	response.Version = result.Version
	response.Directory = result.Directory
	response.Dependencies = a.refreshDependencies(a.context())
	return response
}

// UpdateTool re-downloads a tool GrabOne manages, which is how a stale yt-dlp
// gets refreshed. A tool installed by a package manager is left alone: it is
// not GrabOne's to replace.
func (a *App) UpdateTool(name string) ToolInstallResponse {
	set := a.GetDependencies()

	for _, status := range set.All() {
		if status.Name == name && status.Available && !status.Managed {
			return ToolInstallResponse{
				Tool:         name,
				Dependencies: set,
				Error:        dependencies.DisplayName(name) + " was installed outside GrabOne, so update it the same way you installed it.",
			}
		}
	}
	return a.InstallTool(name)
}

// clearConfiguredPaths removes the manually set locations for the dependencies
// a download supplies, so the freshly installed copy is the one that is found.
func (a *App) clearConfiguredPaths(name string) {
	source, ok := tooling.SourceFor(name)
	if !ok {
		return
	}

	if _, err := a.store.Update(func(cfg *config.Config) {
		for _, provided := range source.Names {
			switch provided {
			case dependencies.YtDlp:
				cfg.YtDlpPath = ""
			case dependencies.FFmpeg:
				cfg.FFmpegPath = ""
			case dependencies.FFprobe:
				cfg.FFprobePath = ""
			}
		}
	}); err != nil {
		a.logger.Info("could not clear the configured tool paths", "error", err)
	}
}
