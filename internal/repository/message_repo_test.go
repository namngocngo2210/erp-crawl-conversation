package repository

import (
	"context"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

func msg(id string, ts time.Time) entity.Message {
	return entity.Message{PageID: 1, ConversationID: 1, PancakeMessageID: id, InsertedAt: ts, Type: "text", Content: "hi"}
}

func TestUpsertBatch_DedupInvariant(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()
	repo := NewMessageRepository(db)

	ts := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	batch := []entity.Message{msg("dup-1", ts), msg("dup-2", ts)}

	require.NoError(t, repo.UpsertBatch(ctx, batch))
	require.NoError(t, repo.UpsertBatch(ctx, batch)) // second time: no new rows

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM messages WHERE pancake_message_id IN ('dup-1','dup-2')").Scan(&n))
	require.Equal(t, 2, n)
}

func TestBulkLoad_DedupAcrossBatchAndExisting(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()
	repo := NewMessageRepository(db)

	// clean up any rows from a prior test run so n1 is always fresh
	_, _ = db.Pool.Exec(ctx, "DELETE FROM messages WHERE pancake_message_id IN ('bl-1','bl-2')")

	ts := time.Date(2026, 6, 11, 8, 0, 0, 0, time.UTC)
	// intra-batch duplicate ("bl-1" twice) + a unique one
	batch := []entity.Message{msg("bl-1", ts), msg("bl-1", ts), msg("bl-2", ts)}

	n1, err := repo.BulkLoad(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, int64(2), n1) // intra-batch dup collapsed

	n2, err := repo.BulkLoad(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, int64(0), n2) // all already exist

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM messages WHERE pancake_message_id IN ('bl-1','bl-2')").Scan(&n))
	require.Equal(t, 2, n)
}
