/**
 * Types mirroring the Go models exposed by the backend.
 *
 * They are written by hand rather than imported from the generated bindings so
 * the interface type-checks without a code generation step, and so the shape the
 * frontend relies on is stated explicitly in one place.
 */

export type FormatKind = "combined" | "video" | "audio" | "other";

export interface MediaFormat {
  formatId: string;
  extension: string;
  width: number;
  height: number;
  fps: number;
  videoCodec: string;
  audioCodec: string;
  rawVideoCodec: string;
  rawAudioCodec: string;
  videoBitrate: number;
  audioBitrate: number;
  totalBitrate: number;
  fileSize: number;
  fileSizeApprox: number;
  hasVideo: boolean;
  hasAudio: boolean;
  dynamicRange: string;
  protocol: string;
  formatNote: string;
  language: string;
  kind: FormatKind;
  qualityLabel: string;
  resolution: string;
  label: string;
  sizeLabel: string;
  recommended: boolean;
}

export interface SubtitleLanguage {
  code: string;
  name: string;
  automatic: boolean;
  formats: string[];
}

export interface Chapter {
  title: string;
  startTime: number;
  endTime: number;
}

export interface MediaEntry {
  index: number;
  id: string;
  title: string;
  thumbnailUrl: string;
  duration: number;
  durationText: string;
  webpageUrl: string;
  contentType: string;
  formats: MediaFormat[];
  resolved: boolean;
  unavailable: boolean;
  message: string;
}

export interface MediaCapabilities {
  hasVideo: boolean;
  hasAudio: boolean;
  hasCombined: boolean;
  hasSeparateVideo: boolean;
  hasSeparateAudio: boolean;
  hasSubtitles: boolean;
  hasAutomaticCaptions: boolean;
  hasChapters: boolean;
  hasThumbnail: boolean;
  hasDescription: boolean;
  hasMultipleFormats: boolean;
  isCollection: boolean;
  isLive: boolean;
}

export interface MediaInfo {
  platform: string;
  platformSlug: string;
  extractor: string;
  extractorKey: string;
  id: string;
  title: string;
  uploader: string;
  channel: string;
  description: string;
  duration: number;
  durationText: string;
  thumbnailUrl: string;
  webpageUrl: string;
  requestedUrl: string;
  contentType: string;
  uploadDate: string;
  viewCount: number;
  liveStatus: string;
  formats: MediaFormat[];
  subtitles: SubtitleLanguage[];
  chapters: Chapter[];
  isCollection: boolean;
  collectionTitle: string;
  entries: MediaEntry[];
  entriesResolved: boolean;
  capabilities: MediaCapabilities;
  warnings: string[];
}

export interface ResolutionPreset {
  label: string;
  height: number;
  available: boolean;
}

export type ErrorKind =
  | "missing-dependency"
  | "invalid-url"
  | "unsupported-url"
  | "unavailable"
  | "private"
  | "auth-required"
  | "age-restricted"
  | "geo-blocked"
  | "format-unavailable"
  | "subtitle-unavailable"
  | "network"
  | "disk-full"
  | "cancelled"
  | "timeout"
  | "unknown";

export interface EngineError {
  kind: ErrorKind;
  message: string;
  hint?: string;
  details?: string;
}

export interface DependencyStatus {
  name: string;
  displayName: string;
  available: boolean;
  path: string;
  version: string;
  source: string;
  error?: string;
  required: boolean;
  managed: boolean;
}

export interface DependencySet {
  ytDlp: DependencyStatus;
  ffmpeg: DependencyStatus;
  ffprobe: DependencyStatus;
}

export interface InstallStep {
  title: string;
  command?: string;
  url?: string;
  detail: string;
}

export interface InstallGuidance {
  name: string;
  displayName: string;
  summary: string;
  steps: InstallStep[];
  afterInstall: string;
}

export interface ToolSource {
  name: string;
  displayName: string;
  projectUrl: string;
  licence: string;
  provides: string[];
}

export interface ToolProgress {
  tool: string;
  stage: "looking-up" | "downloading" | "extracting" | "finished";
  downloadedBytes: number;
  totalBytes: number;
  percent: number;
  message: string;
}

export interface ToolInstallResponse {
  success: boolean;
  tool: string;
  version: string;
  directory: string;
  dependencies: DependencySet;
  error?: string;
}

export interface AppInfo {
  name: string;
  version: string;
  versionLabel: string;
  description: string;
  legalNotice: string;
  applicationDir: string;
  settingsPath: string;
  logDirectory: string;
}

export type DownloadType = "video+audio" | "video" | "audio";
export type CollectionMode = "single" | "selected" | "all";
export type CookieSource = "none" | "browser" | "file";

export interface Preferences {
  downloadType: string;
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
  subtitleFormat: string;
  subtitleLanguages: string[];
  audioConversionFormat: string;
}

export interface Settings {
  ytDlpPath: string;
  ffmpegPath: string;
  ffprobePath: string;
  outputDirectory: string;
  filenameTemplate: string;
  theme: string;
  maxConcurrentDownloads: number;
  autoCheckUpdates: boolean;
  skippedUpdateVersion: string;
  cookieSource: string;
  cookieBrowser: string;
  cookieFile: string;
  preferences: Preferences;
}

export interface SelectionDefaults {
  downloadType: string;
  combinedFormatId: string;
  videoFormatId: string;
  audioFormatId: string;
  container: string;
  resolvedContainer: string;
  subtitleLanguages: string[];
}

export interface AnalyzeRequest {
  url: string;
  collectionMode: string;
}

export interface AnalyzeResponse {
  success: boolean;
  media?: MediaInfo;
  error?: EngineError;
  resolutionPresets: ResolutionPreset[];
  defaults: SelectionDefaults;
  canMerge: boolean;
  canConvertAudio: boolean;
}

export interface DownloadRequest {
  url: string;
  title: string;
  downloadType: string;
  videoFormatId: string;
  audioFormatId: string;
  combinedFormatId: string;
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
  playlistItems: string;
}

export interface PreviewResponse {
  success: boolean;
  command: string;
  args: string[];
  error?: EngineError;
  resolvedContainer: string;
}

export interface DownloadProgress {
  downloadId: string;
  status: string;
  stage: string;
  completedStages: string[];
  percent: number;
  overallPercent: number;
  downloadedBytes: number;
  totalBytes: number;
  totalIsEstimate: boolean;
  speedBytesPerSecond: number;
  etaSeconds: number;
  filename: string;
  streamLabel: string;
  itemIndex: number;
  itemCount: number;
  message: string;
}

export interface DownloadMetadata {
  title: string;
  uploader: string;
  platform: string;
  platformSlug: string;
  thumbnailUrl: string;
  contentType: string;
  durationText: string;
}

export interface FileSummary {
  path: string;
  container: string;
  duration: number;
  sizeBytes: number;
  videoCodec: string;
  audioCodec: string;
  width: number;
  height: number;
  subtitleTracks: number;
  chapterCount: number;
}

export type DownloadStatus = "queued" | "running" | "completed" | "failed" | "cancelled";

export interface DownloadView {
  id: string;
  url: string;
  metadata: DownloadMetadata;
  status: DownloadStatus;
  stage: string;
  completedStages: string[];
  progress: DownloadProgress;
  downloadType: string;
  outputDirectory: string;
  finalPath: string;
  command: string;
  error?: EngineError;
  fileSummary?: FileSummary;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  queuePosition: number;
}

export interface StartResponse {
  success: boolean;
  download?: DownloadView;
  error?: EngineError;
}

export interface UpdateAsset {
  name: string;
  url: string;
  size: number;
}

export interface UpdateRelease {
  version: string;
  tagName: string;
  name: string;
  notes: string;
  publishedAt: string;
  pageUrl: string;
  installer?: UpdateAsset;
  checksums?: UpdateAsset;
}

export interface UpdateStatus {
  checked: boolean;
  available: boolean;
  currentVersion: string;
  latestVersion?: string;
  release?: UpdateRelease;
  downloading: boolean;
  downloadedPath?: string;
  skipped: boolean;
  checkedAt?: string;
  error?: string;
}

export interface UpdateProgress {
  downloadedBytes: number;
  totalBytes: number;
  percent: number;
}

export interface SettingsResponse {
  settings: Settings;
  dependencies: DependencySet;
  error?: string;
}
