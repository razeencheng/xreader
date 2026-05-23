package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/internal/middleware"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func setupUserHandlerTest(t *testing.T) (*gin.Engine, *Handler, int64, func()) {
	t.Helper()
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)

	var userID int64
	err := pool.QueryRow(ctx, `INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id`, 1, "testuser", "user").Scan(&userID)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r, NewHandler(pool), userID, cleanup
}

func withUser(userID int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user", &middleware.User{ID: userID, GitHubUsername: "testuser", Role: "user"})
		c.Next()
	}
}

func TestHandler_UpdateMeAndGetMe(t *testing.T) {
	r, handler, userID, cleanup := setupUserHandlerTest(t)
	t.Cleanup(cleanup)

	r.Use(withUser(userID))
	r.GET("/api/users/me", handler.GetMe)
	r.PATCH("/api/users/me", handler.UpdateMe)

	payload, err := json.Marshal(map[string]string{
		"native_language": "ja-JP",
		"density_pref":    "compact",
		"theme_pref":      "dark",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("PATCH", "/api/users/me", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var updateResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updateResp))
	require.Equal(t, "ja-JP", updateResp["native_language"])
	require.Equal(t, "compact", updateResp["density_pref"])
	require.Equal(t, "dark", updateResp["theme_pref"])

	w = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/api/users/me", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var getResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &getResp))
	require.Equal(t, "ja-JP", getResp["native_language"])
	require.Equal(t, "compact", getResp["density_pref"])
	require.Equal(t, "dark", getResp["theme_pref"])
}
