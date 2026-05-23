package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(svc *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(svc, nil)
	r.GET("/api/auth/github", h.BeginLogin)
	r.GET("/api/auth/callback", h.HandleCallback)
	return r
}

func TestHandler_BeginLogin_RedirectsToGitHub(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 1, Username: "test"},
		[]string{"test"},
	)
	r := setupTestRouter(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	loc := w.Header().Get("Location")
	require.Contains(t, loc, "github.com/login/oauth/authorize")

	// Should also set the xreader_oauth_state cookie.
	cookies := w.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "xreader_oauth_state" {
			stateCookie = c
		}
	}
	require.NotNil(t, stateCookie, "should set xreader_oauth_state cookie")
	require.True(t, stateCookie.HttpOnly)
	require.NotEmpty(t, stateCookie.Value)
}

func TestHandler_Callback_HappyPath_SetsCookie(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 456, Username: "alice"},
		[]string{"alice"},
	)

	// Generate a valid state token.
	state, err := svc.CookieState.Generate()
	require.NoError(t, err)

	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state="+state+"&code=test-code", nil)
	// Set the state cookie to simulate browser behavior.
	req.AddCookie(&http.Cookie{Name: "xreader_oauth_state", Value: state})
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	cookies := w.Result().Cookies()

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "xreader_session" {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie, "should set xreader_session cookie")
	assert.Equal(t, "session-123", sessionCookie.Value)
}

func TestHandler_Callback_DeniedUser_Returns403(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 123, Username: "stranger"},
		[]string{"alice"},
	)

	state, err := svc.CookieState.Generate()
	require.NoError(t, err)

	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state="+state+"&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "xreader_oauth_state", Value: state})
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandler_Callback_BadState_Returns400(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 123, Username: "alice"},
		[]string{"alice"},
	)

	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=wrong&code=test-code", nil)
	// Set a mismatched cookie.
	req.AddCookie(&http.Cookie{Name: "xreader_oauth_state", Value: "also-wrong"})
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Callback_MissingStateCookie_Returns400(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 123, Username: "alice"},
		[]string{"alice"},
	)

	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=some-state&code=test-code", nil)
	// No cookie set.
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
