/** Formatting helpers for sizes, durations, speeds and countdowns. */

const BYTE_UNITS = ["B", "KB", "MB", "GB", "TB"];

/** formatBytes renders a byte count with the precision reduced as units grow. */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "";

  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024;
    unit += 1;
  }

  if (unit === 0) return `${Math.round(value)} ${BYTE_UNITS[unit]}`;
  if (value >= 100) return `${value.toFixed(0)} ${BYTE_UNITS[unit]}`;
  if (value >= 10) return `${value.toFixed(1)} ${BYTE_UNITS[unit]}`;
  return `${value.toFixed(2)} ${BYTE_UNITS[unit]}`;
}

/** formatSize renders a size, marking an estimate with a leading tilde. */
export function formatSize(bytes: number, approximate = false): string {
  const text = formatBytes(bytes);
  if (!text) return "Size unknown";
  return approximate ? `~${text}` : text;
}

/** formatDuration renders seconds as mm:ss, or hh:mm:ss when an hour or more. */
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "";

  const total = Math.round(seconds);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;

  const pad = (value: number) => value.toString().padStart(2, "0");
  return hours > 0 ? `${pad(hours)}:${pad(minutes)}:${pad(secs)}` : `${pad(minutes)}:${pad(secs)}`;
}

/** formatSpeed renders a transfer rate given in bytes per second. */
export function formatSpeed(bytesPerSecond: number): string {
  const text = formatBytes(bytesPerSecond);
  return text ? `${text}/s` : "";
}

/** formatEta renders a countdown, or an empty string when unknown. */
export function formatEta(seconds: number): string {
  const text = formatDuration(seconds);
  return text ? `ETA ${text}` : "";
}

/** formatPercent renders a whole percentage. */
export function formatPercent(percent: number): string {
  if (!Number.isFinite(percent) || percent <= 0) return "0%";
  return `${Math.min(100, Math.round(percent))}%`;
}

/** formatBitrate renders a bitrate given in kbit/s. */
export function formatBitrate(kbits: number): string {
  if (!Number.isFinite(kbits) || kbits <= 0) return "";
  return kbits >= 1000 ? `${(kbits / 1000).toFixed(1)} Mbps` : `${Math.round(kbits)} kbps`;
}

/** plural renders a count with a noun, pluralised by adding an s. */
export function plural(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

/** stageLabel turns a backend stage identifier into a readable line. */
export function stageLabel(stage: string): string {
  const labels: Record<string, string> = {
    starting: "Starting",
    analyzing: "Reading the page",
    downloading: "Downloading",
    "downloading-video": "Downloading video",
    "downloading-audio": "Downloading audio",
    merging: "Merging with FFmpeg",
    remuxing: "Remuxing container",
    converting: "Converting with FFmpeg",
    "embedding-metadata": "Embedding metadata",
    "embedding-subtitles": "Embedding subtitles",
    "embedding-thumbnail": "Embedding thumbnail",
    "post-processing": "Post-processing",
    finished: "Completed",
    failed: "Failed",
    cancelled: "Cancelled",
  };
  return labels[stage] ?? stage;
}

/** truncate shortens text for a single line of interface. */
export function truncate(text: string, limit: number): string {
  if (text.length <= limit) return text;
  return `${text.slice(0, Math.max(0, limit - 1))}…`;
}
