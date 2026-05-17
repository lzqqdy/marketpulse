package ingest

import (
	"context"
	"log/slog"
	"time"
)

// runPoller invokes fn immediately, then on each interval until ctx ends.
func runPoller(ctx context.Context, interval time.Duration, name string, fn func(context.Context) error) {
	if interval <= 0 {
		interval = time.Minute
	}
	run := func() {
		if err := fn(ctx); err != nil {
			slog.Warn("ingest poll failed", "name", name, "err", err)
		}
	}
	run()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
