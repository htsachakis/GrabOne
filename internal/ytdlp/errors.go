package ytdlp

import (
	"context"
	"errors"
	"strings"
)

// ErrorKind classifies a failure so the interface can say something useful.
// The raw output is always preserved alongside it: the underlying error is
// never hidden, only explained.
type ErrorKind string

const (
	KindMissingDependency ErrorKind = "missing-dependency"
	KindInvalidURL        ErrorKind = "invalid-url"
	KindUnsupportedURL    ErrorKind = "unsupported-url"
	KindUnavailable       ErrorKind = "unavailable"
	KindPrivate           ErrorKind = "private"
	KindAuthRequired      ErrorKind = "auth-required"
	KindAgeRestricted     ErrorKind = "age-restricted"
	KindGeoBlocked        ErrorKind = "geo-blocked"
	KindFormatUnavailable ErrorKind = "format-unavailable"
	KindSubtitleMissing   ErrorKind = "subtitle-unavailable"
	KindNetwork           ErrorKind = "network"
	KindDiskFull          ErrorKind = "disk-full"
	KindCancelled         ErrorKind = "cancelled"
	KindTimeout           ErrorKind = "timeout"
	KindUnknown           ErrorKind = "unknown"
)

// Error is a classified yt-dlp failure.
type Error struct {
	Kind    ErrorKind `json:"kind"`
	Message string    `json:"message"`
	Hint    string    `json:"hint,omitempty"`
	// Details holds the raw output, shown under "Technical details".
	Details string `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// NeedsAuthentication reports whether the failure could be solved by supplying
// cookies.
func (e *Error) NeedsAuthentication() bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case KindAuthRequired, KindPrivate, KindAgeRestricted:
		return true
	default:
		return false
	}
}

// errorRule maps a phrase in yt-dlp's output to a classification. Order
// matters: the first match wins, so specific cases precede general ones.
var errorRules = []struct {
	phrases []string
	kind    ErrorKind
	message string
	hint    string
}{
	{
		phrases: []string{"confirm your age", "age-restricted", "age restricted", "inappropriate for some users"},
		kind:    KindAgeRestricted,
		message: "This media is age restricted.",
		hint:    "Sign in through Settings › Authentication by using cookies from a browser where you are logged in.",
	},
	{
		phrases: []string{"private video", "private post", "this post is private", "is private"},
		kind:    KindPrivate,
		message: "This media is private.",
		hint:    "Only an account with access can download it. Add cookies in Settings › Authentication.",
	},
	{
		phrases: []string{
			"--cookies", "cookies-from-browser", "log in", "logged-in", "logged in",
			"login required", "sign in", "requires authentication", "account credentials",
			"empty media response", "you must be logged", "authentication",
		},
		kind:    KindAuthRequired,
		message: "This media requires authentication.",
		hint:    "Add cookies from a browser where you are signed in, under Settings › Authentication.",
	},
	{
		phrases: []string{"unsupported url"},
		kind:    KindUnsupportedURL,
		message: "No extractor supports this URL.",
		hint:    "yt-dlp did not recognise the site. Check the link, or update yt-dlp for newer site support.",
	},
	{
		phrases: []string{"is not a valid url", "invalid url"},
		kind:    KindInvalidURL,
		message: "That does not look like a valid link.",
	},
	{
		phrases: []string{"available in your country", "geo restricted", "geo-restricted", "blocked in your country", "not available from your location"},
		kind:    KindGeoBlocked,
		message: "This media is not available in your region.",
	},
	{
		phrases: []string{
			"unavailable", "no longer available", "has been removed",
			"been terminated", "does not exist", "http error 404", "http error 410", "not found",
		},
		kind:    KindUnavailable,
		message: "This media is not available.",
		hint:    "The link may be wrong, or the media may have been removed.",
	},
	{
		phrases: []string{"requested format is not available", "requested format not available"},
		kind:    KindFormatUnavailable,
		message: "The selected format is no longer available.",
		hint:    "Analyze the URL again to refresh the available formats.",
	},
	{
		phrases: []string{"no subtitles", "subtitles not available", "requested subtitles"},
		kind:    KindSubtitleMissing,
		message: "The requested subtitles are not available.",
	},
	{
		phrases: []string{"ffmpeg is not installed", "ffprobe is not installed", "ffmpeg not found"},
		kind:    KindMissingDependency,
		message: "FFmpeg is required for this download but was not found.",
		hint:    "Set the FFmpeg location in Settings › Dependencies.",
	},
	{
		phrases: []string{"no space left on device", "not enough space on the disk", "disk full"},
		kind:    KindDiskFull,
		message: "There is not enough free disk space.",
		hint:    "Free some space or choose another output folder.",
	},
	{
		phrases: []string{
			"unable to download webpage", "urlopen error", "name resolution", "getaddrinfo",
			"connection reset", "connection refused", "network is unreachable",
			"temporary failure", "ssl", "timed out", "read timed out",
		},
		kind:    KindNetwork,
		message: "The network request failed.",
		hint:    "Check your connection and try again.",
	},
	{
		phrases: []string{"rate-limit", "rate limit", "too many requests", "http error 429"},
		kind:    KindNetwork,
		message: "The site is rate limiting requests.",
		hint:    "Wait a little and try again.",
	},
}

// ClassifyError turns raw yt-dlp output into a classified error. An unmatched
// failure keeps yt-dlp's own message, so nothing is lost in translation.
func ClassifyError(output string, cause error) *Error {
	details := strings.TrimSpace(output)

	switch {
	case errors.Is(cause, context.Canceled):
		return &Error{Kind: KindCancelled, Message: "The operation was cancelled.", Details: details}
	case errors.Is(cause, context.DeadlineExceeded):
		return &Error{
			Kind:    KindTimeout,
			Message: "The operation took too long and was stopped.",
			Hint:    "The site may be slow or unreachable. Try again.",
			Details: details,
		}
	}

	lowered := strings.ToLower(details)
	for _, rule := range errorRules {
		for _, phrase := range rule.phrases {
			if strings.Contains(lowered, phrase) {
				return &Error{Kind: rule.kind, Message: rule.message, Hint: rule.hint, Details: details}
			}
		}
	}

	message := extractErrorLine(details)
	if message == "" {
		if cause != nil {
			message = cause.Error()
		} else {
			message = "yt-dlp reported an error."
		}
	}
	return &Error{Kind: KindUnknown, Message: message, Details: details}
}

// MissingDependencyError reports that yt-dlp itself is unavailable.
func MissingDependencyError(name, detail string) *Error {
	return &Error{
		Kind:    KindMissingDependency,
		Message: name + " is not available.",
		Hint:    "Set its location in Settings › Dependencies, or place the executable next to the application.",
		Details: detail,
	}
}

// extractErrorLine picks the most informative line out of yt-dlp's output.
func extractErrorLine(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "ERROR:") {
			cleaned := strings.TrimSpace(trimmed[len("ERROR:"):])
			// Extractor prefixes such as "[youtube] id:" add noise.
			if index := strings.Index(cleaned, "] "); index >= 0 && strings.HasPrefix(cleaned, "[") {
				cleaned = strings.TrimSpace(cleaned[index+2:])
			}
			return cleaned
		}
	}
	for index := len(lines) - 1; index >= 0; index-- {
		if trimmed := strings.TrimSpace(lines[index]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
