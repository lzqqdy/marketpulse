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
	go runDynamicPoller(ctx, "equity", s.nextEquityPollInterval, s.pollEquity)
	go runDynamicPoller(ctx, "sge_gold", s.nextGoldPollInterval, s.pollSGE)
	go runPoller(ctx, s.cfg.Ingest.Macro.Interval, "macro", s.pollMacro)
	go runPoller(ctx, s.cfg.Ingest.Macro.Interval, "crypto_meta", s.pollCryptoMeta)
	go runPoller(ctx, s.cfg.Ingest.Equity.Interval, "long_short", s.pollLongShort)
	go runPoller(ctx, time.Minute, "liquidations", s.pollLiquidations)
}

func (s *Service) nextEquityPollInterval() time.Duration {
	return equity.NextPollInterval(equity.ResolveDefs(s.cfg.Ingest.Equity.IndexIDs), time.Now())
}

func (s *Service) nextGoldPollInterval() time.Duration {
	if equity.IsMarketActive("gold", time.Now()) {
		return equity.ActiveTTL
	}
	return equity.InactiveTTL
}

func (s *Service) pollOTC(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	start := time.Now()
	price, err := otc.FetchUSDTCNY(httpClient)
	if err != nil {
		s.ingestStatus.set("otc", "error")
		s.providerHealth.ReportFailure("okx_c2c", err)
		return err
	}
	rates := s.currentRates()
	rates.USDTCNY = price
	rates.UpdatedAt = time.Now().UTC()
	s.store.UpdateRates(rates)
	s.ingestStatus.set("otc", "ok")
	s.providerHealth.ReportSuccess("okx_c2c", time.Since(start))
	s.providerHealth.ReportUsed("okx_c2c", true)
	return nil
}

func (s *Service) pollForex(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	start := time.Now()
	price, err := forex.FetchUSDCNY(httpClient)
	if err != nil {
		s.ingestStatus.set("forex", "error")
		s.providerHealth.ReportFailure("frankfurter_fx", err)
		return err
	}
	rates := s.currentRates()
	rates.USDCNY = price
	rates.UpdatedAt = time.Now().UTC()
	s.store.UpdateRates(rates)
	s.ingestStatus.set("forex", "ok")
	s.providerHealth.ReportSuccess("frankfurter_fx", time.Since(start))
	s.providerHealth.ReportUsed("frankfurter_fx", true)
	return nil
}

func (s *Service) pollEquity(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	defs := equity.ResolveDefs(s.cfg.Ingest.Equity.IndexIDs)
	now := time.Now()
	if rows, ok := s.equityCache.fresh(defs, now, equity.CacheTTL); ok {
		s.store.SetIndices(s.indicesWithSGE(rows))
		s.ingestStatus.set("equity", "ok")
		return nil
	}

	expired := s.equityCache.expiredDefs(defs, now, equity.CacheTTL)
	if len(expired) == 0 {
		rows := s.equityCache.snapshot(defs, false)
		s.store.SetIndices(s.indicesWithSGE(rows))
		s.ingestStatus.set("equity", "ok")
		return nil
	}

	var firstErr error
	for _, provider := range s.cfg.Ingest.Equity.Providers {
		missing := s.equityCache.expiredDefs(defs, time.Now(), equity.CacheTTL)
		if len(missing) == 0 {
			break
		}
		if s.equityBreaker.isOpen(provider, now) {
			s.ingestStatus.set("equity_"+provider, "circuit_open")
			s.providerHealth.ReportCircuitOpen(equityProviderHealthName(provider))
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
			s.providerHealth.ReportDisabled(equityProviderHealthName(provider))
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
			s.providerHealth.ReportSuccess(equityProviderHealthName(provider), elapsed)
			s.providerHealth.ReportUsed(equityProviderHealthName(provider), true)
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
			s.providerHealth.ReportFailure(equityProviderHealthName(provider), err)
			if s.equityBreaker.isOpen(provider, time.Now()) {
				s.providerHealth.ReportCircuitOpen(equityProviderHealthName(provider))
			}
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

	finalMissing := s.equityCache.expiredDefs(defs, time.Now(), equity.CacheTTL)
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
	start := time.Now()
	q, err := metals.FetchAu9999(httpClient)
	if err != nil {
		s.ingestStatus.set("sge_gold", "error")
		s.providerHealth.ReportFailure("sge_gold", err)
		slog.Warn("sge gold fetch failed", "err", err)
		return nil
	}
	q.Source = "sge"
	s.sgeGoldMu.Lock()
	s.sgeGold = q
	s.sgeGoldOK = true
	s.sgeGoldMu.Unlock()
	s.ingestStatus.set("sge_gold", "ok")
	s.providerHealth.ReportSuccess("sge_gold", time.Since(start))
	s.providerHealth.ReportUsed("sge_gold", true)

	defs := equity.ResolveDefs(s.cfg.Ingest.Equity.IndexIDs)
	rows := s.equityCache.snapshot(defs, false)
	if len(rows) > 0 {
		s.store.SetIndices(s.indicesWithSGE(rows))
	}
	return nil
}

func (s *Service) fetchEquityProvider(provider string, defs []equity.IndexDef) (map[string]store.IndexQuote, error, bool) {
	switch provider {
	case "sina":
		rows, err := equity.FetchSinaQuotes(httpClient, defs)
		return rows, err, false
	case "eastmoney":
		rows, err := equity.FetchEastmoneyQuotes(httpClient, defs)
		return rows, err, false
	case "tencent":
		rows, err := equity.FetchTencentQuotes(httpClient, defs)
		return rows, err, false
	default:
		return nil, nil, true
	}
}

func (s *Service) pollMacro(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	start := time.Now()
	m, err := macro.Fetch(httpClient)
	if err != nil {
		s.ingestStatus.set("macro", "error")
		s.providerHealth.ReportFailure("coingecko_macro", err)
		return err
	}
	s.store.SetMacro(m)
	s.ingestStatus.set("macro", "ok")
	s.providerHealth.ReportSuccess("coingecko_macro", time.Since(start))
	s.providerHealth.ReportUsed("coingecko_macro", true)
	return nil
}

func (s *Service) pollCryptoMeta(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	start := time.Now()
	rows, err := crypto.FetchMarketMetadata(httpClient, s.cfg.Symbols)
	if err != nil {
		s.ingestStatus.set("crypto_meta", "error")
		s.providerHealth.ReportFailure("coingecko_meta", err)
		return err
	}
	s.store.UpdateQuoteMetadata(rows)
	s.ingestStatus.set("crypto_meta", "ok")
	s.providerHealth.ReportSuccess("coingecko_meta", time.Since(start))
	s.providerHealth.ReportUsed("coingecko_meta", true)
	return nil
}

func (s *Service) pollLongShort(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	const symbol = "BTCUSDT"
	var firstErr error
	start := time.Now()
	derivativesOK := 0

	ratio, err := derivatives.FetchGlobalLongShort(httpClient, "BTCUSDT")
	if err != nil {
		s.ingestStatus.set("long_short", "error")
		firstErr = err
	} else {
		s.ingestStatus.set("long_short", "ok")
		derivativesOK++
	}

	topRatio, err := derivatives.FetchTopLongShortPosition(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("top_long_short", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("top_long_short", "ok")
		derivativesOK++
	}

	funding, err := derivatives.FetchFunding(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("funding", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("funding", "ok")
		derivativesOK++
	}

	openInterest, err := derivatives.FetchOpenInterest(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("open_interest", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("open_interest", "ok")
		derivativesOK++
	}

	takerBuySell, err := derivatives.FetchTakerBuySell(httpClient, symbol)
	if err != nil {
		s.ingestStatus.set("taker_buy_sell", "error")
		if firstErr == nil {
			firstErr = err
		}
	} else {
		s.ingestStatus.set("taker_buy_sell", "ok")
		derivativesOK++
	}

	s.store.SetDerivatives(funding, openInterest, takerBuySell, ratio, topRatio)
	if derivativesOK > 0 {
		s.providerHealth.ReportSuccess("binance_derivatives", time.Since(start))
		s.providerHealth.ReportUsed("binance_derivatives", true)
	} else {
		s.providerHealth.ReportFailure("binance_derivatives", firstErr)
	}
	return firstErr
}

func (s *Service) pollLiquidations(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.store.SetLiquidations(s.liquidations.snapshot())
	s.ingestStatus.set("liquidations", "ok")
	s.providerHealth.ReportSuccess("binance_liquidations", 0)
	s.providerHealth.ReportUsed("binance_liquidations", true)
	return nil
}

func (s *Service) currentRates() store.Rates {
	return s.store.GetSnapshot().Rates
}

// IngestStatus returns slow-feed health.
func (s *Service) IngestStatus() map[string]string {
	return s.ingestStatus.snapshot()
}

func (s *Service) ProviderStatus() ProviderStatusResponse {
	return s.providerHealth.Snapshot(time.Now())
}

func equityProviderHealthName(provider string) string {
	switch provider {
	case "sina":
		return "sina_index"
	case "eastmoney":
		return "eastmoney_index"
	case "tencent":
		return "tencent_index"
	default:
		return "equity_" + provider
	}
}
