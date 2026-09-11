package media

import (
	"sort"
	"strings"
)

// languageNames covers the codes seen most often. Anything missing keeps its
// own code as the label, so a track is never hidden because it is unlisted.
var languageNames = map[string]string{
	"ar":  "Arabic",
	"bg":  "Bulgarian",
	"bn":  "Bengali",
	"ca":  "Catalan",
	"cs":  "Czech",
	"da":  "Danish",
	"de":  "German",
	"el":  "Greek",
	"en":  "English",
	"es":  "Spanish",
	"et":  "Estonian",
	"fa":  "Persian",
	"fi":  "Finnish",
	"fil": "Filipino",
	"fr":  "French",
	"he":  "Hebrew",
	"hi":  "Hindi",
	"hr":  "Croatian",
	"hu":  "Hungarian",
	"id":  "Indonesian",
	"it":  "Italian",
	"ja":  "Japanese",
	"ko":  "Korean",
	"lt":  "Lithuanian",
	"lv":  "Latvian",
	"ms":  "Malay",
	"nl":  "Dutch",
	"no":  "Norwegian",
	"pl":  "Polish",
	"pt":  "Portuguese",
	"ro":  "Romanian",
	"ru":  "Russian",
	"sk":  "Slovak",
	"sl":  "Slovenian",
	"sr":  "Serbian",
	"sv":  "Swedish",
	"ta":  "Tamil",
	"th":  "Thai",
	"tr":  "Turkish",
	"uk":  "Ukrainian",
	"ur":  "Urdu",
	"vi":  "Vietnamese",
	"zh":  "Chinese",
}

var regionNames = map[string]string{
	"br":   "Brazil",
	"cn":   "China",
	"gb":   "UK",
	"hans": "Simplified",
	"hant": "Traditional",
	"hk":   "Hong Kong",
	"mx":   "Mexico",
	"pt":   "Portugal",
	"tw":   "Taiwan",
	"us":   "US",
}

// LanguageName renders a subtitle language code as a readable name, keeping any
// region or script qualifier: "pt-BR" becomes "Portuguese (Brazil)".
func LanguageName(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ""
	}

	// yt-dlp marks translated caption tracks with a suffix, for example
	// "en-orig" or "es-en" for auto-translated tracks.
	base := strings.NewReplacer("_", "-").Replace(trimmed)
	parts := strings.Split(base, "-")

	name, ok := languageNames[strings.ToLower(parts[0])]
	if !ok {
		return trimmed
	}
	if len(parts) == 1 {
		return name
	}

	qualifiers := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if region, ok := regionNames[strings.ToLower(part)]; ok {
			qualifiers = append(qualifiers, region)
			continue
		}
		if language, ok := languageNames[strings.ToLower(part)]; ok {
			qualifiers = append(qualifiers, language)
			continue
		}
		qualifiers = append(qualifiers, strings.ToUpper(part))
	}
	return name + " (" + strings.Join(qualifiers, ", ") + ")"
}

// SortSubtitles orders tracks so manual subtitles come before automatic
// captions, and languages read alphabetically within each group.
func SortSubtitles(subtitles []SubtitleLanguage) {
	sort.SliceStable(subtitles, func(i, j int) bool {
		left, right := subtitles[i], subtitles[j]
		if left.Automatic != right.Automatic {
			return !left.Automatic
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.Code < right.Code
	})
}

// SubtitleFormatAvailable reports whether a track offers a given file format.
// yt-dlp can convert between subtitle formats, so this only reflects what the
// extractor published.
func SubtitleFormatAvailable(track SubtitleLanguage, format string) bool {
	for _, candidate := range track.Formats {
		if strings.EqualFold(candidate, format) {
			return true
		}
	}
	return false
}
