package business

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
	"github.com/desi/crawl-conversations/internal/transport/worker"
)

type CrawlService interface {
	Run(ctx context.Context, trigger string) error
}

type crawlService struct {
	client    PancakeClient
	processor PageProcessor
	repos     Repos
	runs      repository.CrawlRunRepository
	poolSize  int
	log       *slog.Logger
}

func NewCrawlService(client PancakeClient, processor PageProcessor, repos Repos, runs repository.CrawlRunRepository, poolSize int, log *slog.Logger) CrawlService {
	return &crawlService{client: client, processor: processor, repos: repos, runs: runs, poolSize: poolSize, log: log}
}

func (s *crawlService) Run(ctx context.Context, trigger string) error {
	if n, err := s.runs.FailStale(ctx); err != nil {
		s.log.Warn("failed to reconcile stale runs", "err", err)
	} else if n > 0 {
		s.log.Info("reconciled stale crawl_runs", "count", n)
	}

	runID, err := s.runs.Start(ctx, trigger)
	if err != nil {
		return err
	}

	pages, err := s.client.ListPages(ctx)
	if err != nil {
		_ = s.runs.Finish(ctx, runID, "failed", err.Error(), nil)
		return err
	}

	entities := make([]entity.Page, 0, len(pages))
	for _, p := range pages {
		raw, _ := json.Marshal(p.Raw)
		id, err := s.repos.Page.Upsert(ctx, entity.Page{PancakePageID: p.ID, Name: p.Name, Platform: p.Platform, Raw: raw, Status: "active"})
		if err != nil {
			_ = s.runs.Finish(ctx, runID, "failed", err.Error(), nil)
			return err
		}
		entities = append(entities, entity.Page{ID: id, PancakePageID: p.ID, Name: p.Name, InsertedAt: p.InsertedAt})
	}

	var total Stats
	jobs := make([]worker.Job, len(entities))
	stats := make([]Stats, len(entities))
	for i := range entities {
		i := i
		jobs[i] = func(ctx context.Context) error {
			st, err := s.processor.Process(ctx, entities[i])
			stats[i] = st
			if err != nil {
				s.log.Error("page process failed", "page", entities[i].PancakePageID, "err", err)
			}
			return err
		}
	}
	errs := worker.Run(ctx, s.poolSize, jobs)

	status := "success"
	errMsg := ""
	for i, e := range errs {
		total.Conversations += stats[i].Conversations
		total.Messages += stats[i].Messages
		total.Customers += stats[i].Customers
		if e != nil {
			status = "failed"
			errMsg = e.Error()
		}
	}
	statsJSON, _ := json.Marshal(total)
	return s.runs.Finish(ctx, runID, status, errMsg, statsJSON)
}
