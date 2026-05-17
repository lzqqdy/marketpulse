package ingest

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/ingest/binance"
	"github.com/lzqqdy/marketpulse/internal/store"
)

// Service runs background market data ingestors.
type Service struct {
	cfg   *config.Config
	store *store.MarketStore

	dayOpen       *dayOpenCache
	ingestStatus  *statusTracker
	liquidations  *liquidationWindow
	binanceStatus atomic.Value // string
	lastQuoteAt   atomic.Int64 // unix ms
}

// New creates an ingest service.
func New(cfg *config.Config, st *store.MarketStore) *Service {
	s := &Service{
		cfg:          cfg,
		store:        st,
		dayOpen:      newDayOpenCache(),
		ingestStatus: newStatusTracker(),
		liquidations: newLiquidationWindow(time.Hour),
	}
	s.binanceStatus.Store("starting")
	s.ingestStatus.set("otc", "starting")
	s.ingestStatus.set("forex", "starting")
	s.ingestStatus.set("equity", "starting")
	s.ingestStatus.set("macro", "starting")
	s.ingestStatus.set("crypto_meta", "starting")
	s.ingestStatus.set("long_short", "starting")
	s.ingestStatus.set("top_long_short", "starting")
	s.ingestStatus.set("funding", "starting")
	s.ingestStatus.set("open_interest", "starting")
	s.ingestStatus.set("taker_buy_sell", "starting")
	s.ingestStatus.set("liquidations", "starting")
	s.ingestStatus.set("liquidations_ws", "starting")
	s.ingestStatus.set("sge_gold", "disabled")
	return s
}

// Start launches ingest goroutines until ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	go s.runDayOpenLoop(ctx)
	go s.runBinanceWithRetry(ctx)
	go s.runLiquidationsWithRetry(ctx)
	s.startSlowIngest(ctx)
}

// BinanceStatus returns connected | reconnecting | disconnected | starting.
func (s *Service) BinanceStatus() string {
	if v := s.binanceStatus.Load(); v != nil {
		return v.(string)
	}
	return "unknown"
}

// LastQuoteMs is last ticker event time (0 if none).
func (s *Service) LastQuoteMs() int64 {
	return s.lastQuoteAt.Load()
}

func (s *Service) runBinanceWithRetry(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	url := s.cfg.BinanceStreamURL()

	for {
		if ctx.Err() != nil {
			s.binanceStatus.Store("disconnected")
			return
		}

		s.binanceStatus.Store("connecting")
		slog.Info("binance miniTicker connect", "url", url)

		err := binance.RunMiniTicker(ctx, url, s.onTicker)
		backoff = time.Second
		if ctx.Err() != nil {
			s.binanceStatus.Store("disconnected")
			return
		}

		s.binanceStatus.Store("reconnecting")
		slog.Warn("binance miniTicker disconnected", "err", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			s.binanceStatus.Store("disconnected")
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

func (s *Service) onTicker(t binance.TickerUpdate) {
	s.lastQuoteAt.Store(time.Now().UnixMilli())
	s.binanceStatus.Store("connected")

	now := time.Now()
	dayPct, ok := s.dayOpen.changePct(t.Symbol, t.PriceUsdt, now)
	if !ok {
		dayPct = 0
	}

	s.store.UpdateQuote(store.Quote{
		Symbol:       t.Symbol,
		PriceUsdt:    t.PriceUsdt,
		ChangeDayPct: dayPct,
		Change24hPct: t.Change24hPct,
		UpdatedAt:    t.EventTime,
	})
}
