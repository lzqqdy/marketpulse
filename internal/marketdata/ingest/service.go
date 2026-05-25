package ingest

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/alpha"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/binance"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

// Service runs background market data ingestors.
type Service struct {
	cfg   *config.Config
	store *store.MarketStore

	dayOpen        *dayOpenCache
	equityCache    *equityCache
	equityBreaker  *equityBreakers
	ingestStatus   *statusTracker
	providerHealth *ProviderHealthStore
	liquidations   *liquidationWindow
	binanceStatus  atomic.Value // string
	alphaStatus    atomic.Value // string
	lastQuoteAt    atomic.Int64 // unix ms
	lastAlphaAt    atomic.Int64 // unix ms
	alphaItems     atomic.Value // []alpha.ResolvedItem

	sgeGoldMu sync.RWMutex
	sgeGold   store.IndexQuote
	sgeGoldOK bool
}

// New creates an ingest service.
func New(cfg *config.Config, st *store.MarketStore) *Service {
	s := &Service{
		cfg:            cfg,
		store:          st,
		dayOpen:        newDayOpenCache(),
		equityCache:    newEquityCache(),
		equityBreaker:  newEquityBreakers(),
		ingestStatus:   newStatusTracker(),
		providerHealth: newProviderHealthStore(defaultProviderDefs(cfg.Alpha.Enabled)),
		liquidations:   newLiquidationWindow(time.Hour),
	}
	s.binanceStatus.Store("starting")
	s.alphaStatus.Store("disabled")
	s.ingestStatus.set("otc", "starting")
	s.ingestStatus.set("forex", "starting")
	s.ingestStatus.set("equity", "starting")
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
	if cfg.Alpha.Enabled {
		s.ingestStatus.set("alpha_poll", "starting")
		s.seedAlphaDefaults()
	} else {
		s.providerHealth.ReportDisabled("binance_alpha")
	}
	return s
}

func (s *Service) seedAlphaDefaults() {
	now := time.Now().UTC()
	toRows := func(items []config.AlphaItem, category string) []store.AlphaQuote {
		rows := make([]store.AlphaQuote, 0, len(items))
		for _, item := range items {
			rows = append(rows, store.AlphaQuote{
				ID:        item.ID,
				Name:      item.Name,
				Symbol:    strings.TrimSuffix(strings.ToUpper(item.Symbol), s.cfg.Alpha.QuoteAsset),
				UpdatedAt: now,
				Source:    "binance-alpha",
				Category:  category,
			})
		}
		return rows
	}
	s.store.SetAlphaDefaults(toRows(s.cfg.Alpha.Indices, "index"), toRows(s.cfg.Alpha.Stocks, "stock"))
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
	go s.runAlphaWithRetry(ctx)
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

func (s *Service) AlphaStatus() string {
	if v := s.alphaStatus.Load(); v != nil {
		return v.(string)
	}
	return "unknown"
}

func (s *Service) LastAlphaMs() int64 {
	return s.lastAlphaAt.Load()
}

func (s *Service) AlphaSymbolForBase(symbol string) (string, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return "", false
	}
	if v := s.alphaItems.Load(); v != nil {
		for _, item := range v.([]alpha.ResolvedItem) {
			if item.BaseSymbol == symbol && item.AlphaSymbol != "" {
				return item.AlphaSymbol, true
			}
		}
	}
	resolved := alpha.ResolveItems(httpClient, s.cfg.Alpha.Indices, s.cfg.Alpha.Stocks, s.cfg.Alpha.QuoteAsset)
	s.alphaItems.Store(resolved)
	for _, item := range resolved {
		if item.BaseSymbol == symbol && item.AlphaSymbol != "" {
			return item.AlphaSymbol, true
		}
	}
	return "", false
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
		s.providerHealth.ReportFailure("binance_spot_ws", err)
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

func (s *Service) runAlphaWithRetry(ctx context.Context) {
	if !s.cfg.Alpha.Enabled {
		s.alphaStatus.Store("disabled")
		s.ingestStatus.set("alpha_poll", "disabled")
		s.providerHealth.ReportDisabled("binance_alpha")
		return
	}
	items := s.cfg.AlphaItems()
	if len(items) == 0 {
		s.alphaStatus.Store("disabled")
		s.ingestStatus.set("alpha_poll", "disabled")
		s.providerHealth.ReportDisabled("binance_alpha")
		return
	}

	backoff := time.Second
	const maxBackoff = 30 * time.Second
	const pollInterval = 30 * time.Second
	const resolveInterval = 10 * time.Minute

	for {
		if ctx.Err() != nil {
			s.alphaStatus.Store("disconnected")
			s.ingestStatus.set("alpha_poll", "disconnected")
			return
		}

		s.alphaStatus.Store("polling")
		s.ingestStatus.set("alpha_poll", "polling")
		resolved := s.resolveAlphaItems()
		if len(resolved) == 0 {
			s.alphaStatus.Store("reconnecting")
			s.ingestStatus.set("alpha_poll", "reconnecting")
			slog.Warn("alpha no supported symbols", "transport", "rest_poll", "retry_in", backoff)
			select {
			case <-ctx.Done():
				s.alphaStatus.Store("disconnected")
				s.ingestStatus.set("alpha_poll", "disconnected")
				return
			case <-time.After(backoff):
			}
			backoff = growBackoff(backoff, maxBackoff)
			continue
		}

		backoff = time.Second
		slog.Info("alpha polling started", "symbols", alphaItemSymbols(resolved), "interval", pollInterval, "transport", "rest_poll")
		s.pollAlphaTickers(resolved)

		pollTicker := time.NewTicker(pollInterval)
		resolveTicker := time.NewTicker(resolveInterval)
		for {
			select {
			case <-ctx.Done():
				pollTicker.Stop()
				resolveTicker.Stop()
				s.alphaStatus.Store("disconnected")
				s.ingestStatus.set("alpha_poll", "disconnected")
				return
			case <-pollTicker.C:
				s.pollAlphaTickers(resolved)
			case <-resolveTicker.C:
				pollTicker.Stop()
				resolveTicker.Stop()
				goto refresh
			}
		}
	refresh:
	}
}

func (s *Service) resolveAlphaItems() []alpha.ResolvedItem {
	resolved := alpha.ResolveItems(httpClient, s.cfg.Alpha.Indices, s.cfg.Alpha.Stocks, s.cfg.Alpha.QuoteAsset)
	s.alphaItems.Store(resolved)
	out := make([]alpha.ResolvedItem, 0, len(resolved))
	for _, item := range resolved {
		if !strings.HasPrefix(item.AlphaSymbol, "ALPHA_") {
			slog.Warn("alpha symbol unsupported", "symbol", item.Item.Symbol, "alpha_symbol", item.AlphaSymbol, "id", item.Item.ID)
			continue
		}
		out = append(out, item)
	}
	s.alphaItems.Store(out)
	slog.Info("alpha enabled", "indices", len(s.cfg.Alpha.Indices), "stocks", len(s.cfg.Alpha.Stocks), "supported", len(out))
	slog.Info("alpha subscribed symbols", "symbols", alphaItemSymbols(out), "transport", "rest_poll")
	return out
}

func (s *Service) pollAlphaTickers(items []alpha.ResolvedItem) {
	succeeded := 0
	start := time.Now()
	var lastErr error
	references, err := alpha.FetchReferenceTickers(httpClient, items, s.cfg.Alpha.QuoteAsset)
	if err != nil {
		lastErr = err
		slog.Warn("alpha reference quote poll failed", "requested", len(items), "err", err)
	}
	for _, item := range items {
		ticker, ok := references[item.BaseSymbol]
		err = nil
		if !ok {
			ticker, err = alpha.FetchTicker(httpClient, item.AlphaSymbol)
		}
		if err != nil {
			lastErr = err
			slog.Warn("alpha ticker poll failed", "symbol", item.Item.Symbol, "alpha_symbol", item.AlphaSymbol, "id", item.Item.ID, "err", err)
			continue
		}
		s.onAlphaTicker(item, ticker)
		succeeded++
	}
	if succeeded == 0 {
		s.alphaStatus.Store("error")
		s.ingestStatus.set("alpha_poll", "error")
		s.providerHealth.ReportFailure("binance_alpha", lastErr)
		slog.Warn("alpha ticker poll failed for all symbols", "requested", len(items), "transport", "rest_poll")
		return
	}
	s.providerHealth.ReportSuccess("binance_alpha", time.Since(start))
	s.providerHealth.ReportUsed("binance_alpha", true)
	slog.Info("alpha ticker poll fetched", "requested", len(items), "succeeded", succeeded, "transport", "rest_poll")
}

func (s *Service) onAlphaTicker(item alpha.ResolvedItem, t alpha.Ticker) {
	s.lastAlphaAt.Store(time.Now().UnixMilli())
	s.alphaStatus.Store("connected")
	s.ingestStatus.set("alpha_poll", "connected")

	s.store.UpdateAlphaQuote(store.AlphaQuote{
		ID:           item.Item.ID,
		Name:         item.Item.Name,
		Symbol:       item.BaseSymbol,
		Price:        t.Price,
		Change24hPct: t.Change24hPct,
		ChangeDayPct: t.Change24hPct,
		Volume:       t.Volume,
		UpdatedAt:    t.UpdatedAt,
		Source:       "binance-alpha",
		Category:     item.Category,
	})
}

func growBackoff(current, max time.Duration) time.Duration {
	if current < max {
		current *= 2
		if current > max {
			return max
		}
	}
	return current
}

func alphaItemSymbols(items []alpha.ResolvedItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.AlphaSymbol)
	}
	return out
}

func (s *Service) onTicker(t binance.TickerUpdate) {
	s.lastQuoteAt.Store(time.Now().UnixMilli())
	s.binanceStatus.Store("connected")
	if !t.EventTime.IsZero() {
		s.providerHealth.ReportSuccess("binance_spot_ws", time.Since(t.EventTime))
	} else {
		s.providerHealth.ReportSuccess("binance_spot_ws", 0)
	}
	s.providerHealth.ReportUsed("binance_spot_ws", true)

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
		s.dayOpen.setFallback(t.Symbol, t.PriceUsdt, now)
		if dayPct, ok := s.dayOpen.changePct(t.Symbol, t.PriceUsdt, now); ok {
			q.ChangeDayPct = dayPct
			s.store.UpdateQuote(q)
			return
		}
		s.store.UpdateQuoteKeepDayPct(q)
	}
}
