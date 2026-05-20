package equity

import (
	"testing"
	"time"
)

func TestParseTencentQuotes(t *testing.T) {
	defs := map[string]IndexDef{
		"s_sh000001": {ID: "sh000001", Name: "上证", TencentSymbol: "s_sh000001", MinPrice: 1000, MaxPrice: 10000},
		"sh000688":   {ID: "sh000688", Name: "科创50", TencentSymbol: "sh000688", MinPrice: 100, MaxPrice: 10000},
		"gzN225":     {ID: "n225", Name: "日经225", TencentSymbol: "gzN225", MinPrice: 10000, MaxPrice: 100000},
		"hf_GC":      {ID: "gold", Name: "国际黄金", TencentSymbol: "hf_GC", MinPrice: 500, MaxPrice: 10000},
	}
	body := `v_s_sh000001="1~上证指数~000001~4120.84~-14.55~-0.35~401414142~85979422~~686220.50~ZS~";
v_sh000688="1~科创50~000688~1832.02~1775.13~1764.21~18452542~0~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~~20260520161409~56.89~3.20~1835.22~1764.21~1832.02/18452542/190386612651~18452542~19038661~2.71~171.61~~1835.22~1764.21~4.00~52371.36~54712.51~0.00~-1~-1~0.99~0~1811.83~~~~~~19038661.2651~0.0000~0~ ~ZS~36.29~3.50~~~~1835.22~952.33~10.57~28.71~25.90~68062197871~~-14.62~66.35~68062197871~~~83.63~-0.08~~CNY~0~~0.00~0~";
v_gzN225="N225~日经225指数~2026-05-18 10:35:03~60843.09~-566.20~-0.92~AP~";
v_hf_GC="4539.16,-0.50,4537.00,4537.50,4559.00,4483.50,11:24:07,4561.90,4547.60,0,1,1,2026-05-18,纽约黄金";
`
	rows, err := parseTencentQuotes(body, defs, time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("len=%d rows=%+v", len(rows), rows)
	}
	if rows["sh000001"].Price != 4120.84 || rows["n225"].ChangePct != -0.92 || rows["gold"].ChangePct != -0.50 {
		t.Fatalf("rows=%+v", rows)
	}
	if rows["sh000688"].Price != 1832.02 || rows["sh000688"].ChangePct != 3.20 {
		t.Fatalf("kcb row=%+v", rows["sh000688"])
	}
	if rows["n225"].UpdatedAt.IsZero() {
		t.Fatalf("expected parsed updatedAt, got %+v", rows["n225"])
	}
	if got := rows["n225"].UpdatedAt.In(tencentQuoteLocation).Format("2006-01-02 15:04:05"); got != "2026-05-18 10:35:03" {
		t.Fatalf("n225 updatedAt = %s", got)
	}
}
