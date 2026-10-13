package portfolio

import (
	"context"
	"errors"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
	"testing"
	"time"
)

type snapshotRepo struct {
	portfolioRepository
	holdings  []Holding
	previous  *Snapshot
	written   []Snapshot
	readCount int
	onRead    func()
}

func (r *snapshotRepo) ListUserIDsWithHoldings(context.Context) ([]int64, error) {
	r.readCount++
	return []int64{1}, nil
}
func (r *snapshotRepo) ListHoldings(context.Context, int64) ([]Holding, error) {
	if r.onRead != nil {
		r.onRead()
	}
	return r.holdings, nil
}
func (r *snapshotRepo) GetSettings(context.Context, int64) (Settings, error) {
	return Settings{PrincipalCny: 700}, nil
}
func (r *snapshotRepo) GetDailyOnOrBefore(_ context.Context, _ int64, date string) (*Snapshot, error) {
	for _, s := range r.written {
		if s.Date == date {
			v := s
			return &v, nil
		}
	}
	if r.previous != nil && r.previous.Date <= date {
		return r.previous, nil
	}
	return nil, nil
}
func (r *snapshotRepo) GetLatestDaily(context.Context, int64) (*Snapshot, error) {
	return r.previous, nil
}
func (r *snapshotRepo) InsertSnapshotSkip(_ context.Context, s Snapshot) (bool, error) {
	r.written = append(r.written, s)
	return true, nil
}

type snapshotMarket struct {
	marketdata.MarketDataService
	snap  marketdata.Snapshot
	reads int
}

func (m *snapshotMarket) Snapshot() marketdata.Snapshot { m.reads++; return m.snap }

func snapshotFixture() (*service, *snapshotRepo, *snapshotMarket, time.Time) {
	tz := time.FixedZone("CST", 8*3600)
	at := time.Date(2026, 10, 13, 23, 59, 10, 0, tz)
	r := &snapshotRepo{holdings: []Holding{{AssetType: AssetTypeCrypto, Symbol: "BTC", Quantity: 2, UpdatedAt: at.Add(-time.Hour)}}}
	m := &snapshotMarket{snap: marketdata.Snapshot{Quotes: []marketdata.Quote{{Symbol: "BTC", PriceUsdt: 100, UpdatedAt: at}}, Rates: store.Rates{USDTCNY: 7, USDTCNYUpdatedAt: at}}}
	s := &service{cfg: config.PortfolioConfig{Enabled: true, DefaultUsdtCny: 7}, repo: r, md: m, tz: tz, now: func() time.Time { return at }}
	return s, r, m, at
}

func TestSnapshotRejectsLateOrHistoricalValuation(t *testing.T) {
	for _, tc := range []struct {
		hour, minute int
		date         string
	}{{15, 0, ""}, {0, 5, "2026-10-12"}, {23, 59, "2026-10-12"}, {23, 58, ""}} {
		s, r, _, at := snapshotFixture()
		s.now = func() time.Time { return time.Date(at.Year(), at.Month(), at.Day(), tc.hour, tc.minute, 0, 0, s.tz) }
		if _, err := s.RunDailySnapshot(context.Background(), tc.date); !errors.Is(err, ErrSnapshotWindow) {
			t.Fatalf("case=%+v err=%v", tc, err)
		}
		if r.readCount != 0 || len(r.written) != 0 {
			t.Fatal("late task read or wrote historical portfolio")
		}
	}
}

func TestFirstSnapshotAndMissingDayEstablishBaseline(t *testing.T) {
	for _, previous := range []*Snapshot{nil, {Date: "2026-10-11", TotalValueCny: 1000}} {
		s, r, _, at := snapshotFixture()
		r.previous = previous
		n, err := s.RunDailySnapshot(context.Background(), "")
		if err != nil || n != 1 {
			t.Fatalf("n=%d err=%v", n, err)
		}
		v := r.written[0]
		if v.Date != "2026-10-13" || v.ValuedAt == nil || !v.ValuedAt.Equal(at) || v.TotalValueCny != 1400 || v.DailyProfit != 0 || v.DailyProfitRate != 0 || v.HasDailyBaseline == nil || *v.HasDailyBaseline {
			t.Fatalf("bad baseline %+v", v)
		}
		if v.TotalProfit != 700 {
			t.Fatalf("cumulative profit %v", v.TotalProfit)
		}
		if n, err := s.RunDailySnapshot(context.Background(), ""); err != nil || n != 0 || len(r.written) != 1 {
			t.Fatalf("completed snapshot overwritten n=%d err=%v", n, err)
		}
	}
}

func TestSnapshotUsesCapturedMarketAndYesterdayBaseline(t *testing.T) {
	s, r, m, _ := snapshotFixture()
	r.previous = &Snapshot{Date: "2026-10-12", TotalValueCny: 1000}
	r.onRead = func() {
		m.snap.Quotes = []marketdata.Quote{{Symbol: "BTC", PriceUsdt: 999, UpdatedAt: s.currentTime()}}
	}
	if _, err := s.RunDailySnapshot(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	v := r.written[0]
	if v.TotalValueCny != 1400 || v.DailyProfit != 400 || v.DailyProfitRate != 0.4 || v.HasDailyBaseline == nil || !*v.HasDailyBaseline || m.reads != 1 {
		t.Fatalf("mixed capture: %+v reads=%d", v, m.reads)
	}
}

func TestSnapshotCannotCrossMidnightOrUseChangedHoldings(t *testing.T) {
	s, r, _, at := snapshotFixture()
	r.onRead = func() { s.now = func() time.Time { return at.Add(time.Minute) } }
	if _, err := s.RunDailySnapshot(context.Background(), ""); err == nil || len(r.written) > 0 {
		t.Fatal("cross-midnight snapshot accepted")
	}
	s, r, _, at = snapshotFixture()
	r.holdings[0].UpdatedAt = at.Add(time.Second)
	if _, err := s.RunDailySnapshot(context.Background(), ""); err == nil || len(r.written) > 0 {
		t.Fatal("holdings changed after capture accepted")
	}
}

func TestSnapshotRejectsStaleQuote(t *testing.T) {
	s, r, m, at := snapshotFixture()
	m.snap.Quotes[0].UpdatedAt = at.Add(-time.Hour)
	if _, err := s.RunDailySnapshot(context.Background(), ""); err == nil || len(r.written) > 0 {
		t.Fatal("stale quote snapshotted")
	}
}

func TestTodayOverviewDoesNotUseAnOlderDayAsYesterday(t *testing.T) {
	for _, date := range []string{"2026-10-11", "2026-10-12"} {
		s, r, _, _ := snapshotFixture()
		r.previous = &Snapshot{Date: date, TotalValueCny: 1000}
		s.resolver = stubResolver{crypto: map[string]PricePoint{"BTC": {PriceUsdt: 100, OK: true}}, usdt: 7, ok: true}
		v, err := s.Overview(context.Background(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if date == "2026-10-11" && v.Today != nil {
			t.Fatal("two-day change shown as today profit")
		}
		if date == "2026-10-12" && (v.Today == nil || v.Today.PnlCny != 400) {
			t.Fatalf("yesterday baseline lost: %+v", v.Today)
		}
	}
}

func TestDailyJobStartsInsideWindowWithoutWaitingPastMidnight(t *testing.T) {
	s, r, _, _ := snapshotFixture()
	ctx, cancel := context.WithCancel(context.Background())
	r.onRead = cancel
	// A cancelled write should stop the job, even when it starts within the window.
	s.loopDailyJob(ctx)
	if r.readCount != 1 {
		t.Fatal("startup did not attempt current-date sample")
	}
	if len(r.written) != 0 {
		t.Fatal("cancelled capture persisted")
	}
}
