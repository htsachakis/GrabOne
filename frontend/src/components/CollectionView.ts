import { el, replace } from "../dom";
import { plural } from "../format";
import type { AppState, Selection } from "../state";

/**
 * CollectionView handles URLs that turned out to hold several items: a playlist,
 * a profile, or an Instagram carousel.
 *
 * A collection is never downloaded wholesale by accident. The default is the
 * single item the link points at, and downloading everything is an explicit
 * choice.
 */
export class CollectionView {
  readonly element: HTMLElement;

  private signature = "";

  constructor(private readonly onChange: (patch: Partial<Selection>) => void) {
    this.element = el("section", { className: "panel collection-panel" });
    this.element.hidden = true;
  }

  update(state: AppState): void {
    const media = state.media;
    if (!media || !media.isCollection || media.entries.length === 0) {
      this.element.hidden = true;
      this.signature = "";
      return;
    }
    this.element.hidden = false;

    const selection = state.selection;
    const signature = [
      media.id,
      media.entries.length,
      selection.collectionMode,
      selection.selectedEntries.join(","),
    ].join("|");
    if (signature === this.signature) return;
    this.signature = signature;

    const entries = media.entries;
    const modes: { value: string; label: string }[] = [
      { value: "single", label: "First item only" },
      { value: "selected", label: "Select items" },
      { value: "all", label: `Entire collection (${plural(entries.length, "item")})` },
    ];

    const children: (HTMLElement | false | null)[] = [
      el("div", { className: "panel-header" }, [
        el("h2", { className: "panel-title", text: media.collectionTitle || "Collection" }),
        el("span", { className: "content-badge", text: plural(entries.length, "item") }),
      ]),
      el("p", { className: "hint", text: "This URL contains multiple items." }),
      el(
        "div",
        { className: "radio-row" },
        modes.map((mode) =>
          this.radio("collection-mode", mode.label, selection.collectionMode === mode.value, () =>
            this.onChange({
              collectionMode: mode.value,
              selectedEntries:
                mode.value === "selected" && selection.selectedEntries.length === 0
                  ? [1]
                  : selection.selectedEntries,
            }),
          ),
        ),
      ),
    ];

    if (selection.collectionMode === "selected") {
      children.push(
        el("div", { className: "entry-actions" }, [
          el("button", {
            className: "button subtle small",
            text: "Select all",
            attrs: { type: "button" },
            on: { click: () => this.onChange({ selectedEntries: entries.map((entry) => entry.index) }) },
          }),
          el("button", {
            className: "button subtle small",
            text: "Select none",
            attrs: { type: "button" },
            on: { click: () => this.onChange({ selectedEntries: [] }) },
          }),
        ]),
        el(
          "ul",
          { className: "entry-list" },
          entries.map((entry) => {
            const checked = selection.selectedEntries.includes(entry.index);
            return el("li", { className: `entry ${entry.unavailable ? "unavailable" : ""}` }, [
              el("label", { className: "checkbox entry-label" }, [
                el("input", {
                  attrs: {
                    type: "checkbox",
                    checked: checked ? "" : null,
                    disabled: entry.unavailable ? "" : null,
                  },
                  on: {
                    change: (event) => {
                      const on = (event.currentTarget as HTMLInputElement).checked;
                      const chosen = new Set(selection.selectedEntries);
                      if (on) {
                        chosen.add(entry.index);
                      } else {
                        chosen.delete(entry.index);
                      }
                      this.onChange({ selectedEntries: [...chosen].sort((a, b) => a - b) });
                    },
                  },
                }),
                entry.thumbnailUrl
                  ? el("img", {
                      className: "entry-thumb",
                      attrs: { src: entry.thumbnailUrl, alt: "", loading: "lazy", referrerpolicy: "no-referrer" },
                    })
                  : el("span", { className: "entry-thumb empty" }),
                el("span", { className: "entry-index", text: String(entry.index) }),
                el("span", { className: "entry-title", text: entry.title, title: entry.title }),
                !!entry.contentType && el("span", { className: "entry-type", text: entry.contentType }),
                !!entry.durationText && el("span", { className: "entry-duration", text: entry.durationText }),
                !!entry.message && el("span", { className: "entry-message", text: entry.message }),
              ]),
            ]);
          }),
        ),
        el("p", {
          className: "hint",
          text: media.entriesResolved
            ? "Formats were reported for every item, so one selection applies to all of them."
            : "Items are listed without their formats. Each one is extracted as it downloads, using the same options.",
        }),
      );

      if (selection.selectedEntries.length === 0) {
        children.push(el("p", { className: "hint warning", text: "Select at least one item." }));
      }
    }

    replace(this.element, children);
  }

  private radio(name: string, label: string, checked: boolean, onSelect: () => void): HTMLElement {
    const input = el("input", {
      attrs: { type: "radio", name, checked: checked ? "" : null },
      on: { change: () => onSelect() },
    });
    return el("label", { className: "radio" }, [input, el("span", { text: label })]);
  }
}
