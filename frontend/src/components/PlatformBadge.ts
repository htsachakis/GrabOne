import { el } from "../dom";

/**
 * platformBadge renders the badge shown after a successful analysis.
 *
 * It is informational only. The download pipeline is the same regardless of
 * which platform is shown, including for extractors that have no badge styling
 * of their own and fall back to their friendly extractor name.
 */
export function platformBadge(platform: string, slug: string): HTMLElement {
  const known = ["youtube", "instagram", "tiktok", "facebook", "twitter", "vimeo", "reddit", "twitch", "soundcloud", "web"];
  const styleSlug = known.includes(slug) ? slug : "generic";

  return el("span", {
    className: "platform-badge",
    text: platform || "Web",
    dataset: { platform: styleSlug },
  });
}

/** contentTypeBadge renders what the URL turned out to be, such as Reel. */
export function contentTypeBadge(contentType: string): HTMLElement {
  return el("span", { className: "content-badge", text: contentType });
}
