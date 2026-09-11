/**
 * Application shell: builds the layout, owns the state transitions and connects
 * the components to the backend.
 */

import * as api from "./api";
import { el, replace } from "./dom";
import { AudioSelector } from "./components/AudioSelector";
import { CollectionView } from "./components/CollectionView";
import { CommandPreview } from "./components/CommandPreview";
import { DependencyStatus, detected } from "./components/DependencyStatus";
import { DownloadList } from "./components/DownloadProgress";
import { DownloadOptionsPanel } from "./components/DownloadOptions";
import { ErrorPanel, emptyState } from "./components/ErrorPanel";
import { FormatSelector } from "./components/FormatSelector";
import { MediaInfoCard, warningList } from "./components/MediaInfo";
import { SettingsPanel } from "./components/SettingsPanel";
import { SubtitleSelector } from "./components/SubtitleSelector";
import { UpdateBanner } from "./components/UpdateBanner";
import { UrlInput } from "./components/UrlInput";
import { Store, activeDownloads, defaultSelection, isBusy } from "./state";
import type { AppState, Selection } from "./state";
import type {
  AnalyzeResponse,
  ToolSource,
  DownloadRequest,
  InstallGuidance,
  MediaInfo,
  Settings,
  UpdateStatus,
} from "./types";

export class Application {
  private readonly store = new Store();

  private readonly root: HTMLElement;
  private readonly mainScreen: HTMLElement;
  private readonly emptySlot: HTMLElement;
  private readonly warningSlot: HTMLElement;
  private readonly actionBar: HTMLElement;
  private readonly downloadButton: HTMLButtonElement;
  private readonly statusLine: HTMLElement;
  private readonly versionBadge: HTMLElement;

  private readonly urlInput: UrlInput;
  private readonly dependencyPanel: DependencyStatus;
  private readonly mediaCard: MediaInfoCard;
  private readonly formatSelector: FormatSelector;
  private readonly audioSelector: AudioSelector;
  private readonly subtitleSelector: SubtitleSelector;
  private readonly optionsPanel: DownloadOptionsPanel;
  private readonly collectionView: CollectionView;
  private readonly downloadList: DownloadList;
  private readonly commandPreview: CommandPreview;
  private readonly errorPanel: ErrorPanel;
  private readonly updateBanner: UpdateBanner;

  private settingsPanel: SettingsPanel | null = null;
  private settingsHost: HTMLElement;
  private cookieBrowsers: string[] = [];
  private installGuidance: InstallGuidance[] = [];
  private updateStatus: UpdateStatus | null = null;
  private toolSources: ToolSource[] = [];
  private previewTimer = 0;

  constructor(host: HTMLElement) {
    this.root = host;

    this.versionBadge = el("span", { className: "brand-version" });

    this.urlInput = new UrlInput((url) => void this.analyze(url));
    this.dependencyPanel = new DependencyStatus({
      onLocate: (name) => void this.locateDependency(name),
      onRetry: () => void this.refreshDependencies(),
      onOpenURL: (url) => void api.openURL(url),
      onInstallTool: (name) => void this.installTool(name),
      onUpdateTool: (name) => void this.installTool(name),
    });
    this.mediaCard = new MediaInfoCard();
    this.formatSelector = new FormatSelector((patch) => this.changeSelection(patch));
    this.audioSelector = new AudioSelector((patch) => this.changeSelection(patch));
    this.subtitleSelector = new SubtitleSelector((patch) => this.changeSelection(patch));
    this.optionsPanel = new DownloadOptionsPanel(
      (patch) => this.changeSelection(patch),
      () => void this.browseOutputDirectory(),
      () => void api.openOutputDirectory(),
    );
    this.collectionView = new CollectionView((patch) => this.changeSelection(patch));
    this.downloadList = new DownloadList({
      onCancel: (id) => void api.cancelDownload(id),
      onOpenFile: (path) => void this.openFile(path),
      onOpenFolder: (path) => void this.openFolder(path),
      onClearFinished: () => void this.clearFinished(),
      onDownloadAnother: () => this.resetForNext(),
    });
    this.commandPreview = new CommandPreview((open) => this.changeSelection({ showAdvanced: open }));
    this.errorPanel = new ErrorPanel(() => this.showSettings());
    this.updateBanner = new UpdateBanner({
      onDownload: () => void this.downloadUpdate(),
      onInstall: () => void this.installUpdate(),
      onSkip: (version) => void this.skipUpdate(version),
      onOpenNotes: () => void api.openReleasePage(),
      onDismiss: () => this.updateBanner.update(this.updateStatus),
    });

    this.warningSlot = el("div", { className: "warning-slot" });
    this.emptySlot = el("div", { className: "empty-slot" });

    this.statusLine = el("p", { className: "action-status" });
    this.downloadButton = el("button", {
      className: "button primary large",
      text: "Download",
      attrs: { type: "button", disabled: true },
      on: { click: () => void this.startDownload() },
    });
    this.actionBar = el("div", { className: "action-bar" }, [this.statusLine, this.downloadButton]);
    this.actionBar.hidden = true;

    this.mainScreen = el("div", { className: "screen" }, [
      this.updateBanner.element,
      this.urlInput.element,
      this.dependencyPanel.element,
      this.errorPanel.element,
      this.warningSlot,
      this.mediaCard.element,
      this.collectionView.element,
      this.formatSelector.element,
      this.audioSelector.element,
      this.subtitleSelector.element,
      this.optionsPanel.element,
      this.actionBar,
      this.commandPreview.element,
      this.downloadList.element,
      this.emptySlot,
    ]);

    this.settingsHost = el("div", { className: "screen" });
    this.settingsHost.hidden = true;

    replace(this.root, [this.header(), el("main", { className: "content" }, [this.mainScreen, this.settingsHost])]);

    this.store.subscribe((state) => this.render(state));
  }

  /** start loads the initial state and subscribes to backend events. */
  async start(): Promise<void> {
    api.onProgress((progress) => this.store.patchProgress(progress.downloadId, progress));
    api.onDownloadState((view) => {
      this.store.upsertDownload(view);
      const state = this.store.get();
      if (view.status === "failed" && !state.error) {
        this.store.set({ phase: "failed" });
      }
      if (view.status === "completed") {
        this.store.set({ phase: "completed" });
      }
    });

    api.onToolProgress((progress) => {
      this.dependencyPanel.setToolProgress(progress);
      this.settingsPanel?.setToolProgress(progress);
    });
    api.onUpdateAvailable((status) => this.applyUpdateStatus(status));
    api.onUpdateProgress((progress) => this.updateBanner.setProgress(progress));

    const [appInfo, settingsResponse, browsers, downloads, guidance, toolSources] = await Promise.all([
      api.getAppInfo(),
      api.getSettings(),
      api.getCookieBrowsers(),
      api.listDownloads(),
      api.getInstallGuidance(),
      api.getToolSources(),
    ]);

    this.cookieBrowsers = browsers;
    this.installGuidance = guidance;
    this.dependencyPanel.setGuidance(guidance);
    this.toolSources = toolSources;
    this.dependencyPanel.setToolSources(toolSources);
    this.versionBadge.textContent = appInfo.versionLabel;
    this.applySettings(settingsResponse.settings);

    this.store.set({
      appInfo,
      settings: settingsResponse.settings,
      dependencies: settingsResponse.dependencies,
      downloads,
      selection: {
        ...this.selectionFromPreferences(settingsResponse.settings),
        outputDirectory: settingsResponse.settings.outputDirectory,
      },
    });

    // The dependency probe runs at startup, but re-reading it here covers a
    // slow first detection.
    const dependencies = await api.getDependencies();
    this.store.set({ dependencies });

    this.urlInput.focus();

    // The status of a check that already ran during startup.
    this.applyUpdateStatus(await api.getUpdateStatus());
  }

  private header(): HTMLElement {
    return el("header", { className: "app-header" }, [
      el("div", { className: "brand" }, [
        // The same download mark as the application icon, drawn inline so the
        // header needs no image asset.
        el("span", {
          className: "brand-mark",
          html: `<svg viewBox="0 0 100 100" width="20" height="20" aria-hidden="true">
            <path fill="currentColor" d="M43.5 20h13v32.5h17.5L50 76.5 26 52.5h17.5z" />
            <rect fill="currentColor" x="28.5" y="79" width="43" height="7.5" rx="3.75" />
          </svg>`,
        }),
        el("div", {}, [
          el("h1", { className: "brand-name" }, [el("span", { text: "GrabOne" }), this.versionBadge]),
          el("p", { className: "brand-tagline", text: "Multi-platform media downloader" }),
        ]),
      ]),
      el("button", {
        className: "button subtle icon-button",
        text: "⚙",
        title: "Settings",
        attrs: { type: "button", "aria-label": "Settings" },
        on: { click: () => this.toggleSettings() },
      }),
    ]);
  }

  // ---------------------------------------------------------------- analysis

  private async analyze(url: string): Promise<void> {
    this.store.set({
      url,
      phase: "analyzing",
      error: null,
      media: null,
      analysis: null,
      commandPreview: "",
    });

    const response = await api.analyzeURL({ url, collectionMode: "single" });
    this.applyAnalysis(response);
  }

  private applyAnalysis(response: AnalyzeResponse): void {
    if (!response.media) {
      this.store.set({ phase: "failed", error: response.error ?? null });
      return;
    }

    const media = response.media;
    const settings = this.store.get().settings;
    const selection = this.selectionForMedia(media, response, settings);

    this.store.set({
      phase: "ready",
      media,
      analysis: response,
      resolutionPresets: response.resolutionPresets ?? [],
      canMerge: response.canMerge,
      canConvertAudio: response.canConvertAudio,
      error: response.success ? null : (response.error ?? null),
      selection,
    });

    this.schedulePreview();
  }

  /** selectionForMedia merges the backend's suggested defaults with what the
   * user last chose, keeping only choices the media can actually deliver. */
  private selectionForMedia(media: MediaInfo, response: AnalyzeResponse, settings: Settings | null): Selection {
    const base = settings ? this.selectionFromPreferences(settings) : defaultSelection();
    const defaults = response.defaults;
    const capabilities = media.capabilities;

    let downloadType = defaults.downloadType || base.downloadType;

    // A collection whose items have not been resolved reports no formats yet.
    // That is not a reason to narrow the choice: what the items hold is simply
    // not known until each one is extracted.
    const formatsKnown = media.formats.length > 0 || media.entriesResolved;
    if (formatsKnown) {
      if (downloadType === "video+audio" && !(capabilities.hasVideo && capabilities.hasAudio)) {
        downloadType = capabilities.hasVideo ? "video" : "audio";
      }
      if (downloadType === "video" && !capabilities.hasVideo) downloadType = "audio";
      if (downloadType === "audio" && !capabilities.hasAudio) {
        downloadType = capabilities.hasVideo ? "video" : "audio";
      }
    }

    return {
      ...base,
      downloadType,
      useSeparateStreams: defaults.videoFormatId !== "" && defaults.combinedFormatId === "",
      combinedFormatId: defaults.combinedFormatId,
      videoFormatId: defaults.videoFormatId,
      audioFormatId: defaults.audioFormatId,
      maxHeight: 0,
      container: defaults.container || base.container,
      downloadSubtitles: base.downloadSubtitles && capabilities.hasSubtitles,
      subtitleLanguages: defaults.subtitleLanguages ?? [],
      embedChapters: base.embedChapters && capabilities.hasChapters,
      embedThumbnail: base.embedThumbnail && capabilities.hasThumbnail,
      saveThumbnail: base.saveThumbnail && capabilities.hasThumbnail,
      saveDescription: base.saveDescription && capabilities.hasDescription,
      collectionMode: media.isCollection ? "single" : "single",
      selectedEntries: [],
      outputDirectory: base.outputDirectory || settings?.outputDirectory || "",
    };
  }

  private selectionFromPreferences(settings: Settings): Selection {
    const preferences = settings.preferences;
    return {
      ...defaultSelection(),
      downloadType: preferences.downloadType || "video+audio",
      container: preferences.container || "auto",
      embedMetadata: preferences.embedMetadata,
      embedChapters: preferences.embedChapters,
      embedThumbnail: preferences.embedThumbnail,
      saveThumbnail: preferences.saveThumbnail,
      saveDescription: preferences.saveDescription,
      saveJson: preferences.saveJson,
      downloadSubtitles: preferences.downloadSubtitles,
      embedSubtitles: preferences.embedSubtitles,
      keepSubtitles: preferences.keepSubtitles,
      subtitleFormat: preferences.subtitleFormat || "srt",
      subtitleLanguages: [],
      audioConversionFormat: preferences.audioConversionFormat || "original",
      outputDirectory: settings.outputDirectory,
    };
  }

  // -------------------------------------------------------------- selection

  private changeSelection(patch: Partial<Selection>): void {
    this.store.setSelection(patch);
    this.schedulePreview();
  }

  /** schedulePreview refreshes the command preview, coalescing rapid changes. */
  private schedulePreview(): void {
    window.clearTimeout(this.previewTimer);
    this.previewTimer = window.setTimeout(() => void this.refreshPreview(), 120);
  }

  private async refreshPreview(): Promise<void> {
    const state = this.store.get();
    if (!state.media) return;

    const response = await api.previewCommand(this.buildRequest(state));
    if (response.success) {
      this.store.set({ commandPreview: response.command });
      return;
    }
    this.store.set({ commandPreview: response.error?.message ?? "" });
  }

  /** buildRequest turns the current selection into a backend request. */
  private buildRequest(state: AppState): DownloadRequest {
    const media = state.media;
    const selection = state.selection;

    const useCombined = this.usesCombinedStream(state);

    const collectionMode = media?.isCollection ? selection.collectionMode : "single";

    // Subtitles belong to a video file. An audio-only download hides the panel,
    // so its switches must not be sent either.
    const wantsSubtitles = selection.downloadType !== "audio" && selection.downloadSubtitles;

    return {
      url: media?.requestedUrl ?? state.url,
      title: media?.title ?? "",
      downloadType: selection.downloadType,
      videoFormatId: selection.downloadType === "audio" ? "" : useCombined ? "" : selection.videoFormatId,
      audioFormatId:
        selection.downloadType === "video" || useCombined ? "" : selection.audioFormatId,
      combinedFormatId: selection.downloadType === "audio" || !useCombined ? "" : selection.combinedFormatId,
      maxHeight: selection.maxHeight,
      container: selection.container,
      embedMetadata: selection.embedMetadata,
      embedChapters: selection.embedChapters,
      embedThumbnail: selection.embedThumbnail,
      saveThumbnail: selection.saveThumbnail,
      saveDescription: selection.saveDescription,
      saveJson: selection.saveJson,
      downloadSubtitles: wantsSubtitles,
      embedSubtitles: selection.embedSubtitles,
      keepSubtitles: selection.keepSubtitles,
      subtitleLanguages: wantsSubtitles ? selection.subtitleLanguages : [],
      subtitleFormat: selection.subtitleFormat,
      audioConversionFormat: selection.audioConversionFormat,
      outputDirectory: selection.outputDirectory,
      collectionMode,
      playlistItems: collectionMode === "selected" ? selection.selectedEntries.join(",") : "",
    };
  }

  private usesCombinedStream(state: AppState): boolean {
    const media = state.media;
    if (!media) return true;

    const hasCombined = media.formats.some((format) => format.kind === "combined");
    const hasVideoOnly = media.formats.some((format) => format.kind === "video");

    if (state.selection.downloadType === "audio") return false;
    if (state.selection.downloadType === "video") return !hasVideoOnly && hasCombined;
    if (!hasVideoOnly) return hasCombined;
    if (!hasCombined) return false;
    return !state.selection.useSeparateStreams;
  }

  // -------------------------------------------------------------- downloads

  private async startDownload(): Promise<void> {
    const state = this.store.get();
    if (!state.media) return;

    this.downloadButton.disabled = true;
    const response = await api.startDownload(this.buildRequest(state));

    if (!response.success || !response.download) {
      this.store.set({ error: response.error ?? null, phase: "failed" });
      return;
    }

    // The job may already have started and reported itself through an event
    // while this call was in flight. Inserting the reply's snapshot then would
    // put the card back to "queued" and leave it there.
    const known = this.store.get().downloads.some((job) => job.id === response.download!.id);
    if (!known) this.store.upsertDownload(response.download);

    this.store.set({ phase: "downloading", error: null });

    // Persisting the options may have changed the stored settings.
    const settings = await api.getSettings();
    this.store.set({ settings: settings.settings });
  }

  private async clearFinished(): Promise<void> {
    const downloads = await api.clearFinishedDownloads();
    this.store.set({ downloads });
  }

  private async openFile(path: string): Promise<void> {
    try {
      await api.openFile(path);
    } catch (error) {
      this.store.set({ notice: String(error) });
    }
  }

  private async openFolder(path: string): Promise<void> {
    try {
      await api.openContainingFolder(path);
    } catch (error) {
      this.store.set({ notice: String(error) });
    }
  }

  private resetForNext(): void {
    this.store.set({ phase: "idle", url: "", media: null, analysis: null, error: null, commandPreview: "" });
    this.urlInput.focus();
  }

  /**
   * installTool downloads a missing dependency on request.
   *
   * The panel is left showing the failure when it does not work, because the
   * instructions for installing it by hand are right beside it.
   */
  private async installTool(name: string): Promise<void> {
    const response = await api.installTool(name);

    this.dependencyPanel.setToolProgress(null);
    this.settingsPanel?.setToolProgress(null);
    this.store.set({ dependencies: response.dependencies });

    if (response.error) {
      this.store.set({ notice: response.error });
      this.dependencyPanel.setToolError(name, response.error);
      this.settingsPanel?.setToolError(name, response.error);
    }
  }

  // ---------------------------------------------------------------- updates

  private applyUpdateStatus(status: UpdateStatus | null): void {
    this.updateStatus = status;
    this.updateBanner.update(status);
    if (this.settingsPanel) this.settingsPanel.setUpdateStatus(status);
  }

  private async checkForUpdate(): Promise<void> {
    this.updateBanner.reset();
    this.applyUpdateStatus(await api.checkForUpdate());
  }

  private async downloadUpdate(): Promise<void> {
    // Render the downloading state before the call, which only returns when the
    // download has finished.
    if (this.updateStatus) this.applyUpdateStatus({ ...this.updateStatus, downloading: true });
    this.applyUpdateStatus(await api.downloadUpdate());
  }

  private async installUpdate(): Promise<void> {
    try {
      await api.installUpdate();
    } catch (error) {
      this.applyUpdateStatus({ ...(this.updateStatus as UpdateStatus), error: String(error) });
    }
  }

  private async skipUpdate(version: string): Promise<void> {
    this.applyUpdateStatus(await api.skipUpdateVersion(version));
  }

  // --------------------------------------------------------------- settings

  private toggleSettings(): void {
    const state = this.store.get();
    this.store.set({ screen: state.screen === "settings" ? "main" : "settings" });
  }

  private showSettings(): void {
    this.store.set({ screen: "settings" });
  }

  private async refreshDependencies(): Promise<void> {
    const dependencies = await api.refreshDependencies();
    this.store.set({ dependencies });
  }

  private async locateDependency(name: string): Promise<void> {
    const response = await api.locateDependency(name);
    this.store.set({ settings: response.settings, dependencies: response.dependencies });
    if (response.error) this.store.set({ notice: response.error });
  }

  private async saveSettings(settings: Settings): Promise<void> {
    const response = await api.saveSettings(settings);
    this.applySettings(response.settings);
    this.store.set({ settings: response.settings, dependencies: response.dependencies });

    // The output folder is also part of the current selection.
    this.store.setSelection({ outputDirectory: response.settings.outputDirectory });
    if (response.error) this.store.set({ notice: response.error });
  }

  private async browseOutputDirectory(): Promise<void> {
    const chosen = await api.chooseOutputDirectory();
    if (!chosen) return;

    this.store.setSelection({ outputDirectory: chosen });
    const settings = this.store.get().settings;
    if (settings) await this.saveSettings({ ...settings, outputDirectory: chosen });
  }

  private async browseCookieFile(): Promise<void> {
    const chosen = await api.chooseCookieFile();
    if (!chosen) return;

    const settings = this.store.get().settings;
    if (settings) await this.saveSettings({ ...settings, cookieFile: chosen, cookieSource: "file" });
  }

  private applySettings(settings: Settings): void {
    document.documentElement.dataset.theme = settings.theme === "light" ? "light" : "dark";
  }

  // ----------------------------------------------------------------- render

  private render(state: AppState): void {
    const onSettings = state.screen === "settings";
    this.mainScreen.hidden = onSettings;
    this.settingsHost.hidden = !onSettings;

    if (onSettings) {
      if (!this.settingsPanel) {
        this.settingsPanel = new SettingsPanel(
          {
            onSave: (settings) => void this.saveSettings(settings),
            onLocate: (name) => void this.locateDependency(name),
            onRetry: () => void this.refreshDependencies(),
            onBrowseOutput: () => void this.browseOutputDirectory(),
            onBrowseCookieFile: () => void this.browseCookieFile(),
            onOpenLogs: () => void api.openLogDirectory(),
            onOpenURL: (url) => void api.openURL(url),
            onInstallTool: (name) => void this.installTool(name),
            onUpdateTool: (name) => void this.installTool(name),
            onCheckUpdate: () => void this.checkForUpdate(),
            onDownloadUpdate: () => void this.downloadUpdate(),
            onInstallUpdate: () => void this.installUpdate(),
            onClose: () => this.store.set({ screen: "main" }),
          },
          this.cookieBrowsers,
          this.installGuidance,
          this.toolSources,
        );
        replace(this.settingsHost, [this.settingsPanel.element]);
        this.settingsPanel.setUpdateStatus(this.updateStatus);
      }
      this.settingsPanel.update(state);
      return;
    }

    this.urlInput.update(state);

    // The dependency panel only takes space on the main screen when something
    // is actually wrong; it always lives in settings.
    const dependenciesReady = state.dependencies?.ytDlp.available === true;
    const dependencyTrouble =
      detected(state.dependencies) && (!dependenciesReady || !state.dependencies!.ffmpeg.available);
    this.dependencyPanel.element.hidden = !dependencyTrouble;
    if (dependencyTrouble) this.dependencyPanel.update(state.dependencies);

    this.errorPanel.update(state.error);
    replace(this.warningSlot, [state.media ? warningList(state.media.warnings) : null]);

    this.mediaCard.update(state.media);
    this.collectionView.update(state);
    this.formatSelector.update(state);
    this.audioSelector.update(state);
    this.subtitleSelector.update(state);
    this.optionsPanel.update(state);
    this.commandPreview.update(state);
    this.downloadList.update(state);

    replace(this.emptySlot, [
      !state.media && state.phase !== "analyzing" && state.downloads.length === 0 && !state.error
        ? emptyState(dependenciesReady)
        : null,
    ]);

    this.renderActionBar(state, dependenciesReady);
  }

  private renderActionBar(state: AppState, dependenciesReady: boolean): void {
    if (!state.media) {
      this.actionBar.hidden = true;
      return;
    }
    this.actionBar.hidden = false;

    const problem = this.blockingProblem(state, dependenciesReady);
    this.downloadButton.disabled = problem !== null;
    this.statusLine.textContent = problem ?? this.readyLine(state);
    this.statusLine.className = problem ? "action-status warning" : "action-status";
  }

  /** blockingProblem returns why downloading is unavailable, or null when ready. */
  private blockingProblem(state: AppState, dependenciesReady: boolean): string | null {
    if (!dependenciesReady) return "yt-dlp is required before anything can be downloaded.";

    const selection = state.selection;
    if (selection.downloadSubtitles && selection.subtitleLanguages.length === 0) {
      return "Choose at least one subtitle language, or turn subtitles off.";
    }
    if (
      state.media?.isCollection &&
      selection.collectionMode === "selected" &&
      selection.selectedEntries.length === 0
    ) {
      return "Choose at least one item from the collection.";
    }
    if (!selection.outputDirectory) return "Choose a folder to save into.";
    return null;
  }

  private readyLine(state: AppState): string {
    const resolved = state.analysis?.defaults.resolvedContainer;
    const selection = state.selection;

    // Starting a download while one is running is allowed: it joins the queue.
    if (isBusy(state)) {
      const running = activeDownloads(state).length;
      const limit = state.settings?.maxConcurrentDownloads ?? 1;
      if (running >= limit) {
        return `${running === 1 ? "A download is" : `${running} downloads are`} in progress. This one will join the queue.`;
      }
    }

    if (selection.downloadType === "audio") {
      const target =
        selection.audioConversionFormat === "original"
          ? "the source audio stream"
          : `${selection.audioConversionFormat.toUpperCase()} audio`;
      return `Ready to save ${target}.`;
    }

    const container =
      selection.container === "auto"
        ? (resolved ? resolved.toUpperCase() : "the best container")
        : selection.container.toUpperCase();
    return `Ready to save as ${container}.`;
  }
}
