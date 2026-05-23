package platform

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {Data: []byte("<html><head><title>x</title></head><body>hi</body></html>")},
		"app.js":     {Data: []byte("console.log('x')")},
	}
}

func serve(t *testing.T, h http.Handler, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestSPAHandler_InjectsGAWhenEnabled(t *testing.T) {
	h := NewSPAHandler(newTestFS(), "G-ABC123")
	resp := serve(t, h, "/")
	body, _ := io.ReadAll(resp.Body)
	got := string(body)

	if !strings.Contains(got, `https://www.googletagmanager.com/gtag/js?id=G-ABC123`) {
		t.Errorf("expected gtag.js src in body, got:\n%s", got)
	}
	if !strings.Contains(got, `gtag('config','G-ABC123')`) {
		t.Errorf("expected gtag config call in body, got:\n%s", got)
	}
	// Snippet must land inside <head>, before </head>.
	if strings.Index(got, "googletagmanager") > strings.Index(got, "</head>") {
		t.Errorf("gtag snippet was not injected before </head>:\n%s", got)
	}
	// Content-Length must match the rewritten body, not the original file.
	if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length = %s, want %d", cl, len(body))
	}
}

func TestSPAHandler_NoInjectionWhenDisabled(t *testing.T) {
	h := NewSPAHandler(newTestFS(), "")
	resp := serve(t, h, "/")
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "googletagmanager") {
		t.Errorf("expected no gtag injection when GA disabled, got:\n%s", body)
	}
}

func TestSPAHandler_RejectsMalformedGAID(t *testing.T) {
	// A value with markup characters must never reach the page.
	h := NewSPAHandler(newTestFS(), `"><script>evil()</script>`)
	resp := serve(t, h, "/")
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "evil()") || strings.Contains(string(body), "googletagmanager") {
		t.Errorf("malformed GA ID must not be injected, got:\n%s", body)
	}
}

func TestSPAHandler_DoesNotTouchNonHTML(t *testing.T) {
	h := NewSPAHandler(newTestFS(), "G-ABC123")
	resp := serve(t, h, "/app.js")
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "console.log('x')" {
		t.Errorf("JS asset was modified: %q", body)
	}
	if strings.Contains(string(body), "googletagmanager") {
		t.Errorf("gtag must not be injected into JS assets")
	}
}

func TestInjectBeforeHeadClose_NoHeadIsUnchanged(t *testing.T) {
	in := []byte("<html><body>no head here</body></html>")
	out := injectBeforeHeadClose(in, []byte("SNIP"))
	if string(out) != string(in) {
		t.Errorf("expected unchanged output when </head> absent, got %q", out)
	}
}

func TestIsValidGAID(t *testing.T) {
	valid := []string{"G-ABC123", "UA-1234-5", "GTM-ABCD"}
	for _, id := range valid {
		if !isValidGAID(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}
	invalid := []string{"", `"><script>`, "G ABC", "id;drop", "a.b"}
	for _, id := range invalid {
		if isValidGAID(id) {
			t.Errorf("expected %q to be invalid", id)
		}
	}
}
