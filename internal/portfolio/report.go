package portfolio

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var reportRangeDays = map[string]int{
	"7d":   7,
	"30d":  30,
	"90d":  90,
	"180d": 180,
	"1y":   365,
}

// NormalizeReportRange returns canonical range key or error.
func NormalizeReportRange(raw string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		key = "30d"
	}
	if key == "all" {
		return "all", nil
	}
	if _, ok := reportRangeDays[key]; ok {
		return key, nil
	}
	return "", fmt.Errorf("%w: unsupported range %q", ErrInvalidInput, raw)
}

// ResolveReportWindow computes [from,to] inclusive dates in Asia/Shanghai.
func ResolveReportWindow(rangeKey string, now time.Time, earliest string) (from, to string) {
	loc := time.FixedZone("CST", 8*3600)
	if loaded, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		loc = loaded
	}
	now = now.In(loc)
	toTime := now
	to = toTime.Format("2006-01-02")

	if rangeKey == "all" {
		if earliest != "" {
			from = earliest
		} else {
			from = to
		}
		return from, to
	}
	days := reportRangeDays[rangeKey]
	if days <= 0 {
		days = 30
	}
	from = toTime.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	return from, to
}

func (s *service) ReportSeries(ctx context.Context, userID int64, rangeKey string) (ReportSeriesResult, error) {
	if !s.Enabled() {
		return ReportSeriesResult{}, ErrDisabled
	}
	key, err := NormalizeReportRange(rangeKey)
	if err != nil {
		return ReportSeriesResult{}, err
	}
	earliest, err := s.repo.EarliestDailyDate(ctx, userID)
	if err != nil {
		return ReportSeriesResult{}, err
	}
	from, to := ResolveReportWindow(key, time.Now().In(s.tz), earliest)
	rows, err := s.repo.ListDailySnapshotsRange(ctx, userID, from, to)
	if err != nil {
		return ReportSeriesResult{}, err
	}
	points := make([]ReportSeriesPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, ReportSeriesPoint{
			Date:            row.Date,
			TotalValue:      row.TotalValue,
			TotalValueCny:   row.TotalValueCny,
			DailyProfit:     row.DailyProfit,
			DailyProfitRate: row.DailyProfitRate,
			TotalProfit:     row.TotalProfit,
			TotalProfitRate: row.TotalProfitRate,
		})
	}
	sum := ReportSeriesSummary{}
	if len(points) > 0 {
		sum.StartCny = points[0].TotalValueCny
		sum.EndCny = points[len(points)-1].TotalValueCny
		sum.PnlCny = RoundMoney(sum.EndCny - sum.StartCny)
		if sum.StartCny != 0 {
			pct := sum.PnlCny / sum.StartCny * 100
			sum.PnlPct = &pct
		}
		// Tighten from/to to actual data span when present
		from = points[0].Date
		to = points[len(points)-1].Date
	}
	return ReportSeriesResult{
		Range:   key,
		From:    from,
		To:      to,
		Summary: sum,
		Points:  points,
	}, nil
}

func (s *service) ReportAllocation(ctx context.Context, userID int64) (AllocationResult, error) {
	if !s.Enabled() {
		return AllocationResult{}, ErrDisabled
	}
	holdings, err := s.repo.ListHoldings(ctx, userID)
	if err != nil {
		return AllocationResult{}, err
	}
	v := ValueHoldings(s.resolver, holdings, s.cfg.DefaultUsdtCny)
	items := make([]AllocationItem, 0, len(v.Holdings))
	for _, h := range v.Holdings {
		if h.Missing || h.ValueCny <= 0 {
			continue
		}
		weight := 0.0
		if v.TotalCny > 0 {
			weight = h.ValueCny / v.TotalCny * 100
		}
		items = append(items, AllocationItem{
			AssetType: h.AssetType,
			Symbol:    h.Symbol,
			ValueCny:  RoundMoney(h.ValueCny),
			ValueUsdt: h.ValueUsdt,
			WeightPct: mathRound2(weight),
		})
	}
	return AllocationResult{
		TotalCny:       RoundMoney(v.TotalCny),
		TotalUsdt:      v.TotalUsdt,
		Items:          items,
		MissingSymbols: v.Missing,
		RateFallback:   v.RateFallback,
	}, nil
}

func mathRound2(v float64) float64 {
	return RoundMoney(v)
}
