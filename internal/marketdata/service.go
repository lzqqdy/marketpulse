// Package marketdata owns market data collection, read models, and streams.
package marketdata

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/alpha"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/bitget"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
	"github.com/lzqqdy/marketpulse/internal/marketdata/stream"
)

var (
	ErrInvalidSymbol = errors.New("invalid symbol")
	ErrInvalidIndex  = errors.New("invalid index")
)

// ServeWSUpgrader is the market data WebSocket upgrader used by API adapters.
var ServeWSUpgrader = stream.ServeWSUpgrader

type ProviderStatusResponse = ingest.ProviderStatusResponse
type Snapshot = store.Snapshot
type Quote = store.Quote

// KlineResponse is returned by market kline APIs.
type KlineResponse struct {
	Symbol   string           `json:"symbol"`
	Pair     string           `json:"pair"`
	Interval string           `json:"interval"`
	Candles  []binance.Candle `json:"candles"`
	Source   string           `json:"source"`
}

// MarketDataService is the public boundary consumed by API and future modules.
type MarketDataService interface {
	Start(ctx context.Context)
	Snapshot() Snapshot
	Quote(symbol string) (Quote, bool)
	Version() uint64
	SymbolCount() int
	ProviderStatus() ProviderStatusResponse
	IngestStatus() map[string]string
	StreamClientCount() int
	ServeStreamWS(conn *websocket.Conn, channels string)
	ServeKlineWS(conn *websocket.Conn, symbol string, interval string)
	Klines(symbol string, interval string, limit int) (KlineResponse, error)
	IndexKlines(id string, interval string, limit int) (KlineResponse, error)
}

// Service wires the market data read model, ingestion, and streaming layers.
type Service struct {
	cfg       *config.Config
	store     *store.MarketStore
	streamHub *stream.StreamHub
	klineHub  *stream.KlineHub
	ingest    *ingest.Service
}

// New creates a complete market data service.
func New(cfg *config.Config) *Service {
	st := store.New(cfg.Symbols...)
	return NewWithStore(cfg, st)
}

// NewWithStore creates a service with an injected store for tests.
func NewWithStore(cfg *config.Config, st *store.MarketStore) *Service {
	streamHub := stream.NewStreamHub(st)
	klineHub := stream.NewKlineHub(cfg)
	ingestSvc := ingest.New(cfg, st)
	return &Service{
		cfg:       cfg,
		store:     st,
		streamHub: streamHub,
		klineHub:  klineHub,
		ingest:    ingestSvc,
	}
}

func (s *Service) Start(ctx context.Context) {
	s.ingest.Start(ctx)
}

func (s *Service) Snapshot() Snapshot {
	return s.store.GetSnapshot()
}

func (s *Service) Quote(symbol string) (Quote, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return Quote{}, false
	}
	for _, q := range s.Snapshot().Quotes {
		if q.Symbol == symbol {
			return q, true
		}
	}
	return Quote{}, false
}

func (s *Service) Version() uint64 {
	return s.store.Version()
}

func (s *Service) SymbolCount() int {
	return len(s.cfg.Symbols)
}

func (s *Service) ProviderStatus() ProviderStatusResponse {
	return s.ingest.ProviderStatus()
}

func (s *Service) IngestStatus() map[string]string {
	out := map[string]string{
		"binance_ws":     s.ingest.BinanceStatus(),
		"alpha_poll":     s.ingest.AlphaStatus(),
		"alpha_ws":       s.ingest.AlphaStatus(),
		"last_quote_ms":  formatLastQuote(s.ingest.LastQuoteMs()),
		"last_alpha_ms":  formatLastQuote(s.ingest.LastAlphaMs()),
		"stream_clients": strconv.Itoa(s.StreamClientCount()),
	}
	for k, v := range s.ingest.IngestStatus() {
		out[k] = v
	}
	return out
}

func (s *Service) StreamClientCount() int {
	return s.streamHub.ClientCount()
}

func (s *Service) ServeStreamWS(conn *websocket.Conn, channels string) {
	s.streamHub.ServeWS(conn, channels)
}

func (s *Service) ServeKlineWS(conn *websocket.Conn, symbol string, interval string) {
	s.klineHub.ServeWS(conn, symbol, interval)
}

func (s *Service) Klines(symbol string, interval string, limit int) (KlineResponse, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" || !s.symbolAllowed(symbol) {
		return KlineResponse{}, ErrInvalidSymbol
	}
	if interval == "" {
		interval = "1h"
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}

	pair := binance.SymbolUSDT(symbol)
	source := "binance"
	var candles []binance.Candle
	var err error
	if s.cfg.Alpha.Enabled && s.cfg.IsAlphaBaseSymbol(symbol) {
		source = s.alphaSource()
		if alphaSymbol, ok := s.ingest.AlphaSymbolForBase(symbol); ok {
			pair = alphaSymbol
		}
		if pair == binance.SymbolUSDT(symbol) && s.cfg.Alpha.Provider != "bitget" {
			if alphaSymbol, ok := resolveAlphaPair(s.cfg, symbol); ok {
				pair = alphaSymbol
			}
		}
		if s.cfg.Alpha.Provider == "bitget" {
			candles, err = bitget.FetchKlines(http.DefaultClient, pair, s.cfg.Alpha.ProductType, interval, limit)
			if err != nil {
				if fallbackPair, ok := resolveAlphaPair(s.cfg, symbol); ok {
					if fallbackCandles, fallbackErr := alpha.FetchKlines(http.DefaultClient, fallbackPair, interval, limit); fallbackErr == nil {
						pair = fallbackPair
						source = "binance-alpha"
						candles = fallbackCandles
						err = nil
					}
				}
			}
		} else {
			candles, err = alpha.FetchKlines(http.DefaultClient, pair, interval, limit)
		}
	} else {
		candles, err = binance.FetchKlines(symbol, interval, limit)
	}
	if err != nil {
		return KlineResponse{}, err
	}
	return KlineResponse{
		Symbol:   symbol,
		Pair:     pair,
		Interval: interval,
		Candles:  candles,
		Source:   source,
	}, nil
}

func (s *Service) alphaSource() string {
	if s.cfg.Alpha.Provider == "bitget" {
		return "bitget"
	}
	return "binance-alpha"
}

func (s *Service) IndexKlines(id string, interval string, limit int) (KlineResponse, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return KlineResponse{}, ErrInvalidIndex
	}
	def, ok := equity.DefaultIndexByID(id)
	if !ok {
		return KlineResponse{}, ErrInvalidIndex
	}
	if interval == "" {
		interval = "1d"
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}

	candles, source, err := equity.FetchCachedEastmoneyKlines(http.DefaultClient, def, interval, limit)
	if err != nil {
		return KlineResponse{}, err
	}
	return KlineResponse{
		Symbol:   def.ID,
		Pair:     def.EastmoneySecID,
		Interval: interval,
		Candles:  candles,
		Source:   source,
	}, nil
}

func (s *Service) symbolAllowed(symbol string) bool {
	for _, item := range s.cfg.Symbols {
		if item == symbol {
			return true
		}
	}
	return s.cfg.Alpha.Enabled && s.cfg.IsAlphaBaseSymbol(symbol)
}

func resolveAlphaPair(cfg *config.Config, symbol string) (string, bool) {
	resolved := alpha.ResolveItems(http.DefaultClient, cfg.Alpha.Indices, cfg.Alpha.Stocks, cfg.Alpha.QuoteAsset)
	for _, item := range resolved {
		if item.BaseSymbol == symbol && item.AlphaSymbol != "" {
			return item.AlphaSymbol, true
		}
	}
	return "", false
}

func formatLastQuote(ms int64) string {
	if ms == 0 {
		return "never"
	}
	return time.UnixMilli(ms).Format(time.RFC3339)
}
