package source

import (
    "context"
    "testing"

    "github.com/razeencheng/xreader/internal/testutil"
    "github.com/stretchr/testify/require"
)

func TestSourceUnique_PerUserAndNormalizedURL(t *testing.T) {
    ctx := context.Background()
    pool, cleanup := testutil.SetupTestDB(t, ctx)
    t.Cleanup(cleanup)

    // Insert a user first
    var userID int64
    err := pool.QueryRow(ctx,
        "INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
        1, "testuser", "user",
    ).Scan(&userID)
    require.NoError(t, err)

    // Insert a source
    _, err = pool.Exec(ctx,
        "INSERT INTO sources (user_id, url, normalized_url, title) VALUES ($1, $2, $3, $4)",
        userID, "https://example.com/feed", "https://example.com/feed", "Example",
    )
    require.NoError(t, err)

    // Duplicate should fail
    _, err = pool.Exec(ctx,
        "INSERT INTO sources (user_id, url, normalized_url, title) VALUES ($1, $2, $3, $4)",
        userID, "https://example.com/feed", "https://example.com/feed", "Example Dup",
    )
    require.Error(t, err)
    require.Contains(t, err.Error(), "duplicate key")
}

func TestArticleUnique_PerSourceAndNormalizedLink(t *testing.T) {
    ctx := context.Background()
    pool, cleanup := testutil.SetupTestDB(t, ctx)
    t.Cleanup(cleanup)

    var userID int64
    err := pool.QueryRow(ctx,
        "INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
        1, "testuser", "user",
    ).Scan(&userID)
    require.NoError(t, err)

    var sourceID int64
    err = pool.QueryRow(ctx,
        "INSERT INTO sources (user_id, url, normalized_url, title) VALUES ($1, $2, $3, $4) RETURNING id",
        userID, "https://example.com/feed", "https://example.com/feed", "Example",
    ).Scan(&sourceID)
    require.NoError(t, err)

    // Insert article
    _, err = pool.Exec(ctx,
        `INSERT INTO articles (source_id, external_id, link, normalized_link, title, language, content_html, content_text, published_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())`,
        sourceID, "guid-1", "https://example.com/post", "https://example.com/post", "Post", "en", "<p>hi</p>", "hi",
    )
    require.NoError(t, err)

    // Duplicate normalized_link should fail
    _, err = pool.Exec(ctx,
        `INSERT INTO articles (source_id, external_id, link, normalized_link, title, language, content_html, content_text, published_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())`,
        sourceID, "guid-2", "https://example.com/post2", "https://example.com/post", "Post Dup", "en", "<p>hi2</p>", "hi2",
    )
    require.Error(t, err)
    require.Contains(t, err.Error(), "duplicate key")
}
