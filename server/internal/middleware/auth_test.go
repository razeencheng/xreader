package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/internal/auth"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memorySessionStore struct {
	sessions map[string]int64
}

func (m *memorySessionStore) Create(_ context.Context, userID int64, _ string) (string, error) {
	id := fmt.Sprintf("sess-%d", userID)
	m.sessions[id] = userID
	return id, nil
}

func (m *memorySessionStore) Get(_ context.Context, sessionID string) (int64, error) {
	uid, ok := m.sessions[sessionID]
	if !ok {
		return 0, fmt.Errorf("not found")
	}
	return uid, nil
}

func (m *memorySessionStore) Delete(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *memorySessionStore) Touch(_ context.Context, _ string) error {
	return nil
}

var _ auth.SessionStore = (*memorySessionStore)(nil)

func setupAuthRouter(sessions auth.SessionStore, pool *pgxpool.Pool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("/api", RequireAuth(sessions, pool))
	authed.GET("/auth/me", func(c *gin.Context) {
		u := GetUser(c)
		c.JSON(http.StatusOK, u)
	})
	admin := authed.Group("/admin", RequireAdmin())
	admin.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"admin": true})
	})
	return r
}

func seedUser(t *testing.T, pool *pgxpool.Pool, githubID int64, username, role string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		githubID, username, role,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestAuthMiddleware_NoCookie_Returns401(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	sessions := &memorySessionStore{sessions: map[string]int64{}}
	r := setupAuthRouter(sessions, pool)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ValidSession_AllowsRequest(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	userID := seedUser(t, pool, 123, "alice", "user")
	sessions := &memorySessionStore{sessions: map[string]int64{"valid-session": userID}}
	r := setupAuthRouter(sessions, pool)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "xreader_session", Value: "valid-session"})
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "alice")
}

func TestAuthMiddleware_ExpiredSession_Returns401(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	sessions := &memorySessionStore{sessions: map[string]int64{}}
	r := setupAuthRouter(sessions, pool)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "xreader_session", Value: "expired-session"})
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAdmin_NonAdmin_Returns403(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	userID := seedUser(t, pool, 123, "alice", "user")
	sessions := &memorySessionStore{sessions: map[string]int64{"sess": userID}}
	r := setupAuthRouter(sessions, pool)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/test", nil)
	req.AddCookie(&http.Cookie{Name: "xreader_session", Value: "sess"})
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireAdmin_Admin_AllowsRequest(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	userID := seedUser(t, pool, 123, "alice", "admin")
	sessions := &memorySessionStore{sessions: map[string]int64{"sess": userID}}
	r := setupAuthRouter(sessions, pool)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/test", nil)
	req.AddCookie(&http.Cookie{Name: "xreader_session", Value: "sess"})
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
