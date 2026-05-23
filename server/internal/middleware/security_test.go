package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func cspFor(t *testing.T, gaEnabled bool) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders(gaEnabled))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Header().Get("Content-Security-Policy")
}

func TestSecurityHeaders_GADisabledLocksDownCSP(t *testing.T) {
	csp := cspFor(t, false)
	if !strings.Contains(csp, "connect-src 'self';") && !strings.HasSuffix(csp, "connect-src 'self'") {
		// connect-src must remain same-origin only.
		if !strings.Contains(csp, "connect-src 'self'") {
			t.Errorf("expected connect-src 'self', got: %s", csp)
		}
	}
	if strings.Contains(csp, "googletagmanager") || strings.Contains(csp, "google-analytics") {
		t.Errorf("CSP must not allow Google domains when GA disabled, got: %s", csp)
	}
}

func TestSecurityHeaders_GAEnabledAllowsGoogle(t *testing.T) {
	csp := cspFor(t, true)
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline' https://www.googletagmanager.com") {
		t.Errorf("expected googletagmanager in script-src, got: %s", csp)
	}
	if !strings.Contains(csp, "https://www.google-analytics.com") {
		t.Errorf("expected google-analytics in connect-src, got: %s", csp)
	}
}
