package equity

import (
	"os"
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

func TestFetchClistPageDebug(t *testing.T) {
	if os.Getenv("LIVE_EASTMONEY") == "" {
		t.Skip("set LIVE_EASTMONEY=1 to run")
	}
	rows, total, err := fetchClistPage(internalsHTTPClient, eastmoneyAShareFS, aShareFields, 1, 200)
	t.Logf("rows=%d total=%d err=%v", len(rows), total, err)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("empty diff on page 1")
	}
}

func TestFetchAShareBreadthLive(t *testing.T) {
	if os.Getenv("LIVE_EASTMONEY") == "" {
		t.Skip("set LIVE_EASTMONEY=1 to run")
	}
	b, err := FetchAShareBreadth(nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Total == 0 {
		t.Fatalf("empty breadth: %+v", b)
	}
	t.Logf("total=%d up=%d up_pct=%.1f", b.Total, b.Up, b.UpPct)
}

func TestComputeMarketBreadth(t *testing.T) {
	rows := []AShareRow{
		{Code: "600000", Name: "浦发银行", ChangePct: 2.1, Amount: 100},
		{Code: "600001", Name: "测试A", ChangePct: -1.2, Amount: 80},
		{Code: "300001", Name: "测试创业", ChangePct: 0.01, Amount: 50},
		{Code: "688001", Name: "测试科创", ChangePct: 20.1, Amount: 60},
		{Code: "600002", Name: "ST测试", ChangePct: -5.0, Amount: 20},
	}
	b := ComputeMarketBreadth(rows, time.Date(2026, 6, 24, 7, 0, 0, 0, time.UTC), "eastmoney")
	if b.Total != 5 || b.Up != 2 || b.Down != 2 || b.Flat != 1 {
		t.Fatalf("counts: total=%d up=%d down=%d flat=%d", b.Total, b.Up, b.Down, b.Flat)
	}
	if b.LimitUp != 1 || b.LimitDown != 1 {
		t.Fatalf("limits: up=%d down=%d", b.LimitUp, b.LimitDown)
	}
	if b.UpPct <= 0 || b.AdvanceDeclineRatio <= 0 {
		t.Fatalf("ratios: upPct=%v adr=%v", b.UpPct, b.AdvanceDeclineRatio)
	}
}

func TestBuildMarketWindStrongBreadth(t *testing.T) {
	breadth := store.MarketBreadth{UpPct: 70, DownPct: 20}
	industry := []store.SectorQuote{{Name: "银行", ChangePct: 1.2}, {Name: "煤炭", ChangePct: 0.8}}
	concept := []store.SectorQuote{{Name: "AI", ChangePct: 4.5}, {Name: "芯片", ChangePct: 3.8}}
	wind := BuildMarketWind(breadth, industry, concept, 0.5, time.Now().UTC())
	if wind.Summary == "" {
		t.Fatal("expected summary")
	}
	if len(wind.Tags) == 0 {
		t.Fatalf("expected tags, got %+v", wind)
	}
}

func TestBuildMarketWindIndexDivergence(t *testing.T) {
	breadth := store.MarketBreadth{UpPct: 30, DownPct: 55}
	wind := BuildMarketWind(breadth, nil, nil, 1.2, time.Now().UTC())
	if wind.Summary == "" {
		t.Fatal("expected summary")
	}
}
