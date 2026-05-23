package highlight

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/middleware"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func setupHighlightTest(t *testing.T) (*gin.Engine, *HighlightHandler, *HighlightService, *gen.Queries, int64, int64, int64, func()) {
	t.Helper()
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)

	var userID int64
	err := pool.QueryRow(ctx, "INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id", 1, "owner", "user").Scan(&userID)
	require.NoError(t, err)

	var otherUserID int64
	err = pool.QueryRow(ctx, "INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id", 2, "other", "user").Scan(&otherUserID)
	require.NoError(t, err)

	queries := gen.New(pool)
	source, err := queries.CreateSource(ctx, gen.CreateSourceParams{
		UserID:        userID,
		Kind:          "rss",
		Url:           "https://example.com/feed.xml",
		NormalizedUrl: "https://example.com/feed.xml",
		Title:         "Test Feed",
		Health:        "unknown",
	})
	require.NoError(t, err)

	article, err := queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       source.ID,
		ExternalID:     "article-1",
		Link:           "https://example.com/article-1",
		NormalizedLink: "https://example.com/article-1",
		Title:          "Article One",
		Language:       "en",
		ContentHtml:    "<p>Hello</p>",
		ContentText:    "Hello",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHighlightHandler(NewHighlightService(pool))
	return r, h, h.Service, queries, userID, otherUserID, article.ID, cleanup
}

func withHighlightUser(userID int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user", &middleware.User{ID: userID, GitHubUsername: "testuser", Role: "user"})
		c.Next()
	}
}

func insertHighlight(t *testing.T, svc *HighlightService, ctx context.Context, userID, articleID int64) gen.Highlight {
	t.Helper()
	note := "initial note"
	created, err := svc.Create(ctx, userID, userID, CreateParams{
		ArticleID:       articleID,
		Layer:           "original",
		ParagraphIndex:  0,
		TextStartOffset: 0,
		TextEndOffset:   5,
		QuotedText:      "Hello",
		Note:            &note,
	})
	require.NoError(t, err)
	return *created
}

func TestHighlightService_CRUD(t *testing.T) {
	_, _, svc, queries, userID, _, articleID, cleanup := setupHighlightTest(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	created := insertHighlight(t, svc, ctx, userID, articleID)
	require.Equal(t, "original", created.Layer)

	list, err := svc.ListByArticle(ctx, userID, articleID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, created.ID, list[0].ID)

	note := "updated note"
	require.NoError(t, svc.UpdateNote(ctx, userID, created.ID, note))

	updated, err := queries.GetHighlight(ctx, gen.GetHighlightParams{ID: created.ID, UserID: userID})
	require.NoError(t, err)
	require.Equal(t, note, updated.Note)

	require.NoError(t, svc.Delete(ctx, userID, created.ID))

	_, err = queries.GetHighlight(ctx, gen.GetHighlightParams{ID: created.ID, UserID: userID})
	require.Error(t, err)
}

func TestHighlightService_RejectsWrongOwner(t *testing.T) {
	r, handler, svc, _, userID, otherUserID, articleID, cleanup := setupHighlightTest(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	created := insertHighlight(t, svc, ctx, userID, articleID)

	err := svc.Delete(ctx, otherUserID, created.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, errNotFound))

	r.Use(withHighlightUser(otherUserID))
	r.PATCH("/api/highlights/:id", handler.UpdateNote)
	r.DELETE("/api/highlights/:id", handler.Delete)

	body, _ := json.Marshal(map[string]string{"note": "not yours"})
	w := httptest.NewRecorder()
	req, err := http.NewRequest("PATCH", fmt.Sprintf("/api/highlights/%d", created.ID), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	req, err = http.NewRequest("DELETE", fmt.Sprintf("/api/highlights/%d", created.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}
