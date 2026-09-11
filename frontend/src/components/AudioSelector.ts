import { el, replace } from "../dom";
import { formatBitrate } from "../format";
import type { AppState, Selection } from "../state";
import type { MediaFormat } from "../types";

const CONVERSIONS: { value: string; label: string }[] = [
  { value: "original", label: "Original — no conversion" },
  { value: "mp3", label: "MP3 — converted with FFmpeg" },
  { value: "m4a", label: "M4A — converted with FFmpeg" },
  { value: "flac", label: "FLAC — converted with FFmpeg" },
  { value: "wav", label: "WAV — converted with FFmpeg" },
  { value: "opus", label: "Opus — converted with FFmpeg" },
];

/**
 * AudioSelector presents the audio stream choice and, for audio-only downloads,
 * the optional conversion.
 *
 * The distinction between keeping the source stream and converting it is stated
 * explicitly, because converting is a re-encode while keeping it is not.
 */
export class AudioSelector {
  readonly element: HTMLElement;

  private signature = "";

  constructor(private readonly onChange: (patch: Partial<Selection>) => void) {
    this.element = el("section", { className: "panel audio-panel" });
    this.element.hidden = true;
  }

  update(state: AppState): void {
    const media = state.media;
    const audioFormats = media?.formats.filter((format) => format.kind === "audio") ?? [];
    const selection = state.selection;

    const needsAudio =
      !!media &&
      (selection.downloadType === "audio" ||
        (selection.downloadType === "video+audio" && this.mergesSeparateStreams(state)));

    if (!needsAudio || audioFormats.length === 0) {
      this.element.hidden = true;
      this.signature = "";
      return;
    }
    this.element.hidden = false;

    const signature = [
      media?.id,
      audioFormats.length,
      selection.downloadType,
      selection.audioFormatId,
      selection.audioConversionFormat,
      selection.useSeparateStreams,
      state.canConvertAudio,
    ].join("|");
    if (signature === this.signature) return;
    this.signature = signature;

    const current = audioFormats.find((format) => format.formatId === selection.audioFormatId) ?? audioFormats[0];

    const children: (HTMLElement | false | null)[] = [
      audioFormats.length > 1
        ? el("div", { className: "field" }, [
            el("span", { className: "field-label", text: "Audio stream" }),
            el(
              "select",
              {
                className: "select",
                attrs: { "aria-label": "Audio format" },
                on: {
                  change: (event) =>
                    this.onChange({ audioFormatId: (event.currentTarget as HTMLSelectElement).value }),
                },
              },
              audioFormats.map((format) =>
                el("option", {
                  text: format.recommended ? `${format.label} — Recommended` : format.label,
                  attrs: { value: format.formatId, selected: format.formatId === current?.formatId },
                }),
              ),
            ),
          ])
        : el("div", { className: "field" }, [
            el("span", { className: "field-label", text: "Audio stream" }),
            el("div", { className: "single-format" }, [el("strong", { text: current?.label ?? "Source audio" })]),
          ]),
    ];

    if (selection.downloadType === "audio") {
      children.push(
        el("div", { className: "field" }, [
          el("span", { className: "field-label", text: "Convert to" }),
          el(
            "select",
            {
              className: "select",
              attrs: { "aria-label": "Audio conversion", disabled: !state.canConvertAudio },
              on: {
                change: (event) =>
                  this.onChange({ audioConversionFormat: (event.currentTarget as HTMLSelectElement).value }),
              },
            },
            CONVERSIONS.map((conversion) =>
              el("option", {
                text: conversion.label,
                attrs: {
                  value: conversion.value,
                  selected: conversion.value === selection.audioConversionFormat,
                },
              }),
            ),
          ),
          !state.canConvertAudio
            ? el("p", { className: "hint warning", text: "Conversion needs FFmpeg. The source stream is kept." })
            : el("p", {
                className: "hint",
                text:
                  selection.audioConversionFormat === "original"
                    ? "The downloaded audio stream is saved untouched."
                    : "FFmpeg re-encodes the audio into the chosen format.",
              }),
        ]),
      );
    }

    children.push(this.details(current));
    replace(this.element, children);
  }

  private mergesSeparateStreams(state: AppState): boolean {
    const media = state.media;
    if (!media) return false;
    const hasCombined = media.formats.some((format) => format.kind === "combined");
    const hasVideoOnly = media.formats.some((format) => format.kind === "video");
    if (!hasVideoOnly) return false;
    if (!hasCombined) return true;
    return state.selection.useSeparateStreams;
  }

  private details(format: MediaFormat | undefined): HTMLElement | null {
    if (!format) return null;
    const rows: [string, string][] = [
      ["Format ID", format.formatId],
      ["Container", format.extension.toUpperCase()],
      ["Codec", format.rawAudioCodec || "none"],
      ["Bitrate", formatBitrate(format.audioBitrate || format.totalBitrate) || "—"],
      ["Language", format.language || "—"],
      ["Protocol", format.protocol || "—"],
    ];
    return el("details", { className: "advanced-details" }, [
      el("summary", { text: "Advanced details" }),
      el(
        "dl",
        { className: "detail-grid" },
        rows.flatMap(([label, value]) => [el("dt", { text: label }), el("dd", { text: value })]),
      ),
    ]);
  }
}
