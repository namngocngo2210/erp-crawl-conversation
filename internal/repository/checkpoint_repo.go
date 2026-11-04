package repository

import (
	"context"
	"errors"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CheckpointRepository interface {
	Get(ctx context.Context, pageID int64) (entity.CrawlCheckpoint, bool, error)
	Save(ctx context.Context, cp entity.CrawlCheckpoint) error
}

type CrawlRunRepository interface {
	Start(ctx context.Context, trigger string) (int64, error)
	Finish(ctx context.Context, id int64, status, errMsg string, stats []byte) error
	FailStale(ctx context.Context) (int64, error)
}

type checkpointRepo struct{ db *DB }
type crawlRunRepo struct{ db *DB }

func NewCheckpointRepository(db *DB) CheckpointRepository { return &checkpointRepo{db} }
func NewCrawlRunRepository(db *DB) CrawlRunRepository     { return &crawlRunRepo{db} }

func (r *checkpointRepo) Get(ctx context.Context, pageID int64) (entity.CrawlCheckpoint, bool, error) {
	var cp entity.CrawlCheckpoint
	err := r.db.Gorm.WithContext(ctx).First(&cp, "page_id = ?", pageID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.CrawlCheckpoint{}, false, nil
	}
	if err != nil {
		return entity.CrawlCheckpoint{}, false, err
	}
	return cp, true, nil
}

func (r *checkpointRepo) Save(ctx context.Context, cp entity.CrawlCheckpoint) error {
	now := time.Now()
	cp.LastRunAt = &now
	return r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"job_type", "cursor", "status", "last_run_at"}),
	}).Create(&cp).Error
}

func (r *crawlRunRepo) Start(ctx context.Context, trigger string) (int64, error) {
	run := entity.CrawlRun{StartedAt: time.Now(), Trigger: trigger, Status: "running"}
	err := r.db.Gorm.WithContext(ctx).Create(&run).Error
	return run.ID, err
}

func (r *crawlRunRepo) Finish(ctx context.Context, id int64, status, errMsg string, stats []byte) error {
	now := time.Now()
	return r.db.Gorm.WithContext(ctx).Model(&entity.CrawlRun{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "error": errMsg, "stats": stats, "finished_at": &now}).Error
}

func (r *crawlRunRepo) FailStale(ctx context.Context) (int64, error) {
	result := r.db.Gorm.WithContext(ctx).Exec(
		"UPDATE crawl_runs SET status='failed', error='interrupted', finished_at=now() WHERE status='running'",
	)
	return result.RowsAffected, result.Error
}
