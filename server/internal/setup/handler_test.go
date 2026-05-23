package setup

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.GET("/api/setup/status", h.Status)
	r.POST("/api/setup/complete", h.Complete)
	return r
}

func TestSetupHandler_Status_NeedsSetup(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	h := NewHandler(pool, "test-token")
	r := setupRouter(h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["needs_setup"])
}

func TestSetupHandler_Complete_WithToken(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	h := NewHandler(pool, "test-token-123")
	r := setupRouter(h)

	body := completeRequest{
		SetupToken:          "test-token-123",
		GitHubClientID:      "gh-client-id",
		GitHubClientSecret:  "gh-client-secret",
		GitHubCallbackURL:   "http://localhost:3000/api/auth/callback",
		AdminGitHubUsername: "testadmin",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify settings were saved
	var clientID string
	err := pool.QueryRow(ctx, "SELECT value FROM settings WHERE key = $1", "github_client_id").Scan(&clientID)
	require.NoError(t, err)
	assert.Equal(t, "gh-client-id", clientID)

	// Verify admin was seeded
	var adminCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM auth_allowlist WHERE github_username = $1", "testadmin").Scan(&adminCount)
	require.NoError(t, err)
	assert.Equal(t, 1, adminCount)

	// Verify setup status is now false
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	r.ServeHTTP(w2, req2)
	var resp map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, false, resp["needs_setup"])
}

func TestSetupHandler_Complete_WrongToken(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	h := NewHandler(pool, "correct-token")
	r := setupRouter(h)

	body := completeRequest{
		SetupToken:          "wrong-token",
		GitHubClientID:      "gh-client-id",
		GitHubClientSecret:  "gh-client-secret",
		GitHubCallbackURL:   "http://localhost:3000/api/auth/callback",
		AdminGitHubUsername: "testadmin",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSetupHandler_Complete_AlreadyDone(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	// Pre-seed an admin to simulate already-completed setup
	_, err := pool.Exec(ctx, "INSERT INTO auth_allowlist (github_username, note) VALUES ($1, $2)", "existing-admin", "pre-existing")
	require.NoError(t, err)

	h := NewHandler(pool, "test-token")
	r := setupRouter(h)

	body := completeRequest{
		SetupToken:          "test-token",
		GitHubClientID:      "gh-client-id",
		GitHubClientSecret:  "gh-client-secret",
		GitHubCallbackURL:   "http://localhost:3000/api/auth/callback",
		AdminGitHubUsername: "testadmin",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetupHandler_Complete_MissingCallbackURL(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	h := NewHandler(pool, "test-token")
	r := setupRouter(h)

	body := completeRequest{
		SetupToken:          "test-token",
		GitHubClientID:      "gh-client-id",
		GitHubClientSecret:  "gh-client-secret",
		GitHubCallbackURL:   "", // missing
		AdminGitHubUsername: "testadmin",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "github_callback_url")
}

func TestSetupHandler_Complete_PartialAIConfig(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	h := NewHandler(pool, "test-token")
	r := setupRouter(h)

	body := completeRequest{
		SetupToken:          "test-token",
		GitHubClientID:      "gh-client-id",
		GitHubClientSecret:  "gh-client-secret",
		GitHubCallbackURL:   "http://localhost:3000/api/auth/callback",
		AIEndpoint:          "https://api.openai.com", // only endpoint, missing model + key
		AdminGitHubUsername: "testadmin",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "ai_endpoint")
}

func TestSetupHandler_Complete_WithAIConfig(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	h := NewHandler(pool, "test-token")
	r := setupRouter(h)

	body := completeRequest{
		SetupToken:          "test-token",
		GitHubClientID:      "gh-client-id",
		GitHubClientSecret:  "gh-client-secret",
		GitHubCallbackURL:   "http://localhost:3000/api/auth/callback",
		AIEndpoint:          "https://api.openai.com",
		AIModel:             "gpt-4o-mini",
		AIAPIKey:            "sk-test-key-12345678",
		AdminGitHubUsername: "testadmin",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify AI settings were saved to ai_provider_settings table
	var endpoint, model string
	err := pool.QueryRow(ctx, "SELECT endpoint, model FROM ai_provider_settings WHERE id = 1").Scan(&endpoint, &model)
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1", endpoint) // normalized
	assert.Equal(t, "gpt-4o-mini", model)
}
