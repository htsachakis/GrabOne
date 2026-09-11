package main

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"grabone/internal/appinfo"
	"grabone/internal/config"
	"grabone/internal/updater"
)

// Event names for the update flow.
const (
	eventUpdateAvailable = "update:available"
	eventUpdateProgress  = "update:progress"
)

// startupCheckDelay lets the window settle before the update check runs.
const startupCheckDelay = 3 * time.Second

// UpdateStatus is what the interface knows about updates.
type UpdateStatus struct {
	// Checked reports whether a check has completed in this session.
	Checked bool `json:"checked"`
	// Available reports whether a newer release was published.
	Available bool `json:"available"`

	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`

	Release *updater.Release `json:"release,omitempty"`

	// Downloading is true while the installer is being fetched.
	Downloading bool `json:"downloading"`
	// DownloadedPath is the verified installer, ready to run.
	DownloadedPath string `json:"downloadedPath,omitempty"`

	// Skipped reports that the user asked not to be reminded about this version.
	Skipped bool `json:"skipped"`

	CheckedAt string `json:"checkedAt,omitempty"`
	Error     string `json:"error,omitempty"`
}

// UpdateProgress is emitted while the installer downloads.
type UpdateProgress struct {
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Percent         float64 `json:"percent"`
}

// updates holds the update state of the running application.
type updates struct {
	mu     sync.Mutex
	status UpdateStatus
}

// updateDirectory is where downloaded installers are kept.
func updateDirectory() string {
	return filepath.Join(config.DataDir(), "updates")
}

// checker builds the update checker for this build.
func (a *App) checker() *updater.Checker {
	return updater.NewChecker(appinfo.RepositoryOwner, appinfo.RepositoryName, appinfo.Version)
}

// GetUpdateStatus returns what is known about updates without checking again.
func (a *App) GetUpdateStatus() UpdateStatus {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()

	status := a.updates.status
	status.CurrentVersion = appinfo.Version
	return status
}

// CheckForUpdate asks GitHub whether a newer release exists.
//
// A failed check is reported in the status rather than as an error: not being
// able to reach GitHub is information, not a fault in the application.
func (a *App) CheckForUpdate() UpdateStatus {
	settings := a.store.Get()

	release, available, err := a.checker().Check(a.context())

	a.updates.mu.Lock()
	status := UpdateStatus{
		Checked:        true,
		CurrentVersion: appinfo.Version,
		CheckedAt:      time.Now().Format(time.RFC3339),
		// A download from an earlier check stays valid while the version matches.
		DownloadedPath: a.updates.status.DownloadedPath,
	}

	switch {
	case err != nil:
		status.Error = err.Error()
		a.logger.Info("update check failed", "error", err)
	default:
		status.Available = available
		status.Release = release
		status.LatestVersion = release.Version
		status.Skipped = available && settings.SkippedUpdateVersion == release.Version

		if status.DownloadedPath != "" && release.Version != a.updates.status.LatestVersion {
			// A newer release makes an earlier download irrelevant.
			status.DownloadedPath = ""
		}
		a.logger.Info("update check finished",
			"current", appinfo.Version,
			"latest", release.Version,
			"available", available,
		)
	}
	a.updates.status = status
	a.updates.mu.Unlock()

	if status.Available && !status.Skipped {
		a.emit(eventUpdateAvailable, status)
	}
	return status
}

// DownloadUpdate fetches the installer of the available release and verifies it.
// The installer is never started by this call.
func (a *App) DownloadUpdate() UpdateStatus {
	a.updates.mu.Lock()
	release := a.updates.status.Release
	if a.updates.status.Downloading {
		defer a.updates.mu.Unlock()
		return a.updates.status
	}
	a.updates.status.Downloading = true
	a.updates.status.Error = ""
	a.updates.mu.Unlock()

	if release == nil {
		return a.finishDownload("", "check for an update first")
	}

	a.logger.Info("downloading update", "version", release.Version)

	directory := updateDirectory()
	path, err := a.checker().Download(a.context(), release, directory, func(progress updater.Progress) {
		percent := 0.0
		if progress.TotalBytes > 0 {
			percent = float64(progress.DownloadedBytes) / float64(progress.TotalBytes) * 100
		}
		a.emit(eventUpdateProgress, UpdateProgress{
			DownloadedBytes: progress.DownloadedBytes,
			TotalBytes:      progress.TotalBytes,
			Percent:         percent,
		})
	})
	if err != nil {
		a.logger.Error("update download failed", "error", err)
		return a.finishDownload("", err.Error())
	}

	updater.CleanDownloads(directory, path)
	a.logger.Info("update downloaded and verified", "path", path)
	return a.finishDownload(path, "")
}

func (a *App) finishDownload(path, failure string) UpdateStatus {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()

	a.updates.status.Downloading = false
	a.updates.status.DownloadedPath = path
	a.updates.status.Error = failure
	return a.updates.status
}

// InstallUpdate starts the downloaded installer and closes the application so
// the files it replaces are not in use.
func (a *App) InstallUpdate() error {
	a.updates.mu.Lock()
	path := a.updates.status.DownloadedPath
	a.updates.mu.Unlock()

	if path == "" {
		return errNoInstaller
	}
	if err := updater.Install(path); err != nil {
		a.logger.Error("could not start the installer", "error", err)
		return err
	}

	a.logger.Info("installer started, closing the application", "path", path)
	a.manager.CancelAll()
	a.quit()
	return nil
}

// SkipUpdateVersion remembers that the user does not want to be reminded about
// this version again.
func (a *App) SkipUpdateVersion(version string) UpdateStatus {
	if _, err := a.store.Update(func(cfg *config.Config) {
		cfg.SkippedUpdateVersion = version
	}); err != nil {
		a.logger.Info("could not store the skipped version", "error", err)
	}

	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	a.updates.status.Skipped = a.updates.status.LatestVersion == version
	return a.updates.status
}

// OpenReleasePage opens the release notes in the browser.
func (a *App) OpenReleasePage() error {
	a.updates.mu.Lock()
	release := a.updates.status.Release
	a.updates.mu.Unlock()

	if release == nil || release.PageURL == "" {
		return errNoRelease
	}
	return a.openReleaseURL(release.PageURL)
}

// startUpdateCheck runs the automatic check shortly after startup, when the
// user has left it enabled.
func (a *App) startUpdateCheck(ctx context.Context) {
	if !a.store.Get().AutoCheckUpdates {
		return
	}

	go func() {
		select {
		case <-time.After(startupCheckDelay):
		case <-ctx.Done():
			return
		}
		a.CheckForUpdate()
	}()
}
