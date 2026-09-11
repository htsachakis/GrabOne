import { el, replace } from "../dom";
import type { AppState } from "../state";

/**
 * CommandPreview shows the extractor, the chosen format identifiers and the
 * yt-dlp command the current selection produces.
 *
 * The preview is written with PowerShell quoting so it can be pasted into a
 * terminal. Internally the application always executes an argument array, never
 * a command string.
 */
export class CommandPreview {
  readonly element: HTMLElement;

  private readonly body: HTMLElement;
  private copyTimer = 0;

  constructor(private readonly onToggle: (open: boolean) => void) {
    this.body = el("div", { className: "preview-body" });

    const details = el(
      "details",
      {
        className: "panel advanced-panel",
        on: {
          toggle: (event) => this.onToggle((event.currentTarget as HTMLDetailsElement).open),
        },
      },
      [el("summary", { className: "panel-title", text: "Advanced" }), this.body],
    );

    this.element = details;
    this.element.hidden = true;
  }

  update(state: AppState): void {
    const media = state.media;
    if (!media) {
      this.element.hidden = true;
      return;
    }
    this.element.hidden = false;

    const selection = state.selection;
    const rows: [string, string][] = [
      ["Extractor", `${media.extractorKey || media.extractor} → ${media.platform}`],
      ["Content type", media.contentType],
      ["Download type", selection.downloadType],
    ];
    if (selection.combinedFormatId) rows.push(["Combined format ID", selection.combinedFormatId]);
    if (selection.videoFormatId) rows.push(["Video format ID", selection.videoFormatId]);
    if (selection.audioFormatId) rows.push(["Audio format ID", selection.audioFormatId]);
    rows.push(["Container", selection.container === "auto" ? "automatic" : selection.container]);
    if (selection.downloadSubtitles) {
      rows.push(["Subtitles", `${selection.subtitleLanguages.join(", ") || "none selected"} (${selection.subtitleFormat})`]);
    }

    const copyButton = el("button", {
      className: "button subtle small",
      text: "Copy command",
      attrs: { type: "button" },
      on: { click: () => this.copy(state.commandPreview, copyButton) },
    });

    replace(this.body, [
      el(
        "dl",
        { className: "detail-grid" },
        rows.flatMap(([label, value]) => [el("dt", { text: label }), el("dd", { text: value })]),
      ),
      el("div", { className: "preview-head" }, [
        el("span", { className: "field-label", text: "Generated command" }),
        copyButton,
      ]),
      el("pre", { className: "command-block", text: state.commandPreview || "Select a format to see the command." }),
      el("p", {
        className: "hint",
        text: "Progress reporting switches are added when the download runs and are left out here for readability.",
      }),
    ]);
  }

  private copy(command: string, button: HTMLButtonElement): void {
    if (!command) return;

    void navigator.clipboard
      .writeText(command)
      .then(() => {
        button.textContent = "Copied";
      })
      .catch(() => {
        button.textContent = "Copy failed";
      })
      .finally(() => {
        window.clearTimeout(this.copyTimer);
        this.copyTimer = window.setTimeout(() => {
          button.textContent = "Copy command";
        }, 1800);
      });
  }
}
