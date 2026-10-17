package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"antares/api/internal/cache"
	"antares/api/internal/config"
	"antares/api/internal/httpapi"
	"antares/api/internal/hub"
	"antares/api/internal/jobs"
	"antares/api/internal/service"
	"antares/api/internal/store"
	"antares/api/internal/upstream"
)

var versionTimeout = 5 * time.Second

const (
	janitorInterval = time.Minute
	roomRetention   = time.Hour
	droppedMessage  = "nobody was connected or polling while the job waited, so it was dropped; resubmit and stay connected or keep polling until it starts"
)

type App struct {
	Handler http.Handler

	svc       *service.Service
	sched     *jobs.Scheduler
	hub       *hub.Hub
	jobs      jobs.Store
	clock     func() time.Time
	log       *slog.Logger
	runCancel context.CancelFunc
	stop      chan struct{}
	stopOnce  sync.Once
}

func New(ctx context.Context, cfg config.Config, db *sql.DB, shaula upstream.Shaula, fang upstream.Fang, log *slog.Logger) (*App, error) {
	clock := func() time.Time { return time.Now().UTC() }
	js := jobs.Store{DB: db}

	interrupted, err := js.FailInterrupted(ctx, clock())
	if err != nil {
		return nil, fmt.Errorf("recover interrupted jobs: %w", err)
	}
	if interrupted > 0 {
		log.Warn("marked jobs from the previous run as failed", "jobs", interrupted)
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	rooms := hub.New(cfg.RingSize, cfg.ConnBuffer, clock)

	svc := &service.Service{
		Targets:     store.Targets{DB: db},
		Features:    cache.FeatureCache{DB: db},
		Predictions: cache.PredictionCache{DB: db},
		Jobs:        js,
		Hub:         rooms,
		Shaula:      shaula,
		Fang:        fang,
		Clock:       clock,
		Cfg:         cfg,
		Log:         log,

		VersionTimeout: versionTimeout,
	}
	if err := svc.RefreshVersions(ctx); err != nil {
		log.Warn("upstream versions are not known yet", "error", err)
	}

	runner := &jobs.Runner{
		Jobs: js, Features: svc.Features, Predictions: svc.Predictions, Hub: rooms,
		Shaula: shaula, Fang: fang, Clock: clock, Log: log,
		JobTimeout: cfg.JobTimeout, StallTimeout: cfg.StallTimeout, TESSTTL: cfg.TESSTTL,
		ShaulaVersion: func() string { v, _ := svc.Versions(); return v },
	}
	sched := jobs.NewScheduler(jobs.SchedulerOptions{
		MaxActive: cfg.MaxActive,
		MaxQueue:  cfg.MaxQueue,
		Grace:     cfg.PromotionGrace,
		Present: func(id string, grace time.Duration) bool {
			room, ok := rooms.Lookup(id)
			return ok && room.Present(grace)
		},
		Run:     func(id string) { runner.Execute(runCtx, id) },
		Dropped: func(id string) { runner.Drop(context.Background(), id, droppedMessage) },
		Position: func(id string, position int) {
			if room, ok := rooms.Lookup(id); ok {
				room.Append(hub.State("queued", fmt.Sprintf("Position %d in the queue", position)))
			}
		},
	})
	svc.Sched = sched

	return &App{
		Handler:   httpapi.New(svc, cfg, log),
		svc:       svc,
		sched:     sched,
		hub:       rooms,
		jobs:      js,
		clock:     clock,
		log:       log,
		runCancel: runCancel,
		stop:      make(chan struct{}),
	}, nil
}

func (a *App) Start() { go a.janitor() }

func (a *App) janitor() {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
			a.Tick(context.Background())
		}
	}
}

func (a *App) Tick(ctx context.Context) {
	a.sched.Sweep()
	a.hub.Reap(a.clock().Add(-roomRetention))
	if n, err := a.jobs.DeleteExpired(ctx, a.clock()); err != nil {
		a.log.Error("delete expired jobs", "error", err)
	} else if n > 0 {
		a.log.Info("deleted expired jobs", "jobs", n)
	}
	if err := a.svc.RefreshVersions(ctx); err != nil {
		a.log.Warn("refresh upstream versions", "error", err)
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	a.stopOnce.Do(func() { close(a.stop) })

	a.sched.Stop()
	drained := make(chan struct{})
	go func() {
		a.sched.Wait()
		close(drained)
	}()
	select {
	case <-drained:
	case <-ctx.Done():
		a.runCancel()
		<-drained
	}
	a.runCancel()
	return nil
}
