package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type Overview struct {
	Totals reporead.Totals
	ByDay  []reporead.DailyCount
	ByPage []reporead.PageCountRow
}

type StatsService interface {
	Overview(ctx context.Context, days int) (Overview, error)
	CrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error)
}

type statsService struct{ repo StatsRepo }

func NewStatsService(repo StatsRepo) StatsService { return &statsService{repo: repo} }

func (s *statsService) Overview(ctx context.Context, days int) (Overview, error) {
	if days <= 0 {
		days = 30
	}
	totals, err := s.repo.Totals(ctx)
	if err != nil {
		return Overview{}, err
	}
	byDay, err := s.repo.MessagesByDay(ctx, days)
	if err != nil {
		return Overview{}, err
	}
	byPage, err := s.repo.CountsByPage(ctx)
	if err != nil {
		return Overview{}, err
	}
	return Overview{Totals: totals, ByDay: byDay, ByPage: byPage}, nil
}

func (s *statsService) CrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error) {
	return s.repo.RecentCrawlRuns(ctx, limit)
}
