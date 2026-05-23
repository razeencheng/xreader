package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

type sequenceMock struct {
	responses []ChatResponse
	idx       int
	Calls     []ChatRequest
}

func (m *sequenceMock) ChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	m.Calls = append(m.Calls, req)
	if m.idx < len(m.responses) {
		resp := m.responses[m.idx]
		m.idx++
		return resp, nil
	}
	return ChatResponse{Content: "default"}, nil
}

func setupPipelineTest(t *testing.T) (*pgxpool.Pool, *gen.Queries, int64, func()) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	queries := gen.New(pool)
	return pool, queries, userID, cleanup
}

func insertTestArticle(t *testing.T, queries *gen.Queries, ctx context.Context, userID int64, title, contentText, lang string) gen.Article {
	t.Helper()
	src, err := queries.CreateSource(ctx, gen.CreateSourceParams{
		UserID:        userID,
		Kind:          "rss",
		Url:           "https://example.com/" + title,
		NormalizedUrl: "https://example.com/" + title,
		Title:         "Feed",
		Health:        "unknown",
	})
	require.NoError(t, err)

	a, err := queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     title,
		Link:           "https://example.com/" + title,
		NormalizedLink: "https://example.com/" + title,
		Title:          title,
		Language:       lang,
		ContentHtml:    "<p>" + contentText + "</p>",
		ContentText:    contentText,
		PublishedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)
	return a
}

func TestEagerJob_SameLanguage_SkipsTitleTranslation(t *testing.T) {
	pool, queries, userID, cleanup := setupPipelineTest(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	longText := strings.Repeat("这是一段很长的中文文本用于测试。", 30)
	article := insertTestArticle(t, queries, ctx, userID, "中文标题", longText, "zh-CN")

	mock := &sequenceMock{responses: []ChatResponse{{Content: "• 要点一\n• 要点二\n• 要点三"}}}
	job := NewEagerJob(pool, mock, article.ID, "zh-CN")
	require.NoError(t, job.Run(ctx))

	ai, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{
		ArticleID:      article.ID,
		TargetLanguage: "zh-CN",
	})
	require.NoError(t, err)
	require.Equal(t, "中文标题", ai.TitleTranslated)
	require.Equal(t, "done", ai.SummaryStatus)
	require.Contains(t, ai.Summary, "要点")
	require.Len(t, mock.Calls, 1)
}

func TestEagerJob_ShortItem_SkipsSummary(t *testing.T) {
	pool, queries, userID, cleanup := setupPipelineTest(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	article := insertTestArticle(t, queries, ctx, userID, "Short Post", "just a tweet", "en")

	mock := &sequenceMock{responses: []ChatResponse{{Content: "翻译后的标题"}}}
	job := NewEagerJob(pool, mock, article.ID, "zh-CN")
	require.NoError(t, job.Run(ctx))

	ai, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{
		ArticleID:      article.ID,
		TargetLanguage: "zh-CN",
	})
	require.NoError(t, err)
	require.Equal(t, "skipped", ai.SummaryStatus)
	require.Equal(t, "short", ai.SummarySkipReason)
	require.Equal(t, "翻译后的标题", ai.TitleTranslated)
	require.Len(t, mock.Calls, 1)
}

func TestEagerJob_TranslatesTitleAndSummary_Combined(t *testing.T) {
	pool, queries, userID, cleanup := setupPipelineTest(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	longText := strings.Repeat("This is a long English text for testing purposes. ", 20)
	article := insertTestArticle(t, queries, ctx, userID, "English Title", longText, "en")

	mock := &sequenceMock{responses: []ChatResponse{
		{Content: "TITLE: 英文标题\nSUMMARY: 这是一段摘要"},
	}}
	job := NewEagerJob(pool, mock, article.ID, "zh-CN")
	require.NoError(t, job.Run(ctx))

	ai, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{
		ArticleID:      article.ID,
		TargetLanguage: "zh-CN",
	})
	require.NoError(t, err)
	require.Equal(t, "英文标题", ai.TitleTranslated)
	require.Equal(t, "done", ai.SummaryStatus)
	require.Equal(t, "这是一段摘要", ai.Summary)
	require.Len(t, mock.Calls, 1)
}

func TestParseCombinedResponse(t *testing.T) {
	title, summary := parseCombinedResponse("TITLE: 测试标题\nSUMMARY: 这是摘要内容")
	require.Equal(t, "测试标题", title)
	require.Equal(t, "这是摘要内容", summary)

	title2, summary2 := parseCombinedResponse("TITLE: 标题\n\n这是一段没有前缀的摘要")
	require.Equal(t, "标题", title2)
	require.Equal(t, "这是一段没有前缀的摘要", summary2)
}

func TestEagerTranslatesEnglishTitleOnChineseBody(t *testing.T) {
	ctx := context.Background()
	pool, queries, userID, cleanup := setupPipelineTest(t)
	t.Cleanup(cleanup)

	body := strings.Repeat("这是一段中文正文内容用于测试摘要与语言检测。", 20)
	art := insertTestArticle(t, queries, ctx, userID, "Breaking News: AI Progress", body, "en")

	// English title + Chinese body: the title needs translation but the body
	// is already the target language, so the pipeline takes the title-only
	// path (no summary call). The title-translation prompt returns the bare
	// translated title.
	mock := &sequenceMock{responses: []ChatResponse{
		{Content: "突发新闻：AI 的进展"},
	}}
	job := NewEagerJob(pool, mock, art.ID, "zh-CN")
	require.NoError(t, job.Run(ctx))

	row, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: art.ID, TargetLanguage: "zh-CN"})
	require.NoError(t, err)
	require.Equal(t, "突发新闻：AI 的进展", row.TitleTranslated)
	require.NotEqual(t, art.Title, row.TitleTranslated)
}

func TestEagerCombinedParseFailureDoesNotPoisonTitle(t *testing.T) {
	ctx := context.Background()
	pool, queries, userID, cleanup := setupPipelineTest(t)
	t.Cleanup(cleanup)

	body := strings.Repeat("This is a long English body used to require a summary call. ", 20)
	art := insertTestArticle(t, queries, ctx, userID, "Some English Headline", body, "en")

	mock := &sequenceMock{responses: []ChatResponse{{Content: "text without the expected prefixes"}}}
	job := NewEagerJob(pool, mock, art.ID, "zh-CN")
	require.NoError(t, job.Run(ctx))

	row, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: art.ID, TargetLanguage: "zh-CN"})
	require.NoError(t, err)
	require.NotEqual(t, art.Title, row.TitleTranslated)
	require.Equal(t, "", row.TitleTranslated)
}

func TestEagerKeepsOriginalWhenTitleAlreadyTargetLang(t *testing.T) {
	ctx := context.Background()
	pool, queries, userID, cleanup := setupPipelineTest(t)
	t.Cleanup(cleanup)

	body := strings.Repeat("这是中文正文。", 50)
	art := insertTestArticle(t, queries, ctx, userID, "人工智能最新进展报道", body, "zh-CN")

	mock := &sequenceMock{}
	job := NewEagerJob(pool, mock, art.ID, "zh-CN")
	require.NoError(t, job.Run(ctx))

	row, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: art.ID, TargetLanguage: "zh-CN"})
	require.NoError(t, err)
	require.Equal(t, art.Title, row.TitleTranslated)
}

func TestEagerEnglishTitleEnUSUserNotSpuriouslyTranslated(t *testing.T) {
	ctx := context.Background()
	pool, queries, userID, cleanup := setupPipelineTest(t)
	t.Cleanup(cleanup)

	body := strings.Repeat("This is a long English body for an English-native user. ", 20)
	art := insertTestArticle(t, queries, ctx, userID, "An English Headline", body, "en")

	mock := &sequenceMock{}
	job := NewEagerJob(pool, mock, art.ID, "en-US")
	require.NoError(t, job.Run(ctx))

	row, err := queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: art.ID, TargetLanguage: "en-US"})
	require.NoError(t, err)
	require.Equal(t, art.Title, row.TitleTranslated)
}
