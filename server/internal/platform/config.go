package platform

import (
	"context"
	"encoding/hex"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/internal/crypto"
)

// ConfigResolver checks environment variables first, then falls back to
// the settings database table. This allows env-var overrides while
// supporting runtime configuration via the Setup Wizard.
type ConfigResolver struct {
	pool *pgxpool.Pool
}

// NewConfigResolver creates a new ConfigResolver backed by the given pool.
func NewConfigResolver(pool *pgxpool.Pool) *ConfigResolver {
	return &ConfigResolver{pool: pool}
}

// Get checks the environment variable envKey first. If unset or empty, it
// queries the settings table for dbKey.
func (r *ConfigResolver) Get(ctx context.Context, envKey, dbKey string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return r.getFromDB(ctx, dbKey)
}

// GetEncryptedSecret checks the environment variable envKey first. If unset
// or empty, it reads the ciphertext and nonce from the settings table
// (dbKeyPrefix+"_ct" and dbKeyPrefix+"_nonce"), hex-decodes them, and
// decrypts via crypto.DecryptSecret.
func (r *ConfigResolver) GetEncryptedSecret(ctx context.Context, envKey, dbKeyPrefix string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	ctHex := r.getFromDB(ctx, dbKeyPrefix+"_ct")
	nonceHex := r.getFromDB(ctx, dbKeyPrefix+"_nonce")
	if ctHex == "" || nonceHex == "" {
		return ""
	}
	ct, err := hex.DecodeString(ctHex)
	if err != nil {
		return ""
	}
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return ""
	}
	plaintext, err := crypto.DecryptSecret(ct, nonce)
	if err != nil {
		return ""
	}
	return plaintext
}

func (r *ConfigResolver) getFromDB(ctx context.Context, key string) string {
	if r.pool == nil {
		return ""
	}
	var value string
	err := r.pool.QueryRow(ctx, "SELECT value FROM settings WHERE key = $1", key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}
