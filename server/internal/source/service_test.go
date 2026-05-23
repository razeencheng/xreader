package source

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

type mockAdapter struct {
	validateErr  error
	validateMeta SourceMetadata
	fetchErr     error
	fetchItems   []RawItem
}

func (m *mockAdapter) Kind() string { return "rss" }
func (m *mockAdapter) Fetch(ctx context.Context, src Source) ([]RawItem, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.fetchItems, nil
}
func (m *mockAdapter) Validate(ctx context.Context, url string) (SourceMetadata, error) {
	if m.validateErr != nil {
		return SourceMetadata{}, m.validateErr
	}
	return m.validateMeta, nil
}

type sequenceAIClient struct {
	responses []ai.ChatResponse
	calls     []ai.ChatRequest
}

func (c *sequenceAIClient) ChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	c.calls = append(c.calls, req)
	if len(c.responses) == 0 {
		return ai.ChatResponse{}, nil
	}
	resp := c.responses[0]
	c.responses = c.responses[1:]
	return resp, nil
}

func setupService(t *testing.T) (*SourceService, int64, func()) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	adapter := &mockAdapter{
		validateMeta: SourceMetadata{Title: "Test Feed", LanguageHint: "en"},
	}
	svc := NewSourceService(pool, adapter)

	return svc, userID, cleanup
}

func TestSourceService_Create_ListSuccess(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.NoError(t, err)
	require.Equal(t, "Test Feed", src.Title)
	require.Equal(t, "rss", src.Kind)

	sources, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, src.ID, sources[0].ID)
	require.EqualValues(t, 0, sources[0].UnreadCount)
}

func TestSourceService_List_TracksUnreadCounts(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "Technology")
	require.NoError(t, err)

	_, err = svc.pool.Exec(ctx, `
		INSERT INTO articles (source_id, external_id, link, normalized_link, title, language, content_html, content_text, published_at)
		VALUES
		  ($1, $2, $3, $4, $5, $6, $7, $8, now()),
		  ($1, $9, $10, $11, $12, $13, $14, $15, now())
	`,
		src.ID,
		"guid-1", "https://example.com/post-1", "https://example.com/post-1", "Post 1", "en", "<p>hi</p>", "hi",
		"guid-2", "https://example.com/post-2", "https://example.com/post-2", "Post 2", "en", "<p>bye</p>", "bye",
	)
	require.NoError(t, err)

	var articleID int64
	err = svc.pool.QueryRow(ctx, "SELECT id FROM articles WHERE normalized_link = $1", "https://example.com/post-1").Scan(&articleID)
	require.NoError(t, err)

	_, err = svc.pool.Exec(ctx, `
		INSERT INTO article_states (user_id, article_id, is_read, is_starred)
		VALUES ($1, $2, true, false)
	`, userID, articleID)
	require.NoError(t, err)

	sources, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.EqualValues(t, 1, sources[0].UnreadCount)
	require.Equal(t, "Technology", sources[0].Category)
}

func TestSourceService_List_IncludesFetchHealthMetadata(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "Technology")
	require.NoError(t, err)

	fetchedAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	successAt := time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = svc.pool.Exec(ctx, `
		UPDATE sources
		SET last_fetched_at = $2,
		    last_success_at = $3,
		    consecutive_fails = 4,
		    health = 'warn'
		WHERE id = $1
	`, src.ID, fetchedAt, successAt)
	require.NoError(t, err)

	sources, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.NotNil(t, sources[0].LastFetchedAt)
	require.NotNil(t, sources[0].LastSuccessAt)
	require.WithinDuration(t, fetchedAt, *sources[0].LastFetchedAt, time.Second)
	require.WithinDuration(t, successAt, *sources[0].LastSuccessAt, time.Second)
	require.EqualValues(t, 4, sources[0].ConsecutiveFails)
	require.Equal(t, "warn", sources[0].Health)
}

func TestSourceService_Refresh_FetchesNowAndResetsHealth(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	adapter := &mockAdapter{
		validateMeta: SourceMetadata{Title: "Test Feed", LanguageHint: "en"},
		fetchItems: []RawItem{
			{
				ExternalID:   "manual-1",
				Link:         "https://example.com/manual-1",
				Title:        "Manual Refresh",
				ContentHTML:  "<p>fresh</p>",
				LanguageHint: "en",
				PublishedAt:  time.Now(),
			},
		},
	}
	svc := NewSourceService(pool, adapter)
	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.NoError(t, err)

	inserted, err := svc.Refresh(ctx, userID, src.ID)
	require.NoError(t, err)
	require.Equal(t, 1, inserted)

	sources, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.NotNil(t, sources[0].LastFetchedAt)
	require.NotNil(t, sources[0].LastSuccessAt)
	require.EqualValues(t, 0, sources[0].ConsecutiveFails)
	require.Equal(t, "ok", sources[0].Health)
}

func TestSourceService_RefreshRunsEagerAIForInsertedArticles(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role, native_language) VALUES ($1, $2, $3, $4) RETURNING id",
		1, "testuser", "user", "zh-CN",
	).Scan(&userID)
	require.NoError(t, err)

	adapter := &mockAdapter{
		validateMeta: SourceMetadata{Title: "Test Feed", LanguageHint: "en"},
		fetchItems: []RawItem{
			{
				ExternalID:   "manual-ai-1",
				Link:         "https://example.com/manual-ai-1",
				Title:        "Manual Refresh Title",
				ContentHTML:  "<p>" + strings.Repeat("This is a long English article body used to trigger summary generation. ", 8) + "</p>",
				LanguageHint: "en",
				PublishedAt:  time.Now(),
			},
		},
	}
	client := &sequenceAIClient{responses: []ai.ChatResponse{
		{Content: "TITLE: 手动刷新标题\nSUMMARY: 要点摘要内容"},
	}}
	svc := NewSourceService(pool, adapter)
	svc.SetAIClient(client)

	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.NoError(t, err)

	inserted, err := svc.Refresh(ctx, userID, src.ID)
	require.NoError(t, err)
	require.Equal(t, 1, inserted)
	require.Len(t, client.calls, 1)

	var articleID int64
	err = pool.QueryRow(ctx, "SELECT id FROM articles WHERE normalized_link = $1", "https://example.com/manual-ai-1").Scan(&articleID)
	require.NoError(t, err)

	aiRow, err := svc.queries.GetArticleAI(ctx, gen.GetArticleAIParams{
		ArticleID:      articleID,
		TargetLanguage: "zh-CN",
	})
	require.NoError(t, err)
	require.Equal(t, "手动刷新标题", aiRow.TitleTranslated)
	require.Equal(t, "要点摘要内容", aiRow.Summary)
	require.Equal(t, "done", aiRow.SummaryStatus)
}

func TestSourceService_Create_DuplicateURL_ReturnsError(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	_, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.NoError(t, err)

	_, err = svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.Error(t, err)
}

func TestSourceService_Create_RestoresSoftDeletedURL(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.NoError(t, err)

	err = svc.Delete(ctx, userID, src.ID)
	require.NoError(t, err)

	restored, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "Technology")
	require.NoError(t, err)
	require.Equal(t, src.ID, restored.ID)
	require.False(t, restored.DeletedAt.Valid)
	require.Equal(t, "Technology", restored.Category)

	sources, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, src.ID, sources[0].ID)
}

func TestSourceService_Create_InvalidURL_ReturnsError(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	_, err := svc.Create(ctx, userID, "://bad-url", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "discover feed")
}

func TestSourceService_Create_ValidateFails_ReturnsError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	adapter := &mockAdapter{validateErr: fmt.Errorf("connection refused")}
	svc := NewSourceService(pool, adapter)

	_, err = svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "discover feed")
}

func TestSourceService_Delete_OwnerOnly(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.NoError(t, err)

	err = svc.Delete(ctx, userID+999, src.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")

	err = svc.Delete(ctx, userID, src.ID)
	require.NoError(t, err)

	sources, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, sources, 0)
}

func TestSourceService_Rename(t *testing.T) {
	svc, userID, cleanup := setupService(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	src, err := svc.Create(ctx, userID, "https://example.com/feed.xml", "")
	require.NoError(t, err)

	err = svc.Rename(ctx, userID, src.ID, "New Title")
	require.NoError(t, err)

	sources, err := svc.List(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, "New Title", sources[0].Title)
}
