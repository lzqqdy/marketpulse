package equity

import (
	"encoding/json"
	"testing"
)

func Test_yahooChart_latestChange(t *testing.T) {
	raw := `{
	  "chart": {
	    "result": [{
	      "meta": {
	        "regularMarketPrice": 3100.5,
	        "previousClose": 3120.0
	      },
	      "indicators": {
	        "quote": [{ "close": [3110.0, 3100.5] }]
	      }
	    }]
	  }
	}`
	var y yahooChart
	if err := json.Unmarshal([]byte(raw), &y); err != nil {
		t.Fatal(err)
	}
	price, chg, err := y.latestChange()
	if err != nil {
		t.Fatal(err)
	}
	if price != 3100.5 {
		t.Fatalf("price %v", price)
	}
	if chg >= 0 {
		t.Fatalf("expected negative change, got %v", chg)
	}
}

func Test_yahooKlineChart_candles(t *testing.T) {
	raw := `{
	  "chart": {
	    "result": [{
	      "timestamp": [1710000000, 1710086400, 1710172800],
	      "indicators": {
	        "quote": [{
	          "open": [100.0, null, 103.0],
	          "high": [105.0, null, 108.0],
	          "low": [99.0, null, 102.0],
	          "close": [104.0, null, 107.0],
	          "volume": [1000, null, 1200]
	        }]
	      }
	    }]
	  }
	}`
	var y yahooKlineChart
	if err := json.Unmarshal([]byte(raw), &y); err != nil {
		t.Fatal(err)
	}
	candles, err := y.candles()
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 {
		t.Fatalf("candles len %d", len(candles))
	}
	if candles[0].Open != 100 || candles[1].Close != 107 {
		t.Fatalf("candles = %+v", candles)
	}
}

func TestResolveDefs_defaultWatchlist(t *testing.T) {
	defs := ResolveDefs([]string{"sh000001", "sz399001", "hsi", "n225", "ks11", "dji", "ixic", "gspc", "gold"})
	if len(defs) != 9 {
		t.Fatalf("len %d", len(defs))
	}
}

func Test_normalizeKlineInterval(t *testing.T) {
	interval, queryRange, err := normalizeKlineInterval("1w")
	if err != nil {
		t.Fatal(err)
	}
	if interval != "1wk" || queryRange != "5y" {
		t.Fatalf("got %s %s", interval, queryRange)
	}
}
