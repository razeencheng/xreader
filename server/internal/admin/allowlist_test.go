package admin

import (
	"context"
	"testing"

	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestAllowlist_AddRemoveList(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	svc := NewAllowlistService(pool)

	require.NoError(t, svc.Add(ctx, "alice", nil, "initial admin"))

	entries, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "alice", entries[0].GithubUsername)

	allowed, err := svc.IsAllowlisted(ctx, "alice")
	require.NoError(t, err)
	require.True(t, allowed)

	require.NoError(t, svc.Remove(ctx, "alice"))

	entries, err = svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 0)
}

func TestSeedAdmin_AddsToAllowlistAndPromotesUser(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	// Create user first
	_, err := pool.Exec(ctx,
		"INSERT INTO users (github_id, github_username, role) VALUES ($1, $2, $3)",
		123, "razeencheng", "user",
	)
	require.NoError(t, err)

	svc := NewAllowlistService(pool)
	require.NoError(t, svc.SeedAdmin(ctx, "razeencheng"))

	// Check allowlist
	allowed, err := svc.IsAllowlisted(ctx, "razeencheng")
	require.NoError(t, err)
	require.True(t, allowed)

	// Check role promoted
	var role string
	err = pool.QueryRow(ctx, "SELECT role FROM users WHERE github_username = $1", "razeencheng").Scan(&role)
	require.NoError(t, err)
	require.Equal(t, "admin", role)
}
