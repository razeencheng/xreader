package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSettingsServiceCurrentMasksAPIKeyFromRepository(t *testing.T) {
	repo := NewMemorySettingsRepository()
	require.NoError(t, repo.SaveAISettings(context.Background(), settingsOverrides{
		Endpoint: "https://api.example.com/v1",
		Model:    "qwen-turbo",
		APIKey:   "sk-test-secret",
	}))

	service := NewSettingsService(repo)
	current, err := service.Current(context.Background())
	require.NoError(t, err)

	require.Equal(t, "https://api.example.com/v1", current.Endpoint)
	require.Equal(t, "qwen-turbo", current.Model)
	require.True(t, current.APIKeySet)
	require.Equal(t, "sk-...cret", current.APIKeyHint)
}

func TestSettingsServiceUpdateOverridesResolvedConfig(t *testing.T) {
	service := NewSettingsService(NewMemorySettingsRepository())
	updated, err := service.Update(context.Background(), SettingsUpdate{
		Endpoint: "https://relay.example.com",
		Model:    "deepseek-chat",
		APIKey:   "sk-new-secret",
	})
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/v1", updated.Endpoint)
	require.Equal(t, "deepseek-chat", updated.Model)
	require.Equal(t, "sk-...cret", updated.APIKeyHint)

	resolved, err := service.LoadResolved(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/v1", resolved.BaseURL)
	require.Equal(t, "deepseek-chat", resolved.Model)
	require.Equal(t, "sk-new-secret", resolved.APIKey)
	require.Equal(t, 3, resolved.MaxRetries)
	require.Equal(t, 30*time.Second, resolved.Timeout)
	require.Equal(t, 5, resolved.BatchSize)
}

func TestSettingsServiceRequiresStoredEndpointAndModel(t *testing.T) {
	service := NewSettingsService(NewMemorySettingsRepository())

	_, err := service.LoadResolved(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "AI settings are not configured")
}
