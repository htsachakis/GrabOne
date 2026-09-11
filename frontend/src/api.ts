/**
 * Typed wrapper around the generated Wails bindings.
 *
 * Every call into the backend goes through here, which keeps the generated
 * bindings out of the components and gives one place to type the payloads.
 */

import * as App from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";

import type {
  AnalyzeRequest,
  AnalyzeResponse,
  AppInfo,
  DependencySet,
  DownloadProgress,
  DownloadRequest,
  DownloadView,
  InstallGuidance,
  PreviewResponse,
  Settings,
  SettingsResponse,
  StartResponse,
  ToolInstallResponse,
  ToolProgress,
  ToolSource,
  UpdateProgress,
  UpdateStatus,
} from "./types";

/** Event names emitted by the download manager. */
export const EVENT_PROGRESS = "download:progress";
export const EVENT_STATE = "download:state";

/** Event names for the update flow. */
export const EVENT_UPDATE_AVAILABLE = "update:available";
export const EVENT_UPDATE_PROGRESS = "update:progress";

/** Event name for the progress of a tool download. */
export const EVENT_TOOL_PROGRESS = "tool:progress";

export function getAppInfo(): Promise<AppInfo> {
  return App.GetAppInfo() as Promise<AppInfo>;
}

export function getDependencies(): Promise<DependencySet> {
  return App.GetDependencies() as Promise<DependencySet>;
}

export function refreshDependencies(): Promise<DependencySet> {
  return App.RefreshDependencies() as Promise<DependencySet>;
}

export function locateDependency(name: string): Promise<SettingsResponse> {
  return App.LocateDependency(name) as Promise<SettingsResponse>;
}

export function getSettings(): Promise<SettingsResponse> {
  return App.GetSettings() as Promise<SettingsResponse>;
}

export function saveSettings(settings: Settings): Promise<SettingsResponse> {
  return App.SaveSettings(settings as never) as Promise<SettingsResponse>;
}

export function chooseOutputDirectory(): Promise<string> {
  return App.ChooseOutputDirectory();
}

export function chooseCookieFile(): Promise<string> {
  return App.ChooseCookieFile();
}

export function getInstallGuidance(): Promise<InstallGuidance[]> {
  return App.GetInstallGuidance() as Promise<InstallGuidance[]>;
}

export function openURL(url: string): Promise<void> {
  return App.OpenURL(url);
}

export function getCookieBrowsers(): Promise<string[]> {
  return App.GetCookieBrowsers();
}

export function analyzeURL(request: AnalyzeRequest): Promise<AnalyzeResponse> {
  return App.AnalyzeURL(request as never) as Promise<AnalyzeResponse>;
}

export function resolveEntry(url: string): Promise<AnalyzeResponse> {
  return App.ResolveEntry(url) as Promise<AnalyzeResponse>;
}

export function previewCommand(request: DownloadRequest): Promise<PreviewResponse> {
  return App.PreviewCommand(request as never) as Promise<PreviewResponse>;
}

export function startDownload(request: DownloadRequest): Promise<StartResponse> {
  return App.StartDownload(request as never) as Promise<StartResponse>;
}

export function cancelDownload(id: string): Promise<void> {
  return App.CancelDownload(id);
}

export function listDownloads(): Promise<DownloadView[]> {
  return App.ListDownloads() as Promise<DownloadView[]>;
}

export function clearFinishedDownloads(): Promise<DownloadView[]> {
  return App.ClearFinishedDownloads() as Promise<DownloadView[]>;
}

export function openFile(path: string): Promise<void> {
  return App.OpenFile(path);
}

export function openContainingFolder(path: string): Promise<void> {
  return App.OpenContainingFolder(path);
}

export function openOutputDirectory(): Promise<void> {
  return App.OpenOutputDirectory();
}

export function openLogDirectory(): Promise<void> {
  return App.OpenLogDirectory();
}

export function getToolSources(): Promise<ToolSource[]> {
  return App.GetToolSources() as Promise<ToolSource[]>;
}

export function installTool(name: string): Promise<ToolInstallResponse> {
  return App.InstallTool(name) as Promise<ToolInstallResponse>;
}

export function updateTool(name: string): Promise<ToolInstallResponse> {
  return App.UpdateTool(name) as Promise<ToolInstallResponse>;
}

/** onToolProgress reports how a tool download is going. */
export function onToolProgress(handler: (progress: ToolProgress) => void): void {
  EventsOn(EVENT_TOOL_PROGRESS, (...args: unknown[]) => handler(args[0] as ToolProgress));
}

export function getUpdateStatus(): Promise<UpdateStatus> {
  return App.GetUpdateStatus() as Promise<UpdateStatus>;
}

export function checkForUpdate(): Promise<UpdateStatus> {
  return App.CheckForUpdate() as Promise<UpdateStatus>;
}

export function downloadUpdate(): Promise<UpdateStatus> {
  return App.DownloadUpdate() as Promise<UpdateStatus>;
}

export function installUpdate(): Promise<void> {
  return App.InstallUpdate();
}

export function skipUpdateVersion(version: string): Promise<UpdateStatus> {
  return App.SkipUpdateVersion(version) as Promise<UpdateStatus>;
}

export function openReleasePage(): Promise<void> {
  return App.OpenReleasePage();
}

/** onUpdateAvailable fires when the startup check finds a newer release. */
export function onUpdateAvailable(handler: (status: UpdateStatus) => void): void {
  EventsOn(EVENT_UPDATE_AVAILABLE, (...args: unknown[]) => handler(args[0] as UpdateStatus));
}

/** onUpdateProgress reports how far the installer download has come. */
export function onUpdateProgress(handler: (progress: UpdateProgress) => void): void {
  EventsOn(EVENT_UPDATE_PROGRESS, (...args: unknown[]) => handler(args[0] as UpdateProgress));
}

/** onProgress subscribes to per-download progress events. */
export function onProgress(handler: (progress: DownloadProgress) => void): void {
  EventsOn(EVENT_PROGRESS, (...args: unknown[]) => handler(args[0] as DownloadProgress));
}

/** onDownloadState subscribes to job state changes. */
export function onDownloadState(handler: (view: DownloadView) => void): void {
  EventsOn(EVENT_STATE, (...args: unknown[]) => handler(args[0] as DownloadView));
}
