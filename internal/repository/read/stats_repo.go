package read

import (
	"context"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

type Totals struct {
	Pages         int64
	Conversations int64
	Messages      int64
}

type DailyCount struct {
	Day   time.Time
	Count int64
}

type PageCountRow struct {
	PageID        int64
	PageName      string
	Conversations int64
	Messages      int64
}

type StatsRepository interface {
	Totals(ctx context.Context) (Totals, error)
	MessagesByDay(ctx context.Context, days int) ([]DailyCount, error)
	CountsByPage(ctx context.Context) ([]PageCountRow, error)
	RecentCrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error)
}

type statsRepo struct{ db *repository.DB }

func NewStatsRepository(db *repository.DB) StatsRepository { return &statsRepo{db} }

func (r *statsRepo) Totals(ctx context.Context) (Totals, error) {
	var t Totals
	g := r.db.Gorm.WithContext(ctx)
	if err := g.Model(&entity.Page{}).Count(&t.Pages).Error; err != nil {
		return t, err
	}
	if err := g.Model(&entity.Conversation{}).Count(&t.Conversations).Error; err != nil {
		return t, err
	}
	if err := g.Model(&entity.Message{}).Count(&t.Messages).Error; err != nil {
		return t, err
	}
	return t, nil
}

func (r *statsRepo) MessagesByDay(ctx context.Context, days int) ([]DailyCount, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	var out []DailyCount
	err := r.db.Gorm.WithContext(ctx).
		Model(&entity.Message{}).
		Select("date_trunc('day', inserted_at) AS day, count(*) AS count").
		Where("inserted_at >= ?", since).
		Group("day").
		Order("day ASC").
		Scan(&out).Error
	return out, err
}

func (r *statsRepo) CountsByPage(ctx context.Context) ([]PageCountRow, error) {
	var out []PageCountRow
	err := r.db.Gorm.WithContext(ctx).
		Table("pages p").
		Select(`p.id AS page_id, p.name AS page_name,
			(SELECT count(*) FROM conversations c WHERE c.page_id = p.id) AS conversations,
			(SELECT count(*) FROM messages m WHERE m.page_id = p.id) AS messages`).
		Order("messages DESC").
		Scan(&out).Error
	return out, err
}

func (r *statsRepo) RecentCrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error) {
	if limit <= 0 {
		limit = 20
	}
	var out []entity.CrawlRun
	err := r.db.Gorm.WithContext(ctx).
		Order("started_at DESC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
