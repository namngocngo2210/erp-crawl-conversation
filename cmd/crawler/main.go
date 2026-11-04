package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/samber/do/v2"

	"github.com/desi/crawl-conversations/internal/business"
	"github.com/desi/crawl-conversations/internal/config"
	"github.com/desi/crawl-conversations/internal/repository"
	httpx "github.com/desi/crawl-conversations/internal/transport/http"
	api "github.com/desi/crawl-conversations/internal/transport/http/api"
	"github.com/desi/crawl-conversations/internal/transport/scheduler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	if err := repository.RunMigrations(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		panic(err)
	}

	injector := buildInjector(cfg)
	defer func() { _ = injector.Shutdown() }()

	log := do.MustInvoke[*slog.Logger](injector)
	db := do.MustInvoke[*repository.DB](injector)
	defer db.Close()

	sched := do.MustInvoke[scheduler.Scheduler](injector)
	if err := sched.Start(); err != nil {
		log.Error("scheduler start failed", "err", err)
		os.Exit(1)
	}
	defer sched.Stop()

	// touch crawl service so a wiring error surfaces at startup, not at 21:00
	_ = do.MustInvoke[business.CrawlService](injector)

	apiDeps := do.MustInvoke[api.Deps](injector)
	if err := apiDeps.Auth.SeedAdmin(context.Background(), cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Error("seed admin failed", "err", err)
	}

	srv := httpx.NewServer(cfg.HTTPAddr, log, sched.Trigger, apiDeps)
	go func() {
		log.Info("http server starting", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("shutdown complete")
}
