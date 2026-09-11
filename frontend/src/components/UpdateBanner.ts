import { el, replace } from "../dom";
import { formatBytes, formatPercent } from "../format";
import type { UpdateProgress, UpdateStatus } from "../types";

export interface UpdateBannerHandlers {
  onDownload: () => void;
  onInstall: () => void;
  onSkip: (version: string) => void;
  onOpenNotes: () => void;
  onDismiss: () => void;
}

/**
 * UpdateBanner offers a newer version when one is published.
 *
 * Nothing is installed without being asked for: the banner downloads only when
 * told to, and the installer only runs on a second, explicit confirmation.
 */
export class UpdateBanner {
  readonly element: HTMLElement;

  private progress: UpdateProgress | null = null;
  private status: UpdateStatus | null = null;
  private dismissed = false;

  constructor(private readonly handlers: UpdateBannerHandlers) {
    this.element = el("section", { className: "panel update-banner" });
    this.element.hidden = true;
  }

  /** setProgress renders the download progress as it arrives. */
  setProgress(progress: UpdateProgress): void {
    this.progress = progress;
    if (this.status) this.update(this.status);
  }

  /** reset clears a dismissal, used when the user checks again by hand. */
  reset(): void {
    this.dismissed = false;
  }

  update(status: UpdateStatus | null): void {
    this.status = status;

    const show = !!status && status.available && !status.skipped && !this.dismissed;
    if (!show || !status) {
      this.element.hidden = true;
      return;
    }
    this.element.hidden = false;

    const version = status.latestVersion ?? "";
    const notes = status.release?.notes ?? "";

    const children: (HTMLElement | false | null)[] = [
      el("div", { className: "update-head" }, [
        el("span", { className: "update-mark", text: "↑", attrs: { "aria-hidden": "true" } }),
        el("div", { className: "update-title-block" }, [
          el("strong", { text: `GrabOne ${version} is available` }),
          el("span", {
            className: "update-subtitle",
            text: `You are running ${status.currentVersion}.`,
          }),
        ]),
        el("button", {
          className: "button subtle small",
          text: "Dismiss",
          attrs: { type: "button", "aria-label": "Dismiss this update notice" },
          on: {
            click: () => {
              this.dismissed = true;
              this.handlers.onDismiss();
            },
          },
        }),
      ]),
    ];

    if (notes) {
      children.push(
        el("details", { className: "advanced-details" }, [
          el("summary", { text: "What changed" }),
          el("pre", { className: "update-notes", text: notes }),
        ]),
      );
    }

    if (status.error) {
      children.push(el("p", { className: "hint warning", text: status.error }));
    }

    if (status.downloading) {
      const percent = this.progress?.percent ?? 0;
      children.push(
        el("div", { className: "progress-track" }, [
          el("div", {
            className: `progress-fill ${percent > 0 ? "" : "indeterminate"}`,
            attrs: { style: `width: ${Math.min(100, Math.max(0, percent))}%` },
          }),
        ]),
        el("div", { className: "progress-stats" }, [
          el("span", { className: "progress-percent", text: formatPercent(percent) }),
          this.progress && this.progress.totalBytes > 0
            ? el("span", {
                text: `${formatBytes(this.progress.downloadedBytes)} / ${formatBytes(this.progress.totalBytes)}`,
              })
            : el("span", { text: "Downloading the installer…" }),
        ]),
      );
    } else if (status.downloadedPath) {
      children.push(
        el("p", {
          className: "hint",
          text: "The installer was downloaded and its checksum verified. GrabOne will close while it runs.",
        }),
        el("div", { className: "download-actions" }, [
          el("button", {
            className: "button primary",
            text: "Install and restart",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onInstall() },
          }),
          this.notesButton(status),
        ]),
      );
    } else {
      children.push(
        el("div", { className: "download-actions" }, [
          el("button", {
            className: "button primary",
            text: "Download update",
            attrs: { type: "button", disabled: !status.release?.installer },
            on: { click: () => this.handlers.onDownload() },
          }),
          this.notesButton(status),
          el("button", {
            className: "button subtle",
            text: "Skip this version",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onSkip(version) },
          }),
        ]),
        !status.release?.installer
          ? el("p", {
              className: "hint warning",
              text: "This release publishes no verifiable installer. Use the release page to update by hand.",
            })
          : null,
      );
    }

    replace(this.element, children);
  }

  private notesButton(status: UpdateStatus): HTMLElement | false {
    return (
      !!status.release?.pageUrl &&
      el("button", {
        className: "button subtle",
        text: "Release page",
        attrs: { type: "button" },
        on: { click: () => this.handlers.onOpenNotes() },
      })
    );
  }
}
