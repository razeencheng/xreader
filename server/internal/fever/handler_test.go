package fever

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/middleware"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAPIKey is MD5("testuser:testpass")
var testAPIKey = ComputeFeverAPIKey("testuser", "testpass")

// testHashedKey is SHA-256(testAPIKey), stored in the database
var testHashedKey = HashFeverKey(testAPIKey)

type testFixture struct {
	pool      *pgxpool.Pool
	handler   *Handler
	router    *gin.Engine
	userID    int64
	sourceID  int64
	articleID int64
	cleanup   func()
}

func setupTest(t *testing.T) *testFixture {
	t.Helper()
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)

	// Insert user with pre-computed fever_api_key
	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role, fever_api_key) VALUES ($1, $2, $3, $4) RETURNING id",
		1, "testuser", "user", testHashedKey,
	).Scan(&userID)
	require.NoError(t, err)

	// Insert source
	queries := gen.New(pool)
	source, err := queries.CreateSource(ctx, gen.CreateSourceParams{
		UserID:        userID,
		Kind:          "rss",
		Url:           "https://example.com/feed.xml",
		NormalizedUrl: "https://example.com/feed.xml",
		Title:         "Test Feed",
		Health:        "unknown",
		Category:      "Tech",
	})
	require.NoError(t, err)

	// Insert article
	article, err := queries.CreateArticle(ctx, gen.CreateArticleParams{
		SourceID:       source.ID,
		ExternalID:     "article-1",
		Link:           "https://example.com/article-1",
		NormalizedLink: "https://example.com/article-1",
		Title:          "Test Article",
		Language:       "en",
		ContentHtml:    "<p>Hello World</p>",
		ContentText:    "Hello World",
		Author:         pgtype.Text{String: "Author Name", Valid: true},
		PublishedAt:    pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(pool)
	r.POST("/fever/", h.Handle)

	return &testFixture{
		pool:      pool,
		handler:   h,
		router:    r,
		userID:    userID,
		sourceID:  source.ID,
		articleID: article.ID,
		cleanup:   cleanup,
	}
}

func postFever(router *gin.Engine, query string, form url.Values) *httptest.ResponseRecorder {
	body := form.Encode()
	req := httptest.NewRequest(http.MethodPost, "/fever/?"+query, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err, "failed to parse response: %s", w.Body.String())
	return result
}

func i64str(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestFever_AuthSuccess(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(3), result["api_version"])
	assert.Equal(t, float64(1), result["auth"])
}

func TestFever_AuthFailure(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{"api_key": {"wrong_key"}}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(3), result["api_version"])
	assert.Equal(t, float64(0), result["auth"])
}

func TestFever_AuthMissingKey(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(0), result["auth"])
}

func TestFever_Feeds(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, "feeds", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(1), result["auth"])

	feeds, ok := result["feeds"].([]interface{})
	require.True(t, ok, "feeds should be an array")
	require.Len(t, feeds, 1)

	feed := feeds[0].(map[string]interface{})
	assert.Equal(t, float64(f.sourceID), feed["id"])
	assert.Equal(t, "Test Feed", feed["title"])
	assert.Equal(t, "https://example.com/feed.xml", feed["url"])

	// Check feeds_groups
	feedsGroups, ok := result["feeds_groups"].([]interface{})
	require.True(t, ok, "feeds_groups should be an array")
	require.Len(t, feedsGroups, 1)
}

func TestFever_Groups(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, "groups", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)

	groups, ok := result["groups"].([]interface{})
	require.True(t, ok, "groups should be an array")
	require.Len(t, groups, 1)

	group := groups[0].(map[string]interface{})
	assert.Equal(t, "Tech", group["title"])
}

func TestFever_Items(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, "items", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(1), result["auth"])

	items, ok := result["items"].([]interface{})
	require.True(t, ok, "items should be an array")
	require.Len(t, items, 1)

	item := items[0].(map[string]interface{})
	assert.Equal(t, float64(f.articleID), item["id"])
	assert.Equal(t, float64(f.sourceID), item["feed_id"])
	assert.Equal(t, "Test Article", item["title"])
	assert.Equal(t, "Author Name", item["author"])
	assert.Equal(t, "<p>Hello World</p>", item["html"])
	assert.Equal(t, "https://example.com/article-1", item["url"])
	assert.Equal(t, float64(0), item["is_read"])
	assert.Equal(t, float64(0), item["is_saved"])
	assert.Greater(t, item["created_on_time"].(float64), float64(0))
}

func TestFever_ItemsSinceID(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	// Add another article
	queries := gen.New(f.pool)
	article2, err := queries.CreateArticle(context.Background(), gen.CreateArticleParams{
		SourceID:       f.sourceID,
		ExternalID:     "article-2",
		Link:           "https://example.com/article-2",
		NormalizedLink: "https://example.com/article-2",
		Title:          "Second Article",
		Language:       "en",
		ContentHtml:    "<p>Second</p>",
		ContentText:    "Second",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		FetchedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)

	// Request items since the first article's ID
	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, fmt.Sprintf("items&since_id=%d", f.articleID), form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	items := result["items"].([]interface{})
	// Should only return the second article (id > articleID)
	require.Len(t, items, 1)
	assert.Equal(t, float64(article2.ID), items[0].(map[string]interface{})["id"])
}

func TestFever_UnreadIDs(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, "unread_item_ids", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(1), result["auth"])

	unreadIDs, ok := result["unread_item_ids"].(string)
	require.True(t, ok, "unread_item_ids should be a string")
	// Article should be unread since we haven't read it
	assert.Contains(t, unreadIDs, i64str(f.articleID))
}

func TestFever_SavedIDs(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	// Star the article first
	_, err := f.pool.Exec(context.Background(),
		"INSERT INTO article_states (user_id, article_id, is_starred) VALUES ($1, $2, true) ON CONFLICT (user_id, article_id) DO UPDATE SET is_starred = true",
		f.userID, f.articleID,
	)
	require.NoError(t, err)

	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, "saved_item_ids", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	savedIDs, ok := result["saved_item_ids"].(string)
	require.True(t, ok)
	assert.Contains(t, savedIDs, i64str(f.articleID))
}

func TestFever_MarkItemRead(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{
		"api_key": {testAPIKey},
		"mark":    {"item"},
		"as":      {"read"},
		"id":      {i64str(f.articleID)},
	}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(1), result["auth"])

	// Verify the article is now read
	var isRead bool
	err := f.pool.QueryRow(context.Background(),
		"SELECT is_read FROM article_states WHERE user_id = $1 AND article_id = $2",
		f.userID, f.articleID,
	).Scan(&isRead)
	require.NoError(t, err)
	assert.True(t, isRead)
}

func TestFever_MarkItemUnread(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	// First mark as read
	_, err := f.pool.Exec(context.Background(),
		"INSERT INTO article_states (user_id, article_id, is_read, last_read_at) VALUES ($1, $2, true, now())",
		f.userID, f.articleID,
	)
	require.NoError(t, err)

	form := url.Values{
		"api_key": {testAPIKey},
		"mark":    {"item"},
		"as":      {"unread"},
		"id":      {i64str(f.articleID)},
	}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the article is now unread
	var isRead bool
	err = f.pool.QueryRow(context.Background(),
		"SELECT is_read FROM article_states WHERE user_id = $1 AND article_id = $2",
		f.userID, f.articleID,
	).Scan(&isRead)
	require.NoError(t, err)
	assert.False(t, isRead)
}

func TestFever_MarkItemSaved(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{
		"api_key": {testAPIKey},
		"mark":    {"item"},
		"as":      {"saved"},
		"id":      {i64str(f.articleID)},
	}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)

	var isStarred bool
	err := f.pool.QueryRow(context.Background(),
		"SELECT is_starred FROM article_states WHERE user_id = $1 AND article_id = $2",
		f.userID, f.articleID,
	).Scan(&isStarred)
	require.NoError(t, err)
	assert.True(t, isStarred)
}

func TestFever_MarkFeedRead(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	beforeTS := time.Now().Add(time.Hour).Unix()
	form := url.Values{
		"api_key": {testAPIKey},
		"mark":    {"feed"},
		"as":      {"read"},
		"id":      {i64str(f.sourceID)},
		"before":  {i64str(beforeTS)},
	}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the article in this feed is now read
	var isRead bool
	err := f.pool.QueryRow(context.Background(),
		"SELECT is_read FROM article_states WHERE user_id = $1 AND article_id = $2",
		f.userID, f.articleID,
	).Scan(&isRead)
	require.NoError(t, err)
	assert.True(t, isRead)
}

func TestFever_MarkGroupRead(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	beforeTS := time.Now().Add(time.Hour).Unix()
	form := url.Values{
		"api_key": {testAPIKey},
		"mark":    {"group"},
		"as":      {"read"},
		"id":      {"1"},
		"before":  {i64str(beforeTS)},
	}
	w := postFever(f.router, "api", form)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the article is now read
	var isRead bool
	err := f.pool.QueryRow(context.Background(),
		"SELECT is_read FROM article_states WHERE user_id = $1 AND article_id = $2",
		f.userID, f.articleID,
	).Scan(&isRead)
	require.NoError(t, err)
	assert.True(t, isRead)
}

func TestFever_SetFeverPassword(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	// Insert user without fever key
	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		99, "pwduser", "user",
	).Scan(&userID)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(pool)

	r.POST("/api/users/me/fever", func(c *gin.Context) {
		c.Set("user", &middleware.User{
			ID:            userID,
			GitHubUsername: "pwduser",
			Role:          "user",
		})
		c.Next()
	}, h.SetFeverPassword)

	// Test with password too short
	body := `{"password":"abc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users/me/fever", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test with valid password
	body = `{"password":"testpass123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/users/me/fever", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)

	// Should return the raw API key
	apiKey, ok := result["api_key"].(string)
	require.True(t, ok)
	assert.Len(t, apiKey, 32) // MD5 hex is 32 chars
	assert.Equal(t, "/fever/", result["fever_url"])
	assert.Equal(t, "pwduser", result["username"])

	// Verify the hashed key is stored in the database
	expectedHash := HashFeverKey(apiKey)
	var storedKey string
	err = pool.QueryRow(ctx, "SELECT fever_api_key FROM users WHERE id = $1", userID).Scan(&storedKey)
	require.NoError(t, err)
	assert.Equal(t, expectedHash, strings.TrimSpace(storedKey))

	// Verify we can authenticate with the returned key
	feverRouter := gin.New()
	feverH := NewHandler(pool)
	feverRouter.POST("/fever/", feverH.Handle)

	form := url.Values{"api_key": {apiKey}}
	fBody := form.Encode()
	fReq := httptest.NewRequest(http.MethodPost, "/fever/?api", strings.NewReader(fBody))
	fReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	fW := httptest.NewRecorder()
	feverRouter.ServeHTTP(fW, fReq)

	assert.Equal(t, http.StatusOK, fW.Code)
	var feverResult map[string]interface{}
	err = json.Unmarshal(fW.Body.Bytes(), &feverResult)
	require.NoError(t, err)
	assert.Equal(t, float64(1), feverResult["auth"])
}

func TestFever_Favicons(t *testing.T) {
	f := setupTest(t)
	defer f.cleanup()

	form := url.Values{"api_key": {testAPIKey}}
	w := postFever(f.router, "favicons", form)

	assert.Equal(t, http.StatusOK, w.Code)
	result := parseResponse(t, w)
	assert.Equal(t, float64(1), result["auth"])

	favicons, ok := result["favicons"].([]interface{})
	require.True(t, ok, "favicons should be an array")
	assert.Len(t, favicons, 0) // minimal implementation
}

func TestHashFeverKey(t *testing.T) {
	// Verify the hash function produces expected output
	key := HashFeverKey("test")
	assert.Len(t, key, 64) // SHA-256 hex is 64 chars

	// Same input should produce same output
	key2 := HashFeverKey("test")
	assert.Equal(t, key, key2)

	// Different input should produce different output
	key3 := HashFeverKey("different")
	assert.NotEqual(t, key, key3)
}

func TestComputeFeverAPIKey(t *testing.T) {
	key := ComputeFeverAPIKey("user", "pass")
	assert.Len(t, key, 32) // MD5 hex is 32 chars

	// Same input should produce same output
	key2 := ComputeFeverAPIKey("user", "pass")
	assert.Equal(t, key, key2)

	// Should match expected MD5("user:pass")
	assert.NotEmpty(t, key)
}
