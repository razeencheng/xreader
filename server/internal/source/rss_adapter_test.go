package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func serveFixture(t *testing.T, path string) http.HandlerFunc {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(data)
	}
}

func testRSSAdapter(t *testing.T) *RSSAdapter {
	t.Helper()
	return NewRSSAdapter(WithHTTPClient(&http.Client{Timeout: feedCandidateTimeout}))
}

func TestRSSAdapter_FetchesAndParsesAtomFeed(t *testing.T) {
	ts := httptest.NewServer(serveFixture(t, "testdata/atom_feed.xml"))
	defer ts.Close()

	a := testRSSAdapter(t)
	items, err := a.Fetch(context.Background(), Source{URL: ts.URL})
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, "Welcome", items[0].Title)
}

func TestRSSAdapter_FetchesRSS2Feed(t *testing.T) {
	ts := httptest.NewServer(serveFixture(t, "testdata/rss2_feed.xml"))
	defer ts.Close()

	a := testRSSAdapter(t)
	items, err := a.Fetch(context.Background(), Source{URL: ts.URL})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "RSS Item One", items[0].Title)
}

func TestRSSAdapter_Sanitizes_StripsScripts(t *testing.T) {
	ts := httptest.NewServer(serveFixture(t, "testdata/script_feed.xml"))
	defer ts.Close()

	a := testRSSAdapter(t)
	items, err := a.Fetch(context.Background(), Source{URL: ts.URL})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NotContains(t, items[0].ContentHTML, "<script>")
	require.NotContains(t, items[0].ContentHTML, "onclick")
}

func TestRSSAdapter_MalformedFeed_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(serveFixture(t, "testdata/malformed_feed.xml"))
	defer ts.Close()

	a := testRSSAdapter(t)
	_, err := a.Fetch(context.Background(), Source{URL: ts.URL})
	require.Error(t, err)
}

func TestRSSAdapter_Validate_ReturnsMetadata(t *testing.T) {
	ts := httptest.NewServer(serveFixture(t, "testdata/atom_feed.xml"))
	defer ts.Close()

	a := testRSSAdapter(t)
	meta, err := a.Validate(context.Background(), ts.URL)
	require.NoError(t, err)
	require.Equal(t, "Test Atom Feed", meta.Title)
}

func TestSanitizeHTML_StripsDangerousContent(t *testing.T) {
	input := `<p>Hello</p><script>alert(1)</script><p onclick="evil()">click</p><img src="data:image/png;base64,abc">`
	out := SanitizeHTML(input)
	require.NotContains(t, out, "<script>")
	require.NotContains(t, out, "onclick")
	require.True(t, strings.Contains(out, "Hello"))
}

func TestSanitizeHTML_StripsHiddenHeadingAnchors(t *testing.T) {
	input := `<h3 id="详细配置">详细配置<a hidden class="anchor" aria-hidden="true" href="#详细配置">#</a></h3><h3>总结#</h3>`

	out := SanitizeHTML(input)

	require.Contains(t, out, "详细配置")
	require.Contains(t, out, "总结")
	require.NotContains(t, out, "详细配置#")
	require.NotContains(t, out, "总结#")
	require.NotContains(t, out, "anchor")
}
