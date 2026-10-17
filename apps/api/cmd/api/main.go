// Command api serves the Antares public HTTP API in front of the private Shaula and Fang gRPC
// services. It reads its settings from ANTARES_* environment variables (see internal/config), exits
// with status 2 on invalid configuration, and shuts down gracefully on SIGINT or SIGTERM.
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

	"antares/api/internal/app"
	"antares/api/internal/config"
	"antares/api/internal/store"
	"antares/api/internal/upstream"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	if err := run(log, cfg); err != nil {
		log.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}

	shaulaConn, err := upstream.Dial(cfg.ShaulaAddr)
	if err != nil {
		return err
	}
	defer shaulaConn.Close()
	fangConn, err := upstream.Dial(cfg.FangAddr)
	if err != nil {
		return err
	}
	defer fangConn.Close()

	application, err := app.New(ctx, cfg, db, upstream.NewShaula(shaulaConn), upstream.NewFang(fangConn), log)
	if err != nil {
		return err
	}
	application.Start()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           application.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr, "shaula", cfg.ShaulaAddr, "fang", cfg.FangAddr)

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}

	// Stop taking requests first, then drain running jobs; both steps share one 30 s budget.
	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	return application.Shutdown(shutdown)
}
