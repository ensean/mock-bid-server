package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mock-bid-server/internal"
)

func main() {
	cfg, err := internal.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	if len(cfg.DSPs) == 0 {
		slog.Error("no DSPs configured — add at least one entry under 'dsps:' in config.yaml")
		os.Exit(1)
	}

	slog.Info("adx starting",
		"port", cfg.AdxPort,
		"floor_cpm", cfg.AdxFloorCPM,
		"timeout_ms", cfg.AdxTimeoutMS,
		"dsp_count", len(cfg.DSPs))
	for _, d := range cfg.DSPs {
		slog.Info("dsp registered", "id", d.ID, "url", d.URL)
	}

	adx := internal.NewADX(cfg)
	handler := internal.NewADXHandler(adx)
	mux := http.NewServeMux()
	mux.Handle("/openrtb", handler)

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.AdxPort), Handler: mux}

	go func() {
		slog.Info("adx server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("adx server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("adx shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("adx shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("adx stopped")
}
