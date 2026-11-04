package repository

import (
	"context"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

func TestPageUpsert_Idempotent(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()
	repo := NewPageRepository(db)

	id1, err := repo.Upsert(ctx, entity.Page{PancakePageID: "pg-1", Name: "A"})
	require.NoError(t, err)
	id2, err := repo.Upsert(ctx, entity.Page{PancakePageID: "pg-1", Name: "B"})
	require.NoError(t, err)
	require.Equal(t, id1, id2) // same row reused

	var name string
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT name FROM pages WHERE id=$1", id1).Scan(&name))
	require.Equal(t, "B", name) // updated
}
