package repository

import (
	"context"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

func TestFailStale(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()

	repo := NewCrawlRunRepository(db)

	// Insert a running crawl_run
	runID, err := repo.Start(ctx, "test")
	require.NoError(t, err)
	require.NotZero(t, runID)

	// FailStale should mark it as failed
	n, err := repo.FailStale(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(1)) // at least our row

	// Verify the run is now failed
	var run entity.CrawlRun
	db.Gorm.WithContext(ctx).First(&run, runID)
	require.Equal(t, "failed", run.Status)
	require.Equal(t, "interrupted", run.Error)
	require.NotNil(t, run.FinishedAt)
}

func TestCheckpoint_SaveGet(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()

	// cleanup: delete any existing checkpoint/page from prior run for isolation
	db.Gorm.Exec("DELETE FROM crawl_checkpoints WHERE page_id IN (SELECT id FROM pages WHERE pancake_page_id='cp-page')")
	db.Gorm.Exec("DELETE FROM pages WHERE pancake_page_id='cp-page'")

	pr := NewPageRepository(db)
	pid, err := pr.Upsert(ctx, entity.Page{PancakePageID: "cp-page"})
	require.NoError(t, err)

	repo := NewCheckpointRepository(db)
	_, found, err := repo.Get(ctx, pid)
	require.NoError(t, err)
	require.False(t, found)

	cur := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Save(ctx, entity.CrawlCheckpoint{PageID: pid, JobType: "conversations", Cursor: &cur, Status: "ok"}))
	got, found, err := repo.Get(ctx, pid)
	require.NoError(t, err)
	require.True(t, found)
	require.WithinDuration(t, cur, *got.Cursor, time.Second)
}
