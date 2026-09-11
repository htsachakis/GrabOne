/**
 * Application state and a minimal store.
 *
 * The interface is driven entirely by this state: every control reads its
 * enabled state and its value from here, which is what keeps the phases
 * (idle, analyzing, ready, downloading) consistent across the window.
 */

import type {
  AnalyzeResponse,
  AppInfo,
  DependencySet,
  DownloadView,
  EngineError,
  MediaInfo,
  ResolutionPreset,
  Settings,
} from "./types";

export type Phase = "idle" | "analyzing" | "ready" | "downloading" | "completed" | "failed";
export type Screen = "main" | "settings";

/** Selection holds every choice the user can make about a download. */
export interface Selection {
  downloadType: string;

  /** useSeparateStreams picks between a combined stream and a video+audio pair. */
  useSeparateStreams: boolean;
  combinedFormatId: string;
  videoFormatId: string;
  audioFormatId: string;

  /** maxHeight is the resolution preset filter; 0 means "Best". */
  maxHeight: number;

  container: string;

  embedMetadata: boolean;
  embedChapters: boolean;
  embedThumbnail: boolean;
  saveThumbnail: boolean;
  saveDescription: boolean;
  saveJson: boolean;

  downloadSubtitles: boolean;
  embedSubtitles: boolean;
  keepSubtitles: boolean;
  subtitleLanguages: string[];
  subtitleFormat: string;

  audioConversionFormat: string;

  outputDirectory: string;

  collectionMode: string;
  /** selectedEntries holds the 1-based indexes chosen from a collection. */
  selectedEntries: number[];

  showAdvanced: boolean;
}

export interface AppState {
  phase: Phase;
  screen: Screen;

  appInfo: AppInfo | null;
  dependencies: DependencySet | null;
  settings: Settings | null;

  url: string;
  media: MediaInfo | null;
  analysis: AnalyzeResponse | null;
  resolutionPresets: ResolutionPreset[];
  canMerge: boolean;
  canConvertAudio: boolean;

  error: EngineError | null;
  notice: string | null;

  selection: Selection;

  downloads: DownloadView[];
  commandPreview: string;
}

export function defaultSelection(): Selection {
  return {
    downloadType: "video+audio",
    useSeparateStreams: false,
    combinedFormatId: "",
    videoFormatId: "",
    audioFormatId: "",
    maxHeight: 0,
    container: "auto",
    embedMetadata: true,
    embedChapters: true,
    embedThumbnail: false,
    saveThumbnail: false,
    saveDescription: false,
    saveJson: false,
    downloadSubtitles: false,
    embedSubtitles: true,
    keepSubtitles: false,
    subtitleLanguages: [],
    subtitleFormat: "srt",
    audioConversionFormat: "original",
    outputDirectory: "",
    collectionMode: "single",
    selectedEntries: [],
    showAdvanced: false,
  };
}

export function initialState(): AppState {
  return {
    phase: "idle",
    screen: "main",
    appInfo: null,
    dependencies: null,
    settings: null,
    url: "",
    media: null,
    analysis: null,
    resolutionPresets: [],
    canMerge: false,
    canConvertAudio: false,
    error: null,
    notice: null,
    selection: defaultSelection(),
    downloads: [],
    commandPreview: "",
  };
}

/**
 * lifecycleRank orders the states a job passes through, so a snapshot can be
 * recognised as describing an earlier moment than what is already known.
 */
function lifecycleRank(status: DownloadView["status"]): number {
  switch (status) {
    case "queued":
      return 0;
    case "running":
      return 1;
    default:
      return 2;
  }
}

type Listener = (state: AppState) => void;

/** Store holds the state and notifies subscribers after every change. */
export class Store {
  private state: AppState = initialState();
  private listeners: Listener[] = [];

  get(): AppState {
    return this.state;
  }

  subscribe(listener: Listener): void {
    this.listeners.push(listener);
  }

  /** set merges a partial update and notifies subscribers. */
  set(update: Partial<AppState>): void {
    this.state = { ...this.state, ...update };
    this.notify();
  }

  /** setSelection merges into the selection without replacing the rest. */
  setSelection(update: Partial<Selection>): void {
    this.state = { ...this.state, selection: { ...this.state.selection, ...update } };
    this.notify();
  }

  /**
   * upsertDownload inserts or replaces a job, keeping newest first.
   *
   * A snapshot that describes an earlier point in the job's life is ignored.
   * Events and call replies race each other, and a reply that was prepared
   * before the job started must not put a running download back to "queued".
   */
  upsertDownload(view: DownloadView): void {
    const existing = this.state.downloads.findIndex((job) => job.id === view.id);
    const downloads = [...this.state.downloads];

    if (existing >= 0) {
      if (lifecycleRank(view.status) < lifecycleRank(downloads[existing].status)) return;
      downloads[existing] = view;
    } else {
      downloads.unshift(view);
    }
    this.set({ downloads });
  }

  /**
   * patchProgress applies a progress event to the matching job.
   *
   * Progress carries the job's own status, so this also repairs a status that
   * a missed or out-of-order state event left behind.
   */
  patchProgress(downloadId: string, progress: DownloadView["progress"]): void {
    const downloads = this.state.downloads.map((job) => {
      if (job.id !== downloadId) return job;

      const status = (progress.status as DownloadView["status"]) || job.status;
      const advanced = lifecycleRank(status) >= lifecycleRank(job.status);
      return {
        ...job,
        progress,
        stage: progress.stage,
        status: advanced ? status : job.status,
        completedStages: progress.completedStages ?? job.completedStages,
      };
    });
    this.set({ downloads });
  }

  private notify(): void {
    for (const listener of this.listeners) listener(this.state);
  }
}

/** activeDownloads returns the jobs that are queued or running. */
export function activeDownloads(state: AppState): DownloadView[] {
  return state.downloads.filter((job) => job.status === "queued" || job.status === "running");
}

/** isBusy reports whether a download is in flight. */
export function isBusy(state: AppState): boolean {
  return activeDownloads(state).length > 0;
}
