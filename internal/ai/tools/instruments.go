package tools

import (
	"fmt"
	"strings"
)

type instrument struct {
	ID      string
	Symbol  string
	Asset   string
	NewsTag string
	Aliases []string
}

// Language aliases complement the canonical IDs already held by marketdata.
var usNames = map[string][]string{
	"AAPL": {"苹果", "Apple"}, "MSFT": {"微软", "Microsoft"},
	"NVDA": {"英伟达", "Nvidia"}, "AMZN": {"亚马逊", "Amazon"},
	"GOOGL": {"谷歌", "Google", "Alphabet"}, "META": {"Meta"},
	"TSLA": {"特斯拉", "Tesla"}, "MU": {"美光", "Micron"},
	"TSM": {"台积电", "TSMC"}, "SKHY": {"SK海力士", "SK Hynix"},
	"QQQ": {"纳指ETF", "Invesco QQQ"}, "SPY": {"标普ETF", "SPDR S&P 500"},
}

func (r *Registry) resolveInstrument(symbol, asset string) (instrument, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return instrument{}, fmt.Errorf("symbol required")
	}
	asset = strings.ToLower(strings.TrimSpace(asset))
	if asset == "alpha" {
		asset = "us_contract"
	}
	if asset != "" && asset != "crypto" && asset != "index" && asset != "us_equity" && asset != "us_contract" {
		return instrument{}, fmt.Errorf("unsupported assetClass: %s", asset)
	}
	upper := strings.ToUpper(symbol)
	ticker := strings.TrimPrefix(upper, "US-")
	contractSymbol := strings.HasSuffix(ticker, "USDT")
	ticker = strings.TrimSuffix(ticker, "USDT")
	for key, names := range usNames {
		for _, name := range names {
			if strings.EqualFold(symbol, name) {
				ticker = key
			}
		}
	}
	legacySymbol := false
	if base, ok := strings.CutSuffix(ticker, "ON"); ok {
		if _, known := usNames[base]; known {
			ticker, legacySymbol = base, true
		}
	}
	_, knownUS := usNames[ticker]
	spotID := "us-" + strings.ToLower(ticker)
	if _, ok := r.md.IndexQuote(spotID); ok {
		knownUS = true
	}
	if _, ok := r.md.AlphaQuote(strings.ToLower(ticker)); ok {
		knownUS = true
	}
	if asset == "index" && (strings.HasPrefix(upper, "US-") || knownUS) {
		asset = "us_equity"
	}
	if asset == "" {
		switch {
		case knownUS || strings.HasPrefix(upper, "US-"):
			asset = "us_equity"
			if contractSymbol || legacySymbol {
				asset = "us_contract"
			}
		case mainlandIndexID(symbol):
			asset = "index"
		default:
			if _, ok := r.md.IndexQuote(strings.ToLower(symbol)); ok {
				asset = "index"
			} else {
				asset = "crypto"
			}
		}
	}
	inst := instrument{Symbol: ticker, Asset: asset}
	switch asset {
	case "us_equity":
		if contractSymbol || legacySymbol {
			return instrument{}, fmt.Errorf("contract symbol cannot be used as us_equity: %s", symbol)
		}
		inst.ID, inst.NewsTag = spotID, "美股"
		inst.Aliases = append([]string{ticker}, usNames[ticker]...)
	case "us_contract":
		inst.ID, inst.NewsTag = strings.ToLower(ticker), "美股"
		inst.Aliases = append([]string{ticker}, usNames[ticker]...)
	case "index":
		inst.ID = strings.ToLower(symbol)
		inst.NewsTag = "A股"
		switch inst.ID {
		case "hsi":
			inst.NewsTag = "港股"
		case "dji", "ixic", "gspc", "spx", "ndx":
			inst.NewsTag = "美股"
		}
		if q, ok := r.md.IndexQuote(inst.ID); ok {
			inst.Aliases = []string{symbol, q.Name}
		} else {
			inst.Aliases = []string{symbol}
		}
	default:
		inst.ID = normalizeCryptoSymbol(symbol)
		inst.Symbol, inst.NewsTag = inst.ID, "币圈"
		inst.Aliases = []string{inst.Symbol}
		switch inst.Symbol {
		case "BTC":
			inst.Aliases = append(inst.Aliases, "比特币", "Bitcoin")
		case "ETH":
			inst.Aliases = append(inst.Aliases, "以太坊", "Ethereum")
		case "SOL":
			inst.Aliases = append(inst.Aliases, "Solana")
		}
	}
	return inst, nil
}

func mainlandIndexID(symbol string) bool {
	symbol = strings.ToLower(symbol)
	if len(symbol) != 8 || !strings.HasPrefix(symbol, "sh") && !strings.HasPrefix(symbol, "sz") {
		return false
	}
	for _, c := range symbol[2:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func referenceType(source string) (kind, currency string) {
	switch strings.ToLower(source) {
	case "bitget":
		return "usdt_futures", "USDT"
	case "binance-alpha":
		return "tokenized_equity", "USDT"
	default:
		return "equity_reference", ""
	}
}

func (inst instrument) fields(source string) map[string]any {
	typeName, currency, basis := "crypto_spot", "USDT", "rolling_24h"
	id := inst.ID
	switch inst.Asset {
	case "us_equity":
		typeName, currency, basis = "equity_spot", "USD", "trading_session"
	case "us_contract":
		typeName, currency = referenceType(source)
		id = source + ":" + inst.Symbol
	case "index":
		typeName, currency, basis = "index", "", "trading_session"
	}
	return map[string]any{
		"instrumentId": id, "symbol": inst.Symbol, "assetClass": inst.Asset,
		"instrumentType": typeName, "currency": currency, "changeBasis": basis,
	}
}
