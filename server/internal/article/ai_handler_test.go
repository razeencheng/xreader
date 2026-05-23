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
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/middleware"
	"github.com/stretchr/testify/require"
)

func TestAIHandler_DefaultsToCurrentUsersNativeLanguage(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	_, err := pool.Exec(ctx, "UPDATE users SET native_language = $2 WHERE id = $1", userID, "ja-JP")
	require.NoError(t, err)

	article := insertHandlerArticle(t, queries, ctx, sourceID, "ai-native-language", time.Now())
	require.NoError(t, queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{
		ArticleID:      article.ID,
		TargetLanguage: "zh-CN",
	}))
	require.NoError(t, queries.UpsertTitleTranslation(ctx, gen.UpsertTitleTranslationParams{
		ArticleID:       article.ID,
		TargetLanguage:  "zh-CN",
		TitleTranslated: "中文标题",
	}))
	require.NoError(t, queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{
		ArticleID:      article.ID,
		TargetLanguage: "ja-JP",
	}))
	require.NoError(t, queries.UpsertTitleTranslation(ctx, gen.UpsertTitleTranslationParams{
		ArticleID:       article.ID,
		TargetLanguage:  "ja-JP",
		TitleTranslated: "日本語タイトル",
	}))

	handler := NewAIHandler(pool)
	r.Use(func(c *gin.Context) {
		c.Set("user", &middleware.User{ID: userID, GitHubUsername: "testuser", Role: "user", NativeLanguage: "ja-JP"})
		c.Next()
	})
	r.GET("/api/articles/:id/ai", handler.GetArticleAI)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/ai", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "日本語タイトル", resp["title_translated"])
}
