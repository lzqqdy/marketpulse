package ingest

import (
	"context"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ingest/crypto"
	"github.com/lzqqdy/marketpulse/internal/ingest/derivatives"
	"github.com/lzqqdy/marketpulse/internal/ingest/equity"
	"github.com/lzqqdy/marketpulse/internal/ingest/forex"
	"github.com/lzqqdy/marketpulse/internal/ingest/macro"
	"github.com/lzqqdy/marketpulse/internal/ingest/otc"
	"github.com/lzqqdy/marketpulse/internal/store"
)

func (s *Service) startSlowIngest(ctx context.Context) {
	go runPoller(ctx, s.cfg.Ingest.OTC.USDTCNYInterval, "otc", s.pollOTC)
	go runPoller(ctx, s.cfg.Ingest.Forex.USDCNYInterval, "forex", s.pollForex)
	go runPoller(ctx, s.cfg.Ingest.Equity.Interval, "equity", s.pollEquity)
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
	indices, err := equity.FetchAll(httpClient, defs)
	if len(indices) == 0 {
		s.ingestStatus.set("equity", "error")
		if err != nil {
			return err
		}
		return nil
	}
	s.store.SetIndices(indices)
	if err != nil {
		s.ingestStatus.set("equity", "degraded")
		return nil
	}
	s.ingestStatus.set("equity", "ok")
	return nil
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
