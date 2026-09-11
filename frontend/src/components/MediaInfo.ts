import { el, replace } from "../dom";
import { plural, truncate } from "../format";
import type { MediaInfo } from "../types";
import { contentTypeBadge, platformBadge } from "./PlatformBadge";

/**
 * MediaInfoCard presents the analyzed media: thumbnail, platform, title,
 * uploader and duration.
 *
 * Every field is optional, because which of them an extractor fills in varies
 * per site. A missing field is left out rather than shown as empty.
 */
export class MediaInfoCard {
  readonly element: HTMLElement;

  constructor() {
    this.element = el("section", { className: "panel media-card" });
    this.element.hidden = true;
  }

  update(media: MediaInfo | null): void {
    if (!media) {
      this.element.hidden = true;
      return;
    }
    this.element.hidden = false;

    const uploader = media.uploader || media.channel;
    const details: (HTMLElement | false)[] = [
      el("div", { className: "media-badges" }, [
        platformBadge(media.platform, media.platformSlug),
        contentTypeBadge(media.contentType),
        media.capabilities.isLive && el("span", { className: "content-badge live", text: "Live" }),
      ]),
      el("h2", { className: "media-title", text: media.title, title: media.title }),
      !!uploader && el("p", { className: "media-uploader", text: uploader }),
      el("div", { className: "media-meta" }, [
        !!media.durationText && el("span", { text: media.durationText }),
        media.isCollection && el("span", { text: plural(media.entries.length, "item") }),
        !!media.uploadDate && el("span", { text: media.uploadDate }),
        media.viewCount > 0 && el("span", { text: `${media.viewCount.toLocaleString()} views` }),
      ]),
      !!media.description &&
        el("details", { className: "media-description" }, [
          el("summary", { text: "Description" }),
          el("p", { text: truncate(media.description, 1200) }),
        ]),
    ];

    replace(this.element, [
      media.thumbnailUrl
        ? el("div", { className: "thumbnail" }, [
            el("img", {
              attrs: { src: media.thumbnailUrl, alt: "", loading: "lazy", referrerpolicy: "no-referrer" },
              on: {
                error: (event) => {
                  // A thumbnail URL can expire or be blocked; drop it quietly.
                  const image = event.currentTarget as HTMLImageElement;
                  image.closest(".thumbnail")?.classList.add("empty");
                  image.remove();
                },
              },
            }),
          ])
        : el("div", { className: "thumbnail empty" }, [el("span", { text: "No preview" })]),
      el("div", { className: "media-details" }, details),
    ]);
  }
}

/** warningList renders non-fatal messages produced during extraction. */
export function warningList(warnings: string[]): HTMLElement | null {
  if (warnings.length === 0) return null;
  return el("div", { className: "warning-list" }, [
    el("strong", { text: warnings.length === 1 ? "Warning" : "Warnings" }),
    el(
      "ul",
      {},
      warnings.map((warning) => el("li", { text: warning })),
    ),
  ]);
}
