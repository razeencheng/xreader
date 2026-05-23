package sync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
	"github.com/razeencheng/xreader/internal/source"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

type mockAdapter struct {
	items    []source.RawItem
	fetchErr error
}

func (m *mockAdapter) Kind() string { return "rss" }

func (m *mockAdapter) Fetch(ctx context.Context, src source.Source) ([]source.RawItem, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.items, nil
}

func (m *mockAdapter) Validate(ctx context.Context, url string) (source.SourceMetadata, error) {
	return source.SourceMetadata{}, nil
}

func setupTestSource(t *testing.T, pool *pgxpool.Pool, ctx context.Context) gen.Source {
	t.Helper()
	queries := gen.New(pool)

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		1, "testuser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	src, err := queries.CreateSource(ctx, gen.CreateSourceParams{
		UserID:        userID,
		Kind:          "rss",
		Url:           "https://example.com/feed.xml",
		NormalizedUrl: "https://example.com/feed.xml",
		Title:         "Test Feed",
		Health:        "unknown",
	})
	require.NoError(t, err)
	return src
}

func TestFetchJob_DedupesByNormalizedLink(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)

	queries := gen.New(pool)
	_, err := queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       src.ID,
		ExternalID:     "existing-1",
		Link:           "https://example.com/post1",
		NormalizedLink: "https://example.com/post1",
		Title:          "Existing Post",
		Language:       "en",
		ContentHtml:    "<p>existing</p>",
		ContentText:    "existing",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	adapter := &mockAdapter{
		items: []source.RawItem{
			{ExternalID: "existing-1", Link: "https://example.com/post1", Title: "Existing Post", ContentHTML: "<p>existing</p>", PublishedAt: time.Now()},
			{ExternalID: "new-2", Link: "https://example.com/post2", Title: "New Post 2", ContentHTML: "<p>new 2</p>", PublishedAt: time.Now()},
			{ExternalID: "new-3", Link: "https://example.com/post3", Title: "New Post 3", ContentHTML: "<p>new 3</p>", PublishedAt: time.Now()},
		},
	}

	job := NewFetchJob(pool, adapter)
	inserted, _, err := job.Run(ctx, src)
	require.NoError(t, err)
	require.Equal(t, 2, inserted)

	articles, err := queries.ListArticlesBySource(ctx, src.ID)
	require.NoError(t, err)
	require.Len(t, articles, 3)
}

func TestFetchJob_MarksFailureIncrementsCounter(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)

	adapter := &mockAdapter{fetchErr: fmt.Errorf("connection refused")}
	job := NewFetchJob(pool, adapter)
	_, _, err := job.Run(ctx, src)
	require.Error(t, err)

	queries := gen.New(pool)
	updated, err := queries.GetSourceByID(ctx, src.ID)
	require.NoError(t, err)
	require.Equal(t, int32(1), updated.ConsecutiveFails)
	require.Equal(t, "warn", updated.Health)
}

func TestFetchJob_SuccessResetsFailCounter(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)

	queries := gen.New(pool)
	err := queries.UpdateSourceFetchStatus(ctx, gen.UpdateSourceFetchStatusParams{
		ID:               src.ID,
		LastFetchedAt:    pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		LastSuccessAt:    pgtype.Timestamptz{},
		ConsecutiveFails: 3,
		Health:           "warn",
	})
	require.NoError(t, err)

	adapter := &mockAdapter{
		items: []source.RawItem{
			{ExternalID: "1", Link: "https://example.com/p1", Title: "Post", ContentHTML: "<p>hi</p>", PublishedAt: time.Now()},
		},
	}
	job := NewFetchJob(pool, adapter)
	inserted, _, err := job.Run(ctx, src)
	require.NoError(t, err)
	require.Equal(t, 1, inserted)

	updated, err := queries.GetSourceByID(ctx, src.ID)
	require.NoError(t, err)
	require.Equal(t, int32(0), updated.ConsecutiveFails)
	require.Equal(t, "ok", updated.Health)
}

func TestFetchJob_FirstFetchMarksHistoricalBacklogRead(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)
	now := time.Now()
	items := make([]source.RawItem, 0, 25)
	for i := 0; i < 25; i++ {
		publishedAt := now.Add(-time.Duration(i+20) * 24 * time.Hour)
		items = append(items, source.RawItem{
			ExternalID:  fmt.Sprintf("old-%02d", i),
			Link:        fmt.Sprintf("https://example.com/old-%02d", i),
			Title:       fmt.Sprintf("Old Post %02d", i),
			ContentHTML: "<p>old</p>",
			PublishedAt: publishedAt,
		})
	}

	job := NewFetchJob(pool, &mockAdapter{items: items})
	inserted, _, err := job.Run(ctx, src)
	require.NoError(t, err)
	require.Equal(t, 25, inserted)

	var unreadCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM articles a
		LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $2
		WHERE a.source_id = $1 AND (st.is_read IS NULL OR st.is_read = false)
	`, src.ID, src.UserID).Scan(&unreadCount)
	require.NoError(t, err)
	require.Equal(t, 20, unreadCount)

	var readCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM articles a
		JOIN article_states st ON st.article_id = a.id AND st.user_id = $2
		WHERE a.source_id = $1 AND st.is_read = true
	`, src.ID, src.UserID).Scan(&readCount)
	require.NoError(t, err)
	require.Equal(t, 5, readCount)
}

func TestFetchJob_DoesNotApplyInitialBacklogRuleAfterFirstSuccess(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)
	queries := gen.New(pool)
	err := queries.UpdateSourceFetchStatus(ctx, gen.UpdateSourceFetchStatusParams{
		ID:               src.ID,
		LastFetchedAt:    pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		LastSuccessAt:    pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		ConsecutiveFails: 0,
		Health:           "ok",
	})
	require.NoError(t, err)
	src, err = queries.GetSourceByID(ctx, src.ID)
	require.NoError(t, err)

	oldItem := source.RawItem{
		ExternalID:  "later-old",
		Link:        "https://example.com/later-old",
		Title:       "Later Old Post",
		ContentHTML: "<p>old</p>",
		PublishedAt: time.Now().Add(-90 * 24 * time.Hour),
	}
	job := NewFetchJob(pool, &mockAdapter{items: []source.RawItem{oldItem}})
	inserted, _, err := job.Run(ctx, src)
	require.NoError(t, err)
	require.Equal(t, 1, inserted)

	var unreadCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM articles a
		LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $2
		WHERE a.source_id = $1 AND (st.is_read IS NULL OR st.is_read = false)
	`, src.ID, src.UserID).Scan(&unreadCount)
	require.NoError(t, err)
	require.Equal(t, 1, unreadCount)
}

func TestWorker_RetranslateLoopProcessesQueuedArticle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)

	var articleID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO articles
		  (source_id, external_id, link, normalized_link, title, language,
		   content_html, content_text, published_at, fetched_at)
		VALUES ($1,'rt-1','https://example.com/rt-1','https://example.com/rt-1',
		        'Breaking News Today','en','<p>body</p>','body', now(), now())
		RETURNING id
	`, src.ID).Scan(&articleID)
	require.NoError(t, err)

	queue := ai.NewRetranslateQueue(8)
	client := &ai.MockClient{Response: ai.ChatResponse{Content: "今日要闻"}}
	adapter := &mockAdapter{}
	worker := NewWorker(pool, adapter, client, queue)

	go worker.retranslateLoop(ctx)

	require.True(t, queue.Enqueue(articleID, "zh-CN"))

	require.Eventually(t, func() bool {
		var translated string
		row := pool.QueryRow(ctx, `
			SELECT title_translated FROM article_ai
			WHERE article_id = $1 AND target_language = 'zh-CN'
		`, articleID)
		if err := row.Scan(&translated); err != nil {
			return false
		}
		return translated != "" && translated != "Breaking News Today"
	}, 10*time.Second, 100*time.Millisecond)
}

func TestWorker_EagerAIFansOutToDistinctNativeLanguages(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)
	_, err := pool.Exec(ctx,
		"INSERT INTO users (github_id, github_username, role, native_language) VALUES ($1, $2, $3, $4)",
		2, "japanese-reader", "user", "ja-JP",
	)
	require.NoError(t, err)

	adapter := &mockAdapter{
		items: []source.RawItem{
			{
				ExternalID:   "multilang-1",
				Link:         "https://example.com/multilang-1",
				Title:        "A tiny English update",
				ContentHTML:  "<p>A tiny English update.</p>",
				LanguageHint: "en",
				PublishedAt:  time.Now(),
			},
		},
	}
	client := &ai.MockClient{Response: ai.ChatResponse{Content: "translated"}}
	worker := NewWorker(pool, adapter, client, ai.NewRetranslateQueue(8))

	worker.tick(ctx)

	var articleID int64
	err = pool.QueryRow(ctx, "SELECT id FROM articles WHERE source_id = $1", src.ID).Scan(&articleID)
	require.NoError(t, err)

	rows, err := pool.Query(ctx, `
		SELECT target_language
		FROM article_ai
		WHERE article_id = $1
		ORDER BY target_language
	`, articleID)
	require.NoError(t, err)
	defer rows.Close()

	var languages []string
	for rows.Next() {
		var lang string
		require.NoError(t, rows.Scan(&lang))
		languages = append(languages, lang)
	}
	require.NoError(t, rows.Err())

	require.Equal(t, []string{"ja-JP", "zh-CN"}, languages)
}

func TestWorker_RunCatchUpSkipsWhenRunRecently(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	src := setupTestSource(t, pool, ctx)

	// A user whose native language differs from the English title, so the
	// eager pipeline actually calls the AI client during catch-up.
	_, err := pool.Exec(ctx,
		"INSERT INTO users (github_id, github_username, role, native_language) VALUES ($1,$2,$3,$4)",
		2, "zh-reader", "user", "zh-CN")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO articles
		  (source_id, external_id, link, normalized_link, title, language,
		   content_html, content_text, published_at, fetched_at)
		VALUES ($1,'cu-1','https://example.com/cu-1','https://example.com/cu-1',
		        'A catch-up headline','en','<p>body</p>','body', now(), now())
	`, src.ID)
	require.NoError(t, err)

	client := &ai.MockClient{Response: ai.ChatResponse{Content: "已翻译"}}
	worker := NewWorker(pool, &mockAdapter{}, client, ai.NewRetranslateQueue(8))

	// First run processes the missing article -> the AI client is called.
	worker.runCatchUp(ctx)
	require.NotEmpty(t, client.Calls, "first catch-up should call the AI client")
	callsAfterFirst := len(client.Calls)

	// Immediate second run is within catchUpInterval -> the in-memory guard
	// must skip it entirely, so the AI client is NOT called again.
	worker.runCatchUp(ctx)
	require.Equal(t, callsAfterFirst, len(client.Calls),
		"a second catch-up within the interval must not call the AI client again")
}
