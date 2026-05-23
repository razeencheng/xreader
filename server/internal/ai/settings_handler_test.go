package ai

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/internal/middleware"
	"github.com/stretchr/testify/require"
)

func TestSettingsHandlerUpdateRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := NewMemorySettingsRepository()
	service := NewSettingsService(repo)
	handler := NewSettingsHandler(service)
	router := gin.New()
	router.PATCH("/api/ai/settings", func(c *gin.Context) {
		c.Set("user", &middleware.User{ID: 1, Role: "user"})
		handler.Update(c)
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/ai/settings", bytes.NewBufferString(`{"endpoint":"https://relay.example.com","model":"qwen-turbo","api_key":"sk-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	current, err := repo.LoadAISettings(context.Background())
	require.NoError(t, err)
	_, resolvedErr := service.LoadResolved(context.Background())
	require.Empty(t, current.Endpoint)
	require.Error(t, resolvedErr)
}

func TestSettingsHandlerUpdateAllowsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := NewSettingsService(NewMemorySettingsRepository())
	handler := NewSettingsHandler(service)
	router := gin.New()
	router.PATCH("/api/ai/settings", func(c *gin.Context) {
		c.Set("user", &middleware.User{ID: 1, Role: "admin"})
		handler.Update(c)
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/ai/settings", bytes.NewBufferString(`{"endpoint":"https://relay.example.com","model":"qwen-turbo","api_key":"sk-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"endpoint":"https://relay.example.com/v1"`)
}
