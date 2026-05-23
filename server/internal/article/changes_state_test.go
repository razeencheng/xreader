package article

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestToArticleChangeResponseMapsState(t *testing.T) {
	ts := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	row := gen.ListStateChangesSinceRow{
		ArticleID: 77,
		ChangedAt: pgtype.Timestamptz{Time: ts, Valid: true},
		IsRead:    true,
		IsStarred: true,
	}

	got := toArticleChangeResponse(row)

	require.Equal(t, int64(77), got.ArticleID)
	require.Equal(t, "2026-05-16T01:02:03Z", got.ChangedAt)
	require.True(t, got.IsRead)
	require.True(t, got.IsStarred)
}

func TestChangesHandlerReturnsReadAndStarred(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	var userID int64
	require.NoError(t, pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID))

	q := gen.New(pool)
	src, err := q.CreateSource(ctx, gen.CreateSourceParams{
		UserID: userID, Kind: "rss", Url: "https://example.com/feed.xml",
		NormalizedUrl: "https://example.com/feed.xml", Title: "Feed", Health: "unknown",
	})
	require.NoError(t, err)

	mk := func(ext string) gen.Article {
		a, e := q.CreateArticle(ctx, gen.CreateArticleParams{
			SourceID: src.ID, ExternalID: ext, Link: "https://example.com/" + ext,
			NormalizedLink: "https://example.com/" + ext, Title: ext, Language: "en",
			ContentHtml: "<p>x</p>", ContentText: "x",
			PublishedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			FetchedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		require.NoError(t, e)
		return a
	}
	readArt := mk("read-one")
	starArt := mk("star-one")

	since := time.Now().UTC().Add(-time.Minute)
	_, err = q.SetArticleRead(ctx, gen.SetArticleReadParams{UserID: userID, ID: readArt.ID, IsRead: true})
	require.NoError(t, err)
	require.NoError(t, q.RecordStateChange(ctx, gen.RecordStateChangeParams{UserID: userID, ArticleID: readArt.ID}))
	_, err = q.SetArticleStarred(ctx, gen.SetArticleStarredParams{UserID: userID, ID: starArt.ID, IsStarred: true})
	require.NoError(t, err)
	require.NoError(t, q.RecordStateChange(ctx, gen.RecordStateChangeParams{UserID: userID, ArticleID: starArt.ID}))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := NewArticleHandler(NewArticleService(pool))
	r.GET("/api/articles/changes", withArticleUser(userID), handler.Changes)

	req := httptest.NewRequest(http.MethodGet,
		"/api/articles/changes?since="+since.Format(time.RFC3339Nano), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Items []articleChangeResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	byID := map[int64]articleChangeResponse{}
	for _, it := range body.Items {
		byID[it.ArticleID] = it
	}
	require.True(t, byID[readArt.ID].IsRead)
	require.False(t, byID[readArt.ID].IsStarred)
	require.True(t, byID[starArt.ID].IsStarred)
}
