package ytdlp

import (
	"fmt"
	"strconv"
	"strings"

	"grabone/internal/config"
	"grabone/internal/media"
)

// Markers prefixed to the machine readable lines yt-dlp is asked to print.
// Progress is read from these rather than from the human readable progress bar.
const (
	progressMarker     = "GRABONE-PROGRESS:"
	postProcessMarker  = "GRABONE-POST:"
	destinationMarker  = "GRABONE-FILE:"
	progressDeltaValue = "0.5"
)

// encodingArgs forces yt-dlp to write its output as UTF-8.
//
// Without this yt-dlp encodes what it prints using an encoding derived from the
// locale, with errors="ignore", so any character the code page cannot represent
// is dropped without a word. A title holding an emoji then comes back as a path
// that does not match the file just written, and everything keyed off that path
// fails. Note that PYTHONIOENCODING does not help: yt-dlp picks the encoding
// itself rather than leaving it to the interpreter.
func encodingArgs() []string {
	return []string{"--encoding", "utf-8"}
}

// ReportingArgs returns the switches that make yt-dlp report progress in a
// parsable form. They are kept apart from the download arguments so the command
// preview shows what the user asked for, not the application's plumbing.
//
//   - --encoding keeps printed paths byte for byte what was written to disk.
//   - --newline stops progress being rewritten on one line with carriage
//     returns, so each update is a separate line.
//   - --no-quiet is required because --print otherwise implies --quiet, which
//     would silence progress entirely.
//   - the progress templates emit the progress dictionary as JSON.
//   - --print after_move reports the final path once post-processing moved the
//     file into place.
func ReportingArgs() []string {
	return append(encodingArgs(),
		"--newline",
		"--no-quiet",
		"--progress-delta", progressDeltaValue,
		"--progress-template", "download:" + progressMarker + "%(progress)j",
		"--progress-template", "postprocess:" + postProcessMarker + "%(progress)j",
		"--print", "after_move:"+destinationMarker+"%(filepath)s",
		"--no-simulate",
	)
}

// BuildDownloadArgs turns validated options into a yt-dlp argument list.
//
// Format identifiers are never hardcoded: every identifier comes from the
// analysis of the URL being downloaded. When no identifier is given, a format
// selector expression is used so yt-dlp picks the best available stream.
func BuildDownloadArgs(options DownloadOptions) ([]string, error) {
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("build download arguments: %w", err)
	}

	plan := resolvePlan(options)

	args := make([]string, 0, 32)

	args = append(args, "-f", plan.formatSelector)

	if plan.container != "" {
		// Merging and remuxing rewrite the container without re-encoding the
		// streams. Changing the container is never a reason to transcode.
		args = append(args, "--merge-output-format", plan.container)
		if plan.explicitContainer {
			args = append(args, "--remux-video", plan.container)
		}
	}

	if options.DownloadType == DownloadTypeAudioOnly {
		args = append(args, "--extract-audio")
		if plan.audioConversion != "" {
			args = append(args, "--audio-format", plan.audioConversion, "--audio-quality", "0")
		}
	}

	if options.EmbedMetadata {
		args = append(args, "--embed-metadata")
	}
	if options.EmbedChapters {
		args = append(args, "--embed-chapters")
	}
	if plan.embedThumbnail {
		args = append(args, "--embed-thumbnail")
	}
	if plan.saveThumbnail {
		args = append(args, "--write-thumbnail")
	}
	if options.SaveDescription {
		args = append(args, "--write-description")
	}
	if options.SaveJSON {
		args = append(args, "--write-info-json")
	}

	args = append(args, subtitleArgs(options, plan)...)
	args = append(args, collectionArgs(options)...)
	args = append(args, outputArgs(options)...)
	args = append(args, options.Cookies.Args()...)

	// "--" ends the switches, so a URL can never be read as one.
	return append(args, "--", options.URL), nil
}

// downloadPlan holds the decisions derived from the options.
type downloadPlan struct {
	formatSelector    string
	container         string
	explicitContainer bool
	audioConversion   string
	subtitleFormat    string
	embedThumbnail    bool
	saveThumbnail     bool
}

func resolvePlan(options DownloadOptions) downloadPlan {
	plan := downloadPlan{
		formatSelector: formatSelector(options),
		subtitleFormat: options.SubtitleFormat,
		saveThumbnail:  options.SaveThumbnail,
		embedThumbnail: options.EmbedThumbnail,
	}

	if options.DownloadType == DownloadTypeAudioOnly {
		if options.AudioConversionFormat != "" && options.AudioConversionFormat != media.AudioConversionOriginal {
			plan.audioConversion = options.AudioConversionFormat
		}
		// The container of an audio download follows the audio pipeline.
		return plan
	}

	switch {
	case options.Container == "" || options.Container == media.ContainerAuto:
		plan.container = media.AutomaticContainer(
			options.SelectedVideoCodec,
			options.SelectedAudioCodec,
			options.DownloadSubtitles && options.EmbedSubtitles,
		)
	default:
		plan.container = options.Container
		plan.explicitContainer = true
	}

	if plan.container == media.ContainerWebM {
		// WebM carries only WebVTT subtitles, and cannot hold cover art at all.
		if options.DownloadSubtitles && options.EmbedSubtitles && plan.subtitleFormat == "srt" {
			plan.subtitleFormat = "vtt"
		}
		if plan.embedThumbnail {
			plan.embedThumbnail = false
			plan.saveThumbnail = true
		}
	}
	return plan
}

// formatSelector builds the -f expression. Explicitly chosen identifiers are
// used as given; otherwise an expression lets yt-dlp choose, optionally under a
// resolution ceiling.
func formatSelector(options DownloadOptions) string {
	switch options.DownloadType {
	case DownloadTypeAudioOnly:
		if options.AudioFormatID != "" {
			return options.AudioFormatID
		}
		return "ba/b"

	case DownloadTypeVideoOnly:
		if options.VideoFormatID != "" {
			return options.VideoFormatID
		}
		if ceiling := heightFilter(options.MaxHeight); ceiling != "" {
			return "bv*" + ceiling + "/b" + ceiling
		}
		return "bv*/b"

	default: // video + audio
		if options.CombinedFormatID != "" {
			return options.CombinedFormatID
		}
		if options.VideoFormatID != "" && options.AudioFormatID != "" {
			// Separate streams are merged by FFmpeg. The fallbacks keep the
			// download working if one of the streams disappeared.
			return strings.Join([]string{
				options.VideoFormatID + "+" + options.AudioFormatID,
				options.VideoFormatID + "+ba",
				options.VideoFormatID,
			}, "/")
		}
		if options.VideoFormatID != "" {
			return options.VideoFormatID + "+ba/" + options.VideoFormatID
		}
		if ceiling := heightFilter(options.MaxHeight); ceiling != "" {
			return "bv*" + ceiling + "+ba/b" + ceiling + "/bv*+ba/b"
		}
		return "bv*+ba/b"
	}
}

func heightFilter(maxHeight int) string {
	if maxHeight <= 0 {
		return ""
	}
	return "[height<=" + strconv.Itoa(maxHeight) + "]"
}

// subtitleArgs renders the subtitle switches.
//
// Whether the subtitle file is kept is decided by combining --write-subs with
// --embed-subs: embedding alone leaves no file behind, while writing alone
// produces a file without touching the media.
func subtitleArgs(options DownloadOptions, plan downloadPlan) []string {
	if !options.DownloadSubtitles || len(options.SubtitleLanguages) == 0 {
		return nil
	}

	args := make([]string, 0, 10)

	keepFile := options.KeepSubtitles || !options.EmbedSubtitles
	if keepFile {
		args = append(args, "--write-subs")
	}
	if options.EmbedSubtitles {
		args = append(args, "--embed-subs")
	}
	if options.IncludeAutomaticCaptions {
		// Machine generated captions live in a separate list and are not
		// written unless asked for explicitly.
		args = append(args, "--write-auto-subs")
	}

	args = append(args, "--sub-langs", strings.Join(options.SubtitleLanguages, ","))

	if format := plan.subtitleFormat; format != "" {
		// Ask for the format, then convert whatever arrived. Sites publish
		// subtitles in their own formats, so the conversion is what guarantees
		// the user gets the format they picked.
		args = append(args, "--sub-format", format+"/best", "--convert-subs", format)
	}
	return args
}

// collectionArgs decides how much of a multi-item URL is downloaded. The
// default is a single item: a playlist is only downloaded when asked for.
func collectionArgs(options DownloadOptions) []string {
	switch options.CollectionMode {
	case CollectionModeAll:
		return []string{"--yes-playlist"}
	case CollectionModeSelected:
		return []string{"--yes-playlist", "--playlist-items", strings.TrimSpace(options.PlaylistItems)}
	default:
		return []string{"--no-playlist"}
	}
}

// outputArgs sets the destination folder and filename template. yt-dlp owns
// filename sanitisation, so no escaping is attempted here.
func outputArgs(options DownloadOptions) []string {
	template := strings.TrimSpace(options.FilenameTemplate)
	if template == "" {
		template = config.DefaultFilenameTemplate
	}
	if options.CollectionMode == CollectionModeAll || options.CollectionMode == CollectionModeSelected {
		// Keep a collection together and in order on disk.
		template = "%(playlist_title,playlist_id)s/%(playlist_index)03d - " + template
	}

	args := []string{"-o", template}
	if directory := strings.TrimSpace(options.OutputDirectory); directory != "" {
		args = append(args, "--paths", directory)
	}
	return args
}

// BuildCommandPreview renders a command the user can read and copy. It uses
// PowerShell quoting because that is the shell on the target platform. The
// application itself always executes an argument array, never a command string.
func BuildCommandPreview(binaryPath string, args []string) string {
	name := binaryPath
	if name == "" {
		name = "yt-dlp"
	}
	// A bare command name reads better than an absolute path in a preview.
	if index := strings.LastIndexAny(name, "\\/"); index >= 0 {
		name = name[index+1:]
	}
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".EXE")

	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteForPowerShell(name))
	for _, arg := range args {
		parts = append(parts, quoteForPowerShell(arg))
	}
	return strings.Join(parts, " ")
}

// powerShellSpecial lists the characters that make an argument worth quoting in
// the preview. Some of them, such as "<", are genuinely reserved by PowerShell;
// the rest are quoted because a format selector or a URL reads more clearly as
// one quoted token.
const powerShellSpecial = " \t\"'`$&|<>(){}[];,%*+/:?="

func quoteForPowerShell(value string) string {
	if value == "" {
		return "\"\""
	}
	if !strings.ContainsAny(value, powerShellSpecial) {
		return value
	}
	escaped := strings.ReplaceAll(value, "`", "``")
	escaped = strings.ReplaceAll(escaped, "\"", "`\"")
	escaped = strings.ReplaceAll(escaped, "$", "`$")
	return "\"" + escaped + "\""
}
