import { el, replace } from "../dom";
import { formatBytes, formatEta, formatPercent, formatSize, formatSpeed, stageLabel } from "../format";
import { groupDownloads, type AppState } from "../state";
import type { DownloadView } from "../types";
import { errorBlock } from "./ErrorPanel";

export interface DownloadListHandlers {
  onCancel: (id: string) => void;
  /** onMove puts a waiting job at a queue position, counted from 1. */
  onMove: (id: string, position: number) => void;
  onOpenFile: (path: string) => void;
  onOpenFolder: (path: string) => void;
  onClearFinished: () => void;
  onDownloadAnother: () => void;
}

/**
 * DownloadList shows every job of the session: what stage it is at, how far it
 * has come, and what to do with the finished file.
 *
 * The jobs are grouped: what is downloading, then the queue in the order it
 * will start, then what has finished. Up on the screen means sooner.
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

    const groups = groupDownloads(state);
    const focused = this.focusedMove();

    replace(this.list, [
      ...this.group("Downloading", groups.running.map((job) => this.jobCard(job))),
      ...this.group(
        "Up next",
        groups.waiting.map((job, index) => this.jobCard(job, index + 1, groups.waiting.length)),
      ),
      ...this.group("Finished", groups.finished.map((job) => this.jobCard(job))),
    ]);

    if (focused) this.restoreFocus(focused.job, focused.move);
  }

  /** group puts a heading above a set of cards, and nothing when there are none. */
  private group(title: string, cards: HTMLElement[]): HTMLElement[] {
    if (cards.length === 0) return [];
    return [el("h3", { className: "download-group-title", text: title }), ...cards];
  }

  /** focusedMove reports which move button holds the keyboard focus, if one does. */
  private focusedMove(): { job: string; move: string } | null {
    const active = document.activeElement;
    if (!(active instanceof HTMLElement) || !this.list.contains(active)) return null;

    const { job, move } = active.dataset;
    return job && move ? { job, move } : null;
  }

  /**
   * restoreFocus returns the focus to a job's move button after the list was
   * rebuilt, so the same key press can move the job again. "Start next" is gone
   * once the job is at the front, and the up button takes the focus then.
   */
  private restoreFocus(job: string, move: string): void {
    const buttons = Array.from(this.list.querySelectorAll<HTMLButtonElement>("button[data-move]")).filter(
      (button) => button.dataset.job === job,
    );
    const find = (wanted: string) => buttons.find((button) => button.dataset.move === wanted);
    (find(move) ?? find("up"))?.focus();
  }

  /** moveButtons are the controls that change a waiting job's queue position. */
  private moveButtons(job: DownloadView, position: number, waiting: number): (HTMLElement | false)[] {
    // A button with nowhere to go is marked aria-disabled, not disabled, so it
    // can keep the focus. Pressing up until the job reaches the front then ends
    // on a button that does nothing, not on the one that moves it back down.
    const button = (move: string, text: string, label: string, target: number, usable: boolean): HTMLElement =>
      el("button", {
        className: "button subtle",
        text,
        title: label,
        attrs: { type: "button", "aria-label": label, "aria-disabled": usable ? null : "true" },
        dataset: { job: job.id, move },
        on: {
          click: () => {
            if (usable) this.handlers.onMove(job.id, target);
          },
        },
      });

    // "Start next" comes first so the arrows stay in the same place on every
    // card, whether or not it is shown.
    return [
      el("span", { className: "download-actions-gap" }),
      position > 1 && button("front", "Start next", "Start next", 1, true),
      button("up", "↑", "Move up", position - 1, position > 1),
      button("down", "↓", "Move down", position + 1, position < waiting),
    ];
  }

  /**
   * jobCard draws one job. A waiting job is given its queue position and the
   * number of waiting jobs, which decide the move buttons it shows.
   */
  private jobCard(job: DownloadView, position = 0, waiting = 0): HTMLElement {
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
          text: position > 0 ? `Waiting in the queue at position ${position}.` : "Waiting to start.",
        }),
        // A job that has not started yet can still be taken out of the queue,
        // or moved within it when there is another job to move past.
        el("div", { className: "download-actions" }, [
          el("button", {
            className: "button subtle",
            text: "Cancel",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onCancel(job.id) },
          }),
          ...(waiting > 1 ? this.moveButtons(job, position, waiting) : []),
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
      if (job.plainRetry) {
        // The stage line moves on with the retry's own progress, so the fact
        // that this is a second attempt is kept on a line of its own.
        body.push(el("p", { className: "hint", text: "Retrying without speed settings." }));
      }
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

    if (job.aria2cMissing && !job.plainRetry && job.status !== "cancelled") {
      body.push(
        el("p", {
          className: "hint",
          text:
            job.status === "queued" || job.status === "running"
              ? "aria2c was not found, so this download runs without it."
              : "aria2c was not found, so this download ran without it.",
        }),
      );
    }

    if (job.plainRetry && (job.status === "completed" || job.status === "failed")) {
      body.push(
        el("p", {
          className: "hint",
          text:
            job.status === "completed"
              ? "The first attempt failed, so this was downloaded without the speed settings. Your settings were not changed."
              : "The first attempt failed, and so did a second one without the speed settings.",
        }),
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
