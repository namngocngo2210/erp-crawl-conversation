package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/do/v2"

	"github.com/desi/crawl-conversations/internal/business"
	"github.com/desi/crawl-conversations/internal/config"
	"github.com/desi/crawl-conversations/internal/observability"
	"github.com/desi/crawl-conversations/internal/pancake"
	"github.com/desi/crawl-conversations/internal/repository"
	readbiz "github.com/desi/crawl-conversations/internal/business/read"
	readrepo "github.com/desi/crawl-conversations/internal/repository/read"
	"github.com/desi/crawl-conversations/internal/token"
	"github.com/desi/crawl-conversations/internal/transport/scheduler"
	api "github.com/desi/crawl-conversations/internal/transport/http/api"
)

// buildInjector registers every component as a provider and returns the container.
func buildInjector(cfg config.Config) *do.RootScope {
	i := do.New()

	do.ProvideValue(i, cfg)
	do.ProvideValue(i, observability.NewLogger(cfg.LogLevel))

	do.Provide(i, func(in do.Injector) (*repository.DB, error) {
		return repository.New(context.Background(), do.MustInvoke[config.Config](in).DatabaseURL)
	})
	do.Provide(i, func(in do.Injector) (*redis.Client, error) {
		opt, err := redis.ParseURL(do.MustInvoke[config.Config](in).RedisURL)
		if err != nil {
			return nil, err
		}
		return redis.NewClient(opt), nil
	})
	do.Provide(i, func(in do.Injector) (*pancake.RateLimiter, error) {
		c := do.MustInvoke[config.Config](in)
		return pancake.NewRateLimiter(c.RateLimitPerPageRPS, c.RateLimitPerPageBurst), nil
	})
	do.Provide(i, func(in do.Injector) (pancake.Client, error) {
		c := do.MustInvoke[config.Config](in)
		rl := do.MustInvoke[*pancake.RateLimiter](in)
		// NewClient(pageBaseURL, accountBaseURL, accessToken, rl, hc) — see api-addendum
		return pancake.NewClient(c.PancakeBaseURL, c.PancakeAccountBaseURL, c.PancakeAccessToken, rl, nil), nil
	})
	do.Provide(i, func(in do.Injector) (token.Manager, error) {
		c := do.MustInvoke[config.Config](in)
		return token.NewManager(do.MustInvoke[*redis.Client](in), do.MustInvoke[pancake.Client](in), c.TokenTTL), nil
	})
	do.Provide(i, func(in do.Injector) (business.Repos, error) {
		db := do.MustInvoke[*repository.DB](in)
		return business.Repos{
			Page:         repository.NewPageRepository(db),
			Customer:     repository.NewCustomerRepository(db),
			Tag:          repository.NewTagRepository(db),
			Conversation: repository.NewConversationRepository(db),
			Message:      repository.NewMessageRepository(db),
			Checkpoint:   repository.NewCheckpointRepository(db),
		}, nil
	})
	do.Provide(i, func(in do.Injector) (business.PageProcessor, error) {
		return business.NewPageProcessor(
			do.MustInvoke[pancake.Client](in),
			do.MustInvoke[token.Manager](in),
			do.MustInvoke[business.Repos](in),
			5*time.Second,
		), nil
	})
	do.Provide(i, func(in do.Injector) (business.CrawlService, error) {
		c := do.MustInvoke[config.Config](in)
		db := do.MustInvoke[*repository.DB](in)
		return business.NewCrawlService(
			do.MustInvoke[pancake.Client](in),
			do.MustInvoke[business.PageProcessor](in),
			do.MustInvoke[business.Repos](in),
			repository.NewCrawlRunRepository(db),
			c.WorkerPoolSize,
			do.MustInvoke[*slog.Logger](in),
		), nil
	})
	do.Provide(i, func(in do.Injector) (scheduler.Scheduler, error) {
		c := do.MustInvoke[config.Config](in)
		svc := do.MustInvoke[business.CrawlService](in)
		return scheduler.NewScheduler(c.CronSchedule, c.CronTimezone, svc.Run, do.MustInvoke[*slog.Logger](in))
	})

	do.Provide(i, func(in do.Injector) (readbiz.AuthService, error) {
		c := do.MustInvoke[config.Config](in)
		db := do.MustInvoke[*repository.DB](in)
		users := readrepo.NewUserRepository(db)
		return readbiz.NewAuthService(users, c.JWTSecret, c.JWTTTL), nil
	})
	do.Provide(i, func(in do.Injector) (readbiz.QueryService, error) {
		db := do.MustInvoke[*repository.DB](in)
		return readbiz.NewQueryService(readrepo.NewReadRepository(db)), nil
	})
	do.Provide(i, func(in do.Injector) (readbiz.SearchService, error) {
		db := do.MustInvoke[*repository.DB](in)
		return readbiz.NewSearchService(readrepo.NewSearchRepository(db)), nil
	})
	do.Provide(i, func(in do.Injector) (readbiz.StatsService, error) {
		db := do.MustInvoke[*repository.DB](in)
		return readbiz.NewStatsService(readrepo.NewStatsRepository(db)), nil
	})
	do.Provide(i, func(in do.Injector) (api.Deps, error) {
		c := do.MustInvoke[config.Config](in)
		return api.Deps{
			Auth:       do.MustInvoke[readbiz.AuthService](in),
			Query:      do.MustInvoke[readbiz.QueryService](in),
			Search:     do.MustInvoke[readbiz.SearchService](in),
			Stats:      do.MustInvoke[readbiz.StatsService](in),
			CORSOrigin: c.CORSOrigin,
		}, nil
	})

	return i
}
