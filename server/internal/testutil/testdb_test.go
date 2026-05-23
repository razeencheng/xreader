package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupTestDB_Bootstraps(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := SetupTestDB(t, ctx)
	t.Cleanup(cleanup)
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
