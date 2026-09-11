import { el, replace } from "../dom";
import { formatBytes, formatEta, formatPercent, formatSize, formatSpeed, stageLabel } from "../format";
import type { AppState } from "../state";
import type { DownloadView } from "../types";
import { errorBlock } from "./ErrorPanel";

export interface DownloadListHandlers {
  onCancel: (id: string) => void;
  onOpenFile: (path: string) => void;
  onOpenFolder: (path: string) => void;
  onClearFinished: () => void;
  onDownloadAnother: () => void;
}

/**
 * DownloadList shows every job of the session: what stage it is at, how far it
 * has come, and what to do with the finished file.
 *
 * Progress comes from yt-dlp's machine readable output. When a site reports no
 * size, the bar falls back to what is known instead of inventing a percentage.
 */
export class DownloadList {
  readonly element: HTMLElement;

  private readonly list: HTMLElement;

  constructor(private readonly handlers: DownloadListHandlers) {
    this.list = el("div", { className: "download-list" });

    this.element = el("section", { className: "panel downloads-panel" }, [
      el("div", { className: "panel-header" }, [
        el("h2", { className: "panel-title", text: "Downloads" }),
        el("button", {
          className: "button subtle small",
          text: "Clear finished",
          attrs: { type: "button" },
          on: { click: () => this.handlers.onClearFinished() },
        }),
      ]),
      this.list,
    ]);
    this.element.hidden = true;
  }

  update(state: AppState): void {
    if (state.downloads.length === 0) {
      this.element.hidden = true;
      return;
    }
    this.element.hidden = false;

    replace(
      this.list,
      state.downloads.map((job) => this.jobCard(job)),
    );
  }

  private jobCard(job: DownloadView): HTMLElement {
    const progress = job.progress;
    const running = job.status === "running";
    const percent = progress.overallPercent || progress.percent;

    const header = el("div", { className: "download-header" }, [
      job.metadata.thumbnailUrl
        ? el("img", {
            className: "download-thumb",
            attrs: { src: job.metadata.thumbnailUrl, alt: "", loading: "lazy", referrerpolicy: "no-referrer" },
          })
        : el("span", { className: "download-thumb empty" }),
      el("div", { className: "download-title-block" }, [
        el("span", { className: "download-title", text: job.metadata.title, title: job.metadata.title }),
        el("span", { className: "download-subtitle", text: this.subtitle(job) }),
      ]),
      el("span", { className: `status-pill ${job.status}`, text: this.statusText(job) }),
    ]);

    const body: (HTMLElement | false | null)[] = [header];

    if (job.status === "queued") {
      body.push(
        el("p", {
          className: "hint",
          text: job.queuePosition > 0 ? `Waiting in the queue at position ${job.queuePosition}.` : "Waiting to start.",
        }),
        // A job that has not started yet can still be taken out of the queue.
        el("div", { className: "download-actions" }, [
          el("button", {
            className: "button subtle",
            text: "Cancel",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onCancel(job.id) },
          }),
        ]),
      );
    }

    if (running || job.status === "completed") {
      body.push(
        el("div", { className: "progress-track" }, [
          el("div", {
            className: `progress-fill ${percent > 0 ? "" : "indeterminate"}`,
            attrs: { style: `width: ${Math.min(100, Math.max(0, percent))}%` },
          }),
        ]),
      );
    }

    if (running) {
      const stats: (HTMLElement | false)[] = [
        el("span", { className: "progress-percent", text: formatPercent(percent) }),
        progress.totalBytes > 0 &&
          el("span", {
            text: `${formatBytes(progress.downloadedBytes)} / ${formatSize(
              progress.totalBytes,
              progress.totalIsEstimate,
            )}`,
          }),
        progress.speedBytesPerSecond > 0 && el("span", { text: formatSpeed(progress.speedBytesPerSecond) }),
        progress.etaSeconds > 0 && el("span", { text: formatEta(progress.etaSeconds) }),
        progress.itemCount > 1 &&
          el("span", { text: `Item ${progress.itemIndex} of ${progress.itemCount}` }),
      ];
      body.push(el("div", { className: "progress-stats" }, stats));
      body.push(
        el("div", { className: "stage-line" }, [
          el("span", { text: progress.message || stageLabel(job.stage) }),
          this.stageTicks(job),
        ]),
      );
      body.push(
        el("div", { className: "download-actions" }, [
          el("button", {
            className: "button subtle",
            text: "Cancel",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onCancel(job.id) },
          }),
        ]),
      );
    }

    if (job.status === "completed") {
      body.push(
        el("div", { className: "completed-line" }, [
          el("span", { className: "ok-mark", text: "✓" }),
          el("span", { text: "Download completed" }),
          !!job.finalPath && el("span", { className: "final-path", text: job.finalPath, title: job.finalPath }),
        ]),
      );
      if (job.fileSummary) {
        const summary = job.fileSummary;
        const parts = [
          summary.container?.split(",")[0]?.toUpperCase(),
          summary.width > 0 ? `${summary.width} × ${summary.height}` : "",
          summary.videoCodec,
          summary.audioCodec,
          summary.subtitleTracks > 0 ? `${summary.subtitleTracks} subtitle tracks` : "",
          summary.chapterCount > 0 ? `${summary.chapterCount} chapters` : "",
          formatBytes(summary.sizeBytes),
        ].filter(Boolean);
        body.push(el("p", { className: "hint", text: `FFprobe reports: ${parts.join(" • ")}` }));
      }
      body.push(
        el("div", { className: "download-actions" }, [
          !!job.finalPath &&
            el("button", {
              className: "button primary",
              text: "Open file",
              attrs: { type: "button" },
              on: { click: () => this.handlers.onOpenFile(job.finalPath) },
            }),
          el("button", {
            className: "button subtle",
            text: "Open folder",
            attrs: { type: "button" },
            on: {
              click: () =>
                job.finalPath
                  ? this.handlers.onOpenFolder(job.finalPath)
                  : this.handlers.onOpenFolder(job.outputDirectory),
            },
          }),
          el("button", {
            className: "button subtle",
            text: "Download another",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onDownloadAnother() },
          }),
        ]),
      );
    }

    if (job.error && job.status !== "cancelled") {
      body.push(errorBlock(job.error));
    }
    if (job.status === "cancelled") {
      body.push(el("p", { className: "hint", text: "Cancelled. Partial files were left in the output folder." }));
    }

    if (job.command) {
      body.push(
        el("details", { className: "advanced-details" }, [
          el("summary", { text: "Effective command" }),
          el("pre", { className: "command-block", text: job.command }),
        ]),
      );
    }

    return el("article", { className: `download-card ${job.status}` }, body);
  }

  /** stageTicks shows the parts of a multi-stage download that are done. */
  private stageTicks(job: DownloadView): HTMLElement {
    const interesting = (job.completedStages ?? []).filter((stage) => stage.startsWith("downloading"));
    return el(
      "span",
      { className: "stage-ticks" },
      interesting.map((stage) => el("span", { className: "tick", text: `${stageLabel(stage)} ✓` })),
    );
  }

  private subtitle(job: DownloadView): string {
    const parts = [job.metadata.platform, job.metadata.contentType, job.metadata.uploader].filter(Boolean);
    return parts.join(" • ");
  }

  private statusText(job: DownloadView): string {
    switch (job.status) {
      case "queued":
        return "Queued";
      case "running":
        return stageLabel(job.stage);
      case "completed":
        return "Completed";
      case "failed":
        return "Failed";
      case "cancelled":
        return "Cancelled";
      default:
        return job.status;
    }
  }
}
