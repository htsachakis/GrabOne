import { el, replace } from "../dom";
import { formatBytes, formatPercent } from "../format";
import type {
  DependencySet,
  DependencyStatus as Status,
  InstallGuidance,
  ToolProgress,
  ToolSource,
} from "../types";
import { installGuide } from "./InstallGuide";

/**
 * detected reports whether the backend has actually probed the executables. A
 * zero-valued set arrives when the interface loads before the first probe has
 * finished.
 */
export function detected(dependencies: DependencySet | null): boolean {
  return !!dependencies && dependencies.ytDlp.name !== "";
}

export interface DependencyStatusHandlers {
  onLocate: (name: string) => void;
  onRetry: () => void;
  onOpenURL: (url: string) => void;
  onInstallTool: (name: string) => void;
  onUpdateTool: (name: string) => void;
}

/**
 * DependencyStatus shows whether yt-dlp, FFmpeg and FFprobe were found, with
 * their versions and where they came from.
 *
 * Availability is reported by the backend only after running each executable,
 * so a present but unusable binary shows as unavailable rather than as ready.
 */
export class DependencyStatus {
  readonly element: HTMLElement;

  private readonly list: HTMLElement;
  private readonly summary: HTMLElement;
  private guidance: InstallGuidance[] = [];
  private lastSet: DependencySet | null = null;
  private toolSources: ToolSource[] = [];
  private toolProgress: Record<string, ToolProgress> = {};
  private toolErrors: Record<string, string> = {};

  constructor(
    private readonly handlers: DependencyStatusHandlers,
    private readonly detailed = false,
  ) {
    this.list = el("div", { className: "dependency-list" });
    this.summary = el("p", { className: "hint" });

    this.element = el("section", { className: "panel dependency-panel" }, [
      el("div", { className: "panel-header" }, [
        el("h2", { className: "panel-title", text: "Dependencies" }),
        el("button", {
          className: "button subtle",
          text: "Retry",
          attrs: { type: "button" },
          on: { click: () => this.handlers.onRetry() },
        }),
      ]),
      this.list,
      this.summary,
    ]);
  }

  /** setGuidance supplies the install help, which arrives once at startup. */
  setGuidance(guidance: InstallGuidance[]): void {
    this.guidance = guidance;
    if (this.lastSet) this.update(this.lastSet);
  }

  /** setToolSources says which tools GrabOne can download for the user. */
  setToolSources(sources: ToolSource[]): void {
    this.toolSources = sources;
    if (this.lastSet) this.update(this.lastSet);
  }

  /** setToolProgress renders the progress of a download in place. */
  setToolProgress(progress: ToolProgress | null): void {
    if (!progress) {
      this.toolProgress = {};
    } else if (progress.stage === "looking-up") {
      this.toolErrors = {};
      this.toolProgress[progress.tool] = progress;
    } else {
      // One download can supply several dependencies, so it is shown against
      // every one it provides.
      const source = this.toolSources.find((entry) => entry.name === progress.tool);
      for (const name of source?.provides ?? [progress.tool]) {
        this.toolProgress[name] = progress;
      }
    }
    if (this.lastSet) this.update(this.lastSet);
  }

  /** setToolError records why a download did not work, shown on the row. */
  setToolError(name: string, message: string): void {
    const source = this.toolSources.find((entry) => entry.name === name);
    for (const provided of source?.provides ?? [name]) {
      this.toolErrors[provided] = message;
    }
    if (this.lastSet) this.update(this.lastSet);
  }

  /** downloadableSource returns the tool download that supplies a dependency. */
  private downloadableSource(name: string): ToolSource | undefined {
    return this.toolSources.find((source) => source.provides.includes(name));
  }

  update(dependencies: DependencySet | null): void {
    this.lastSet = dependencies;
    // An unprobed set carries no names. Rendering it would claim the tools are
    // missing when they have simply not been looked for yet.
    if (!dependencies || !detected(dependencies)) {
      replace(this.list, [el("p", { className: "hint", text: "Checking for yt-dlp and FFmpeg…" })]);
      this.summary.textContent = "";
      return;
    }

    const statuses = [dependencies.ytDlp, dependencies.ffmpeg, dependencies.ffprobe];
    replace(
      this.list,
      statuses.map((status) => this.row(status)),
    );

    if (!dependencies.ytDlp.available) {
      this.summary.textContent =
        "yt-dlp is required. Install it, place yt-dlp.exe next to GrabOne, or point GrabOne at it.";
      this.summary.className = "hint warning";
      return;
    }
    if (!dependencies.ffmpeg.available) {
      this.summary.textContent =
        "FFmpeg is missing. Merging, remuxing, audio conversion and embedding are unavailable without it.";
      this.summary.className = "hint warning";
      return;
    }
    this.summary.textContent = "yt-dlp, FFmpeg and FFprobe are updated independently of GrabOne.";
    this.summary.className = "hint";
  }

  private row(status: Status): HTMLElement {
    const marker = status.available
      ? el("span", { className: "status-marker ok", text: "✓", attrs: { "aria-hidden": "true" } })
      : el("span", {
          className: `status-marker ${status.required ? "bad" : "warn"}`,
          text: status.required ? "✕" : "⚠",
          attrs: { "aria-hidden": "true" },
        });

    const details: (HTMLElement | false)[] = [
      el("span", { className: "dependency-name", text: status.displayName }),
      el("span", {
        className: "dependency-version",
        text: status.available ? status.version || "version unknown" : "not found",
      }),
    ];

    const meta: (HTMLElement | false | null)[] = [];
    if (this.detailed) {
      meta.push(
        el("div", { className: "dependency-path", text: status.path || "—", title: status.path }),
        status.available
          ? el("div", { className: "dependency-source", text: `found via ${status.source}` })
          : null,
      );
    }
    if (status.error) {
      meta.push(el("div", { className: "dependency-error", text: status.error }));
    }
    if (!status.available && !this.toolProgress[status.name]) {
      // A missing tool is only useful news if it comes with how to fix it.
      const help = this.guidance.find((entry) => entry.name === status.name);
      if (help) {
        meta.push(installGuide(help, { onOpenURL: this.handlers.onOpenURL }));
      }
    }

    const source = this.downloadableSource(status.name);
    const progress = this.toolProgress[status.name];
    const busy = !!progress && progress.stage !== "finished";

    if (busy) {
      meta.push(this.progressRow(progress));
    } else if (this.toolErrors[status.name]) {
      meta.push(el("div", { className: "dependency-error", text: this.toolErrors[status.name] }));
    }

    const actions: (HTMLElement | false)[] = [];
    if (!status.available && source && !busy) {
      // The offer only appears where the tool can actually be verified.
      actions.push(
        el("button", {
          className: "button primary small",
          text: "Download",
          title: `Downloads ${source.displayName} from its official release and verifies the published checksum`,
          attrs: { type: "button" },
          on: { click: () => this.handlers.onInstallTool(source.name) },
        }),
      );
    }
    if (status.available && status.managed && source && !busy) {
      actions.push(
        el("button", {
          className: "button subtle small",
          text: "Update",
          attrs: { type: "button" },
          on: { click: () => this.handlers.onUpdateTool(source.name) },
        }),
      );
    }
    actions.push(
      el("button", {
        className: "button subtle small",
        text: status.available ? "Change" : "Locate",
        attrs: { type: "button", disabled: busy ? "" : null },
        on: { click: () => this.handlers.onLocate(status.name) },
      }),
    );

    return el("div", { className: `dependency-row ${status.available ? "" : "unavailable"}` }, [
      el("div", { className: "dependency-main" }, [marker, ...details]),
      meta.length > 0 ? el("div", { className: "dependency-meta" }, meta) : false,
      el("div", { className: "dependency-actions" }, actions),
    ]);
  }

  /** progressRow shows what a running tool download is doing. */
  private progressRow(progress: ToolProgress): HTMLElement {
    const size =
      progress.totalBytes > 0
        ? `${formatBytes(progress.downloadedBytes)} / ${formatBytes(progress.totalBytes)}`
        : "";

    return el("div", { className: "tool-progress" }, [
      el("div", { className: "progress-track" }, [
        el("div", {
          className: `progress-fill ${progress.percent > 0 ? "" : "indeterminate"}`,
          attrs: { style: `width: ${Math.min(100, Math.max(0, progress.percent))}%` },
        }),
      ]),
      el("div", { className: "progress-stats" }, [
        el("span", { text: progress.message }),
        progress.percent > 0 ? el("span", { text: formatPercent(progress.percent) }) : false,
        size ? el("span", { text: size }) : false,
      ]),
    ]);
  }
}
