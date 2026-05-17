package ingest

import (
	"context"
	"log/slog"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ingest/crypto"
	"github.com/lzqqdy/marketpulse/internal/ingest/derivatives"
	"github.com/lzqqdy/marketpulse/internal/ingest/equity"
	"github.com/lzqqdy/marketpulse/internal/ingest/forex"
	"github.com/lzqqdy/marketpulse/internal/ingest/macro"
	"github.com/lzqqdy/marketpulse/internal/ingest/metals"
	"github.com/lzqqdy/marketpulse/internal/ingest/otc"
	"github.com/lzqqdy/marketpulse/internal/store"
)

func (s *Service) startSlowIngest(ctx context.Context) {
	go runPoller(ctx, s.cfg.Ingest.OTC.USDTCNYInterval, "otc", s.pollOTC)
	go runPoller(ctx, s.cfg.Ingest.Forex.USDCNYInterval, "forex", s.pollForex)
	go runPoller(ctx, s.cfg.Ingest.Equity.Interval, "equity", s.pollEquity)
	go runPoller(ctx, s.cfg.Ingest.Equity.Interval, "sge_gold", s.pollSGE)
	go runPoller(ctx, s.cfg.Ingest.Macro.Interval, "macro", s.pollMacro)
	go runPoller(ctx, s.cfg.Ingest.Macro.Interval, "crypto_meta", s.pollCryptoMeta)
	go runPoller(ctx, s.cfg.Ingest.Equity.Interval, "long_short", s.pollLongShort)
	go runPoller(ctx, time.Minute, "liquidations", s.pollLiquidations)
}

func (s *Service) pollOTC(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	price, err := otc.FetchUSDTCNY(httpClient)
	if err != nil {
		s.ingestStatus.set("otc", "error")
		return err
	}
	rates := s.currentRates()
	rates.USDTCNY = price
	rates.UpdatedAt = time.Now().UTC()
	s.store.UpdateRates(rates)
	s.ingestStatus.set("otc", "ok")
	return nil
}

func (s *Service) pollForex(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	price, err := forex.FetchUSDCNY(httpClient)
	if err != nil {
		s.ingestStatus.set("forex", "error")
		return err
	}
	rates := s.currentRates()
	rates.USDCNY = price
	rates.UpdatedAt = time.Now().UTC()
	s.store.UpdateRates(rates)
	s.ingestStatus.set("forex", "ok")
	return nil
}

func (s *Service) pollEquity(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	defs := equity.ResolveDefs(s.cfg.Ingest.Equity.IndexIDs)
	now := time.Now()
	ttl := equity.CacheTTL(now)
	if rows, ok := s.equityCache.fresh(defs, now, ttl); ok {
		s.store.SetIndices(s.indicesWithSGE(rows))
		s.ingestStatus.set("equity", "ok")
		return nil
	}

	expired := s.equityCache.expiredDefs(defs, now, ttl)
	if len(expired) == 0 {
		rows := s.equityCache.snapshot(defs, false)
		s.store.SetIndices(s.indicesWithSGE(rows))
		s.ingestStatus.set("equity", "ok")
		return nil
	}

	var firstErr error
	for _, provider := range s.cfg.Ingest.Equity.Providers {
		missing := s.equityCache.expiredDefs(defs, time.Now(), ttl)
		if len(missing) == 0 {
			break
		}
		if s.equityBreaker.isOpen(provider, now) {
			s.ingestStatus.set("equity_"+provider, "circuit_open")
			slog.Info("equity provider skipped",
				"provider", provider,
				"reason", "circuit_open",
				"symbols", len(missing),
			)
			continue
		}
		start := time.Now()
		rows, err, skipped := s.fetchEquityProvider(provider, missing)
		elapsed := time.Since(start)
		if skipped {
			s.ingestStatus.set("equity_"+provider, "disabled")
			slog.Info("equity provider skipped",
				"provider", provider,
				"reason", "disabled",
				"symbols", len(missing),
			)
			continue
		}
		if len(rows) > 0 {
			s.equityCache.merge(rows, time.Now())
			s.equityBreaker.success(provider)
			s.ingestStatus.set("equity_"+provider, "ok")
			slog.Info("equity provider fetched",
				"provider", provider,
				"requested", len(missing),
				"succeeded", len(rows),
				"duration_ms", elapsed.Milliseconds(),
			)
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.equityBreaker.failure(provider, now, equity.IsRateLimitErr(err))
			s.ingestStatus.set("equity_"+provider, "error")
			slog.Warn("equity provider failed",
				"provider", provider,
				"requested", len(missing),
				"succeeded", len(rows),
				"duration_ms", elapsed.Milliseconds(),
				"rate_limited", equity.IsRateLimitErr(err),
				"err", err,
			)
		}
	}

	finalMissing := s.equityCache.expiredDefs(defs, time.Now(), ttl)
	rows := s.equityCache.snapshot(defs, len(finalMissing) > 0)
	if len(rows) == 0 {
		s.ingestStatus.set("equity", "error")
		if firstErr != nil {
			return firstErr
		}
		return nil
	}
	s.store.SetIndices(s.indicesWithSGE(rows))
	if firstErr != nil || len(rows) < len(defs) || len(finalMissing) > 0 {
		s.ingestStatus.set("equity", "degraded")
		return nil
	}
	s.ingestStatus.set("equity", "ok")
	return nil
}

func (s *Service) pollSGE(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	q, err := metals.FetchAu9999(httpClient)
	if err != nil {
		s.ingestStatus.set("sge_gold", "error")
		slog.Warn("sge gold fetch failed", "err", err)
		return nil
	}
	q.Source = "sge"
	s.sgeGoldMu.Lock()
	s.sgeGold = q
	s.sgeGoldOK = true
	s.sgeGoldMu.Unlock()
	s.ingestStatus.set("sge_gold", "ok")

	defs := equity.ResolveDefs(s.cfg.Ingest.Equity.IndexIDs)
	rows := s.equityCache.snapshot(defs, false)
	if len(rows) > 0 {
		s.store.SetIndices(s.indicesWithSGE(rows))
	}
	return nil
}

func (s *Service) fetchEquityProvider(provider string, defs []equity.IndexDef) (map[string]store.IndexQuote, error, bool) {
	switch provider {
	case "yahoo":
		rows, err := equity.FetchYahooChartQuotes(httpClient, defs)
		return rows, err, false
	case "finnhub":
		if s.cfg.Ingest.Equity.FinnhubAPIKey == "" {
			return nil, nil, true
		}
		rows, err := equity.FetchFinnhubQuotes(httpClient, defs, s.cfg.Ingest.Equity.FinnhubAPIKey)
		return rows, err, false
	case "twelvedata":
		if s.cfg.Ingest.Equity.TwelveDataAPIKey == "" {
			return nil, nil, true
		}
		rows, err := equity.FetchTwelveDataQuotes(httpClient, defs, s.cfg.Ingest.Equity.TwelveDataAPIKey)
		return rows, err, false
	case "stooq":
		rows, err := equity.FetchStooqQuotes(httpClient, defs)
		return rows, err, false
	default:
		return nil, nil, true
	}
}

func (s *Service) pollMacro(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	m, err := macro.Fetch(httpClient)
	if err != nil {
		s.ingestStatus.set("macro", "error")
		return err
	}
	s.store.SetMacro(m)
	s.ingestStatus.set("macro", "ok")
	return nil
}

func (s *Service) pollCryptoMeta(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	rows, err := crypto.FetchMarketMetadata(httpClient, s.cfg.Symbols)
	if err != nil {
		s.ingestStatus.set("crypto_meta", "error")
		return err
	}
	s.store.UpdateQuoteMetadata(rows)
	s.ingestStatus.set("crypto_meta", "ok")
	return nil
}

func (s *Service) pollLongShort(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	const symbol = "BTCUSDT"
	var firstErr error

	ratio, err := derivatives.FetchGlobalLongShort(httpClient, "BTCUSDT")
	if err != nil {
		s.ingestStatus.set("long_short", "error")
		firstErr = err
	} else {
		s.ingestStatus.set("long_short", "ok")
	}

	topRatio, err := derivatives.FetchTopLongShortPosition(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("top_long_short", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("top_long_short", "ok")
	}

	funding, err := derivatives.FetchFunding(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("funding", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("funding", "ok")
	}

	openInterest, err := derivatives.FetchOpenInterest(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("open_interest", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("open_interest", "ok")
	}

	takerBuySell, err := derivatives.FetchTakerBuySell(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("taker_buy_sell", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("taker_buy_sell", "ok")
	}

	s.store.SetDerivatives(funding, openInterest, takerBuySell, ratio, topRatio)
	return firstErr
}

func (s *Service) pollLiquidations(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.store.SetLiquidations(s.liquidations.snapshot())
	s.ingestStatus.set("liquidations", "ok")
	return nil
}

func (s *Service) currentRates() store.Rates {
	return s.store.GetSnapshot().Rates
}

// IngestStatus returns slow-feed health.
func (s *Service) IngestStatus() map[string]string {
	return s.ingestStatus.snapshot()
}
