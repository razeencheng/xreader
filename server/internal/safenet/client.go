// Package safenet provides an HTTP client that blocks requests to private/internal
// network addresses, preventing Server-Side Request Forgery (SSRF) attacks.
//
// The client validates resolved IP addresses at dial time (not just URL text) and
// re-validates on redirects to defend against DNS rebinding and open-redirect chains.
package safenet

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Errors returned by the safe client.
var (
	ErrUnsupportedURL = errors.New("safenet: unsupported URL scheme or format")
	ErrBlockedAddress = errors.New("safenet: request blocked — target resolves to a private/reserved address")
	ErrResponseTooBig = errors.New("safenet: response body exceeds size limit")
)

// Options configures the safe HTTP client.
type Options struct {
	// Timeout is the overall request timeout. Default: 15s.
	Timeout time.Duration
	// DialTimeout is the per-connection dial timeout. Default: 5s.
	DialTimeout time.Duration
	// MaxRedirects is the maximum number of redirects to follow. Default: 5.
	MaxRedirects int
	// MaxResponseBytes limits the response body size. 0 means no limit.
	MaxResponseBytes int64
	// UserAgent is the User-Agent header value. Default: "xReader/1.0".
	UserAgent string
}

func (o Options) withDefaults() Options {
	if o.Timeout == 0 {
		o.Timeout = 15 * time.Second
	}
	if o.DialTimeout == 0 {
		o.DialTimeout = 5 * time.Second
	}
	if o.MaxRedirects == 0 {
		o.MaxRedirects = 5
	}
	if o.UserAgent == "" {
		o.UserAgent = "xReader/1.0"
	}
	return o
}

// NewClient returns an *http.Client that blocks connections to private/reserved
// IP addresses. It validates at DNS resolution time and re-validates on redirects.
func NewClient(opts Options) *http.Client {
	opts = opts.withDefaults()
	return &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&safeDialer{
				resolver: net.DefaultResolver,
				dialer:   &net.Dialer{Timeout: opts.DialTimeout},
			}).DialContext,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opts.MaxRedirects {
				return errors.New("safenet: too many redirects")
			}
			if err := ValidateURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

// ValidateURL checks that a URL string uses http/https, has a host, and does
// not obviously point to a private address (text-level check). DNS-level
// validation happens at dial time.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return ErrUnsupportedURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrUnsupportedURL
	}
	if u.User != nil || u.Hostname() == "" {
		return ErrBlockedAddress
	}
	if isBlockedHost(u.Hostname()) {
		return ErrBlockedAddress
	}
	return nil
}

// ReadLimited reads up to max bytes from r. If the body exceeds max bytes,
// ErrResponseTooBig is returned.
func ReadLimited(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return io.ReadAll(r)
	}
	body, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, ErrResponseTooBig
	}
	return body, nil
}

// --- internal ---

type safeDialer struct {
	resolver *net.Resolver
	dialer   *net.Dialer
}

func (d *safeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if isBlockedHost(host) {
		return nil, ErrBlockedAddress
	}

	addrs, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		parsed, ok := netip.AddrFromSlice(addr.IP)
		if !ok || isBlockedIP(parsed) {
			continue
		}
		conn, err := d.dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if err != nil {
			continue
		}
		return conn, nil
	}
	return nil, ErrBlockedAddress
}

// isBlockedHost performs a text-level check on the hostname.
func isBlockedHost(host string) bool {
	normalized := strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}
	// If it parses as an IP literal, check it directly.
	parsed, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return err == nil && isBlockedIP(parsed)
}

// isBlockedIP returns true for loopback, private, link-local, and multicast addresses.
func isBlockedIP(ip netip.Addr) bool {
	return !ip.IsValid() ||
		ip.IsUnspecified() ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast()
}
