package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/internal/middleware"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func setupHandlerTest(t *testing.T) (*gin.Engine, *SourceHandler, int64, func()) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	adapter := &mockAdapter{
		validateMeta: SourceMetadata{Title: "Test Feed", LanguageHint: "en"},
	}
	svc := NewSourceService(pool, adapter)
	handler := NewSourceHandler(svc, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	return r, handler, userID, cleanup
}

func withUser(userID int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user", &middleware.User{ID: userID, GitHubUsername: "testuser", Role: "user"})
		c.Next()
	}
}

func TestHandler_POST_Creates_Source(t *testing.T) {
	r, handler, userID, cleanup := setupHandlerTest(t)
	t.Cleanup(cleanup)

	r.Use(withUser(userID))
	r.POST("/api/sources", handler.Create)

	body, _ := json.Marshal(createSourceRequest{URL: "https://example.com/feed.xml"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestHandler_POST_MissingURL_Returns400(t *testing.T) {
	r, handler, userID, cleanup := setupHandlerTest(t)
	t.Cleanup(cleanup)

	r.Use(withUser(userID))
	r.POST("/api/sources", handler.Create)

	body, _ := json.Marshal(map[string]string{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GET_ListsSources(t *testing.T) {
	r, handler, userID, cleanup := setupHandlerTest(t)
	t.Cleanup(cleanup)

	r.Use(withUser(userID))
	r.POST("/api/sources", handler.Create)
	r.GET("/api/sources", handler.List)

	body, _ := json.Marshal(createSourceRequest{URL: "https://example.com/feed.xml"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/sources", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var sources []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sources))
	require.Len(t, sources, 1)
}

func TestHandler_POST_RefreshFetchesSource(t *testing.T) {
	r, handler, userID, cleanup := setupHandlerTest(t)
	t.Cleanup(cleanup)

	adapter := handler.Service.adapters["rss"].(*mockAdapter)
	adapter.fetchItems = []RawItem{
		{
			ExternalID:  "refresh-1",
			Link:        "https://example.com/refresh-1",
			Title:       "Refresh Item",
			ContentHTML: "<p>fresh</p>",
			PublishedAt: time.Now(),
		},
	}

	r.Use(withUser(userID))
	r.POST("/api/sources", handler.Create)
	r.POST("/api/sources/:id/refresh", handler.Refresh)
	r.GET("/api/sources", handler.List)

	body, _ := json.Marshal(createSourceRequest{URL: "https://example.com/feed.xml"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/sources/"+formatID(created["id"])+"/refresh", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/sources", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var sources []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sources))
	require.Equal(t, "ok", sources[0]["health"])
	require.EqualValues(t, 0, sources[0]["consecutive_fails"])
	require.NotEmpty(t, sources[0]["last_fetched_at"])
}

func TestHandler_DELETE_OwnerOnly(t *testing.T) {
	r, handler, userID, cleanup := setupHandlerTest(t)
	t.Cleanup(cleanup)

	r.Use(withUser(userID))
	r.POST("/api/sources", handler.Create)
	r.DELETE("/api/sources/:id", handler.Delete)

	body, _ := json.Marshal(createSourceRequest{URL: "https://example.com/feed.xml"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	r2 := gin.New()
	r2.Use(withUser(userID + 999))
	r2.DELETE("/api/sources/:id", handler.Delete)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/sources/"+formatID(created["id"]), nil)
	r2.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_GetJobOwnerOnly(t *testing.T) {
	r, handler, userID, cleanup := setupHandlerTest(t)
	t.Cleanup(cleanup)

	jobID := fmt.Sprintf("import-%d-123", userID)
	handler.JobStore.Set(jobID, JobStatus{Status: "running", Total: 1})

	r.Use(withUser(userID))
	r.GET("/api/sources/jobs/:jobID", handler.GetJob)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/sources/jobs/"+jobID, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetJobHidesOtherUsersJobs(t *testing.T) {
	r, handler, userID, cleanup := setupHandlerTest(t)
	t.Cleanup(cleanup)

	jobID := fmt.Sprintf("import-%d-123", userID)
	handler.JobStore.Set(jobID, JobStatus{Status: "running", Total: 1})

	r.Use(withUser(userID + 999))
	r.GET("/api/sources/jobs/:jobID", handler.GetJob)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/sources/jobs/"+jobID, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func formatID(v any) string {
	switch id := v.(type) {
	case float64:
		return fmt.Sprintf("%d", int64(id))
	default:
		return fmt.Sprintf("%v", id)
	}
}
