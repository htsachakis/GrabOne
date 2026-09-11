package media

import (
	"strings"
	"unicode"
)

// Platform is the normalized presentation of an extractor.
type Platform struct {
	// Name is shown on the badge, for example "YouTube".
	Name string `json:"name"`
	// Slug is a stable lowercase identifier, used for styling.
	Slug string `json:"slug"`
}

// platformRules maps extractor identifiers to a normalized platform. Matching
// is done on the extractor reported by yt-dlp rather than on the hostname, so
// the badge follows whichever extractor actually handled the URL.
//
// This table is presentation only. It never gates which URLs are accepted: an
// unlisted extractor still analyzes and downloads through the same pipeline.
var platformRules = []struct {
	prefixes []string
	platform Platform
}{
	{[]string{"youtube"}, Platform{Name: "YouTube", Slug: "youtube"}},
	{[]string{"instagram"}, Platform{Name: "Instagram", Slug: "instagram"}},
	{[]string{"tiktok"}, Platform{Name: "TikTok", Slug: "tiktok"}},
	{[]string{"facebook", "fb"}, Platform{Name: "Facebook", Slug: "facebook"}},
	{[]string{"twitter", "x.com"}, Platform{Name: "Twitter/X", Slug: "twitter"}},
	{[]string{"vimeo"}, Platform{Name: "Vimeo", Slug: "vimeo"}},
	{[]string{"reddit"}, Platform{Name: "Reddit", Slug: "reddit"}},
	{[]string{"twitch"}, Platform{Name: "Twitch", Slug: "twitch"}},
	{[]string{"soundcloud"}, Platform{Name: "SoundCloud", Slug: "soundcloud"}},
	{[]string{"dailymotion"}, Platform{Name: "Dailymotion", Slug: "dailymotion"}},
	{[]string{"bilibili"}, Platform{Name: "Bilibili", Slug: "bilibili"}},
	{[]string{"generic"}, Platform{Name: "Web", Slug: "web"}},
}

// NormalizePlatform turns extractor information into a platform badge. An
// unknown extractor keeps its own friendly name instead of being rejected.
func NormalizePlatform(extractorKey, extractor string) Platform {
	for _, candidate := range []string{extractorKey, extractor} {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if normalized == "" {
			continue
		}
		for _, rule := range platformRules {
			for _, prefix := range rule.prefixes {
				if strings.HasPrefix(normalized, prefix) {
					return rule.platform
				}
			}
		}
	}

	name := friendlyExtractorName(extractorKey, extractor)
	if name == "" {
		return Platform{Name: "Web", Slug: "web"}
	}
	return Platform{Name: name, Slug: slugify(name)}
}

// friendlyExtractorName produces a readable label for an extractor that has no
// dedicated rule, for example "ArteTVPlaylist" becomes "Arte TV Playlist".
func friendlyExtractorName(extractorKey, extractor string) string {
	source := strings.TrimSpace(extractorKey)
	if source == "" {
		source = strings.TrimSpace(extractor)
	}
	if source == "" {
		return ""
	}

	// Extractor keys use colons for sub-extractors, such as "youtube:tab".
	source = strings.ReplaceAll(source, ":", " ")

	var builder strings.Builder
	runes := []rune(source)
	for index, current := range runes {
		if index > 0 && unicode.IsUpper(current) {
			previous := runes[index-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextIsLower) {
				builder.WriteRune(' ')
			}
		}
		builder.WriteRune(current)
	}

	words := strings.Fields(builder.String())
	for index, word := range words {
		if len([]rune(word)) == 1 {
			words[index] = strings.ToUpper(word)
			continue
		}
		runes := []rune(word)
		words[index] = string(unicode.ToUpper(runes[0])) + string(runes[1:])
	}
	return strings.Join(words, " ")
}

func slugify(value string) string {
	var builder strings.Builder
	for _, current := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(current) || unicode.IsDigit(current):
			builder.WriteRune(current)
		case builder.Len() > 0:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}
