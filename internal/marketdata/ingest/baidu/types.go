package baidu

import "encoding/json"

const (
	DefaultBaseURL = "https://finance.pae.baidu.com"
	DefaultCDNBase = "https://sp0.baidu.com/5LMDcjW6BwF3otqbppnN2DJv/finance.pae.baidu.com"
	DefaultWSURL   = "wss://finance-ws.pae.baidu.com"
)

// Config holds Baidu Finance ingest settings.
type Config struct {
	Enabled               bool
	BaseURL               string
	WSURL                 string
	WSEnabled             bool
	WSReconnectMax        int
	WSReconnectDelay      int // seconds
	WSPatchInterval       int // seconds
}

// APIResponse is the common Baidu Finance JSON envelope.
type APIResponse struct {
	QueryID    string          `json:"QueryID"`
	ResultCode json.RawMessage `json:"ResultCode"`
	ResultMsg  string          `json:"ResultMsg"`
	Result     json.RawMessage `json:"Result"`
}

func (r APIResponse) OK() bool {
	if len(r.ResultCode) == 0 {
		return len(r.Result) > 0
	}
	var code int
	if err := json.Unmarshal(r.ResultCode, &code); err == nil {
		return code == 0
	}
	var codeStr string
	if err := json.Unmarshal(r.ResultCode, &codeStr); err == nil {
		return codeStr == "0" || codeStr == ""
	}
	return false
}

type quotationResult struct {
	Cur struct {
		Price    string `json:"price"`
		Ratio    string `json:"ratio"`
		Increase string `json:"increase"`
		Volume   string `json:"volume"`
		Amount   string `json:"amount"`
	} `json:"cur"`
	UseWS string `json:"useWS"`
}

type stockQuotationResult struct {
	MarketData    string             `json:"marketData"`
	Keys          string             `json:"keys"`
	NewMarketData *newMarketDataBlock `json:"newMarketData"`
}

type newMarketDataBlock struct {
	MarketData string          `json:"marketData"`
	Keys       json.RawMessage `json:"keys"`
}

type indexBannerItem struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Price  string `json:"price"`
	Ratio  string `json:"ratio"`
	Market string `json:"market"`
}

type wsSubscribeMessage struct {
	Source  string        `json:"source"`
	Method  string        `json:"method"`
	Product string        `json:"product"`
	Items   []wsSubscribe `json:"items"`
}

type wsSubscribe struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Market      string `json:"market"`
	FinanceType string `json:"financeType"`
}
