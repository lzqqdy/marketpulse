package ingest

import (
	"context"
	"log/slog"
	"sync"
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
	equityCache   *equityCache
	equityBreaker *equityBreakers
	ingestStatus  *statusTracker
	liquidations  *liquidationWindow
	binanceStatus atomic.Value // string
	lastQuoteAt   atomic.Int64 // unix ms

	sgeGoldMu sync.RWMutex
	sgeGold   store.IndexQuote
	sgeGoldOK bool
}

// New creates an ingest service.
func New(cfg *config.Config, st *store.MarketStore) *Service {
	s := &Service{
		cfg:           cfg,
		store:         st,
		dayOpen:       newDayOpenCache(),
		equityCache:   newEquityCache(),
		equityBreaker: newEquityBreakers(),
		ingestStatus:  newStatusTracker(),
		liquidations:  newLiquidationWindow(time.Hour),
	}
	s.binanceStatus.Store("starting")
	s.ingestStatus.set("otc", "starting")
	s.ingestStatus.set("forex", "starting")
	s.ingestStatus.set("equity", "starting")
	s.ingestStatus.set("equity_sina", "starting")
	s.ingestStatus.set("equity_eastmoney", "starting")
	s.ingestStatus.set("equity_tencent", "starting")
	s.ingestStatus.set("macro", "starting")
	s.ingestStatus.set("crypto_meta", "starting")
	s.ingestStatus.set("long_short", "starting")
	s.ingestStatus.set("top_long_short", "starting")
	s.ingestStatus.set("funding", "starting")
	s.ingestStatus.set("open_interest", "starting")
	s.ingestStatus.set("taker_buy_sell", "starting")
	s.ingestStatus.set("liquidations", "starting")
	s.ingestStatus.set("liquidations_ws", "starting")
	s.ingestStatus.set("sge_gold", "starting")
	return s
}

func (s *Service) indicesWithSGE(rows []store.IndexQuote) []store.IndexQuote {
	s.sgeGoldMu.RLock()
	defer s.sgeGoldMu.RUnlock()
	if !s.sgeGoldOK {
		return rows
	}
	out := make([]store.IndexQuote, 0, len(rows)+1)
	for _, r := range rows {
		if r.ID != "sge-au9999" {
			out = append(out, r)
		}
	}
	q := s.sgeGold
	out = append(out, q)
	return out
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
	q := store.Quote{
		Symbol:       t.Symbol,
		PriceUsdt:    t.PriceUsdt,
		Change24hPct: t.Change24hPct,
		UpdatedAt:    t.EventTime,
	}
	if dayPct, ok := s.dayOpen.changePct(t.Symbol, t.PriceUsdt, now); ok {
		q.ChangeDayPct = dayPct
		s.store.UpdateQuote(q)
	} else {
		s.store.UpdateQuoteKeepDayPct(q)
	}
}
