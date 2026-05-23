package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverFeed_AcceptsBareHostAndFindsAlternateFeed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><link rel="alternate" type="application/atom+xml" href="/atom.xml"></head></html>`))
		case "/atom.xml":
			http.ServeFile(w, r, "testdata/atom_feed.xml")
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// Use a plain HTTP client for test (bypasses SSRF protection for localhost test server)
	testClient := &http.Client{Timeout: feedCandidateTimeout}
	hostOnly := strings.TrimPrefix(ts.URL, "http://")
	feed, err := discoverFeedWithClient(context.Background(), hostOnly, NewRSSAdapter(WithHTTPClient(testClient)), testClient)

	require.NoError(t, err)
	require.Equal(t, ts.URL+"/atom.xml", feed.URL)
	require.Equal(t, "Test Atom Feed", feed.Metadata.Title)
}

func TestDiscoverFeed_FallsBackToCommonFeedPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><title>No feed link</title></head></html>`))
		case "/feed":
			http.ServeFile(w, r, "testdata/rss2_feed.xml")
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	testClient := &http.Client{Timeout: feedCandidateTimeout}
	feed, err := discoverFeedWithClient(context.Background(), ts.URL, NewRSSAdapter(WithHTTPClient(testClient)), testClient)

	require.NoError(t, err)
	require.Equal(t, ts.URL+"/feed", feed.URL)
	require.Equal(t, "Test RSS Feed", feed.Metadata.Title)
}

func TestDiscoverFeed_RejectsUnsupportedScheme(t *testing.T) {
	_, err := discoverFeed(context.Background(), "file:///etc/passwd", NewRSSAdapter())

	require.Error(t, err)
	require.Contains(t, err.Error(), "http or https")
}
