package auth

import (
	"context"
	"testing"

	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestPgSessionStore_Create(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	// Insert a user first (foreign key constraint).
	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username) VALUES ($1, $2) RETURNING id",
		12345, "testuser",
	).Scan(&userID)
	require.NoError(t, err)

	store := NewPgSessionStore(pool)
	sessionID, err := store.Create(ctx, userID, "test-agent")
	require.NoError(t, err)
	require.Len(t, sessionID, 64) // 32 bytes hex-encoded

	// Verify session exists in DB.
	got, err := store.Get(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, userID, got)
}

func TestPgSessionStore_Get(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username) VALUES ($1, $2) RETURNING id",
		12345, "testuser",
	).Scan(&userID)
	require.NoError(t, err)

	store := NewPgSessionStore(pool)
	sessionID, err := store.Create(ctx, userID, "test-agent")
	require.NoError(t, err)

	got, err := store.Get(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, userID, got)
}

func TestPgSessionStore_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	store := NewPgSessionStore(pool)
	_, err := store.Get(ctx, "nonexistent-session-id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "session not found")
}

func TestPgSessionStore_Delete(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username) VALUES ($1, $2) RETURNING id",
		12345, "testuser",
	).Scan(&userID)
	require.NoError(t, err)

	store := NewPgSessionStore(pool)
	sessionID, err := store.Create(ctx, userID, "test-agent")
	require.NoError(t, err)

	err = store.Delete(ctx, sessionID)
	require.NoError(t, err)

	// Session should no longer be found.
	_, err = store.Get(ctx, sessionID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "session not found")
}

func TestPgSessionStore_Touch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	var userID int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (github_id, github_username) VALUES ($1, $2) RETURNING id",
		12345, "testuser",
	).Scan(&userID)
	require.NoError(t, err)

	store := NewPgSessionStore(pool)
	sessionID, err := store.Create(ctx, userID, "test-agent")
	require.NoError(t, err)

	// Backdate last_seen_at to simulate an old session.
	_, err = pool.Exec(ctx,
		"UPDATE auth_sessions SET last_seen_at = now() - interval '29 days' WHERE id = $1",
		sessionID,
	)
	require.NoError(t, err)

	// Session should still be valid (within 30-day window).
	got, err := store.Get(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, userID, got)

	// Touch should refresh last_seen_at.
	err = store.Touch(ctx, sessionID)
	require.NoError(t, err)

	// Verify it's still accessible.
	got, err = store.Get(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, userID, got)

	// Now backdate past the 30-day window.
	_, err = pool.Exec(ctx,
		"UPDATE auth_sessions SET last_seen_at = now() - interval '31 days' WHERE id = $1",
		sessionID,
	)
	require.NoError(t, err)

	// Session should now be expired (not found).
	_, err = store.Get(ctx, sessionID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "session not found")

	// Touch should bring it back.
	err = store.Touch(ctx, sessionID)
	require.NoError(t, err)

	got, err = store.Get(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, userID, got)
}
