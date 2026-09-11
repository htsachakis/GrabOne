package ytdlp

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"grabone/internal/media"
)

// AnalyzeOptions describes one analysis request.
type AnalyzeOptions struct {
	URL string
	// CollectionMode decides whether a link that belongs to a playlist is
	// analyzed as the single item or as the whole collection. Single is the
	// default so a playlist is never pulled in unexpectedly.
	CollectionMode string
	Cookies        CookieOptions
}

// ValidateURL performs the only check the application makes on a link: that it
// is a plausible HTTP or HTTPS address. Deciding whether the site is supported
// is yt-dlp's job, which is what lets new sites work without a code change.
func ValidateURL(raw string) (string, *Error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", &Error{Kind: KindInvalidURL, Message: "Paste a link to analyze."}
	}

	// A bare "youtube.com/watch?v=..." is a reasonable thing to paste.
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", &Error{Kind: KindInvalidURL, Message: "That does not look like a valid link.", Details: err.Error()}
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", &Error{Kind: KindInvalidURL, Message: "Only http and https links are supported."}
	}
	if parsed.Host == "" {
		return "", &Error{Kind: KindInvalidURL, Message: "That link is missing a website address."}
	}
	return parsed.String(), nil
}

// AnalyzeArgs renders the arguments used for analysis. Analysis never downloads
// media: it only asks the extractor what is available.
func (c *Client) AnalyzeArgs(opts AnalyzeOptions) []string {
	args := append(encodingArgs(),
		"--dump-single-json",
		// Listing entries without resolving each one keeps analysis fast and
		// makes sure a playlist link cannot trigger a full extraction.
		"--flat-playlist",
		"--no-progress",
	)
	if opts.CollectionMode == CollectionModeAll || opts.CollectionMode == CollectionModeSelected {
		args = append(args, "--yes-playlist")
	} else {
		args = append(args, "--no-playlist")
	}
	args = append(args, opts.Cookies.Args()...)
	return append(args, "--", opts.URL)
}

// Analyze inspects a URL and returns the normalized description of what it
// holds. The structured output of yt-dlp is the only source of information:
// no human readable listing is parsed.
func (c *Client) Analyze(ctx context.Context, opts AnalyzeOptions) (*media.MediaInfo, *Error) {
	if !c.Available() {
		return nil, MissingDependencyError("yt-dlp", "no executable configured")
	}

	normalizedURL, urlErr := ValidateURL(opts.URL)
	if urlErr != nil {
		return nil, urlErr
	}
	opts.URL = normalizedURL

	stdout, stderr, err := c.Run(ctx, c.AnalyzeArgs(opts)...)
	if err != nil {
		return nil, ClassifyError(stderr, err)
	}

	trimmed := strings.TrimSpace(string(stdout))
	if trimmed == "" {
		return nil, ClassifyError(stderr, nil)
	}

	var raw rawInfo
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, &Error{
			Kind:    KindUnknown,
			Message: "The analysis result could not be read.",
			Details: err.Error() + "\n" + firstBytes(trimmed, 2000),
		}
	}

	info := normalizeInfo(&raw, opts.URL)
	info.Warnings = append(info.Warnings, collectWarnings(stderr)...)

	if len(info.Formats) == 0 && !info.IsCollection {
		return info, &Error{
			Kind:    KindFormatUnavailable,
			Message: "The extractor did not report any downloadable stream.",
			Hint:    "The media may be an image-only post, or it may require authentication.",
			Details: strings.TrimSpace(stderr),
		}
	}
	return info, nil
}

// ResolveEntry analyzes one item of a collection so its formats can be chosen.
// Entries of a lazily listed playlist carry no formats until this runs.
func (c *Client) ResolveEntry(ctx context.Context, entryURL string, cookies CookieOptions) (*media.MediaInfo, *Error) {
	return c.Analyze(ctx, AnalyzeOptions{
		URL:            entryURL,
		CollectionMode: CollectionModeSingle,
		Cookies:        cookies,
	})
}

// collectWarnings picks yt-dlp's warning lines out of stderr so they can be
// surfaced without being treated as failures.
func collectWarnings(stderr string) []string {
	warnings := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(trimmed), "WARNING:") {
			continue
		}
		message := strings.TrimSpace(trimmed[len("WARNING:"):])
		if message == "" || seen[message] {
			continue
		}
		seen[message] = true
		warnings = append(warnings, message)
	}
	return warnings
}

func firstBytes(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}
