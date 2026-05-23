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

func TestArticleAI_InsertAndRead(t *testing.T) {
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

	now := time.Now().UTC()
	article, err := queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     "guid-1",
		Link:           "https://example.com/post",
		NormalizedLink: "https://example.com/post",
		Title:          "Test Article",
		Language:       "en",
		ContentHtml:    "<p>Hello</p>",
		ContentText:    "Hello",
		PublishedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	})
	require.NoError(t, err)

	err = queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{
		ArticleID:      article.ID,
		TargetLanguage: "zh-CN",
	})
	require.NoError(t, err)

	err = queries.UpsertTitleTranslation(ctx, gen.UpsertTitleTranslationParams{
		ArticleID:       article.ID,
		TargetLanguage:  "zh-CN",
		TitleTranslated: "测试文章",
	})
	require.NoError(t, err)

	err = queries.UpsertSummary(ctx, gen.UpsertSummaryParams{
		ArticleID:         article.ID,
		TargetLanguage:    "zh-CN",
		Summary:           "这是要点",
		SummaryStatus:     "done",
		SummarySkipReason: "",
	})
	require.NoError(t, err)

	aiRow, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{
		ArticleID:      article.ID,
		TargetLanguage: "zh-CN",
	})
	require.NoError(t, err)
	require.Equal(t, article.ID, aiRow.ArticleID)
	require.Equal(t, "zh-CN", aiRow.TargetLanguage)
	require.Equal(t, "测试文章", aiRow.TitleTranslated)
	require.Equal(t, "这是要点", aiRow.Summary)
	require.Equal(t, "done", aiRow.SummaryStatus)
	require.Equal(t, "", aiRow.SummarySkipReason)
}
