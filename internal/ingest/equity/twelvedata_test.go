package equity

import (
	"testing"
	"time"
)

func TestParseTwelveQuotesBatch(t *testing.T) {
	def := IndexDef{ID: "gspc", Name: "标普500", TwelveSymbol: "SPX"}
	body := []byte(`{
  "SPX": {
    "symbol": "SPX",
    "close": "5200.50",
    "previous_close": "5100.00",
    "percent_change": "1.9706",
    "datetime": "2026-05-15"
  }
}`)
	rows, err := parseTwelveQuotes(body, map[string]IndexDef{"SPX": def}, time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	row := rows["gspc"]
	if row.Price != 5200.50 || row.Source != "twelvedata" {
		t.Fatalf("row=%+v", row)
	}
}
