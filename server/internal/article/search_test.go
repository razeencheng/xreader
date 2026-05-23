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

func TestSearch_MatchesTitleAndBody(t *testing.T) {
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
		ExternalID:     "1",
		Link:           "https://example.com/k8s",
		NormalizedLink: "https://example.com/k8s",
		Title:          "Kubernetes Best Practices",
		Language:       "en",
		ContentHtml:    "<p>Deploy containers efficiently</p>",
		ContentText:    "Deploy containers efficiently",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	_, err = queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     "2",
		Link:           "https://example.com/deploy",
		NormalizedLink: "https://example.com/deploy",
		Title:          "Container Deployment Guide",
		Language:       "en",
		ContentHtml:    "<p>Learn to deploy kubernetes clusters at scale</p>",
		ContentText:    "Learn to deploy kubernetes clusters at scale",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	_, err = queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     "3",
		Link:           "https://example.com/go",
		NormalizedLink: "https://example.com/go",
		Title:          "Go Programming Tips",
		Language:       "en",
		ContentHtml:    "<p>Write better Go code</p>",
		ContentText:    "Write better Go code",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	svc := NewArticleService(pool)
	result, err := svc.Search(ctx, userID, "kubernetes")
	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestSearch_ReturnsHeadlineSnippet(t *testing.T) {
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
		ExternalID:     "1",
		Link:           "https://example.com/post",
		NormalizedLink: "https://example.com/post",
		Title:          "Database Performance",
		Language:       "en",
		ContentHtml:    "<p>PostgreSQL provides excellent full text search capabilities with tsvector and tsquery</p>",
		ContentText:    "PostgreSQL provides excellent full text search capabilities with tsvector and tsquery",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	rows, err := queries.SearchArticles(ctx, gen.SearchArticlesParams{
		UserID: userID,
		Q:      "postgresql",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Contains(t, string(rows[0].Headline), "<b>PostgreSQL</b>")
}

func TestSearch_CJKBigramMatchesChinese(t *testing.T) {
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
		ExternalID:     "1",
		Link:           "https://example.com/quantum",
		NormalizedLink: "https://example.com/quantum",
		Title:          "量子加密技术的最新进展",
		Language:       "zh",
		ContentHtml:    "<p>量子计算与密码学的交叉研究</p>",
		ContentText:    "量子计算与密码学的交叉研究",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	_, err = queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     "2",
		Link:           "https://example.com/ai",
		NormalizedLink: "https://example.com/ai",
		Title:          "人工智能发展报告",
		Language:       "zh",
		ContentHtml:    "<p>深度学习模型的最新突破</p>",
		ContentText:    "深度学习模型的最新突破",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	svc := NewArticleService(pool)

	// Search for "量子加密" — should match the first article via CJK bigrams
	result, err := svc.Search(ctx, userID, "量子加密")
	require.NoError(t, err)
	require.Len(t, result, 1, "expected exactly 1 result for CJK bigram search '量子加密'")
	require.Equal(t, "量子加密技术的最新进展", result[0].Title)

	// Also verify English search still works alongside CJK
	_, err = queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     "3",
		Link:           "https://example.com/mixed",
		NormalizedLink: "https://example.com/mixed",
		Title:          "Quantum Computing Advances",
		Language:       "en",
		ContentHtml:    "<p>Latest in quantum computing</p>",
		ContentText:    "Latest in quantum computing",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	englishResult, err := svc.Search(ctx, userID, "quantum")
	require.NoError(t, err)
	require.Len(t, englishResult, 1, "expected exactly 1 result for English search 'quantum'")
	require.Equal(t, "Quantum Computing Advances", englishResult[0].Title)
}

func TestSearch_NoResults(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	svc := NewArticleService(pool)
	result, err := svc.Search(ctx, userID, "nonexistent")
	require.NoError(t, err)
	require.Len(t, result, 0)
}
