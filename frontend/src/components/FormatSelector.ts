import { el, replace } from "../dom";
import { formatBitrate, formatSize } from "../format";
import type { Selection } from "../state";
import type { AppState } from "../state";
import type { MediaFormat, MediaInfo } from "../types";

export type SelectionPatch = Partial<Selection>;

/**
 * FormatSelector presents the download type and the video stream choice.
 *
 * Which controls appear is decided by the capabilities reported for the analyzed
 * media, not by the platform. Media that exposes a single useful stream gets a
 * plain summary instead of selectors that would have only one option.
 */
export class FormatSelector {
  readonly element: HTMLElement;

  private signature = "";

  constructor(private readonly onChange: (patch: SelectionPatch) => void) {
    this.element = el("section", { className: "panel format-panel" });
    this.element.hidden = true;
  }

  update(state: AppState): void {
    const media = state.media;
    if (!media || media.formats.length === 0) {
      this.element.hidden = true;
      this.signature = "";
      return;
    }
    this.element.hidden = false;

    const signature = this.signatureOf(state);
    if (signature === this.signature) return;
    this.signature = signature;

    const { selection } = state;
    const capabilities = media.capabilities;

    const children: (HTMLElement | false | null)[] = [];

    const typeOptions = this.downloadTypeOptions(media);
    if (typeOptions.length > 1) {
      children.push(
        el("div", { className: "field" }, [
          el("span", { className: "field-label", text: "Download" }),
          el(
            "div",
            { className: "radio-row" },
            typeOptions.map((option) =>
              this.radio("download-type", option.label, selection.downloadType === option.value, () =>
                this.onChange({ downloadType: option.value }),
              ),
            ),
          ),
        ]),
      );
    }

    if (selection.downloadType !== "audio") {
      const combined = media.formats.filter((format) => format.kind === "combined");
      const videoOnly = media.formats.filter((format) => format.kind === "video");

      if (selection.downloadType === "video+audio" && combined.length > 0 && videoOnly.length > 0) {
        children.push(
          el("div", { className: "field" }, [
            el("span", { className: "field-label", text: "Streams" }),
            el("div", { className: "radio-row" }, [
              this.radio("stream-mode", "Combined stream", !selection.useSeparateStreams, () =>
                this.onChange({ useSeparateStreams: false }),
              ),
              this.radio("stream-mode", "Separate video and audio", selection.useSeparateStreams, () =>
                this.onChange({ useSeparateStreams: true }),
              ),
            ]),
            !state.canMerge && selection.useSeparateStreams
              ? el("p", { className: "hint warning", text: "Merging separate streams needs FFmpeg." })
              : null,
          ]),
        );
      }

      const useCombined = this.usesCombined(state);
      const candidates = useCombined ? combined : videoOnly;

      if (candidates.length > 0) {
        children.push(this.resolutionPresets(state, candidates, useCombined));
        children.push(this.formatField(state, candidates, useCombined));
      }
    }

    if (capabilities.isLive) {
      children.push(
        el("p", {
          className: "hint warning",
          text: "This is a live stream. Progress and size cannot be reported until it ends.",
        }),
      );
    }

    replace(this.element, children);
  }

  /** usesCombined reports whether the current selection targets one stream. */
  private usesCombined(state: AppState): boolean {
    const media = state.media;
    if (!media) return true;

    const hasCombined = media.formats.some((format) => format.kind === "combined");
    const hasVideoOnly = media.formats.some((format) => format.kind === "video");

    if (state.selection.downloadType === "video") return !hasVideoOnly ? hasCombined : false;
    if (!hasVideoOnly) return true;
    if (!hasCombined) return false;
    return !state.selection.useSeparateStreams;
  }

  /** downloadTypeOptions lists only the types the media can actually deliver. */
  private downloadTypeOptions(media: MediaInfo): { value: string; label: string }[] {
    const capabilities = media.capabilities;
    const options: { value: string; label: string }[] = [];

    if (capabilities.hasVideo && capabilities.hasAudio) {
      options.push({ value: "video+audio", label: "Video + Audio" });
    }
    if (capabilities.hasVideo) {
      options.push({ value: "video", label: "Video only" });
    }
    if (capabilities.hasAudio) {
      options.push({ value: "audio", label: "Audio only" });
    }
    return options;
  }

  /**
   * resolutionPresets renders the convenience filters.
   *
   * A preset is a shortcut, so choosing one also moves the selection to the best
   * stream it allows. Filtering the list while leaving a 4K stream selected
   * would download 4K after the user asked for 480p.
   */
  private resolutionPresets(state: AppState, candidates: MediaFormat[], useCombined: boolean): HTMLElement {
    return el("div", { className: "field" }, [
      el("span", { className: "field-label", text: "Resolution" }),
      el(
        "div",
        { className: "chip-row" },
        state.resolutionPresets.map((preset) =>
          el("button", {
            className: `chip ${state.selection.maxHeight === preset.height ? "active" : ""}`,
            text: preset.label,
            attrs: { type: "button", disabled: !preset.available },
            title: preset.available ? "" : "Not available for this media",
            on: {
              click: () => {
                const best = this.filterByPreset(candidates, preset.height)[0];
                const patch: SelectionPatch = { maxHeight: preset.height };
                if (best) {
                  if (useCombined) {
                    patch.combinedFormatId = best.formatId;
                  } else {
                    patch.videoFormatId = best.formatId;
                  }
                }
                this.onChange(patch);
              },
            },
          }),
        ),
      ),
    ]);
  }

  private formatField(state: AppState, candidates: MediaFormat[], useCombined: boolean): HTMLElement {
    const visible = this.filterByPreset(candidates, state.selection.maxHeight);
    const selectedId = useCombined ? state.selection.combinedFormatId : state.selection.videoFormatId;
    const current = visible.find((format) => format.formatId === selectedId) ?? visible[0];

    if (visible.length === 1 && !state.media?.capabilities.hasMultipleFormats) {
      // A single useful stream needs no selector, only a statement of what it is.
      return el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Video" }),
        el("div", { className: "single-format" }, [
          el("strong", { text: current?.resolution || current?.qualityLabel || "Single stream" }),
          el("span", { text: this.codecSummary(current) }),
        ]),
        this.advancedDetails(current, state.selection.showAdvanced),
      ]);
    }

    const select = el(
      "select",
      {
        className: "select",
        attrs: { "aria-label": "Video format" },
        on: {
          change: (event) => {
            const value = (event.currentTarget as HTMLSelectElement).value;
            this.onChange(useCombined ? { combinedFormatId: value } : { videoFormatId: value });
          },
        },
      },
      visible.map((format) =>
        el("option", {
          text: format.recommended ? `${format.label} — Recommended` : format.label,
          attrs: { value: format.formatId, selected: format.formatId === current?.formatId },
        }),
      ),
    );

    return el("div", { className: "field" }, [
      el("span", { className: "field-label", text: useCombined ? "Video + audio stream" : "Video stream" }),
      select,
      el("p", {
        className: "hint",
        text: useCombined
          ? "One file, no merging required."
          : "Downloaded separately and merged with the chosen audio stream.",
      }),
      this.advancedDetails(current, state.selection.showAdvanced),
    ]);
  }

  /** filterByPreset narrows the list to a resolution bucket, never hiding all. */
  private filterByPreset(formats: MediaFormat[], maxHeight: number): MediaFormat[] {
    if (maxHeight <= 0) return formats;
    const filtered = formats.filter((format) => {
      const edge = format.width > 0 && format.width < format.height ? format.width : format.height;
      return edge > 0 && edge <= maxHeight;
    });
    return filtered.length > 0 ? filtered : formats;
  }

  private codecSummary(format: MediaFormat | undefined): string {
    if (!format) return "";
    const parts = [format.videoCodec, format.audioCodec].filter(Boolean);
    return parts.join(" + ");
  }

  private advancedDetails(format: MediaFormat | undefined, open: boolean): HTMLElement | null {
    if (!format) return null;

    const rows: [string, string][] = [
      ["Format ID", format.formatId],
      ["Container", format.extension.toUpperCase()],
      ["Resolution", format.resolution || "—"],
      ["Frame rate", format.fps > 0 ? `${format.fps} FPS` : "—"],
      ["Video codec", format.rawVideoCodec || "none"],
      ["Audio codec", format.rawAudioCodec || "none"],
      ["Video bitrate", formatBitrate(format.videoBitrate) || "—"],
      ["Total bitrate", formatBitrate(format.totalBitrate) || "—"],
      ["Dynamic range", format.dynamicRange || "—"],
      ["Protocol", format.protocol || "—"],
      ["Size", formatSize(format.fileSize || format.fileSizeApprox, format.fileSize === 0)],
    ];

    return el("details", { className: "advanced-details", attrs: { open: open ? "" : null } }, [
      el("summary", { text: "Advanced details" }),
      el(
        "dl",
        { className: "detail-grid" },
        rows.flatMap(([label, value]) => [el("dt", { text: label }), el("dd", { text: value })]),
      ),
    ]);
  }

  private radio(name: string, label: string, checked: boolean, onSelect: () => void): HTMLElement {
    const input = el("input", {
      attrs: { type: "radio", name, checked: checked ? "" : null },
      on: { change: () => onSelect() },
    });
    return el("label", { className: "radio" }, [input, el("span", { text: label })]);
  }

  private signatureOf(state: AppState): string {
    const { selection } = state;
    return [
      state.media?.id,
      state.media?.requestedUrl,
      state.media?.formats.length,
      selection.downloadType,
      selection.useSeparateStreams,
      selection.combinedFormatId,
      selection.videoFormatId,
      selection.maxHeight,
      selection.showAdvanced,
      state.canMerge,
    ].join("|");
  }
}
