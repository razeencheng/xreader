package article

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
	"github.com/stretchr/testify/require"
)

func TestSSE_ServesCachedTranslation(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "cached", time.Now())
	cached := []ai.TranslatedParagraph{{Index: 0, Original: "cached", Translation: "已缓存"}}
	payload, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{ArticleID: article.ID, TargetLanguage: "zh-CN"}))
	require.NoError(t, queries.SetBodyTranslationContent(ctx, gen.SetBodyTranslationContentParams{ArticleID: article.ID, TargetLanguage: "zh-CN", BodyTranslationContent: payload}))

	job := &ai.MockClient{}
	h := NewSSEHandler(pool, job, 1)

	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), `event: paragraph`)
	require.Contains(t, w.Body.String(), `data: {"index":0,"original":"cached","translation":"已缓存"}`)
	require.Contains(t, w.Body.String(), `event: done`)
	require.Contains(t, w.Body.String(), `data: {}`)
}

func TestSSE_IgnoreStaleCachedTranslationAndRebuild(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "mismatch", time.Now())
	stale := []ai.TranslatedParagraph{
		{Index: 0, Original: "First", Translation: "第一段"},
		{Index: 1, Original: "Second", Translation: "第二段"},
	}
	payload, err := json.Marshal(stale)
	require.NoError(t, err)
	require.NoError(t, queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{ArticleID: article.ID, TargetLanguage: "zh-CN"}))
	require.NoError(t, queries.SetBodyTranslationContent(ctx, gen.SetBodyTranslationContentParams{
		ArticleID: article.ID, TargetLanguage: "zh-CN", BodyTranslationContent: payload,
	}))

	job := &ai.MockClient{}
	h := NewSSEHandler(pool, job, 1)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), `event: paragraph`)
	require.Contains(t, w.Body.String(), `data: {"index":0,"original":"mismatch","translation":""}`)
	require.NotContains(t, w.Body.String(), `"index":1`)
	require.NotContains(t, w.Body.String(), `"original":"First"`)
	require.Contains(t, w.Body.String(), `event: done`)
}

func TestSSE_StreamsRequestedRangeOnly(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "range", time.Now())
	article, err := queries.UpdateArticleContent(ctx, gen.UpdateArticleContentParams{
		ID:          article.ID,
		ContentHtml: "<p>One</p><p>Two</p><p>Three</p>",
		ContentText: "One Two Three",
	})
	require.NoError(t, err)

	job := &ai.MockClient{Response: ai.ChatResponse{Content: "[1] 第二段"}}
	h := NewSSEHandler(pool, job, 10)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation?start=1&count=1", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `data: {"index":1,"original":"Two","translation":"第二段"}`)
	require.NotContains(t, w.Body.String(), `"index":0`)
	require.NotContains(t, w.Body.String(), `"index":2`)
	require.Len(t, job.Calls, 1)
	require.Contains(t, job.Calls[0].Messages[1].Content, "[1] Two")
	require.NotContains(t, job.Calls[0].Messages[1].Content, "[0] One")
	require.NotContains(t, job.Calls[0].Messages[1].Content, "[2] Three")
}

func TestSSE_UsesPartialCacheAndTranslatesOnlyMissingParagraphs(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "partial", time.Now())
	article, err := queries.UpdateArticleContent(ctx, gen.UpdateArticleContentParams{
		ID:          article.ID,
		ContentHtml: "<p>One</p><p>Two</p><p>Three</p>",
		ContentText: "One Two Three",
	})
	require.NoError(t, err)

	cached := []ai.TranslatedParagraph{{Index: 1, Original: "Two", Translation: "第二段"}}
	payload, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{ArticleID: article.ID, TargetLanguage: "zh-CN"}))
	require.NoError(t, queries.SetBodyTranslation(ctx, gen.SetBodyTranslationParams{
		ArticleID: article.ID, TargetLanguage: "zh-CN", BodyTranslationContent: payload, BodyTranslationStatus: "processing",
	}))

	job := &ai.MockClient{Response: ai.ChatResponse{Content: "[0] 第一段\n[2] 第三段"}}
	h := NewSSEHandler(pool, job, 10)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation?start=0&count=3", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `data: {"index":0,"original":"One","translation":"第一段"}`)
	require.Contains(t, w.Body.String(), `data: {"index":1,"original":"Two","translation":"第二段"}`)
	require.Contains(t, w.Body.String(), `data: {"index":2,"original":"Three","translation":"第三段"}`)
	require.Len(t, job.Calls, 1)
	require.Contains(t, job.Calls[0].Messages[1].Content, "[0] One")
	require.Contains(t, job.Calls[0].Messages[1].Content, "[2] Three")
	require.NotContains(t, job.Calls[0].Messages[1].Content, "[1] Two")
}

func TestSSE_MapsUnnumberedTranslationLinesByRequestedOrder(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "unnumbered", time.Now())
	article, err := queries.UpdateArticleContent(ctx, gen.UpdateArticleContentParams{
		ID:          article.ID,
		ContentHtml: "<p>One</p><p>Two</p>",
		ContentText: "One Two",
	})
	require.NoError(t, err)

	job := &ai.MockClient{Response: ai.ChatResponse{Content: "第一段\n第二段"}}
	h := NewSSEHandler(pool, job, 10)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation?start=0&count=2", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `data: {"index":0,"original":"One","translation":"第一段"}`)
	require.Contains(t, w.Body.String(), `data: {"index":1,"original":"Two","translation":"第二段"}`)
}

func TestSSE_KeepsWrappedNumberedParagraphTranslationsTogether(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "wrapped", time.Now())
	article, err := queries.UpdateArticleContent(ctx, gen.UpdateArticleContentParams{
		ID:          article.ID,
		ContentHtml: "<p>One</p><p>Two</p>",
		ContentText: "One Two",
	})
	require.NoError(t, err)

	job := &ai.MockClient{Response: ai.ChatResponse{Content: "[0] 第一段第一行\n继续第一段\n[1] 第二段"}}
	h := NewSSEHandler(pool, job, 10)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation?start=0&count=2", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"index":0,"original":"One","translation":"第一段第一行\n继续第一段"`)
	require.Contains(t, w.Body.String(), `data: {"index":1,"original":"Two","translation":"第二段"}`)
}

func TestSSE_PersistsPartialTranslationWithoutMarkingDone(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "partial-status", time.Now())
	article, err := queries.UpdateArticleContent(ctx, gen.UpdateArticleContentParams{
		ID:          article.ID,
		ContentHtml: "<p>One</p><p>Two</p><p>Three</p>",
		ContentText: "One Two Three",
	})
	require.NoError(t, err)

	job := &ai.MockClient{Response: ai.ChatResponse{Content: "[0] 第一段"}}
	h := NewSSEHandler(pool, job, 10)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation?start=0&count=1", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Eventually(t, func() bool {
		aiRow, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: article.ID, TargetLanguage: "zh-CN"})
		if err != nil {
			return false
		}
		var content []ai.TranslatedParagraph
		if err := json.Unmarshal(aiRow.BodyTranslationContent, &content); err != nil {
			return false
		}
		return aiRow.BodyTranslationStatus == "processing" &&
			len(content) == 1 &&
			content[0].Index == 0 &&
			content[0].Translation == "第一段"
	}, time.Second, 10*time.Millisecond)
}

func TestSSE_StreamsTranslationForNewRequest(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "new", time.Now())

	h := NewSSEHandler(pool, &ai.MockClient{}, 1)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), `event: paragraph`)
	require.Contains(t, w.Body.String(), `data: {"index":0,"original":"new","translation":""}`)
	require.Contains(t, w.Body.String(), `event: done`)
}

func TestSSE_ProcessingStreamsTranslation(t *testing.T) {
	r, _, queries, pool, userID, sourceID, cleanup := setupArticleHandlerTest(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	article := insertHandlerArticle(t, queries, ctx, sourceID, "processing", time.Now())
	require.NoError(t, queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{ArticleID: article.ID, TargetLanguage: "zh-CN"}))
	require.NoError(t, queries.SetBodyTranslationStatus(ctx, gen.SetBodyTranslationStatusParams{ArticleID: article.ID, TargetLanguage: "zh-CN", BodyTranslationStatus: "processing"}))

	h := NewSSEHandler(pool, &ai.MockClient{}, 1)
	r.Use(withArticleUser(userID))
	r.GET("/api/articles/:id/body-translation", h.BodyTranslation)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/articles/%d/body-translation", article.ID), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, w.Body.String(), `event: paragraph`)
	require.Contains(t, w.Body.String(), `data: {"index":0,"original":"processing","translation":""}`)
	require.Contains(t, w.Body.String(), `event: done`)
}
