package ai

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/crypto"
)

type SettingsSnapshot struct {
	Endpoint   string `json:"endpoint"`
	Model      string `json:"model"`
	APIKeySet  bool   `json:"api_key_set"`
	APIKeyHint string `json:"api_key_hint"`
}

type SettingsUpdate struct {
	Endpoint        string
	Model           string
	APIKey          string
	UpdatedByUserID int64
}

type settingsOverrides struct {
	Endpoint        string
	Model           string
	APIKey          string
	UpdatedByUserID int64
}

type SettingsRepository interface {
	LoadAISettings(ctx context.Context) (settingsOverrides, error)
	SaveAISettings(ctx context.Context, settings settingsOverrides) error
}

type MemorySettingsRepository struct {
	mu       sync.Mutex
	settings settingsOverrides
}

func NewMemorySettingsRepository() *MemorySettingsRepository {
	return &MemorySettingsRepository{}
}

func (r *MemorySettingsRepository) LoadAISettings(context.Context) (settingsOverrides, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settings, nil
}

func (r *MemorySettingsRepository) SaveAISettings(_ context.Context, settings settingsOverrides) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings = settings
	return nil
}

type PostgresSettingsRepository struct {
	queries *gen.Queries
}

func NewPostgresSettingsRepository(pool *pgxpool.Pool) *PostgresSettingsRepository {
	return &PostgresSettingsRepository{queries: gen.New(pool)}
}

func (r *PostgresSettingsRepository) LoadAISettings(ctx context.Context) (settingsOverrides, error) {
	if r == nil || r.queries == nil {
		return settingsOverrides{}, nil
	}
	row, err := r.queries.GetAIProviderSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return settingsOverrides{}, nil
	}
	if err != nil {
		return settingsOverrides{}, err
	}
	apiKey := ""
	if len(row.ApiKeyCiphertext) > 0 && len(row.ApiKeyNonce) > 0 {
		apiKey, err = crypto.DecryptSecret(row.ApiKeyCiphertext, row.ApiKeyNonce)
		if err != nil {
			return settingsOverrides{}, fmt.Errorf("decrypt api key: %w", err)
		}
	}
	return settingsOverrides{
		Endpoint: row.Endpoint,
		Model:    row.Model,
		APIKey:   apiKey,
	}, nil
}

func (r *PostgresSettingsRepository) SaveAISettings(ctx context.Context, settings settingsOverrides) error {
	if r == nil || r.queries == nil {
		return nil
	}
	ciphertext, nonce, err := crypto.EncryptSecret(settings.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}
	updatedBy := pgtype.Int8{}
	if settings.UpdatedByUserID > 0 {
		updatedBy = pgtype.Int8{Int64: settings.UpdatedByUserID, Valid: true}
	}
	_, err = r.queries.UpsertAIProviderSettings(ctx, gen.UpsertAIProviderSettingsParams{
		Endpoint:         settings.Endpoint,
		Model:            settings.Model,
		ApiKeyCiphertext: ciphertext,
		ApiKeyNonce:      nonce,
		ApiKeyHint:       maskAPIKey(settings.APIKey),
		UpdatedByUserID:  updatedBy,
	})
	return err
}

type SettingsService struct {
	repo SettingsRepository
}

func NewSettingsService(repo SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

func (s *SettingsService) Current(ctx context.Context) (SettingsSnapshot, error) {
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	return SettingsSnapshot{
		Endpoint:   settings.Endpoint,
		Model:      settings.Model,
		APIKeySet:  strings.TrimSpace(settings.APIKey) != "",
		APIKeyHint: maskAPIKey(settings.APIKey),
	}, nil
}

func (s *SettingsService) Update(ctx context.Context, update SettingsUpdate) (SettingsSnapshot, error) {
	if s.repo == nil {
		return SettingsSnapshot{}, errors.New("AI settings repository not configured")
	}

	current, err := s.repo.LoadAISettings(ctx)
	if err != nil {
		return SettingsSnapshot{}, fmt.Errorf("load settings: %w", err)
	}

	if strings.TrimSpace(update.Endpoint) != "" {
		endpoint, err := normalizeEndpoint(update.Endpoint)
		if err != nil {
			return SettingsSnapshot{}, err
		}
		current.Endpoint = endpoint
	}
	if strings.TrimSpace(update.Model) != "" {
		current.Model = strings.TrimSpace(update.Model)
	}
	if strings.TrimSpace(update.APIKey) != "" {
		current.APIKey = strings.TrimSpace(update.APIKey)
	}
	current.UpdatedByUserID = update.UpdatedByUserID
	if current.Endpoint == "" {
		return SettingsSnapshot{}, errors.New("endpoint is required")
	}
	if current.Model == "" {
		return SettingsSnapshot{}, errors.New("model is required")
	}

	if err := s.repo.SaveAISettings(ctx, current); err != nil {
		return SettingsSnapshot{}, fmt.Errorf("save settings: %w", err)
	}
	return s.Current(ctx)
}

func (s *SettingsService) LoadResolved(ctx context.Context) (ResolvedConfig, error) {
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return ResolvedConfig{}, err
	}
	if settings.Endpoint == "" || settings.Model == "" || settings.APIKey == "" {
		return ResolvedConfig{}, errors.New("AI settings are not configured")
	}
	baseURL := settings.Endpoint
	baseURL, err = normalizeEndpoint(baseURL)
	if err != nil {
		return ResolvedConfig{}, err
	}

	return ResolvedConfig{
		BaseURL:    baseURL,
		APIKey:     settings.APIKey,
		Model:      settings.Model,
		MaxRetries: 3,
		Timeout:    30 * time.Second,
		BatchSize:  5,
	}, nil
}

func (s *SettingsService) loadSettings(ctx context.Context) (settingsOverrides, error) {
	if s.repo == nil {
		return settingsOverrides{}, errors.New("AI settings repository not configured")
	}
	settings, err := s.repo.LoadAISettings(ctx)
	if err != nil {
		return settingsOverrides{}, fmt.Errorf("load settings: %w", err)
	}
	settings.Endpoint = strings.TrimSpace(settings.Endpoint)
	settings.Model = strings.TrimSpace(settings.Model)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	return settings, nil
}

func normalizeEndpoint(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", errors.New("endpoint is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("endpoint must be a valid http(s) URL")
	}
	if !strings.HasSuffix(trimmed, "/v1") {
		trimmed += "/v1"
	}
	return trimmed, nil
}

func maskAPIKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return "***"
	}
	return trimmed[:3] + "..." + trimmed[len(trimmed)-4:]
}

