package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AllowlistEntry struct {
	GithubUsername string    `json:"github_username"`
	AddedByUserID *int64    `json:"added_by_user_id,omitempty"`
	AddedAt       time.Time `json:"added_at"`
	Note          *string   `json:"note,omitempty"`
}

type AllowlistService struct {
	pool *pgxpool.Pool
}

func NewAllowlistService(pool *pgxpool.Pool) *AllowlistService {
	return &AllowlistService{pool: pool}
}

func (s *AllowlistService) List(ctx context.Context) ([]AllowlistEntry, error) {
	rows, err := s.pool.Query(ctx, "SELECT github_username, added_by_user_id, added_at, note FROM auth_allowlist ORDER BY added_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AllowlistEntry
	for rows.Next() {
		var e AllowlistEntry
		if err := rows.Scan(&e.GithubUsername, &e.AddedByUserID, &e.AddedAt, &e.Note); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *AllowlistService) Add(ctx context.Context, githubUsername string, addedByUserID *int64, note string) error {
	_, err := s.pool.Exec(ctx,
		"INSERT INTO auth_allowlist (github_username, added_by_user_id, note) VALUES ($1, $2, $3) ON CONFLICT (github_username) DO NOTHING",
		githubUsername, addedByUserID, note,
	)
	return err
}

func (s *AllowlistService) Remove(ctx context.Context, githubUsername string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM auth_allowlist WHERE github_username = $1", githubUsername)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("username %q not found in allowlist", githubUsername)
	}
	return nil
}

func (s *AllowlistService) IsAllowlisted(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM auth_allowlist WHERE github_username = $1)", username).Scan(&exists)
	return exists, err
}

func (s *AllowlistService) SeedAdmin(ctx context.Context, githubUsername string) error {
	if err := s.Add(ctx, githubUsername, nil, "seed-admin CLI"); err != nil {
		return err
	}
	_, _ = s.pool.Exec(ctx, "UPDATE users SET role = 'admin' WHERE github_username = $1", githubUsername)
	return nil
}
