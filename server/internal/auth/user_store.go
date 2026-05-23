package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgUserStore struct {
	pool *pgxpool.Pool
}

func NewPgUserStore(pool *pgxpool.Pool) *PgUserStore {
	return &PgUserStore{pool: pool}
}

func (s *PgUserStore) UpsertUser(ctx context.Context, githubID int64, username, avatarURL string) (int64, error) {
	var desiredRole string
	if err := s.pool.QueryRow(ctx,
		`SELECT CASE
			WHEN EXISTS (
				SELECT 1
				FROM auth_allowlist
				WHERE github_username = $1
				  AND note IN ('seed-admin CLI', 'setup-wizard')
			) THEN 'admin'
			ELSE 'user'
		END`,
		username,
	).Scan(&desiredRole); err != nil {
		return 0, err
	}

	var id int64
	err := s.pool.QueryRow(ctx,
		`WITH updated_by_id AS (
		   UPDATE users
		      SET github_username = $2,
		          avatar_url = $3,
		          role = CASE
		            WHEN users.role = 'admin' OR $4 = 'admin' THEN 'admin'
		            ELSE users.role
		          END
		    WHERE github_id = $1
		    RETURNING id
		 ), updated_by_username AS (
		   UPDATE users
		      SET github_id = $1,
		          avatar_url = $3,
		          role = CASE
		            WHEN users.role = 'admin' OR $4 = 'admin' THEN 'admin'
		            ELSE users.role
		          END
		    WHERE github_username = $2
		      AND NOT EXISTS (SELECT 1 FROM updated_by_id)
		    RETURNING id
		 ), inserted AS (
		   INSERT INTO users (github_id, github_username, avatar_url, role)
		   SELECT $1, $2, $3, $4
		    WHERE NOT EXISTS (SELECT 1 FROM updated_by_id)
		      AND NOT EXISTS (SELECT 1 FROM updated_by_username)
		    RETURNING id
		 )
		 SELECT id FROM updated_by_id
		 UNION ALL
		 SELECT id FROM updated_by_username
		 UNION ALL
		 SELECT id FROM inserted
		 LIMIT 1`,
		githubID, username, avatarURL, desiredRole,
	).Scan(&id)
	return id, err
}
