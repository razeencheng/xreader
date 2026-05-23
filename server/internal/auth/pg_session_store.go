package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgSessionStore implements SessionStore using Postgres only,
// replacing the Redis+Postgres dual-write RedisSessionStore.
type PgSessionStore struct {
	pool *pgxpool.Pool
}

func NewPgSessionStore(pool *pgxpool.Pool) *PgSessionStore {
	return &PgSessionStore{pool: pool}
}

func (s *PgSessionStore) Create(ctx context.Context, userID int64, userAgent string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	sessionID := hex.EncodeToString(b)

	_, err := s.pool.Exec(ctx,
		"INSERT INTO auth_sessions (id, user_id, user_agent) VALUES ($1, $2, $3)",
		sessionID, userID, userAgent,
	)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	return sessionID, nil
}

func (s *PgSessionStore) Get(ctx context.Context, sessionID string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx,
		"SELECT user_id FROM auth_sessions WHERE id = $1 AND last_seen_at > now() - interval '30 days'",
		sessionID,
	).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("session not found")
		}
		return 0, fmt.Errorf("get session: %w", err)
	}
	return userID, nil
}

func (s *PgSessionStore) Delete(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM auth_sessions WHERE id = $1", sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *PgSessionStore) Touch(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, "UPDATE auth_sessions SET last_seen_at = now() WHERE id = $1", sessionID)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}
