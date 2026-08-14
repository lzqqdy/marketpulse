package marketdata

import "testing"

func TestAlphaIDAliases(t *testing.T) {
	got := AlphaIDAliases("QQQon")
	if len(got) != 2 || got[0] != "qqqon" || got[1] != "qqq" {
		t.Fatalf("got %#v", got)
	}
	got = AlphaIDAliases("qqq")
	if len(got) != 2 || got[0] != "qqq" || got[1] != "qqqon" {
		t.Fatalf("got %#v", got)
	}
}

func TestFindAlphaQuote_aliasesAndSymbol(t *testing.T) {
	indices := []AlphaQuote{{ID: "qqq", Symbol: "QQQ", Price: 100}}
	stocks := []AlphaQuote{{ID: "aapl", Symbol: "AAPL", Price: 200}}

	if q, ok := FindAlphaQuote(indices, stocks, "qqqon"); !ok || q.Price != 100 {
		t.Fatalf("qqqon -> %#v ok=%v", q, ok)
	}
	if q, ok := FindAlphaQuote(indices, stocks, "QQQ"); !ok || q.ID != "qqq" {
		t.Fatalf("symbol QQQ -> %#v ok=%v", q, ok)
	}
	if q, ok := FindAlphaQuote(indices, stocks, "aaplon"); !ok || q.Price != 200 {
		t.Fatalf("aaplon -> %#v ok=%v", q, ok)
	}
}
