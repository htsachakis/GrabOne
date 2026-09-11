package ytdlp

import (
	"sort"
	"strings"

	"grabone/internal/media"
)

// normalizeInfo converts one yt-dlp result into the application's media model.
// This is the only place that knows yt-dlp's JSON vocabulary; everything above
// it works with the normalized model.
func normalizeInfo(raw *rawInfo, requestedURL string) *media.MediaInfo {
	isCollection := isPlaylistResult(raw)

	platform := media.NormalizePlatform(extractorKeyOf(raw), raw.Extractor)

	info := &media.MediaInfo{
		Platform:     platform.Name,
		PlatformSlug: platform.Slug,
		Extractor:    raw.Extractor,
		ExtractorKey: extractorKeyOf(raw),

		ID:          raw.ID,
		Title:       strings.TrimSpace(raw.Title),
		Uploader:    strings.TrimSpace(raw.Uploader),
		Channel:     strings.TrimSpace(raw.Channel),
		Description: raw.Description,

		Duration: raw.Duration.Float(),

		ThumbnailURL: bestThumbnail(raw),
		WebpageURL:   webpageURLOf(raw),
		RequestedURL: requestedURL,

		UploadDate: formatUploadDate(raw.UploadDate),
		ViewCount:  raw.ViewCount.Int(),
		LiveStatus: liveStatusOf(raw),

		IsCollection: isCollection,

		Formats:   normalizeFormats(raw),
		Subtitles: normalizeSubtitles(raw),
		Chapters:  normalizeChapters(raw.Chapters),
		Warnings:  []string{},
	}
	info.DurationText = media.FormatDuration(info.Duration)

	if info.Title == "" {
		info.Title = raw.ID
	}
	if info.Uploader == "" {
		info.Uploader = raw.UploaderID
	}

	if isCollection {
		info.CollectionTitle = firstNonEmpty(raw.PlaylistTitle, raw.Title)
		info.Entries = normalizeEntries(raw, requestedURL)
		info.EntriesResolved = entriesResolved(info.Entries)
	} else {
		info.Entries = []media.MediaEntry{}
	}

	info.ContentType = media.DetectContentType(info, isCollection, len(info.Entries))
	info.Capabilities = media.DetectCapabilities(info)

	// A language that has both manual subtitles and automatic captions keeps
	// only the manual track, so the capability is taken from what the extractor
	// reported rather than from the deduplicated list.
	if len(raw.AutomaticCaptions) > 0 {
		info.Capabilities.HasAutomaticCaptions = true
	}

	return info
}

// isPlaylistResult reports whether a result describes several media items.
// A playlist is the extractor's own decision; the application never guesses it
// from the URL.
func isPlaylistResult(raw *rawInfo) bool {
	switch raw.Type {
	case "playlist", "multi_video":
		return true
	}
	return len(raw.Entries) > 0
}

func extractorKeyOf(raw *rawInfo) string {
	return firstNonEmpty(raw.ExtractorKey, raw.IEKey, raw.Extractor)
}

func webpageURLOf(raw *rawInfo) string {
	return firstNonEmpty(raw.WebpageURL, raw.OriginalURL, raw.URL)
}

func liveStatusOf(raw *rawInfo) string {
	if raw.LiveStatus != "" {
		return raw.LiveStatus
	}
	if raw.IsLive {
		return "is_live"
	}
	return ""
}

// normalizeFormats converts and classifies the format list. Storyboard and
// other non-media entries are left out; genuine alternatives such as VP9, AV1
// and Opus are all kept.
func normalizeFormats(raw *rawInfo) []media.MediaFormat {
	rawFormats := raw.Formats
	if len(rawFormats) == 0 && raw.FormatID != "" {
		// Some extractors return a single stream inline instead of a list.
		rawFormats = []rawFormat{{
			FormatID:   raw.FormatID,
			Extension:  raw.Extension,
			VideoCodec: raw.VideoCodec,
			AudioCodec: raw.AudioCodec,
			Width:      raw.Width,
			Height:     raw.Height,
			FPS:        raw.FPS,
			Protocol:   raw.Protocol,
		}}
	}

	formats := make([]media.MediaFormat, 0, len(rawFormats))
	for _, current := range rawFormats {
		format := media.MediaFormat{
			FormatID:  current.FormatID,
			Extension: current.Extension,

			Width:  current.Width.IntValue(),
			Height: current.Height.IntValue(),
			FPS:    current.FPS.Float(),

			RawVideoCodec: current.VideoCodec,
			RawAudioCodec: current.AudioCodec,

			VideoBitrate: current.VideoBitrate.Float(),
			AudioBitrate: current.AudioBitrate.Float(),
			TotalBitrate: current.TotalBitrate.Float(),

			FileSize:       current.Filesize.Int(),
			FileSizeApprox: current.FilesizeApprox.Int(),

			DynamicRange: current.DynamicRange,
			Protocol:     current.Protocol,
			FormatNote:   firstNonEmpty(current.FormatNote, current.Format),
			Language:     current.Language,
		}
		media.Classify(&format)

		if format.Kind == media.FormatKindOther {
			continue
		}
		if format.FormatID == "" {
			continue
		}
		formats = append(formats, format)
	}

	media.SortFormats(formats)
	media.MarkRecommended(formats)
	media.DisambiguateLabels(formats)
	return formats
}

// normalizeSubtitles merges manual subtitles and automatic captions into one
// list, keeping the distinction between them.
func normalizeSubtitles(raw *rawInfo) []media.SubtitleLanguage {
	tracks := make([]media.SubtitleLanguage, 0, len(raw.Subtitles)+len(raw.AutomaticCaptions))

	manualCodes := map[string]bool{}
	for code, variants := range raw.Subtitles {
		manualCodes[code] = true
		tracks = append(tracks, subtitleTrack(code, variants, false))
	}
	for code, variants := range raw.AutomaticCaptions {
		if manualCodes[code] {
			// A manual track for the same language is always preferable.
			continue
		}
		tracks = append(tracks, subtitleTrack(code, variants, true))
	}

	media.SortSubtitles(tracks)
	return tracks
}

func subtitleTrack(code string, variants []rawSubtitle, automatic bool) media.SubtitleLanguage {
	track := media.SubtitleLanguage{
		Code:      code,
		Name:      media.LanguageName(code),
		Automatic: automatic,
		Formats:   make([]string, 0, len(variants)),
	}

	seen := map[string]bool{}
	for _, variant := range variants {
		extension := strings.ToLower(strings.TrimSpace(variant.Extension))
		if extension == "" || seen[extension] {
			continue
		}
		seen[extension] = true
		track.Formats = append(track.Formats, extension)
		if track.Name == code && variant.Name != "" {
			track.Name = variant.Name
		}
	}
	sort.Strings(track.Formats)
	return track
}

func normalizeChapters(raw []rawChapter) []media.Chapter {
	chapters := make([]media.Chapter, 0, len(raw))
	for _, current := range raw {
		title := strings.TrimSpace(current.Title)
		if title == "" {
			continue
		}
		chapters = append(chapters, media.Chapter{
			Title:     title,
			StartTime: current.StartTime.Float(),
			EndTime:   current.EndTime.Float(),
		})
	}
	return chapters
}

// normalizeEntries converts the items of a collection. Lazily listed entries
// carry no formats yet and are marked unresolved, so the interface can resolve
// one on demand instead of extracting a whole playlist up front.
func normalizeEntries(raw *rawInfo, requestedURL string) []media.MediaEntry {
	entries := make([]media.MediaEntry, 0, len(raw.Entries))
	for index, current := range raw.Entries {
		entry := media.MediaEntry{
			Index:        index + 1,
			ID:           current.ID,
			Title:        strings.TrimSpace(current.Title),
			ThumbnailURL: bestThumbnail(&current),
			Duration:     current.Duration.Float(),
			WebpageURL:   firstNonEmpty(webpageURLOf(&current), requestedURL),
			Formats:      normalizeFormats(&current),
		}
		entry.DurationText = media.FormatDuration(entry.Duration)
		entry.Resolved = len(entry.Formats) > 0

		// An entry that has not been resolved carries no formats, which says
		// nothing about what it holds. Naming a type from that absence would
		// describe a video as an image, so the type is left unknown until the
		// entry is resolved.
		if entry.Resolved {
			entryInfo := &media.MediaInfo{
				ExtractorKey: extractorKeyOf(&current),
				Extractor:    current.Extractor,
				WebpageURL:   entry.WebpageURL,
				RequestedURL: entry.WebpageURL,
				Formats:      entry.Formats,
				LiveStatus:   liveStatusOf(&current),
			}
			entry.ContentType = media.DetectContentType(entryInfo, false, 0)
		}

		if entry.Title == "" {
			entry.Title = firstNonEmpty(current.ID, "Item")
		}
		if strings.EqualFold(current.Availability, "unavailable") {
			entry.Unavailable = true
			entry.Message = "This item is not available"
		}
		entries = append(entries, entry)
	}
	return entries
}

func entriesResolved(entries []media.MediaEntry) bool {
	if len(entries) == 0 {
		return false
	}
	for _, entry := range entries {
		if !entry.Resolved {
			return false
		}
	}
	return true
}

// bestThumbnail picks the highest quality thumbnail offered, preferring the
// extractor's own choice when it made one.
func bestThumbnail(raw *rawInfo) string {
	if url := strings.TrimSpace(raw.Thumbnail); url != "" {
		return url
	}

	best := ""
	bestScore := int64(-1)
	for _, thumbnail := range raw.Thumbnails {
		if strings.TrimSpace(thumbnail.URL) == "" {
			continue
		}
		score := thumbnail.Preference.Int()*1_000_000 + thumbnail.Width.Int()*thumbnail.Height.Int()
		if score > bestScore {
			bestScore = score
			best = thumbnail.URL
		}
	}
	return best
}

// formatUploadDate turns a compact YYYYMMDD value into an ISO date.
func formatUploadDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 8 {
		return trimmed
	}
	return trimmed[:4] + "-" + trimmed[4:6] + "-" + trimmed[6:]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
