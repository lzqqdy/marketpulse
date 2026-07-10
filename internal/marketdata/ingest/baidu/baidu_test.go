package baidu

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseBaiduPercent(t *testing.T) {
	if got := parseBaiduPercent("+1.93%"); got != 1.93 {
		t.Fatalf("got %v", got)
	}
	if got := parseBaiduPercent("-0.35%"); got != -0.35 {
		t.Fatalf("got %v", got)
	}
}

func TestParseQuotationResult(t *testing.T) {
	ref := IndexRef{ID: "sh000001", Name: "上证", MinPrice: 1000, MaxPrice: 10000}
	raw := []byte(`{"cur":{"price":"4120.84","ratio":"-0.35%","increase":"-14.55"}}`)
	row, err := ParseQuotationResult(ref, raw, time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if row.Price != 4120.84 || row.ChangePct != -0.35 || row.Source != "baidu" {
		t.Fatalf("row=%+v", row)
	}
}

func TestParseMarketData(t *testing.T) {
	keys := "timestamp,time,open,close,volume,high,low,amount"
	marketData := "1720569600,2024-07-10,4100.00,4120.84,1000,4130.00,4090.00,50000000;1720656000,2024-07-11,4120.84,4110.00,900,4125.00,4100.00,45000000"
	candles, err := parseMarketData(marketData, keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 {
		t.Fatalf("len=%d", len(candles))
	}
	if candles[0].Open != 4100 || candles[0].Close != 4120.84 || candles[1].Close != 4110 {
		t.Fatalf("candles=%+v", candles)
	}
}

func TestDecodeStockQuotationResultNewFormat(t *testing.T) {
	raw := []byte(`{"newMarketData":{"keys":["timestamp","time","open","close","high","low","volume"],"marketData":"1720569600,2024-07-10,4100.00,4120.84,4130.00,4090.00,1000"}}`)
	marketData, keys, err := decodeStockQuotationResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(keys, "timestamp") || marketData == "" {
		t.Fatalf("marketData=%q keys=%q", marketData, keys)
	}
	candles, err := parseMarketData(marketData, keys)
	if err != nil || len(candles) != 1 || candles[0].Close != 4120.84 {
		t.Fatalf("candles=%+v err=%v", candles, err)
	}
}

func TestQuoteParamsIndex(t *testing.T) {
	ref := IndexRef{ID: "sh000001", Name: "上证", Code: "000001", Market: "ab", FinanceType: "index"}
	q, err := quoteParams(ref)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("code") != "000001" || q.Get("market_type") != "ab" || q.Get("isIndex") != "1" {
		t.Fatalf("query=%v", q)
	}
}

func TestKlineGroupIndex(t *testing.T) {
	if got := klineGroup("ab", "index"); got != "quotation_index_kline" {
		t.Fatalf("ab index group=%s", got)
	}
	if got := klineGroup("hk", "index"); got != "quotation_index_kline" {
		t.Fatalf("hk index group=%s", got)
	}
	if got := klineGroup("us", "index"); got != "quotation_index_kline" {
		t.Fatalf("us index group=%s", got)
	}
}

func TestKlinePeriodMapping(t *testing.T) {
	period, ktype, err := klinePeriod("1d")
	if err != nil || period != "dayK" || ktype != "1" {
		t.Fatalf("period=%s ktype=%s err=%v", period, ktype, err)
	}
	if _, _, err := klinePeriod("15m"); err == nil {
		t.Fatal("expected 15m unsupported")
	}
}

func TestWSExtractQuote(t *testing.T) {
	ref := IndexRef{ID: "sh000001", Name: "上证", Code: "000001", Market: "ab", MinPrice: 1000, MaxPrice: 10000}
	byCode := map[string]IndexRef{"ab:000001": ref}
	raw, err := json.Marshal(map[string]any{
		"code":   "000001",
		"market": "ab",
		"price":  "4120.84",
		"ratio":  "-0.35%",
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := parseWSUpdates(raw, byCode, time.Now().UTC())
	if len(rows) != 1 || rows["sh000001"].Price != 4120.84 {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestWSExtractQuoteDataWrapper(t *testing.T) {
	ref := IndexRef{ID: "hsi", Name: "恒生", Code: "HSI", Market: "hk", MinPrice: 5000, MaxPrice: 50000}
	byCode := map[string]IndexRef{"hk:HSI": ref}
	raw := []byte(`{"queryId":"1","data":{"financeType":"index","code":"HSI","market":"hk","cur":{"price":"24175.12","ratio":"+0.60%"}}}`)
	rows := parseWSUpdates(raw, byCode, time.Now().UTC())
	if len(rows) != 1 || rows["hsi"].Price != 24175.12 || rows["hsi"].ChangePct != 0.60 {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestIndexRefValidatePrice(t *testing.T) {
	ref := IndexRef{ID: "gold", MinPrice: 500, MaxPrice: 10000}
	if err := ref.validatePrice(1200); err != nil {
		t.Fatal(err)
	}
	if err := ref.validatePrice(10); err == nil {
		t.Fatal("expected suspicious price error")
	}
}
