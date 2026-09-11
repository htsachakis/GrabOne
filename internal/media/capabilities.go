package media

import "strings"

// DetectCapabilities derives what the media supports from the analysis result.
// The interface is built from these flags, which is what keeps it free of
// platform specific branches: a TikTok video with one combined stream and a
// YouTube video with fifty separate streams differ only in these booleans.
func DetectCapabilities(info *MediaInfo) MediaCapabilities {
	capabilities := MediaCapabilities{
		IsCollection:   info.IsCollection,
		HasThumbnail:   strings.TrimSpace(info.ThumbnailURL) != "",
		HasDescription: strings.TrimSpace(info.Description) != "",
		HasChapters:    len(info.Chapters) > 0,
		IsLive:         isLiveStatus(info.LiveStatus),
	}

	for _, subtitle := range info.Subtitles {
		if subtitle.Automatic {
			capabilities.HasAutomaticCaptions = true
		} else {
			capabilities.HasSubtitles = true
		}
	}

	formats := info.Formats
	if len(formats) == 0 && info.IsCollection {
		// A collection's capabilities are the union of its resolved entries, so
		// the interface can still offer the right controls for a carousel.
		for _, entry := range info.Entries {
			formats = append(formats, entry.Formats...)
		}
	}

	selectable := 0
	for _, format := range formats {
		switch format.Kind {
		case FormatKindCombined:
			capabilities.HasVideo = true
			capabilities.HasAudio = true
			capabilities.HasCombined = true
			selectable++
		case FormatKindVideo:
			capabilities.HasVideo = true
			capabilities.HasSeparateVideo = true
			selectable++
		case FormatKindAudio:
			capabilities.HasAudio = true
			capabilities.HasSeparateAudio = true
			selectable++
		}
	}
	capabilities.HasMultipleFormats = selectable > 1

	return capabilities
}

func isLiveStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "is_live", "live", "post_live":
		return true
	default:
		return false
	}
}

// DetectContentType decides how to describe the analyzed URL. The result is
// informational: it changes wording and which controls are worth showing, never
// how the download is performed.
func DetectContentType(info *MediaInfo, isPlaylistResult bool, entryCount int) string {
	extractor := strings.ToLower(info.ExtractorKey + " " + info.Extractor)
	url := strings.ToLower(info.WebpageURL + " " + info.RequestedURL)

	if isPlaylistResult {
		switch {
		case strings.Contains(extractor, "instagram"):
			if entryCount > 1 {
				return ContentTypeCarousel
			}
			return ContentTypePost
		case strings.Contains(extractor, "playlist") || strings.Contains(url, "list="):
			return ContentTypePlaylist
		default:
			return ContentTypeCollection
		}
	}

	if isLiveStatus(info.LiveStatus) {
		return ContentTypeLive
	}

	hasVideo := false
	hasAudio := false
	for _, format := range info.Formats {
		if format.HasVideo {
			hasVideo = true
		}
		if format.HasAudio {
			hasAudio = true
		}
	}

	switch {
	case strings.Contains(url, "/stories/"):
		return ContentTypeStory
	case strings.Contains(url, "/reel/") || strings.Contains(url, "/reels/"):
		return ContentTypeReel
	case strings.Contains(url, "/shorts/"):
		return ContentTypeShort
	}

	switch {
	case !hasVideo && hasAudio:
		return ContentTypeAudio
	case !hasVideo && !hasAudio:
		if strings.Contains(extractor, "instagram") || strings.Contains(extractor, "facebook") {
			return ContentTypePost
		}
		return ContentTypeImage
	case strings.Contains(extractor, "instagram"):
		return ContentTypePost
	}

	return ContentTypeVideo
}
