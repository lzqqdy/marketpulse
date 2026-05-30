package equity

import "strings"

// IndexDef maps internal id to upstream symbols and display name.
type IndexDef struct {
	ID             string
	Name           string
	EastmoneySecID string
	TencentSymbol  string
	MinPrice       float64
	MaxPrice       float64
}

// DefaultIndices is the global index watchlist plus international gold.
var DefaultIndices = []IndexDef{
	{ID: "sh000001", Name: "上证", EastmoneySecID: "1.000001", TencentSymbol: "s_sh000001", MinPrice: 1000, MaxPrice: 10000},
	{ID: "sz399001", Name: "深证", EastmoneySecID: "0.399001", TencentSymbol: "s_sz399001", MinPrice: 5000, MaxPrice: 30000},
	{ID: "sz399006", Name: "创业板", EastmoneySecID: "0.399006", TencentSymbol: "s_sz399006", MinPrice: 500, MaxPrice: 10000},
	{ID: "sh000300", Name: "沪深300", EastmoneySecID: "1.000300", TencentSymbol: "s_sh000300", MinPrice: 1000, MaxPrice: 10000},
	{ID: "sh000688", Name: "科创50", EastmoneySecID: "1.000688", TencentSymbol: "sh000688", MinPrice: 100, MaxPrice: 10000},
	{ID: "hsi", Name: "恒生", EastmoneySecID: "100.HSI", TencentSymbol: "s_hkHSI", MinPrice: 5000, MaxPrice: 50000},
	{ID: "dji", Name: "道琼斯", EastmoneySecID: "100.DJIA", TencentSymbol: "s_usDJI", MinPrice: 10000, MaxPrice: 100000},
	{ID: "ixic", Name: "纳斯达克", EastmoneySecID: "100.NDX", TencentSymbol: "s_usIXIC", MinPrice: 5000, MaxPrice: 50000},
	{ID: "gspc", Name: "标普500", EastmoneySecID: "100.SPX", TencentSymbol: "s_usINX", MinPrice: 1000, MaxPrice: 15000},
	{ID: "n225", Name: "日经225", EastmoneySecID: "100.N225", TencentSymbol: "gzN225", MinPrice: 10000, MaxPrice: 100000},
	{ID: "ks11", Name: "KOSPI", EastmoneySecID: "100.KS11", TencentSymbol: "gzKS11", MinPrice: 1000, MaxPrice: 10000},
	{ID: "gold", Name: "国际黄金", EastmoneySecID: "101.GC00Y", TencentSymbol: "hf_GC", MinPrice: 500, MaxPrice: 10000},
	{ID: "silver", Name: "国际白银", EastmoneySecID: "101.SI00Y", TencentSymbol: "hf_SI", MinPrice: 5, MaxPrice: 300},
	{ID: "crude", Name: "WTI原油", EastmoneySecID: "102.CL00Y", TencentSymbol: "hf_CL", MinPrice: 10, MaxPrice: 300},
}

// DefaultIndexByID finds a configured index definition by internal id.
func DefaultIndexByID(id string) (IndexDef, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, def := range DefaultIndices {
		if strings.ToLower(def.ID) == id {
			return def, true
		}
	}
	return IndexDef{}, false
}

// ResolveDefs maps configured index ids to index definitions.
func ResolveDefs(ids []string) []IndexDef {
	if len(ids) == 0 {
		return DefaultIndices
	}
	out := make([]IndexDef, 0, len(ids))
	for _, id := range ids {
		if def, ok := DefaultIndexByID(id); ok {
			out = append(out, def)
		}
	}
	return out
}

func IsRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") || strings.Contains(msg, "503")
}
