package marketcenter

import (
	"fmt"
	"strings"
)

const (
	MarketAB = "ab"
	MarketHK = "hk"
	MarketUS = "us"
)

// NormalizeMarket validates and normalizes market code.
func NormalizeMarket(raw string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(raw))
	switch m {
	case MarketAB, MarketHK, MarketUS:
		return m, nil
	default:
		return "", fmt.Errorf("unsupported market: %s", raw)
	}
}

// HeatmapTypeCode returns Baidu typeCode for heatmap API.
func HeatmapTypeCode(market string) string {
	if market == MarketHK {
		return "HSHY"
	}
	return "HY"
}

// OverviewTabName maps block type to display label.
func OverviewTabName(market, blockType string) string {
	switch blockType {
	case "HY", "HSHY":
		return "行业板块"
	case "GN":
		return "概念板块"
	case "DY":
		return "地域板块"
	default:
		return blockType
	}
}

// FundflowBlockTypes lists available fundflow groups per market.
func FundflowBlockTypes(market string) []string {
	if market == MarketAB {
		return []string{"HY", "GN", "DY"}
	}
	return []string{"HY", "HSHY"}
}
