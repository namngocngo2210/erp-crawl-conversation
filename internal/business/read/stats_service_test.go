package read

import (
	"context"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type fakeStats struct{ days int }

func (f *fakeStats) Totals(context.Context) (reporead.Totals, error) {
	return reporead.Totals{Pages: 2, Conversations: 5, Messages: 99}, nil
}
func (f *fakeStats) MessagesByDay(_ context.Context, days int) ([]reporead.DailyCount, error) {
	f.days = days
	return nil, nil
}
func (f *fakeStats) CountsByPage(context.Context) ([]reporead.PageCountRow, error) { return nil, nil }
func (f *fakeStats) RecentCrawlRuns(context.Context, int) ([]entity.CrawlRun, error) {
	return nil, nil
}

func TestOverviewDefaultDays(t *testing.T) {
	fs := &fakeStats{}
	svc := NewStatsService(fs)
	ov, err := svc.Overview(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if ov.Totals.Messages != 99 {
		t.Fatalf("want 99, got %d", ov.Totals.Messages)
	}
	if fs.days != 30 {
		t.Fatalf("want default 30 days, got %d", fs.days)
	}
}
