package article

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestFTS_SearchVecPopulatedOnInsert(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	queries := gen.New(pool)
	src, err := queries.CreateSource(ctx, gen.CreateSourceParams{
		UserID:        userID,
		Kind:          "rss",
		Url:           "https://example.com/feed",
		NormalizedUrl: "https://example.com/feed",
		Title:         "Test Feed",
		Health:        "unknown",
	})
	require.NoError(t, err)

	_, err = queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     "guid-1",
		Link:           "https://example.com/post",
		NormalizedLink: "https://example.com/post",
		Title:          "PostgreSQL Full-Text Search Guide",
		Language:       "en",
		ContentHtml:    "<p>Learn about tsvector and tsquery</p>",
		ContentText:    "Learn about tsvector and tsquery",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx,
		"SELECT count(*) FROM articles WHERE search_vec @@ plainto_tsquery('simple', $1)",
		"postgresql search",
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
