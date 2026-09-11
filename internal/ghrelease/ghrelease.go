// Package ghrelease fetches release assets from GitHub and verifies them
// against the checksums published alongside them.
//
// It is shared by the application's own updater and by the downloader for
// yt-dlp and FFmpeg, so there is one implementation of "download this file and
// prove it is the file that was published" rather than one per caller.
package ghrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPIBaseURL is GitHub's API.
const DefaultAPIBaseURL = "https://api.github.com"

// metadataTimeout bounds a release lookup.
const metadataTimeout = 20 * time.Second

// allowedHosts are the only hosts an asset may be fetched from. An asset URL
// pointing anywhere else is a reason to stop rather than to follow it.
var allowedHosts = map[string]bool{
	"github.com":                           true,
	"api.github.com":                       true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
	"codeload.github.com":                  true,
	"raw.githubusercontent.com":            true,
}

// Asset is one file published with a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// Release is a published release, reduced to what callers need.
type Release struct {
	TagName     string  `json:"tagName"`
	Name        string  `json:"name"`
	Notes       string  `json:"notes"`
	PublishedAt string  `json:"publishedAt"`
	PageURL     string  `json:"pageUrl"`
	Assets      []Asset `json:"assets"`
}

// Find returns the first asset whose name satisfies match.
func (r *Release) Find(match func(name string) bool) *Asset {
	if r == nil {
		return nil
	}
	for index := range r.Assets {
		if match(r.Assets[index].Name) {
			return &r.Assets[index]
		}
	}
	return nil
}

// FindNamed returns the asset with exactly this name, ignoring case.
func (r *Release) FindNamed(name string) *Asset {
	return r.Find(func(candidate string) bool { return strings.EqualFold(candidate, name) })
}

// FindSuffix returns the first asset whose name ends with suffix.
func (r *Release) FindSuffix(suffix string) *Asset {
	lowered := strings.ToLower(suffix)
	return r.Find(func(candidate string) bool {
		return strings.HasSuffix(strings.ToLower(candidate), lowered)
	})
}

// Client reads releases and downloads their assets.
type Client struct {
	// APIBaseURL defaults to GitHub's API.
	APIBaseURL string
	// HTTPClient defaults to a client with a metadata timeout.
	HTTPClient *http.Client
	// UserAgent identifies the caller; GitHub requires one.
	UserAgent string

	// allowPlainHTTP relaxes the address check so tests can serve assets from a
	// local server. Nothing outside this package can set it.
	allowPlainHTTP bool
}

// NewClient builds a client that identifies itself with userAgent.
func NewClient(userAgent string) *Client {
	return &Client{
		APIBaseURL: DefaultAPIBaseURL,
		HTTPClient: &http.Client{Timeout: metadataTimeout},
		UserAgent:  userAgent,
	}
}

// rawRelease mirrors the part of GitHub's payload that is used here.
type rawRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// Latest returns the newest published release of a repository.
func (c *Client) Latest(ctx context.Context, owner, repo string) (*Release, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL(), owner, repo)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build the release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", c.userAgent())

	response, err := c.metadataClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("contact GitHub: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("no published release was found for %s/%s", owner, repo)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, fmt.Errorf("GitHub is rate limiting requests, try again later")
	default:
		return nil, fmt.Errorf("GitHub replied with %s", response.Status)
	}

	var raw rawRelease
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("read the release information: %w", err)
	}
	if raw.Draft {
		return nil, fmt.Errorf("no published release was found for %s/%s", owner, repo)
	}

	release := &Release{
		TagName:     raw.TagName,
		Name:        strings.TrimSpace(raw.Name),
		Notes:       strings.TrimSpace(raw.Body),
		PublishedAt: raw.PublishedAt,
		PageURL:     raw.HTMLURL,
	}
	for _, asset := range raw.Assets {
		release.Assets = append(release.Assets, Asset{
			Name: asset.Name,
			URL:  asset.BrowserDownloadURL,
			Size: asset.Size,
		})
	}
	return release, nil
}

// ValidateURL refuses anything that is not an HTTPS address on a known GitHub
// host, so a tampered release cannot redirect a download elsewhere.
func (c *Client) ValidateURL(raw string) error {
	return validateURL(raw, c.allowPlainHTTP)
}

func validateURL(raw string, allowPlainHTTP bool) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("the download address could not be read: %w", err)
	}
	if parsed.Scheme != "https" && !(allowPlainHTTP && parsed.Scheme == "http") {
		return fmt.Errorf("the download address is not https")
	}
	if !allowedHosts[strings.ToLower(parsed.Hostname())] {
		return fmt.Errorf("the download address points at an unexpected host (%s)", parsed.Hostname())
	}
	return nil
}

func (c *Client) baseURL() string {
	if strings.TrimSpace(c.APIBaseURL) != "" {
		return strings.TrimRight(c.APIBaseURL, "/")
	}
	return DefaultAPIBaseURL
}

func (c *Client) metadataClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: metadataTimeout}
}

func (c *Client) userAgent() string {
	if strings.TrimSpace(c.UserAgent) != "" {
		return c.UserAgent
	}
	return "GrabOne"
}
