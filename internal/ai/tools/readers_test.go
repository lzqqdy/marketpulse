package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/evidence"
	"github.com/lzqqdy/marketpulse/internal/event"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/portfolio"
)

type runnerReaderMarket struct{ marketdata.MarketDataService }

type portfolioReadStub struct{ stale bool }

func (*portfolioReadStub) Enabled() bool { return true }
func (*portfolioReadStub) Overview(ctx context.Context, id int64) (portfolio.Overview, error) {
	return portfolio.Overview{TotalCny: float64(id)}, nil
}
func (p *portfolioReadStub) GetHoldings(ctx context.Context, id int64) (portfolio.HoldingsResult, error) {
	return portfolio.HoldingsResult{Holdings: []portfolio.HoldingView{{Symbol: fmt.Sprint(id), PriceUpdatedAt: time.Now(), Stale: p.stale}}}, nil
}

func TestPortfolioReaderIsRequestScopedAndCannotOverrideIdentity(t *testing.T) {
	r := NewRegistry(&runnerReaderMarket{}).WithReaders(&portfolioReadStub{}, nil)
	var wg sync.WaitGroup
	for _, id := range []int64{11, 22} {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			raw, err := r.ForUser(id).Execute(context.Background(), "get_portfolio_overview", `{}`)
			if err != nil {
				t.Error(err)
				return
			}
			var payload struct {
				Overview portfolio.Overview
				Evidence []evidence.Evidence
			}
			if json.Unmarshal([]byte(raw), &payload) != nil || payload.Overview.TotalCny != float64(id) || len(payload.Evidence) != 1 {
				t.Errorf("wrong user/evidence: %s", raw)
			}
		}(id)
	}
	wg.Wait()
	if _, err := r.ForUser(11).Execute(context.Background(), "get_portfolio_overview", `{"userId":22}`); err == nil {
		t.Fatal("identity override accepted")
	}
	raw, err := r.Execute(context.Background(), "get_portfolio_overview", `{}`)
	if ParseOutcome(raw, err).Status != evidence.Unavailable {
		t.Fatal("unbound registry could read assets")
	}
}

func TestStalePortfolioIsPartialAndCarriesValuationLimitations(t *testing.T) {
	r := NewRegistry(&runnerReaderMarket{}).WithReaders(&portfolioReadStub{stale: true}, nil).ForUser(11)
	raw, err := r.Execute(context.Background(), "get_portfolio_overview", `{}`)
	out := ParseOutcome(raw, err)
	if out.Status != evidence.Partial || len(out.Evidence) != 1 || len(out.Evidence[0].Limitations) < 2 || out.Evidence[0].Facts["holdings"] == nil {
		t.Fatalf("%s %v", raw, err)
	}
}

type eventReadStub struct {
	filter event.ListFilter
	fail   bool
}

func (*eventReadStub) Enabled() bool { return true }
func (e *eventReadStub) List(ctx context.Context, f event.ListFilter) (event.ListResult, error) {
	e.filter = f
	if e.fail {
		return event.ListResult{}, fmt.Errorf("event read failed")
	}
	return event.ListResult{Items: []event.MarketEvent{{ID: "event-1", Title: "test event", Symbols: []string{"BTC"}, UpdatedAt: time.Now(), Status: event.StatusActive}}}, nil
}

func TestEventReaderFiltersAndBoundsEvidence(t *testing.T) {
	stub := &eventReadStub{}
	r := NewRegistry(&runnerReaderMarket{}).WithReaders(nil, stub)
	raw, err := r.Execute(context.Background(), "get_market_events", `{"symbol":"BTCUSDT","hours":24,"limit":2}`)
	out := ParseOutcome(raw, err)
	if err != nil || stub.filter.Symbol != "BTC" || stub.filter.Limit != 2 || stub.filter.Start == nil || stub.filter.End == nil || len(out.Evidence) != 1 || out.Evidence[0].Facts["eventStatus"] != event.StatusActive {
		t.Fatalf("%s %+v %v", raw, stub.filter, err)
	}
	for _, args := range []string{`{"limit":11}`, `{"hours":0}`, `{"userId":2}`} {
		if _, err := r.Execute(context.Background(), "get_market_events", args); err == nil {
			t.Fatal("invalid filter accepted", args)
		}
	}
	stub.fail = true
	raw, err = r.Execute(context.Background(), "get_market_events", `{}`)
	if ParseOutcome(raw, err).Status != evidence.Unavailable {
		t.Fatal("failed event read promoted")
	}
}

func TestDisabledReadersAreUnavailable(t *testing.T) {
	r := NewRegistry(&runnerReaderMarket{}).ForUser(1)
	for _, name := range []string{"get_market_events", "get_portfolio_overview"} {
		raw, err := r.Execute(context.Background(), name, `{}`)
		if ParseOutcome(raw, err).Status != evidence.Unavailable {
			t.Fatal(name, raw, err)
		}
	}
}
