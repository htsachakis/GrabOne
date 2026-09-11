# GrabOne architecture

GrabOne is a graphical controller around three external programs. It decides
what to ask for and how to present the answer; it never extracts media itself.

```text
Frontend (TypeScript, no framework)
        │  Wails bindings (method calls)   ▲  Wails events (progress, job state)
        ▼                                  │
App service  (app.go, updates.go, tools.go)
        │
        ├── config        settings on disk
        ├── dependencies  locating and verifying the tools
        ├── downloads     job queue, cancellation, progress fan-out
        │        │
        │        ▼
        ├── ytdlp     argument building, process handling, output parsing
        │        │
        │        ▼
        │    yt-dlp.exe ──▶ ffmpeg.exe   merging, remuxing, conversion
        │                   ffprobe.exe  inspecting the finished file
        │
        └── ghrelease   one verified download path
                 ├── updater   a newer GrabOne
                 └── tooling    yt-dlp and FFmpeg themselves
```

## The rule the design follows

> The application is a user interface around yt-dlp. Anything that could be
> called extraction, scraping or site knowledge belongs to yt-dlp.

Concretely, this means the codebase contains:

- no site-specific extraction, no HTML parsing, no private APIs;
- no list of allowed domains — any HTTP or HTTPS link is handed to yt-dlp, and
  the extractor it picks is what decides whether the link is supported;
- no hardcoded format identifiers — every identifier comes from the analysis of
  the URL being downloaded;
- no parsing of human-readable output where machine-readable output exists.

The one place that knows yt-dlp's vocabulary is `internal/ytdlp`. Everything
above it works with the normalized model in `internal/media`.

## The flow

```text
URL
 ↓  ValidateURL            is it a plausible http(s) address?
 ↓  yt-dlp --dump-single-json --flat-playlist
 ↓  rawInfo                a tolerant mirror of yt-dlp's JSON
 ↓  normalizeInfo          one media model, whatever the site
 ↓  Classify / DetectCapabilities
 ↓  interface built from capabilities
 ↓  BuildDownloadArgs      validated options → argument array
 ↓  yt-dlp                 downloads, and calls FFmpeg when needed
 ↓  ProgressParser         machine-readable progress → events
 ↓  FFprobe                what the finished file actually contains
```

### 1. Analysis

`Client.Analyze` runs yt-dlp with:

| Switch | Why |
| --- | --- |
| `--dump-single-json` | Structured output. The application never parses `-F` listings |
| `--flat-playlist` | Lists a collection's items without extracting each one, so a playlist link cannot trigger a long extraction |
| `--no-playlist` | The default: a link that belongs to a playlist is analyzed as the single item it points at |
| `--ignore-config`, `--no-config-locations` | A `yt-dlp.conf` elsewhere on the system must not silently change what the application reports it will do |
| `--color no_color` | Clean output to parse |

Warnings printed to stderr are collected and shown; they are not treated as
failures.

### 2. Normalization

`internal/ytdlp/raw_models.go` mirrors yt-dlp's JSON. Every numeric field uses
`flexFloat` / `flexInt`, which accept a number, a numeric string or `null`,
because extractors are not consistent. Unknown fields are ignored by the
decoder, so a newer yt-dlp adding keys cannot break analysis.

`normalize.go` then produces a `media.MediaInfo`:

| Raw | Normalized |
| --- | --- |
| `extractor_key`, `extractor` | `Platform` — a badge name and a styling slug |
| `formats[]` | `[]MediaFormat`, classified and labelled |
| `subtitles` + `automatic_captions` | one `[]SubtitleLanguage`, with automatic tracks marked |
| `chapters` | `[]Chapter` |
| `entries[]` | `[]MediaEntry`, each marked resolved or not |
| `thumbnail`, `thumbnails[]` | one best thumbnail URL |

### 3. Classification

`media.Classify` is the only place raw codec values are interpreted:

- `vcodec == "none"` means no video, `acodec == "none"` means no audio, which
  gives each format a kind: combined, video-only, audio-only or other;
- storyboards (`mhtml`, `images`) and image entries are classified as other and
  left out of the selectable formats;
- when an extractor reports nothing at all — common for direct media links —
  the missing side is inferred rather than dropping a downloadable format;
- codec strings become readable names, with the raw values kept for the
  Advanced details panel.

### 4. Capability-driven presentation

`media.DetectCapabilities` reduces the analysis to a set of booleans, and the
interface is built from those rather than from the platform name:

```text
HasVideo  HasAudio  HasCombined  HasSeparateVideo  HasSeparateAudio
HasSubtitles  HasAutomaticCaptions  HasChapters  HasThumbnail
HasDescription  HasMultipleFormats  IsCollection  IsLive
```

This is why a TikTok video and a YouTube video need no separate code paths:
TikTok reports one combined stream and no chapters, so the stream selectors and
the chapter option are simply not rendered. Platform badges and content type
labels (Reel, Short, Story, Carousel) affect wording only.

### 5. Building the command

`BuildDownloadArgs` turns validated options into an argument array. Nothing from
the interface is trusted: download types, containers, audio formats, subtitle
formats and collection modes are checked against known values, format
identifiers must be plain tokens, and item selections must be digits and
separators. The URL is passed after `--` so it can never be read as a switch.

Format selection is an expression built from the identifiers the analysis
returned:

| Selection | `-f` expression |
| --- | --- |
| Separate streams | `137+140/137+ba/137` — with fallbacks if a stream disappeared |
| Combined stream | `22` |
| Video with any audio | `616+ba/616` |
| Nothing chosen | `bv*+ba/b` |
| Resolution ceiling | `bv*[height<=1080]+ba/b[height<=1080]/bv*+ba/b` |

Container handling distinguishes remuxing from transcoding. `--merge-output-format`
applies when two streams are merged; `--remux-video` is added only when the user
picked a container explicitly. Video is never re-encoded to satisfy a container
choice.

Subtitle handling combines two switches to express three outcomes:

| Wanted | Switches |
| --- | --- |
| Embedded only | `--embed-subs` |
| File only | `--write-subs` |
| Both | `--write-subs --embed-subs` |

with `--sub-format <f>/best --convert-subs <f>` so the format the user picked is
what ends up on disk, whatever the site published. WebM falls back to WebVTT
because it carries nothing else, and a thumbnail is saved beside a WebM file
rather than embedded, because WebM has no cover art.

### 6. Running and reporting

`Client.Download` adds the reporting switches, which are deliberately kept apart
from the command shown in the interface:

```text
--newline --no-quiet --progress-delta 0.5
--progress-template  "download:GRABONE-PROGRESS:%(progress)j"
--progress-template  "postprocess:GRABONE-POST:%(progress)j"
--print              "after_move:GRABONE-FILE:%(filepath)s"
--no-simulate
```

`%(progress)j` emits yt-dlp's own progress dictionary as JSON, so percentages,
byte counts, speed and ETA are read rather than scraped. `--no-quiet` is
required because `--print` otherwise implies `--quiet`, which would silence
progress entirely. `ProgressParser` is the only component that reads yt-dlp's
output text; when a site reports no size it falls back to fragment counts, and
when nothing is known it reports no percentage rather than inventing one.

Stages reported to the interface:

```text
starting · analyzing · downloading · downloading-video · downloading-audio
merging · remuxing · converting · embedding-metadata · embedding-subtitles
embedding-thumbnail · post-processing · finished · failed · cancelled
```

### 7. Jobs

`downloads.Manager` owns a map of jobs, an order, and a queue. Each job has a
UUID, a status (queued, running, completed, failed, cancelled) and its own
cancellation function. The concurrency limit is configurable; the default is one
at a time, with the rest queued.

Cancellation uses `context.WithCancel`, and `cmd.Cancel` kills the whole process
tree — yt-dlp launches FFmpeg, and a cancelled download must not leave it
running.

When a download finishes, FFprobe inspects the result so the interface can
report what was actually produced. A failure there is not a failed download.

### 8. Errors

`ClassifyError` maps yt-dlp's output to a kind, a readable message and a hint,
while always keeping the raw output for the Technical details panel:

```text
missing-dependency · invalid-url · unsupported-url · unavailable · private
auth-required · age-restricted · geo-blocked · format-unavailable
subtitle-unavailable · network · disk-full · cancelled · timeout · unknown
```

Login-required media is reported as exactly that, never as an invalid or
unsupported link. The rules are ordered so the specific cases win: an age
restriction is recognised before the generic authentication case, which is
recognised before the generic unavailability case.

## Authentication

Cookies are never read by GrabOne. The user picks a browser or a `cookies.txt`
file in Settings, and the choice becomes `--cookies-from-browser <browser>` or
`--cookies <path>`. Before any argument list is logged it passes through
`logging.RedactArgs`, which replaces the values of cookie, credential and header
switches.

## Threading

- Analysis and downloads run in goroutines with a `context` for cancellation.
- A download's stdout and stderr are consumed by two goroutines feeding one
  parser; the job's own state is guarded by a mutex.
- Progress reaches the frontend as Wails events, so the window never blocks on a
  running download.

## Tests

`go test ./...` runs without network access. `testdata/` holds real yt-dlp
output, trimmed: a YouTube video with chapters and subtitles, a YouTube
playlist, a TikTok video, a generic-extractor direct link, and an Instagram
carousel. The suites cover JSON parsing and robustness, format classification,
codec names, platform normalization, capability detection, content types,
command building, container choice, subtitle switch combinations, progress
parsing, error classification, settings serialization and job lifecycle.

## Getting the tools

GrabOne drives three programs it does not contain. Finding them, and offering to
fetch them, are separate concerns:

```text
configured path  →  application folder  →  managed folder  →  PATH  →  install locations
```

The managed folder, `%LOCALAPPDATA%\GrabOne\tools`, holds copies GrabOne
downloaded itself. Anything found there is marked as managed, which is what makes
an Update button meaningful: a copy installed by winget or Chocolatey belongs to
that package manager and is never replaced.

`internal/tooling` describes where each tool is published:

| Tool | Source | Verified with |
| --- | --- | --- |
| yt-dlp | `yt-dlp/yt-dlp` releases | `SHA2-256SUMS` |
| FFmpeg, FFprobe | `BtbN/FFmpeg-Builds` releases | `checksums.sha256` |

Only a release that publishes checksums can be offered, because the download has
to be provable. The FFmpeg archive is unpacked with `archive/zip`, taking only
`ffmpeg.exe` and `ffprobe.exe` by base name, so an entry such as
`../../evil.exe` cannot write outside the destination.

A successful download clears the configured path for the tools it supplies: a
path set by hand would otherwise take priority and hide the copy just installed.

## Updating the application

`internal/updater` compares the latest release tag with the version stamped
into the binary at build time:

```text
go build -ldflags "-X grabone/internal/appinfo.Version=1.1.0"
```

Versions are compared component by component, so 1.10.0 is newer than 1.9.0, and
a pre-release is older than the release of the same version.

Both the updater and the tool downloader go through `internal/ghrelease`, which
is the only place that fetches anything:

- HTTPS only, and only on GitHub hosts, checked again after redirects;
- the checksum file is read first, then the asset is streamed through SHA-256
  while it is written;
- a file that does not match is deleted, never returned;
- a download that stops short of its published size is refused.

`docs/releasing.md` covers the workflow that produces those releases.

## Extending

| To add | Where |
| --- | --- |
| A platform badge | `platformRules` in `internal/media/platform.go` — presentation only |
| A friendly codec name | `internal/media/codecs.go` |
| An error message | `errorRules` in `internal/ytdlp/errors.go` |
| A download option | `DownloadOptions` and `BuildDownloadArgs`, plus its validation |
| A downloadable tool | `sources` in `internal/tooling/tooling.go` |
| A new content type | `internal/media/capabilities.go` |

Adding support for a new site requires no change at all: update yt-dlp.
