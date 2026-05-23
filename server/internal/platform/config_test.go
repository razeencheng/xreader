package platform

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/razeencheng/xreader/internal/crypto"
	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigResolver_EnvOverridesDB(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	// Insert a value into settings table
	_, err := pool.Exec(ctx, "INSERT INTO settings (key, value) VALUES ($1, $2)", "github_client_id", "db-value")
	require.NoError(t, err)

	// Set env var
	t.Setenv("GITHUB_CLIENT_ID", "env-value")

	cfg := NewConfigResolver(pool)
	got := cfg.Get(ctx, "GITHUB_CLIENT_ID", "github_client_id")
	assert.Equal(t, "env-value", got)
}

func TestConfigResolver_FallbackToDB(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	// Insert a value into settings table
	_, err := pool.Exec(ctx, "INSERT INTO settings (key, value) VALUES ($1, $2)", "github_client_id", "db-value")
	require.NoError(t, err)

	// Ensure env var is unset
	t.Setenv("GITHUB_CLIENT_ID", "")

	cfg := NewConfigResolver(pool)
	got := cfg.Get(ctx, "GITHUB_CLIENT_ID", "github_client_id")
	assert.Equal(t, "db-value", got)
}

func TestConfigResolver_BothEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	// No DB value, no env var
	t.Setenv("GITHUB_CLIENT_ID", "")

	cfg := NewConfigResolver(pool)
	got := cfg.Get(ctx, "GITHUB_CLIENT_ID", "github_client_id")
	assert.Equal(t, "", got)
}

func TestConfigResolver_GetEncryptedSecret_EnvOverride(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	t.Setenv("GITHUB_CLIENT_SECRET", "my-secret-from-env")

	cfg := NewConfigResolver(pool)
	got := cfg.GetEncryptedSecret(ctx, "GITHUB_CLIENT_SECRET", "github_client_secret")
	assert.Equal(t, "my-secret-from-env", got)
}

func TestConfigResolver_GetEncryptedSecret_FallbackToDB(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	t.Setenv("GITHUB_CLIENT_SECRET", "")

	// Encrypt a secret and store in DB
	ct, nonce, err := crypto.EncryptSecret("my-db-secret")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO settings (key, value) VALUES ($1, $2)", "github_client_secret_ct", hex.EncodeToString(ct))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO settings (key, value) VALUES ($1, $2)", "github_client_secret_nonce", hex.EncodeToString(nonce))
	require.NoError(t, err)

	cfg := NewConfigResolver(pool)
	got := cfg.GetEncryptedSecret(ctx, "GITHUB_CLIENT_SECRET", "github_client_secret")
	assert.Equal(t, "my-db-secret", got)
}

func TestConfigResolver_GetEncryptedSecret_BothEmpty(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	defer cleanup()

	t.Setenv("GITHUB_CLIENT_SECRET", "")

	cfg := NewConfigResolver(pool)
	got := cfg.GetEncryptedSecret(ctx, "GITHUB_CLIENT_SECRET", "github_client_secret")
	assert.Equal(t, "", got)
}
