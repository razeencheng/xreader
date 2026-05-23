package ai

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestPostgresSettingsRepositoryEncryptsAPIKeyAtRest(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	repo := NewPostgresSettingsRepository(pool)
	err := repo.SaveAISettings(ctx, settingsOverrides{
		Endpoint: "https://relay.example.com/v1",
		Model:    "qwen-turbo",
		APIKey:   "sk-secret-value",
	})
	require.NoError(t, err)

	var rawCiphertext []byte
	var hint string
	err = pool.QueryRow(ctx, "SELECT api_key_ciphertext, api_key_hint FROM ai_provider_settings WHERE id = 1").Scan(&rawCiphertext, &hint)
	require.NoError(t, err)
	require.NotEmpty(t, rawCiphertext)
	require.NotContains(t, string(rawCiphertext), "sk-secret-value")
	require.NotContains(t, hex.EncodeToString(rawCiphertext), hex.EncodeToString([]byte("sk-secret-value")))
	require.Equal(t, "sk-...alue", hint)

	loaded, err := repo.LoadAISettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/v1", loaded.Endpoint)
	require.Equal(t, "qwen-turbo", loaded.Model)
	require.Equal(t, "sk-secret-value", loaded.APIKey)
}

func TestPostgresSettingsRepositoryKeepsExistingAPIKeyWhenBlank(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	repo := NewPostgresSettingsRepository(pool)
	require.NoError(t, repo.SaveAISettings(ctx, settingsOverrides{
		Endpoint: "https://relay.example.com/v1",
		Model:    "qwen-turbo",
		APIKey:   "sk-secret-value",
	}))

	require.NoError(t, repo.SaveAISettings(ctx, settingsOverrides{
		Endpoint: "https://relay.example.com/v1",
		Model:    "deepseek-chat",
	}))

	loaded, err := repo.LoadAISettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "deepseek-chat", loaded.Model)
	require.Equal(t, "sk-secret-value", loaded.APIKey)
}
