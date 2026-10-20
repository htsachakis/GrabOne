import { readClipboardText } from "../api";
import { el } from "../dom";
import type { AppState } from "../state";

/**
 * UrlInput is the entry point of the application: a field for any media link
 * and the action that analyzes it. While an analysis runs it can be cancelled,
 * since a site can keep yt-dlp waiting far longer than anyone wants to.
 *
 * No link is rejected on the basis of its domain. Anything that looks like an
 * HTTP address is handed to yt-dlp, which decides whether it can extract it.
 */
export class UrlInput {
  readonly element: HTMLElement;

  private readonly input: HTMLInputElement;
  private readonly pasteButton: HTMLButtonElement;
  private readonly button: HTMLButtonElement;
  private readonly spinner: HTMLElement;
  private readonly cancelButton: HTMLButtonElement;
  /** The state URL the field last followed. */
  private shownUrl = "";

  constructor(
    private readonly onAnalyze: (url: string) => void,
    private readonly onCancel: () => void,
  ) {
    this.input = el("input", {
      className: "url-field",
      attrs: {
        type: "text",
        placeholder: "Paste a media URL",
        spellcheck: "false",
        autocomplete: "off",
        "aria-label": "Media URL",
      },
      on: {
        keydown: (event) => {
          const key = (event as KeyboardEvent).key;
          if (key === "Enter") this.submit();
          if (key === "Escape" && !this.cancelButton.hidden) this.onCancel();
        },
        input: () => this.syncButton(),
        paste: () => {
          // Let the paste land before reading the value.
          window.setTimeout(() => this.syncButton(), 0);
        },
      },
    });

    this.spinner = el("span", { className: "spinner", attrs: { "aria-hidden": "true" } });
    this.spinner.hidden = true;

    this.pasteButton = el("button", {
      className: "button",
      text: "Paste",
      title: "Paste the link from the clipboard",
      attrs: { type: "button" },
      on: { click: () => void this.paste() },
    });

    this.button = el(
      "button",
      {
        className: "button primary analyze-button",
        attrs: { type: "button", disabled: true },
        on: { click: () => this.submit() },
      },
      [this.spinner, el("span", { text: "Analyze" })],
    );

    this.cancelButton = el("button", {
      className: "button subtle",
      text: "Cancel",
      title: "Stop analyzing this link",
      attrs: { type: "button" },
      on: { click: () => this.onCancel() },
    });
    this.cancelButton.hidden = true;

    this.element = el("section", { className: "panel url-panel" }, [
      el("label", { className: "field-label", text: "Paste a media URL", attrs: { for: "url-field" } }),
      el("div", { className: "url-row" }, [this.input, this.pasteButton, this.button, this.cancelButton]),
      el("p", {
        className: "hint",
        text: "YouTube, Instagram, TikTok, Facebook, X, Vimeo and every other site yt-dlp supports.",
      }),
    ]);

    this.input.id = "url-field";
  }

  /** focus puts the caret in the field, used on first paint. */
  focus(): void {
    this.input.focus();
  }

  /** value returns the trimmed link. */
  value(): string {
    return this.input.value.trim();
  }

  update(state: AppState): void {
    const analyzing = state.phase === "analyzing";
    this.spinner.hidden = !analyzing;
    this.button.classList.toggle("busy", analyzing);
    this.input.readOnly = analyzing;
    this.pasteButton.disabled = analyzing;
    this.cancelButton.hidden = !analyzing;

    if (state.url !== this.shownUrl) {
      // Keep the field in step when the URL was set elsewhere, for example by
      // starting another download of the same media. Only a change of URL
      // does this, so a link typed but not yet analyzed survives re-renders.
      this.shownUrl = state.url;
      if (document.activeElement !== this.input) this.input.value = state.url;
    }
    this.syncButton(analyzing);
  }

  private syncButton(analyzing = false): void {
    this.button.disabled = analyzing || this.input.value.trim().length === 0;
  }

  /** paste fills the field with the clipboard text and leaves the choice to
   * analyze it to the user. An empty or unreadable clipboard changes nothing. */
  private async paste(): Promise<void> {
    const text = await readClipboardText();
    if (!text || this.input.readOnly) return;
    this.input.value = text;
    this.syncButton();
    this.input.focus();
  }

  private submit(): void {
    const url = this.value();
    if (!url || this.button.disabled) return;
    this.onAnalyze(url);
  }
}
