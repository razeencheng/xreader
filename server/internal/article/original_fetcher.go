package article

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/razeencheng/xreader/internal/source"
)

const (
	originalFetchTimeout = 8 * time.Second
	originalFetchMaxBody = 2 * 1024 * 1024
)

var (
	errOriginalUnsupportedURL = errors.New("unsupported original URL")
	errOriginalUnsafeURL      = errors.New("unsafe original URL")
	errOriginalNotHTML        = errors.New("original URL did not return HTML")
	errOriginalTooLarge       = errors.New("original page is too large")
	errOriginalNoContent      = errors.New("original page has no readable content")
)

type OriginalContent struct {
	URL         string
	Title       string
	ContentHTML string
	ContentText string
}

func fetchOriginalContent(ctx context.Context, rawURL string) (OriginalContent, error) {
	u, err := parseSafeOriginalURL(rawURL)
	if err != nil {
		return OriginalContent{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, originalFetchTimeout)
	defer cancel()

	client := &http.Client{
		Timeout: originalFetchTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&safeDialer{
				resolver: net.DefaultResolver,
				dialer:   &net.Dialer{Timeout: 3 * time.Second},
			}).DialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			_, err := parseSafeOriginalURL(req.URL.String())
			return err
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return OriginalContent{}, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "xReader original loader")

	resp, err := client.Do(req)
	if err != nil {
		return OriginalContent{}, fmt.Errorf("fetch original: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OriginalContent{}, fmt.Errorf("fetch original: status %d", resp.StatusCode)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return OriginalContent{}, errOriginalNotHTML
	}

	body, err := readLimited(resp.Body, originalFetchMaxBody)
	if err != nil {
		return OriginalContent{}, err
	}

	content, err := extractReadableContent(body)
	if err != nil {
		return OriginalContent{}, err
	}
	content.URL = resp.Request.URL.String()
	return content, nil
}

func parseSafeOriginalURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return nil, errOriginalUnsupportedURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errOriginalUnsupportedURL
	}
	if u.User != nil || u.Hostname() == "" {
		return nil, errOriginalUnsafeURL
	}
	if isUnsafeHost(u.Hostname()) {
		return nil, errOriginalUnsafeURL
	}
	return u, nil
}

type safeDialer struct {
	resolver *net.Resolver
	dialer   *net.Dialer
}

func (d *safeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if isUnsafeHost(host) {
		return nil, errOriginalUnsafeURL
	}

	addrs, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		parsed, ok := netip.AddrFromSlice(addr.IP)
		if !ok || isUnsafeIP(parsed) {
			continue
		}
		return d.dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
	}
	return nil, errOriginalUnsafeURL
}

func isUnsafeHost(host string) bool {
	normalized := strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}
	parsed, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return err == nil && isUnsafeIP(parsed)
}

func isUnsafeIP(ip netip.Addr) bool {
	return !ip.IsValid() ||
		ip.IsUnspecified() ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast()
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, errOriginalTooLarge
	}
	return body, nil
}

var boilerplateSelectors = strings.Join([]string{
	"script", "style", "noscript", "svg", "iframe", "form",
	"nav", "header", "footer", "aside",
	".author-info", ".author-bio", ".author-card", ".post-author",
	".share-buttons", ".social-share", ".sharing", ".share-bar",
	".related-posts", ".related-articles", ".recommended",
	".comments", ".comment-section", "#comments", "#disqus_thread",
	".sidebar", ".widget", ".ad", ".advertisement", ".banner",
	".newsletter", ".subscribe-form", ".cta",
	".breadcrumb", ".breadcrumbs", ".pagination",
	".post-meta-bottom", ".post-footer", ".entry-footer", ".article-footer",
	".post-tags", ".tag-list",
	"[role='complementary']", "[role='navigation']", "[role='banner']",
}, ", ")

func extractReadableContent(body []byte) (OriginalContent, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return OriginalContent{}, err
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	doc.Find(boilerplateSelectors).Remove()

	container := bestContentContainer(doc)
	container.Find(boilerplateSelectors).Remove()
	removeBoilerplateByAttr(container)
	stripPresentationAttrs(container)
	container.Find("a").Each(func(_ int, sel *goquery.Selection) {
		sel.RemoveAttr("href")
	})

	html, err := container.Html()
	if err != nil {
		return OriginalContent{}, err
	}
	html = source.SanitizeHTML(html)
	text := strings.Join(strings.Fields(container.Text()), " ")
	if len([]rune(text)) < 80 {
		return OriginalContent{}, errOriginalNoContent
	}

	return OriginalContent{
		Title:       title,
		ContentHTML: html,
		ContentText: text,
	}, nil
}

func bestContentContainer(doc *goquery.Document) *goquery.Selection {
	candidates := []string{
		"article .content",
		"article .post-body",
		"article .article-body",
		"article .entry-content",
		".post-content",
		".entry-content",
		".article-content",
		".article-body",
		".post-body",
		"article",
		"main article",
		"main",
		"[role='main']",
		"[itemprop='articleBody']",
		".content",
	}

	var best *goquery.Selection
	bestScore := 0
	for _, selector := range candidates {
		doc.Find(selector).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
			score := readableScore(sel)
			if score > bestScore {
				best = sel
				bestScore = score
			}
			return true
		})
	}
	if best != nil && bestScore >= 80 {
		return best
	}
	return doc.Find("body").First()
}

var boilerplateAttrPatterns = []string{
	"author", "share", "social", "comment", "related",
	"sidebar", "widget", "footer", "nav", "breadcrumb",
	"subscribe", "newsletter", "recommend", "ad-",
	"follow", "tag-list", "post-meta",
}

func removeBoilerplateByAttr(container *goquery.Selection) {
	container.Find("div, section, span").Each(func(_ int, sel *goquery.Selection) {
		cls, _ := sel.Attr("class")
		id, _ := sel.Attr("id")
		combined := strings.ToLower(cls + " " + id)
		for _, pattern := range boilerplateAttrPatterns {
			if strings.Contains(combined, pattern) {
				sel.Remove()
				return
			}
		}
	})
}

func stripPresentationAttrs(container *goquery.Selection) {
	container.Find("*").Each(func(_ int, sel *goquery.Selection) {
		sel.RemoveAttr("class")
		sel.RemoveAttr("style")
		sel.RemoveAttr("id")
	})
}

func readableScore(sel *goquery.Selection) int {
	textLen := len([]rune(strings.Join(strings.Fields(sel.Text()), " ")))
	blockCount := sel.Find("p, li, blockquote, pre, h1, h2, h3").Length()
	return textLen + blockCount*60
}
