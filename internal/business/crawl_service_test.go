package business

import (
	"context"
	"log/slog"
	"testing"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

type listPagesClient struct{ fakeClient }

func (listPagesClient) ListPages(ctx context.Context) ([]dto.PageDTO, error) {
	return []dto.PageDTO{{ID: "pg1", Name: "Shop", InsertedAt: convTime}}, nil
}

type spyProcessor struct{ calls int }

func (s *spyProcessor) Process(ctx context.Context, page entity.Page) (Stats, error) {
	s.calls++
	return Stats{Conversations: 1, Messages: 2}, nil
}

type spyRuns struct {
	started  bool
	finished string
}

func (s *spyRuns) Start(ctx context.Context, trigger string) (int64, error) {
	s.started = true
	return 1, nil
}
func (s *spyRuns) Finish(ctx context.Context, id int64, status, errMsg string, stats []byte) error {
	s.finished = status
	return nil
}
func (s *spyRuns) FailStale(ctx context.Context) (int64, error) { return 0, nil }

func TestCrawlService_Run_Success(t *testing.T) {
	proc := &spyProcessor{}
	runs := &spyRuns{}
	repos := newRepos(&spyMsg{}, &stubCheckpoint{}, &spyCustomer{}, &stubConv{})
	svc := NewCrawlService(listPagesClient{}, proc, repos, runs, 4, slog.Default())
	require.NoError(t, svc.Run(context.Background(), "manual"))
	require.True(t, runs.started)
	require.Equal(t, "success", runs.finished)
	require.Equal(t, 1, proc.calls)
}
