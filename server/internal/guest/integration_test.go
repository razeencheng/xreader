package guest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/razeencheng/xreader/internal/platform"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuestFlowIntegration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	// Setup: enable guest mode + create admin + add a source with articles
	_, err := pool.Exec(ctx, "INSERT INTO settings (key, value) VALUES ('guest_mode_enabled', 'true')")
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO users (github_id, github_username, role, native_language, density_pref, theme_pref)
		 VALUES (1001, 'testadmin', 'admin', 'zh-CN', 'comfortable', 'system')`)
	require.NoError(t, err)

	var adminID int64
	err = pool.QueryRow(ctx, "SELECT id FROM users WHERE github_username = 'testadmin'").Scan(&adminID)
	require.NoError(t, err)

	// Create a source for admin
	_, err = pool.Exec(ctx,
		`INSERT INTO sources (user_id, kind, url, normalized_url, title)
		 VALUES ($1, 'rss', 'https://example.com/feed', 'example.com/feed', 'Test Feed')`, adminID)
	require.NoError(t, err)

	var sourceID int64
	err = pool.QueryRow(ctx, "SELECT id FROM sources WHERE user_id = $1", adminID).Scan(&sourceID)
	require.NoError(t, err)

	// Create an article
	_, err = pool.Exec(ctx,
		`INSERT INTO articles (source_id, external_id, link, normalized_link, title, language, content_html, content_text, published_at)
		 VALUES ($1, 'ext1', 'https://example.com/1', 'example.com/1', 'Test Article', 'en', '<p>Hello</p>', 'Hello', now())`, sourceID)
	require.NoError(t, err)

	// Build router
	router := platform.NewRouter(platform.RouterDeps{
		Pool:          pool,
		SessionSecret: "test-secret-that-is-at-least-32-chars-long!!",
	})

	// 1. GET /api/guest/status → enabled
	req := httptest.NewRequest("GET", "/api/guest/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":true`)

	// 2. GET /api/articles (no cookie) → should create guest + return articles
	req = httptest.NewRequest("GET", "/api/articles", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "Test Article")

	// Grab the session cookie
	cookies := w.Result().Cookies()
	require.NotEmpty(t, cookies)
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "xreader_session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie, "should have xreader_session cookie")

	// 3. GET /api/sources → should return admin's sources
	req = httptest.NewRequest("GET", "/api/sources", nil)
	req.AddCookie(sessionCookie)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "Test Feed")

	// 4. POST /api/sources (create) → 403 (GuestReadOnly)
	req = httptest.NewRequest("POST", "/api/sources", nil)
	req.AddCookie(sessionCookie)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)

	// 5. GET /api/auth/me → should return guest user
	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(sessionCookie)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"role":"guest"`)

	// 6. Guest mode disabled → existing guest gets 401
	_, _ = pool.Exec(ctx, "UPDATE settings SET value = 'false' WHERE key = 'guest_mode_enabled'")
	req = httptest.NewRequest("GET", "/api/articles", nil)
	req.AddCookie(sessionCookie)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}
