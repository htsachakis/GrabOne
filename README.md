# GrabOne

A Windows desktop frontend for [yt-dlp](https://github.com/yt-dlp/yt-dlp).

Paste a media link, see every stream, subtitle and chapter the site actually
offers, choose what you want, and download it. GrabOne drives `yt-dlp.exe`,
`ffmpeg.exe` and `ffprobe.exe` as external programs: it contains no extraction
or scraping logic of its own, which is why it works on any site yt-dlp supports
and gains new sites whenever yt-dlp is updated.

![GrabOne](build/appicon.png)

## What it does

- **Any supported site, one pipeline.** YouTube, Instagram, TikTok, Facebook,
  X/Twitter, Vimeo and everything else yt-dlp can extract go through the same
  analysis and download path. There is no list of allowed domains.
- **Capability-driven interface.** The controls shown are decided by what the
  analysis reported, not by the platform. A YouTube video with fifty streams
  gets full format selectors; a TikTok video with one combined stream gets a
  short summary and a download button.
- **Every format, plainly labelled.** Codecs are translated (`avc1.640028` →
  H.264), sizes are marked as exact or estimated, and the raw values stay
  available under Advanced details.
- **Video, audio or both.** Combined streams, separate video and audio merged by
  FFmpeg, audio-only downloads with optional conversion to MP3, M4A, FLAC, WAV
  or Opus.
- **Subtitles and chapters.** Manual subtitles and automatic captions are listed
  separately, converted to SRT or VTT, embedded or kept as files. Chapters and
  metadata can be embedded when the media has them.
- **Automatic container choice.** H.264 with AAC becomes MP4, VP9 with Opus
  becomes WebM, anything mixed becomes MKV. Changing the container remuxes; it
  never re-encodes the video.
- **Collections.** Playlists, profiles and Instagram carousels are listed as
  items you can pick from. A playlist link downloads one item unless you ask for
  more.
- **Authentication.** Private, age restricted and login-only media can be
  reached with cookies from a browser or a `cookies.txt` file. GrabOne hands the
  browser name or file path to yt-dlp; it never reads a cookie store itself and
  never logs cookie contents.
- **Real progress.** Per-stream progress, speed, ETA and post-processing stages,
  read from yt-dlp's machine-readable output rather than scraped from its
  progress bar.
- **Queue and cancellation.** Downloads can be queued, run up to five at a time,
  and cancelled, which also stops any FFmpeg process they started.
- **The effective command.** The Advanced section shows the exact yt-dlp command
  your selection produces, ready to copy.
- **Updates.** GrabOne checks GitHub for a newer release, shows what changed, and
  installs it on request after verifying the installer against the published
  SHA-256. The check can be turned off, and nothing installs by itself.
- **Sets up its own tools.** If yt-dlp or FFmpeg is missing, GrabOne says how to
  install them and offers to fetch them for you from their official releases,
  checking the published checksum before putting them to use.

## Installing

Download the installer from the [latest release](https://github.com/htsachakis/GrabOne/releases/latest)
and run it. The wizard asks who to install for: for everyone on the computer, in
`C:\Program Files\GrabOne`, which needs an administrator prompt, or for your
account only, in `%LOCALAPPDATA%\Programs\GrabOne`, which does not. A portable
build is published alongside the installer for use without installing at all.

That prompt comes after you choose, not when the installer starts, so a per-user
install never raises one.

If GrabOne is open when you install over it, the installer says so instead of
failing on a file it cannot replace. Close it and choose Retry, or choose Ignore
to have the installer close it for you, losing anything in progress.

The builds are not code signed, so Windows may warn about an unrecognised
publisher. Every release lists the SHA-256 of its files in checksums.txt, and the
in-app updater verifies that before it runs anything.

## Updating and uninstalling

GrabOne checks GitHub for a newer release when it starts, which can be turned off
in Settings › Updates. Nothing is downloaded or installed without you asking for
it. A downloaded installer is checked against the release's checksums.txt before
it is ever started, and a file that fails that check is deleted rather than kept.

An update keeps the scope of the install it replaces: a per-machine install stays
in `C:\Program Files\GrabOne` and a per-user one stays in your profile, rather
than a second copy appearing beside the first. GrabOne closes itself so the
installer can replace its files, and the installer offers to start it again on
its last page.

Installers are downloaded to `%LOCALAPPDATA%\GrabOne\updates`. Only the most
recent one is kept: applying an update deletes the installers left there by
earlier ones.

Uninstall from Settings › Apps, or run `uninstall.exe` from the install folder.
GrabOne is closed for you if it is still running, and removing a per-machine
install raises an administrator prompt. Your settings, logs and downloads are
left where they are; see [Settings and data](#settings-and-data) for what that
covers and where to find it.

## Requirements

| Requirement | Notes |
| --- | --- |
| Windows 10 or 11, x64 | ARM64 is not built yet, but nothing in the code prevents it |
| [yt-dlp](https://github.com/yt-dlp/yt-dlp/releases) | Required. Analysis and downloading |
| [FFmpeg](https://www.gyan.dev/ffmpeg/builds/) | Needed for merging, remuxing, conversion and embedding |
| FFprobe | Ships with FFmpeg. Used to report what the finished file contains |

GrabOne does not bundle these tools, so you can update them whenever you like
without touching the application. It looks for each one in this order:

1. the path set in Settings › Dependencies
2. the folder GrabOne itself is in — dropping `yt-dlp.exe`, `ffmpeg.exe` and
   `ffprobe.exe` next to `GrabOne.exe` is enough
3. `%LOCALAPPDATA%\GrabOne\tools`, where the copies GrabOne downloaded live
4. your `PATH`, so anything you can run in a terminal is found automatically
5. the standard install folders used by winget, Chocolatey and Scoop, for an
   installation whose folder never made it onto `PATH`

### If a tool is missing

The dependency panel shows what is missing, how to install it by hand, and a
**Download** button. That button fetches the tool from its official GitHub
release, verifies it against the checksum published with that release, and puts
it in the folder above. Nothing is downloaded until you press it.

| Tool | Downloaded from | Licence |
| --- | --- | --- |
| yt-dlp | [yt-dlp/yt-dlp](https://github.com/yt-dlp/yt-dlp) releases, verified with `SHA2-256SUMS` | Unlicense |
| FFmpeg + FFprobe | [BtbN/FFmpeg-Builds](https://github.com/BtbN/FFmpeg-Builds) releases, verified with `checksums.sha256` | GPL |

A copy GrabOne downloaded gets an **Update** button in Settings, which re-fetches
the latest release — useful because extraction usually breaks when yt-dlp falls
behind. A tool you installed with winget, Chocolatey or Scoop is used as it is
and never replaced: update it the way you installed it.

Each one is detected by running it, not by checking that a file exists, so a
broken or blocked executable is reported as unavailable rather than failing
halfway through a download.

## Installing the dependencies

The quickest route on a machine with [winget](https://learn.microsoft.com/windows/package-manager/):

```powershell
winget install yt-dlp.yt-dlp
winget install Gyan.FFmpeg
```

Then confirm they are on your `PATH`:

```powershell
yt-dlp --version
ffmpeg -version
```

## Building from source

### Prerequisites

| Tool | Version used |
| --- | --- |
| [Go](https://go.dev/dl/) | 1.25 or newer |
| [Node.js](https://nodejs.org/) | 20 or newer, for the frontend build |
| [Wails CLI](https://wails.io/docs/gettingstarted/installation) | v2.14 |
| WebView2 runtime | Preinstalled on Windows 11; the Wails installer prompts otherwise |

Install the Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Check that your machine has everything Wails needs:

```powershell
wails doctor
```

### Development

Runs the app with hot reloading of the frontend:

```powershell
wails dev
```

### Production build

Produces `build/bin/GrabOne.exe`:

```powershell
wails build
```

For a smaller binary with debug symbols stripped:

```powershell
wails build -clean -upx
```

### Checks

```powershell
go test ./...
go vet ./...
gofmt -l .
```

```powershell
cd frontend; npm run check
```

The Go tests read stored yt-dlp output from `testdata/` and never touch the
network.

### Building the installer

Needs [NSIS](https://nsis.sourceforge.io/Download) on PATH:

```powershell
wails build -clean -platform windows/amd64 -nsis
```

Releases are built by GitHub Actions from a version tag; see `docs/releasing.md`.

### Regenerating the icon

The application icon is drawn in code rather than committed as an opaque image:

```powershell
go run ./tools/icongen
```

This rewrites `build/appicon.png` and the multi-size `build/windows/icon.ico`.

## Architecture

```text
Frontend (TypeScript)
   ↓  Wails bindings and events
Go services (app.go)
   ↓
YTDLP client  →  yt-dlp.exe  →  FFmpeg / FFprobe
```

| Package | Responsibility |
| --- | --- |
| `internal/ytdlp` | Runs yt-dlp: analysis, argument building, download, progress parsing, error classification |
| `internal/media` | The normalized media model: formats, codecs, capabilities, containers, platforms |
| `internal/downloads` | Job lifecycle, queue, cancellation, progress events |
| `internal/dependencies` | Locating and verifying yt-dlp, FFmpeg and FFprobe |
| `internal/ffmpeg` | FFmpeg and FFprobe versions, inspecting finished files |
| `internal/config` | Settings storage under `%LOCALAPPDATA%\GrabOne` |
| `internal/logging` | File logging with credential redaction |
| `internal/system` | Opening a finished file and revealing it in Explorer |
| `internal/updater` | Checking GitHub for a newer GrabOne, verifying and installing it |
| `internal/ghrelease` | One verified download path: GitHub releases, checksums, host checks |
| `internal/tooling` | Fetching yt-dlp and FFmpeg from their official releases |

`docs/architecture.md` describes the flow, the normalized models and the
capability-driven interface in detail. `docs/releasing.md` covers the release
workflow and how the in-app updater verifies what it installs.

## Settings and data

| What | Where |
| --- | --- |
| Settings | `%LOCALAPPDATA%\GrabOne\settings.json` |
| Logs | `%LOCALAPPDATA%\GrabOne\logs\grabone.log` |
| Default downloads | `%USERPROFILE%\Videos\GrabOne` |
| Downloaded updates | `%LOCALAPPDATA%\GrabOne\updates` |
| Tools GrabOne downloaded | `%LOCALAPPDATA%\GrabOne\tools` |

Logs record what the application did — dependency detection, analysis,
extractor selection, download arguments, completion and cancellation. Cookie
values, credentials and authentication headers are redacted before anything is
written.

## Known limitations

- Windows x64 only for now. Nothing in the code is x64-specific; ARM64 simply
  has not been built or tested.
- Image-only posts are listed as collection items but cannot be downloaded yet:
  the data model carries them, the download path does not.
- Live streams download, but no percentage or size can be shown while they run,
  because no site reports one.
- Only a copy of yt-dlp that GrabOne downloaded can be updated from inside the
  application. One installed with a package manager is left alone, and shows its
  version in Settings so you know when it is getting old.
- The FFmpeg download is a large archive (around 190 MB) because the published
  builds are not split up. Installing it with winget is smaller.
- A collection download applies one set of options to every item, rather than
  per-item choices.
- Cancelling leaves partial `.part` files in the output folder, the same way
  yt-dlp does on the command line.

## Legal

Users are responsible for complying with applicable laws, copyright rules, and
platform terms.
