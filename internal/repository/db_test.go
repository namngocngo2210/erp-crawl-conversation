package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func dsn(t *testing.T) string {
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return v
}

func TestEnsurePartition(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()

	ts := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.EnsurePartition(ctx, ts))
	// idempotent second call (served from cache, no error)
	require.NoError(t, db.EnsurePartition(ctx, ts))

	var n int
	row := db.Pool.QueryRow(ctx, "SELECT count(*) FROM pg_class WHERE relname='messages_y2026m07'")
	require.NoError(t, row.Scan(&n))
	require.Equal(t, 1, n)
}
