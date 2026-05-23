package article

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgtype"
    "github.com/razeencheng/xreader/db/gen"
    "github.com/razeencheng/xreader/internal/ai"
    "github.com/razeencheng/xreader/internal/middleware"
    "github.com/razeencheng/xreader/internal/testutil"
    "github.com/stretchr/testify/require"
)

func TestBodyRetry_ResetsTranslation(t *testing.T) {
    ctx := context.Background()
    pool, cleanup := testutil.SetupTestDB(t, ctx)
    t.Cleanup(cleanup)

    var userID int64
    err := pool.QueryRow(ctx,
        "INSERT INTO users (github_id, github_username, role, native_language) VALUES ($1, $2, $3, $4) RETURNING id",
        1, "testuser", "user", "zh-CN",
    ).Scan(&userID)
    require.NoError(t, err)

    queries := gen.New(pool)
    src, err := queries.CreateSource(ctx, gen.CreateSourceParams{
        UserID: userID, Kind: "rss",
        Url: "https://example.com/retry-feed", NormalizedUrl: "https://example.com/retry-feed",
        Title: "Feed", Health: "unknown",
    })
    require.NoError(t, err)

    article, err := queries.CreateArticle(ctx, gen.CreateArticleParams{
        SourceID: src.ID, ExternalID: "retry-1",
        Link: "https://example.com/retry", NormalizedLink: "https://example.com/retry",
        Title: "Retry Test", Language: "en",
        ContentHtml: "<p>Hello</p>", ContentText: "Hello",
        PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
        FetchedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
    })
    require.NoError(t, err)

    // Set up existing translation
    err = queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{
        ArticleID: article.ID, TargetLanguage: "zh-CN",
    })
    require.NoError(t, err)

    cached, _ := json.Marshal([]ai.TranslatedParagraph{
        {Index: 0, Original: "Hello", Translation: "你好"},
    })
    err = queries.SetBodyTranslationContent(ctx, gen.SetBodyTranslationContentParams{
        ArticleID:              article.ID,
        TargetLanguage:         "zh-CN",
        BodyTranslationContent: cached,
    })
    require.NoError(t, err)

    // Verify it's done
    aiRow, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{
        ArticleID: article.ID, TargetLanguage: "zh-CN",
    })
    require.NoError(t, err)
    require.Equal(t, "done", aiRow.BodyTranslationStatus)

    // Hit retry endpoint
    handler := NewBodyRetryHandler(pool)
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set("user", &middleware.User{ID: userID, NativeLanguage: "zh-CN"})
        c.Next()
    })
    r.POST("/api/articles/:id/body-translation/retry", handler.Retry)

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", fmt.Sprintf("/api/articles/%d/body-translation/retry", article.ID), nil)
    r.ServeHTTP(w, req)
    require.Equal(t, http.StatusOK, w.Code)

    // Verify reset
    aiRow, err = queries.GetArticleAI(ctx, gen.GetArticleAIParams{
        ArticleID: article.ID, TargetLanguage: "zh-CN",
    })
    require.NoError(t, err)
    require.Equal(t, "none", aiRow.BodyTranslationStatus)
    require.Nil(t, aiRow.BodyTranslationContent)
}
