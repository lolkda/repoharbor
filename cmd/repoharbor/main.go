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

	repoharbor "repoharbor"
	"repoharbor/internal/api"
	"repoharbor/internal/config"
	"repoharbor/internal/events"
	"repoharbor/internal/gitrepo"
	"repoharbor/internal/scheduler"
	"repoharbor/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DBPath, repoharbor.Migrations)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	git := gitrepo.New(cfg.ReposDir)
	broker := events.NewBroker()
	sched := scheduler.New(cfg, st, git, broker)
	sched.Start(ctx)

	handler := api.New(cfg, st, git, sched, broker, repoharbor.WebFS).Routes()
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("RepoHarbor listening", "addr", cfg.Addr, "data", cfg.DataDir)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
