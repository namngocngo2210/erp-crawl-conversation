package scheduler

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

type RunFunc func(ctx context.Context, trigger string) error

type Scheduler interface {
	Start() error
	Stop()
	Trigger() bool
}

type scheduler struct {
	cron    *cron.Cron
	spec    string
	run     RunFunc
	log     *slog.Logger
	running atomic.Bool
}

func NewScheduler(spec, tz string, run RunFunc, log *slog.Logger) (Scheduler, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}
	return &scheduler{
		cron: cron.New(cron.WithLocation(loc)),
		spec: spec,
		run:  run,
		log:  log,
	}, nil
}

func (s *scheduler) Start() error {
	_, err := s.cron.AddFunc(s.spec, func() { s.execute("cron") })
	if err != nil {
		return err
	}
	s.cron.Start()
	s.log.Info("scheduler started", "spec", s.spec)
	return nil
}

func (s *scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *scheduler) Trigger() bool {
	if !s.running.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		defer s.running.Store(false)
		if err := s.run(context.Background(), "manual"); err != nil {
			s.log.Error("manual run failed", "err", err)
		}
	}()
	return true
}

func (s *scheduler) execute(trigger string) {
	if !s.running.CompareAndSwap(false, true) {
		s.log.Warn("crawl already running, skipping", "trigger", trigger)
		return
	}
	defer s.running.Store(false)
	if err := s.run(context.Background(), trigger); err != nil {
		s.log.Error("scheduled run failed", "trigger", trigger, "err", err)
	}
}
