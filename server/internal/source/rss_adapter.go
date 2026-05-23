package source

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/razeencheng/xreader/internal/safenet"
)

const feedMaxResponseBytes = 10 * 1024 * 1024 // 10 MB

type RSSAdapter struct {
	parser     *gofeed.Parser
	safeClient *http.Client
}

// NewRSSAdapter creates an RSS adapter with SSRF-safe HTTP client.
// An optional *http.Client may be provided (for testing); if nil, a safe
// client that blocks private/reserved IP addresses is used.
func NewRSSAdapter(opts ...func(*RSSAdapter)) *RSSAdapter {
	client := safenet.NewClient(safenet.Options{
		Timeout:          15 * time.Second,
		DialTimeout:      5 * time.Second,
		MaxRedirects:     5,
		MaxResponseBytes: feedMaxResponseBytes,
		UserAgent:        "xReader feed fetcher",
	})

	p := gofeed.NewParser()
	p.Client = client
	p.UserAgent = "xReader feed fetcher"

	a := &RSSAdapter{parser: p, safeClient: client}
	for _, o := range opts {
		o(a)
	}
	return a
}

// WithHTTPClient overrides the default SSRF-safe client (for testing only).
func WithHTTPClient(c *http.Client) func(*RSSAdapter) {
	return func(a *RSSAdapter) {
		a.parser.Client = c
		a.safeClient = c
	}
}

func (a *RSSAdapter) Kind() string { return "rss" }

func (a *RSSAdapter) Fetch(ctx context.Context, src Source) ([]RawItem, error) {
	feed, err := a.parser.ParseURLWithContext(src.URL, ctx)
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	items := make([]RawItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		ri := RawItem{
			ExternalID:  item.GUID,
			Link:        item.Link,
			Title:       item.Title,
			ContentHTML: SanitizeHTML(bestContent(item)),
		}
		if item.PublishedParsed != nil {
			ri.PublishedAt = *item.PublishedParsed
		} else {
			ri.PublishedAt = time.Now()
		}
		if feed.Language != "" {
			ri.LanguageHint = feed.Language
		}
		items = append(items, ri)
	}
	return items, nil
}

func (a *RSSAdapter) Validate(ctx context.Context, url string) (SourceMetadata, error) {
	feed, err := a.parser.ParseURLWithContext(url, ctx)
	if err != nil {
		return SourceMetadata{}, fmt.Errorf("validate feed: %w", err)
	}
	meta := SourceMetadata{
		Title:        feed.Title,
		LanguageHint: feed.Language,
	}
	if feed.Image != nil {
		meta.IconURL = feed.Image.URL
	}
	return meta, nil
}

func bestContent(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}
