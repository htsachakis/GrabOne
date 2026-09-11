import { el, replace } from "../dom";
import type { AppState, Selection } from "../state";
import type { SubtitleLanguage } from "../types";

/**
 * SubtitleSelector presents subtitle languages, keeping manual subtitles and
 * machine generated captions clearly apart.
 *
 * The panel disappears entirely when the media has no subtitles, rather than
 * offering an empty list.
 */
export class SubtitleSelector {
  readonly element: HTMLElement;

  private signature = "";

  constructor(private readonly onChange: (patch: Partial<Selection>) => void) {
    this.element = el("section", { className: "panel subtitle-panel" });
    this.element.hidden = true;
  }

  update(state: AppState): void {
    const media = state.media;
    const subtitles = media?.subtitles ?? [];

    if (!media || subtitles.length === 0 || state.selection.downloadType === "audio") {
      this.element.hidden = true;
      this.signature = "";
      return;
    }
    this.element.hidden = false;

    const selection = state.selection;
    const signature = [
      media.id,
      subtitles.length,
      selection.downloadSubtitles,
      selection.embedSubtitles,
      selection.keepSubtitles,
      selection.subtitleFormat,
      selection.subtitleLanguages.join(","),
      selection.container,
    ].join("|");
    if (signature === this.signature) return;
    this.signature = signature;

    const manual = subtitles.filter((track) => !track.automatic);
    const automatic = subtitles.filter((track) => track.automatic);

    const children: (HTMLElement | false | null)[] = [
      el("div", { className: "panel-header" }, [
        el("h2", { className: "panel-title", text: "Subtitles" }),
        this.checkbox("Download subtitles", selection.downloadSubtitles, (checked) =>
          this.onChange({ downloadSubtitles: checked }),
        ),
      ]),
    ];

    if (selection.downloadSubtitles) {
      if (manual.length > 0) {
        children.push(this.languageGroup("Manual subtitles", manual, selection));
      }
      if (automatic.length > 0) {
        children.push(this.languageGroup("Automatic captions", automatic, selection));
      }

      children.push(
        el("div", { className: "field inline-field" }, [
          el("span", { className: "field-label", text: "Format" }),
          el(
            "select",
            {
              className: "select narrow",
              attrs: { "aria-label": "Subtitle format" },
              on: {
                change: (event) =>
                  this.onChange({ subtitleFormat: (event.currentTarget as HTMLSelectElement).value }),
              },
            },
            ["srt", "vtt"].map((format) =>
              el("option", {
                text: format.toUpperCase(),
                attrs: { value: format, selected: format === selection.subtitleFormat },
              }),
            ),
          ),
        ]),
        el("div", { className: "checkbox-row" }, [
          this.checkbox("Embed in the media file", selection.embedSubtitles, (checked) =>
            this.onChange({ embedSubtitles: checked }),
          ),
          this.checkbox("Also keep the subtitle file", selection.keepSubtitles, (checked) =>
            this.onChange({ keepSubtitles: checked }),
          ),
        ]),
      );

      if (selection.embedSubtitles && selection.container === "webm") {
        children.push(
          el("p", {
            className: "hint warning",
            text: "WebM only carries WebVTT subtitles. GrabOne will write them as VTT, or choose MKV instead.",
          }),
        );
      }
      if (selection.subtitleLanguages.length === 0) {
        children.push(el("p", { className: "hint warning", text: "Pick at least one language." }));
      }
    } else {
      children.push(
        el("p", {
          className: "hint",
          text: `${manual.length} manual, ${automatic.length} automatic language${
            automatic.length === 1 ? "" : "s"
          } available.`,
        }),
      );
    }

    replace(this.element, children);
  }

  private languageGroup(
    title: string,
    tracks: SubtitleLanguage[],
    selection: Selection,
  ): HTMLElement {
    return el("div", { className: "field" }, [
      el("span", { className: "field-label", text: title }),
      el(
        "div",
        { className: "language-grid" },
        tracks.map((track) =>
          this.checkbox(
            track.name || track.code,
            selection.subtitleLanguages.includes(track.code),
            (checked) => {
              const languages = new Set(selection.subtitleLanguages);
              if (checked) {
                languages.add(track.code);
              } else {
                languages.delete(track.code);
              }
              this.onChange({ subtitleLanguages: [...languages] });
            },
            track.formats.length > 0 ? `Published as ${track.formats.join(", ")}` : "",
          ),
        ),
      ),
    ]);
  }

  private checkbox(
    label: string,
    checked: boolean,
    onToggle: (checked: boolean) => void,
    title = "",
  ): HTMLElement {
    const input = el("input", {
      attrs: { type: "checkbox", checked: checked ? "" : null },
      on: { change: (event) => onToggle((event.currentTarget as HTMLInputElement).checked) },
    });
    return el("label", { className: "checkbox", title }, [input, el("span", { text: label })]);
  }
}
