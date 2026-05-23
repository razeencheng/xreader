package guest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const GuestTTL = 24 * time.Hour

type GuestUser struct {
	ID        int64
	Username  string
	Role      string
	ExpiresAt time.Time
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) IsEnabled(ctx context.Context) (bool, error) {
	var val string
	err := s.pool.QueryRow(ctx,
		"SELECT value FROM settings WHERE key = 'guest_mode_enabled'",
	).Scan(&val)
	if err != nil {
		return false, nil
	}
	if val != "true" {
		return false, nil
	}
	var exists bool
	err = s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE role = 'admin')",
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Service) ContentOwnerID(ctx context.Context) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		"SELECT id FROM users WHERE role = 'admin' ORDER BY id ASC LIMIT 1",
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("no admin user found: %w", err)
	}
	return id, nil
}

func (s *Service) SetEnabled(ctx context.Context, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	_, err := s.pool.Exec(ctx,
		"INSERT INTO settings (key, value) VALUES ('guest_mode_enabled', $1) ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = now()",
		val,
	)
	return err
}

func (s *Service) CreateGuest(ctx context.Context) (*GuestUser, error) {
	username := "guest-" + randomHex(8)
	expiresAt := time.Now().Add(GuestTTL)

	var lang, density, theme string
	err := s.pool.QueryRow(ctx,
		"SELECT native_language, density_pref, theme_pref FROM users WHERE role = 'admin' ORDER BY id ASC LIMIT 1",
	).Scan(&lang, &density, &theme)
	if err != nil {
		lang, density, theme = "zh-CN", "comfortable", "system"
	}

	var id int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO users (github_username, role, expires_at, native_language, density_pref, theme_pref)
		 VALUES ($1, 'guest', $2, $3, $4, $5)
		 RETURNING id`,
		username, expiresAt, lang, density, theme,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create guest user: %w", err)
	}

	return &GuestUser{
		ID:        id,
		Username:  username,
		Role:      "guest",
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) IsExpired(ctx context.Context, userID int64) (bool, error) {
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx,
		"SELECT expires_at FROM users WHERE id = $1",
		userID,
	).Scan(&expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return true, nil
		}
		return false, err
	}
	if expiresAt == nil {
		return false, nil
	}
	return time.Now().After(*expiresAt), nil
}

func (s *Service) CleanupExpired(ctx context.Context) (int64, error) {
	_, _ = s.pool.Exec(ctx,
		"DELETE FROM highlights WHERE user_id IN (SELECT id FROM users WHERE role = 'guest' AND expires_at < now())")
	_, _ = s.pool.Exec(ctx,
		"DELETE FROM article_state_changes WHERE user_id IN (SELECT id FROM users WHERE role = 'guest' AND expires_at < now())")

	tag, err := s.pool.Exec(ctx,
		"DELETE FROM users WHERE role = 'guest' AND expires_at < now()")
	if err != nil {
		return 0, fmt.Errorf("cleanup guests: %w", err)
	}
	return tag.RowsAffected(), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
