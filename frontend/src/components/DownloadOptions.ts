import { el, replace } from "../dom";
import { plural } from "../format";
import type { AppState, Selection } from "../state";

const CONTAINERS: { value: string; label: string }[] = [
  { value: "auto", label: "Automatic" },
  { value: "mkv", label: "MKV" },
  { value: "mp4", label: "MP4" },
  { value: "webm", label: "WebM" },
];

/**
 * DownloadOptionsPanel holds the extras: metadata, chapters, thumbnail, the
 * final container and the output folder.
 *
 * Only options that make sense for the analyzed media are shown: chapters are
 * hidden when the extractor reported none, and the container choice disappears
 * for an audio-only download, where the audio pipeline decides the file type.
 */
export class DownloadOptionsPanel {
  readonly element: HTMLElement;

  private signature = "";

  constructor(
    private readonly onChange: (patch: Partial<Selection>) => void,
    private readonly onBrowse: () => void,
    private readonly onOpenOutput: () => void,
  ) {
    this.element = el("section", { className: "panel options-panel" });
    this.element.hidden = true;
  }

  update(state: AppState): void {
    const media = state.media;
    if (!media) {
      this.element.hidden = true;
      this.signature = "";
      return;
    }
    this.element.hidden = false;

    const selection = state.selection;
    const signature = [
      media.id,
      selection.downloadType,
      selection.container,
      selection.embedMetadata,
      selection.embedChapters,
      selection.embedThumbnail,
      selection.saveThumbnail,
      selection.saveDescription,
      selection.saveJson,
      selection.outputDirectory,
      state.analysis?.defaults.resolvedContainer,
      state.canMerge,
    ].join("|");
    if (signature === this.signature) return;
    this.signature = signature;

    const capabilities = media.capabilities;
    const audioOnly = selection.downloadType === "audio";

    const optionBoxes: HTMLElement[] = [
      this.checkbox("Embed metadata", selection.embedMetadata, (checked) =>
        this.onChange({ embedMetadata: checked }),
      ),
    ];

    if (capabilities.hasChapters) {
      optionBoxes.push(
        this.checkbox(
          `Embed chapters (${plural(media.chapters.length, "chapter")})`,
          selection.embedChapters,
          (checked) => this.onChange({ embedChapters: checked }),
        ),
      );
    }
    if (capabilities.hasThumbnail) {
      optionBoxes.push(
        this.checkbox("Embed thumbnail", selection.embedThumbnail, (checked) =>
          this.onChange({ embedThumbnail: checked }),
        ),
        this.checkbox("Save thumbnail separately", selection.saveThumbnail, (checked) =>
          this.onChange({ saveThumbnail: checked }),
        ),
      );
    }
    if (capabilities.hasDescription) {
      optionBoxes.push(
        this.checkbox("Save description", selection.saveDescription, (checked) =>
          this.onChange({ saveDescription: checked }),
        ),
      );
    }
    optionBoxes.push(
      this.checkbox("Save JSON metadata", selection.saveJson, (checked) => this.onChange({ saveJson: checked })),
    );

    const children: (HTMLElement | false | null)[] = [
      el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Options" }),
        el("div", { className: "checkbox-grid" }, optionBoxes),
        !capabilities.hasChapters
          ? el("p", { className: "hint", text: "No chapters were reported for this media." })
          : null,
        !state.canMerge
          ? el("p", { className: "hint warning", text: "Embedding options need FFmpeg." })
          : null,
      ]),
    ];

    if (!audioOnly) {
      const resolved = state.analysis?.defaults.resolvedContainer ?? "";
      children.push(
        el("div", { className: "field" }, [
          el("span", { className: "field-label", text: "Final container" }),
          el(
            "div",
            { className: "radio-row" },
            CONTAINERS.map((container) =>
              this.radio(
                "container",
                container.value === "auto" && resolved
                  ? `Automatic (${resolved.toUpperCase()})`
                  : container.label,
                selection.container === container.value,
                () => this.onChange({ container: container.value }),
              ),
            ),
          ),
          el("p", {
            className: "hint",
            text: "Changing the container remuxes the streams. The video is never re-encoded for this.",
          }),
        ]),
      );
    }

    children.push(
      el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Save to" }),
        el("div", { className: "path-row" }, [
          el("input", {
            className: "path-field",
            attrs: {
              type: "text",
              readonly: "",
              value: selection.outputDirectory,
              "aria-label": "Output folder",
            },
            title: selection.outputDirectory,
          }),
          el("button", {
            className: "button subtle",
            text: "Browse",
            attrs: { type: "button" },
            on: { click: () => this.onBrowse() },
          }),
          el("button", {
            className: "button subtle",
            text: "Open",
            attrs: { type: "button" },
            on: { click: () => this.onOpenOutput() },
          }),
        ]),
      ]),
    );

    replace(this.element, children);
  }

  private checkbox(label: string, checked: boolean, onToggle: (checked: boolean) => void): HTMLElement {
    const input = el("input", {
      attrs: { type: "checkbox", checked: checked ? "" : null },
      on: { change: (event) => onToggle((event.currentTarget as HTMLInputElement).checked) },
    });
    return el("label", { className: "checkbox" }, [input, el("span", { text: label })]);
  }

  private radio(name: string, label: string, checked: boolean, onSelect: () => void): HTMLElement {
    const input = el("input", {
      attrs: { type: "radio", name, checked: checked ? "" : null },
      on: { change: () => onSelect() },
    });
    return el("label", { className: "radio" }, [input, el("span", { text: label })]);
  }
}
