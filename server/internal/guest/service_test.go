package guest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedAdmin(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (github_id, github_username, role, native_language, density_pref, theme_pref)
		 VALUES (1001, 'testadmin', 'admin', 'zh-CN', 'comfortable', 'system')
		 ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
}

func TestCreateGuestUser(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()
	svc := NewService(pool)

	seedAdmin(t, ctx, pool)

	guest, err := svc.CreateGuest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "guest", guest.Role)
	assert.NotEmpty(t, guest.Username)
	assert.True(t, guest.ExpiresAt.After(time.Now()))
	assert.True(t, guest.ExpiresAt.Before(time.Now().Add(25*time.Hour)))
}

func TestGuestModeStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()
	svc := NewService(pool)

	enabled, err := svc.IsEnabled(ctx)
	require.NoError(t, err)
	assert.False(t, enabled)

	_, _ = pool.Exec(ctx, "INSERT INTO settings (key, value) VALUES ('guest_mode_enabled', 'true')")
	enabled, err = svc.IsEnabled(ctx)
	require.NoError(t, err)
	assert.False(t, enabled)

	seedAdmin(t, ctx, pool)
	enabled, err = svc.IsEnabled(ctx)
	require.NoError(t, err)
	assert.True(t, enabled)
}

func TestContentOwnerID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()
	svc := NewService(pool)

	seedAdmin(t, ctx, pool)
	adminID, err := svc.ContentOwnerID(ctx)
	require.NoError(t, err)
	assert.Greater(t, adminID, int64(0))
}

func TestCleanupExpiredGuests(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()
	svc := NewService(pool)

	seedAdmin(t, ctx, pool)

	_, err := pool.Exec(ctx,
		`INSERT INTO users (github_username, role, expires_at, native_language, density_pref, theme_pref)
		 VALUES ('guest-expired', 'guest', now() - interval '1 hour', 'zh-CN', 'comfortable', 'system')`)
	require.NoError(t, err)

	guest, err := svc.CreateGuest(ctx)
	require.NoError(t, err)

	cleaned, err := svc.CleanupExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleaned)

	var count int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE id = $1", guest.ID).Scan(&count)
	assert.Equal(t, 1, count)
}
