package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mock-bid-server/internal"
)

func main() {
	target := flag.String("target", "http://localhost:8080/bid", "URL to POST bid requests to")
	rps := flag.Int("rps", 10, "requests per second")
	imps := flag.Int("imps", 2, "impressions per request")
	duration := flag.Duration("duration", 30*time.Second, "total run time (0 = run until interrupted)")
	interval := flag.Duration("interval", 5*time.Second, "summary print interval")
	flag.Parse()

	g := &internal.Generator{
		Target:   *target,
		RPS:      *rps,
		Imps:     *imps,
		Duration: *duration,
		Interval: *interval,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := g.Run(ctx); err != nil {
		slog.Error("generator failed", "error", err)
		os.Exit(1)
	}
}
