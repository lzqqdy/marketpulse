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

// runDynamicPoller invokes fn immediately, then sleeps for the interval computed
// after each run. It is useful when market state changes the desired cadence.
func runDynamicPoller(ctx context.Context, name string, interval func() time.Duration, fn func(context.Context) error) {
	run := func() {
		if err := fn(ctx); err != nil {
			slog.Warn("ingest poll failed", "name", name, "err", err)
		}
	}
	run()

	for {
		next := interval()
		if next <= 0 {
			next = time.Minute
		}
		timer := time.NewTimer(next)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			run()
		}
	}
}
