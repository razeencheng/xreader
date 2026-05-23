package auth

import (
	"context"
	"testing"

	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestPgUserStore_UpsertUserClaimsPreseededUsername(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	var existingID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3) RETURNING id",
		int64(12345), "razeencheng", "admin",
	).Scan(&existingID)
	require.NoError(t, err)

	store := NewPgUserStore(pool)
	userID, err := store.UpsertUser(ctx, 16044915, "razeencheng", "https://avatar")
	require.NoError(t, err)
	require.Equal(t, existingID, userID)

	var githubID int64
	var role string
	var avatarURL *string
	err = pool.QueryRow(ctx,
		"SELECT github_id, role, avatar_url FROM users WHERE id = $1",
		existingID,
	).Scan(&githubID, &role, &avatarURL)
	require.NoError(t, err)
	require.Equal(t, int64(16044915), githubID)
	require.Equal(t, "admin", role)
	require.NotNil(t, avatarURL)
	require.Equal(t, "https://avatar", *avatarURL)
}
