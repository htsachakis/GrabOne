import { el, replace } from "../dom";
import type { EngineError } from "../types";

/**
 * errorBlock renders a classified failure: a plain explanation, what to do about
 * it, and the raw output underneath.
 *
 * The underlying error is never hidden. Translating it into a readable sentence
 * is an addition to the technical details, not a replacement for them.
 */
export function errorBlock(error: EngineError): HTMLElement {
  const authRelated = error.kind === "auth-required" || error.kind === "private" || error.kind === "age-restricted";

  return el("div", { className: `error-block ${error.kind}` }, [
    el("div", { className: "error-head" }, [
      el("span", { className: "error-mark", text: authRelated ? "🔒" : "⚠", attrs: { "aria-hidden": "true" } }),
      el("strong", { text: error.message }),
    ]),
    !!error.hint && el("p", { className: "error-hint", text: error.hint }),
    !!error.details &&
      el("details", { className: "advanced-details" }, [
        el("summary", { text: "Technical details" }),
        el("pre", { className: "error-details", text: error.details }),
      ]),
  ]);
}

/** ErrorPanel is the standalone panel used for analysis failures. */
export class ErrorPanel {
  readonly element: HTMLElement;

  constructor(private readonly onOpenSettings: () => void) {
    this.element = el("section", { className: "panel error-panel" });
    this.element.hidden = true;
  }

  update(error: EngineError | null): void {
    if (!error) {
      this.element.hidden = true;
      return;
    }
    this.element.hidden = false;

    const offersSettings =
      error.kind === "missing-dependency" ||
      error.kind === "auth-required" ||
      error.kind === "private" ||
      error.kind === "age-restricted";

    replace(this.element, [
      errorBlock(error),
      offersSettings
        ? el("div", { className: "download-actions" }, [
            el("button", {
              className: "button subtle",
              text: "Open settings",
              attrs: { type: "button" },
              on: { click: () => this.onOpenSettings() },
            }),
          ])
        : null,
    ]);
  }
}

/** emptyState renders the placeholder shown before anything is analyzed. */
export function emptyState(dependenciesReady: boolean): HTMLElement {
  return el("section", { className: "panel empty-state" }, [
    el("h2", { className: "empty-title", text: "Nothing analyzed yet" }),
    el("p", {
      className: "hint",
      text: dependenciesReady
        ? "Paste a link above and GrabOne will show every stream, subtitle and chapter the site offers."
        : "Set up yt-dlp to get started. GrabOne drives it rather than bundling it.",
    }),
  ]);
}
