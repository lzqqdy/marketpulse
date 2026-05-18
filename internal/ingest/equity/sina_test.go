package equity

import (
	"testing"
	"time"
)

func TestParseSinaQuotes(t *testing.T) {
	defs := map[string]IndexDef{
		"s_sh000001": {ID: "sh000001", Name: "上证", SinaSymbol: "s_sh000001", MinPrice: 1000, MaxPrice: 10000},
		"int_sp500":  {ID: "gspc", Name: "标普500", SinaSymbol: "int_sp500", MinPrice: 1000, MaxPrice: 15000},
		"hf_GC":      {ID: "gold", Name: "国际黄金", SinaSymbol: "hf_GC", MinPrice: 500, MaxPrice: 10000},
		"int_kospi":  {ID: "ks11", Name: "KOSPI", SinaSymbol: "int_kospi", MinPrice: 1000, MaxPrice: 10000},
	}
	body := `var hq_str_s_sh000001="上证指数,4128.7117,-6.6777,-0.16,3729760,79957751";
var hq_str_int_sp500="标普指数,6643.70,38.98,0.59";
var hq_str_hf_GC="4536.770,,4534.200,4534.700,4559.000,4483.500,11:04:53,4561.900,4547.600,0,1,1,2026-05-18,纽约黄金,0";
var hq_str_int_kospi="";
`
	rows, err := parseSinaQuotes(body, defs, time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected partial result error")
	}
	if len(rows) != 3 {
		t.Fatalf("len=%d rows=%+v", len(rows), rows)
	}
	if rows["sh000001"].Price != 4128.7117 || rows["gspc"].ChangePct != 0.59 {
		t.Fatalf("rows=%+v", rows)
	}
	if rows["gold"].ChangePct >= 0 {
		t.Fatalf("expected gold negative change, got %+v", rows["gold"])
	}
}
