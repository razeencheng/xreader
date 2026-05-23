package source

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/razeencheng/xreader/internal/safenet"
)

const feedDiscoveryMaxBytes = 1 << 20
const feedDiscoveryTimeout = 10 * time.Second
const feedCandidateTimeout = 3 * time.Second

type discoveredFeed struct {
	URL      string
	Metadata SourceMetadata
}

// newDiscoveryClient creates the default SSRF-safe HTTP client for feed discovery.
func newDiscoveryClient() *http.Client {
	return safenet.NewClient(safenet.Options{
		Timeout:          feedDiscoveryTimeout,
		DialTimeout:      3 * time.Second,
		MaxRedirects:     5,
		MaxResponseBytes: feedDiscoveryMaxBytes,
		UserAgent:        "xReader feed discovery",
	})
}

func discoverFeed(ctx context.Context, input string, adapter SourceAdapter) (discoveredFeed, error) {
	return discoverFeedWithClient(ctx, input, adapter, newDiscoveryClient())
}

func discoverFeedWithClient(ctx context.Context, input string, adapter SourceAdapter, client *http.Client) (discoveredFeed, error) {
	ctx, cancel := context.WithTimeout(ctx, feedDiscoveryTimeout)
	defer cancel()

	candidates, err := initialURLCandidates(input)
	if err != nil {
		return discoveredFeed{}, err
	}

	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		if feed, ok := tryValidateFeed(ctx, adapter, candidate, seen); ok {
			return feed, nil
		}
	}

	for _, pageURL := range candidates {
		for _, candidate := range discoverFeedLinksFromPage(ctx, client, pageURL) {
			if feed, ok := tryValidateFeed(ctx, adapter, candidate, seen); ok {
				return feed, nil
			}
		}
	}

	for _, pageURL := range candidates {
		for _, candidate := range commonFeedURLCandidates(pageURL) {
			if feed, ok := tryValidateFeed(ctx, adapter, candidate, seen); ok {
				return feed, nil
			}
		}
	}

	return discoveredFeed{}, fmt.Errorf("no RSS or Atom feed found for %q", input)
}

func tryValidateFeed(ctx context.Context, adapter SourceAdapter, candidate string, seen map[string]struct{}) (discoveredFeed, bool) {
	if _, ok := seen[candidate]; ok {
		return discoveredFeed{}, false
	}
	seen[candidate] = struct{}{}

	candidateCtx, cancel := context.WithTimeout(ctx, feedCandidateTimeout)
	defer cancel()

	meta, err := adapter.Validate(candidateCtx, candidate)
	if err != nil {
		return discoveredFeed{}, false
	}
	return discoveredFeed{URL: candidate, Metadata: meta}, true
}

func initialURLCandidates(input string) ([]string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("empty URL")
	}
	if strings.ContainsAny(trimmed, " \t\n\r") {
		return nil, fmt.Errorf("URL must not contain whitespace")
	}

	if !strings.Contains(trimmed, "://") {
		return []string{"https://" + trimmed, "http://" + trimmed}, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	if !isHTTPURL(parsed) || parsed.Host == "" {
		return nil, fmt.Errorf("URL must use http or https")
	}
	return []string{parsed.String()}, nil
}

func isHTTPURL(parsed *url.URL) bool {
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func discoverFeedLinksFromPage(ctx context.Context, client *http.Client, pageURL string) []string {
	parsed, err := url.Parse(pageURL)
	if err != nil || !isHTTPURL(parsed) || parsed.Host == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "xReader feed discovery")

	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil
	}
	contentType := strings.ToLower(res.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "html") && !strings.Contains(contentType, "xml") {
		return nil
	}

	body, err := safenet.ReadLimited(res.Body, feedDiscoveryMaxBytes)
	if err != nil {
		return nil
	}
	return extractFeedLinks(string(body), parsed)
}

var linkTagPattern = regexp.MustCompile(`(?is)<link\s+[^>]*>`)
var attrPattern = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("[^"]*"|'[^']*'|[^\s"'>]+)`)

func extractFeedLinks(html string, base *url.URL) []string {
	links := make([]string, 0)
	seen := make(map[string]struct{})
	for _, tag := range linkTagPattern.FindAllString(html, -1) {
		attrs := parseHTMLAttrs(tag)
		rel := strings.ToLower(attrs["rel"])
		typ := strings.ToLower(attrs["type"])
		href := strings.TrimSpace(attrs["href"])
		if href == "" || !strings.Contains(rel, "alternate") || !isFeedContentType(typ) {
			continue
		}
		resolved, err := base.Parse(href)
		if err != nil || !isHTTPURL(resolved) || resolved.Host == "" {
			continue
		}
		value := resolved.String()
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		links = append(links, value)
	}
	return links
}

func parseHTMLAttrs(tag string) map[string]string {
	attrs := make(map[string]string)
	for _, match := range attrPattern.FindAllStringSubmatch(tag, -1) {
		if len(match) != 3 {
			continue
		}
		name := strings.ToLower(match[1])
		value := strings.Trim(match[2], " \t\n\r\"'")
		attrs[name] = value
	}
	return attrs
}

func isFeedContentType(contentType string) bool {
	return strings.Contains(contentType, "rss") || strings.Contains(contentType, "atom") || strings.Contains(contentType, "rdf") || strings.Contains(contentType, "xml")
}

func commonFeedURLCandidates(pageURL string) []string {
	parsed, err := url.Parse(pageURL)
	if err != nil || !isHTTPURL(parsed) || parsed.Host == "" {
		return nil
	}
	root := &url.URL{Scheme: parsed.Scheme, Host: parsed.Host}
	paths := []string{"/feed", "/feed/", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/index.xml", "/?feed=rss2"}
	candidates := make([]string, 0, len(paths))
	for _, path := range paths {
		candidate := *root
		if strings.HasPrefix(path, "/?") {
			candidate.Path = "/"
			candidate.RawQuery = strings.TrimPrefix(path, "/?")
		} else {
			candidate.Path = path
		}
		candidates = append(candidates, candidate.String())
	}
	return candidates
}
