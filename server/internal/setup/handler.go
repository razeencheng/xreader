package setup

import (
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/internal/crypto"
)

// Handler implements the Setup Wizard endpoints. It is secured by a
// one-time setup token printed to stdout on first launch.
type Handler struct {
	pool       *pgxpool.Pool
	setupToken string
}

// NewHandler creates a Handler with the given database pool and setup token.
func NewHandler(pool *pgxpool.Pool, setupToken string) *Handler {
	return &Handler{pool: pool, setupToken: setupToken}
}

type completeRequest struct {
	SetupToken          string `json:"setup_token" binding:"required"`
	GitHubClientID      string `json:"github_client_id"`
	GitHubClientSecret  string `json:"github_client_secret"`
	GitHubCallbackURL   string `json:"github_callback_url"`
	AIEndpoint          string `json:"ai_endpoint"`
	AIModel             string `json:"ai_model"`
	AIAPIKey            string `json:"ai_api_key"`
	AdminGitHubUsername string `json:"admin_github_username" binding:"required"`
}

// Status returns whether setup is still needed.
func (h *Handler) Status(c *gin.Context) {
	ctx := c.Request.Context()
	var count int
	err := h.pool.QueryRow(ctx, "SELECT COUNT(*) FROM auth_allowlist").Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check setup status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"needs_setup": count == 0})
}

// Complete validates the setup token, saves configuration, and seeds the
// admin user — all within a single database transaction.
func (h *Handler) Complete(c *gin.Context) {
	ctx := c.Request.Context()

	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate admin username: must be non-empty after trim, alphanumeric + hyphens (GitHub format)
	adminUsername := strings.TrimSpace(req.AdminGitHubUsername)
	if adminUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin_github_username is required"})
		return
	}
	for _, ch := range adminUsername {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-') {
			c.JSON(http.StatusBadRequest, gin.H{"error": "admin_github_username must contain only letters, numbers, and hyphens"})
			return
		}
	}
	req.AdminGitHubUsername = adminUsername

	// Check if setup is still needed
	var count int
	if err := h.pool.QueryRow(ctx, "SELECT COUNT(*) FROM auth_allowlist").Scan(&count); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check setup status"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "setup already completed"})
		return
	}

	// Validate setup token (constant-time comparison)
	if h.setupToken == "" || subtle.ConstantTimeCompare([]byte(req.SetupToken), []byte(h.setupToken)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid setup token"})
		return
	}

	// Resolve GitHub OAuth config: combine env vars + request fields
	clientID := coalesce(req.GitHubClientID, "")
	clientSecret := coalesce(req.GitHubClientSecret, "")
	callbackURL := coalesce(req.GitHubCallbackURL, "")

	if clientID == "" || clientSecret == "" || callbackURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "github_client_id, github_client_secret, and github_callback_url are all required"})
		return
	}

	// Validate AI config: if any field provided, all three required
	aiEndpoint := strings.TrimSpace(req.AIEndpoint)
	aiModel := strings.TrimSpace(req.AIModel)
	aiAPIKey := strings.TrimSpace(req.AIAPIKey)
	hasAnyAI := aiEndpoint != "" || aiModel != "" || aiAPIKey != ""
	hasAllAI := aiEndpoint != "" && aiModel != "" && aiAPIKey != ""
	if hasAnyAI && !hasAllAI {
		c.JSON(http.StatusBadRequest, gin.H{"error": "if any AI field is provided, all three (ai_endpoint, ai_model, ai_api_key) are required"})
		return
	}

	// Normalize AI endpoint if provided
	if aiEndpoint != "" {
		normalized, err := normalizeAIEndpoint(aiEndpoint)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		aiEndpoint = normalized
	}

	// Encrypt GitHub client secret
	ghCT, ghNonce, err := crypto.EncryptSecret(clientSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt secret"})
		return
	}

	// Begin transaction — all writes are atomic
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Save plaintext settings
	upsertSetting := "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()"
	if _, err := tx.Exec(ctx, upsertSetting, "github_client_id", clientID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}
	if _, err := tx.Exec(ctx, upsertSetting, "github_callback_url", callbackURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}

	// Save encrypted GitHub client secret
	if _, err := tx.Exec(ctx, upsertSetting, "github_client_secret_ct", hex.EncodeToString(ghCT)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}
	if _, err := tx.Exec(ctx, upsertSetting, "github_client_secret_nonce", hex.EncodeToString(ghNonce)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}

	// Save AI settings if provided
	if hasAllAI {
		aiCT, aiNonce, err := crypto.EncryptSecret(aiAPIKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt AI API key"})
			return
		}
		aiKeyHint := maskAPIKey(aiAPIKey)
		_, err = tx.Exec(ctx,
			`INSERT INTO ai_provider_settings (id, endpoint, model, api_key_ciphertext, api_key_nonce, api_key_hint)
			 VALUES (1, $1, $2, $3, $4, $5)
			 ON CONFLICT (id) DO UPDATE SET endpoint = $1, model = $2, api_key_ciphertext = $3, api_key_nonce = $4, api_key_hint = $5, updated_at = NOW()`,
			aiEndpoint, aiModel, aiCT, aiNonce, aiKeyHint,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save AI settings"})
			return
		}
	}

	// Seed admin: add to allowlist
	_, err = tx.Exec(ctx,
		"INSERT INTO auth_allowlist (github_username, note) VALUES ($1, $2) ON CONFLICT (github_username) DO NOTHING",
		strings.TrimSpace(req.AdminGitHubUsername), "setup-wizard",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to seed admin"})
		return
	}
	// Update role if user already exists
	_, _ = tx.Exec(ctx, "UPDATE users SET role = 'admin' WHERE github_username = $1", strings.TrimSpace(req.AdminGitHubUsername))

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit setup"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizeAIEndpoint(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", &validationError{"ai_endpoint must be a valid http(s) URL"}
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

type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }
