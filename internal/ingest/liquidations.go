package ingest

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ingest/derivatives"
	"github.com/lzqqdy/marketpulse/internal/store"
)

type liquidationWindow struct {
	mu       sync.Mutex
	lookback time.Duration
	events   []liquidationSample
}

type liquidationSample struct {
	t        time.Time
	side     string
	notional float64
}

func newLiquidationWindow(lookback time.Duration) *liquidationWindow {
	if lookback <= 0 {
		lookback = time.Hour
	}
	return &liquidationWindow{lookback: lookback}
}

func (w *liquidationWindow) add(order derivatives.LiquidationOrder) store.Liquidations {
	if order.EventTime.IsZero() {
		order.EventTime = time.Now().UTC()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, liquidationSample{
		t:        order.EventTime,
		side:     order.Side,
		notional: order.Notional,
	})
	return w.snapshotLocked(time.Now().UTC())
}

func (w *liquidationWindow) snapshot() store.Liquidations {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshotLocked(time.Now().UTC())
}

func (w *liquidationWindow) snapshotLocked(now time.Time) store.Liquidations {
	cutoff := now.Add(-w.lookback)
	kept := w.events[:0]
	longUsd := 0.0
	shortUsd := 0.0
	var latest time.Time
	for _, ev := range w.events {
		if ev.t.Before(cutoff) {
			continue
		}
		kept = append(kept, ev)
		if ev.t.After(latest) {
			latest = ev.t
		}
		switch ev.side {
		case "SELL":
			longUsd += ev.notional
		case "BUY":
			shortUsd += ev.notional
		}
	}
	w.events = kept
	if latest.IsZero() {
		latest = now
	}
	return store.Liquidations{
		Window:    "1h",
		LongUsd:   longUsd,
		ShortUsd:  shortUsd,
		TotalUsd:  longUsd + shortUsd,
		UpdatedAt: latest,
	}
}

func (s *Service) runLiquidationsWithRetry(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			s.ingestStatus.set("liquidations_ws", "disconnected")
			return
		}

		s.ingestStatus.set("liquidations_ws", "connecting")
		slog.Info("binance liquidation stream connect", "url", derivatives.AllLiquidationsStreamURL)
		err := derivatives.RunAllLiquidations(ctx, derivatives.AllLiquidationsStreamURL, func() {
			s.ingestStatus.set("liquidations_ws", "connected")
		}, s.onLiquidation)
		if ctx.Err() != nil {
			s.ingestStatus.set("liquidations_ws", "disconnected")
			return
		}

		s.ingestStatus.set("liquidations_ws", "reconnecting")
		slog.Warn("binance liquidation stream disconnected", "err", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			s.ingestStatus.set("liquidations_ws", "disconnected")
			return
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (s *Service) onLiquidation(order derivatives.LiquidationOrder) {
	s.ingestStatus.set("liquidations_ws", "connected")
	s.store.SetLiquidations(s.liquidations.add(order))
	s.ingestStatus.set("liquidations", "ok")
}
