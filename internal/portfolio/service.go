package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	portfoliomigrate "github.com/lzqqdy/marketpulse/internal/portfolio/migrate"
	"github.com/lzqqdy/marketpulse/internal/users"
)

// Service is the portfolio module facade.
type Service interface {
	Enabled() bool
	GetHoldings(ctx context.Context, userID int64) (HoldingsResult, error)
	PutHoldings(ctx context.Context, userID int64, in PutHoldingsInput) (HoldingsResult, error)
	PutSettings(ctx context.Context, userID int64, in PutSettingsInput) (Settings, error)
	Overview(ctx context.Context, userID int64) (Overview, error)
	ListSnapshots(ctx context.Context, userID int64, q ListSnapshotsQuery) (ListSnapshotsResult, error)
	EligibleSymbols(ctx context.Context) (EligibleSymbolsResult, error)
	RunDailySnapshot(ctx context.Context, forDate string) (int, error)
}

type service struct {
	cfg      config.PortfolioConfig
	repo     *repository
	md       marketdata.MarketDataService
	resolver PriceResolver
	tz       *time.Location
	jobStop  context.CancelFunc
}

// BootstrapArgs bundles deps for the portfolio module.
type BootstrapArgs struct {
	Portfolio  config.PortfolioConfig
	DB         *sql.DB
	MarketData marketdata.MarketDataService
	Users      users.Service
}

// Bootstrap opens migrations and starts the daily snapshot job when enabled.
func Bootstrap(ctx context.Context, args BootstrapArgs) (Service, error) {
	cfg := args.Portfolio
	if !cfg.Enabled {
		return &service{cfg: cfg}, nil
	}
	if args.DB == nil {
		return nil, fmt.Errorf("portfolio: mysql required when portfolio.enabled")
	}
	if args.Users == nil || !args.Users.Enabled() {
		return nil, fmt.Errorf("portfolio: users module required when portfolio.enabled")
	}
	if args.MarketData == nil {
		return nil, fmt.Errorf("portfolio: market data required when portfolio.enabled")
	}
	if cfg.IsAutoMigrate() {
		if err := portfoliomigrate.Run(ctx, args.DB); err != nil {
			return nil, err
		}
		slog.Info("portfolio migrations applied")
	}

	tz, err := time.LoadLocation(cfg.DailyTimezone)
	if err != nil {
		tz = time.FixedZone("CST", 8*3600)
	}
	svc := &service{
		cfg:      cfg,
		repo:     newRepository(args.DB),
		md:       args.MarketData,
		resolver: marketResolver{md: args.MarketData},
		tz:       tz,
	}
	jobCtx, cancel := context.WithCancel(ctx)
	svc.jobStop = cancel
	go svc.loopDailyJob(jobCtx)
	slog.Info("portfolio module enabled", "timezone", tz.String(), "default_usdt_cny", cfg.DefaultUsdtCny)
	return svc, nil
}

func (s *service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.repo != nil
}

func (s *service) GetHoldings(ctx context.Context, userID int64) (HoldingsResult, error) {
	if !s.Enabled() {
		return HoldingsResult{}, ErrDisabled
	}
	holdings, err := s.repo.ListHoldings(ctx, userID)
	if err != nil {
		return HoldingsResult{}, err
	}
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return HoldingsResult{}, err
	}
	v := ValueHoldings(s.resolver, holdings, s.cfg.DefaultUsdtCny)
	return HoldingsResult{
		Holdings:       v.Holdings,
		PrincipalCny:   settings.PrincipalCny,
		UsdtCny:        v.UsdtCny,
		UsdtPremiumPct: v.UsdtPremiumPct,
		RateFallback:   v.RateFallback,
		MissingSymbols: v.Missing,
	}, nil
}

func (s *service) PutHoldings(ctx context.Context, userID int64, in PutHoldingsInput) (HoldingsResult, error) {
	if !s.Enabled() {
		return HoldingsResult{}, ErrDisabled
	}
	normalized := make([]Holding, 0, len(in.Holdings))
	seen := map[string]struct{}{}
	for _, row := range in.Holdings {
		assetType, ok := NormalizeAssetType(row.AssetType)
		if !ok {
			return HoldingsResult{}, fmt.Errorf("%w: unsupported assetType", ErrInvalidInput)
		}
		var symbol string
		if assetType == AssetTypeCrypto {
			symbol = NormalizeCryptoSymbol(row.Symbol)
		} else {
			symbol = NormalizeAlphaSymbol(row.Symbol)
		}
		if symbol == "" {
			return HoldingsResult{}, fmt.Errorf("%w: empty symbol", ErrInvalidInput)
		}
		if row.Quantity < 0 {
			return HoldingsResult{}, fmt.Errorf("%w: quantity must be >= 0", ErrInvalidInput)
		}
		if row.Quantity == 0 {
			continue
		}
		key := assetType + ":" + symbol
		if _, dup := seen[key]; dup {
			return HoldingsResult{}, fmt.Errorf("%w: duplicate %s", ErrInvalidInput, key)
		}
		seen[key] = struct{}{}
		pp := ResolvePrice(s.resolver, assetType, symbol)
		if !pp.OK {
			return HoldingsResult{}, fmt.Errorf("%w: %s", ErrSymbolUnavailable, symbol)
		}
		normalized = append(normalized, Holding{
			UserID:    userID,
			AssetType: assetType,
			Symbol:    symbol,
			Quantity:  row.Quantity,
		})
	}
	if len(normalized) > 100 {
		return HoldingsResult{}, fmt.Errorf("%w: too many holdings", ErrInvalidInput)
	}
	if err := s.repo.ReplaceHoldings(ctx, userID, normalized); err != nil {
		return HoldingsResult{}, err
	}
	return s.GetHoldings(ctx, userID)
}

func (s *service) PutSettings(ctx context.Context, userID int64, in PutSettingsInput) (Settings, error) {
	if !s.Enabled() {
		return Settings{}, ErrDisabled
	}
	if in.PrincipalCny < 0 {
		return Settings{}, fmt.Errorf("%w: principalCny must be >= 0", ErrInvalidInput)
	}
	var principalUsdt *float64
	rate, _, ok := s.resolver.UsdtCny()
	if !ok || rate <= 0 {
		rate = s.cfg.DefaultUsdtCny
	}
	if rate > 0 {
		v := in.PrincipalCny / rate
		principalUsdt = &v
	}
	return s.repo.UpsertSettings(ctx, userID, RoundMoney(in.PrincipalCny), principalUsdt)
}

func (s *service) Overview(ctx context.Context, userID int64) (Overview, error) {
	if !s.Enabled() {
		return Overview{}, ErrDisabled
	}
	holdings, err := s.repo.ListHoldings(ctx, userID)
	if err != nil {
		return Overview{}, err
	}
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return Overview{}, err
	}
	v := ValueHoldings(s.resolver, holdings, s.cfg.DefaultUsdtCny)

	now := time.Now().In(s.tz)
	today := now.Format("2006-01-02")
	d7 := now.AddDate(0, 0, -7).Format("2006-01-02")
	d30 := now.AddDate(0, 0, -30).Format("2006-01-02")

	latest, err := s.repo.GetLatestDaily(ctx, userID)
	if err != nil {
		return Overview{}, err
	}
	snap7, err := s.repo.GetDailyOnOrBefore(ctx, userID, d7)
	if err != nil {
		return Overview{}, err
	}
	snap30, err := s.repo.GetDailyOnOrBefore(ctx, userID, d30)
	if err != nil {
		return Overview{}, err
	}

	var todayWin *PnLWindow
	if latest != nil && latest.Date < today {
		todayWin = WindowPnL(v.TotalCny, latest.TotalValueCny, true)
	} else if latest != nil && latest.Date == today {
		// same-day snapshot exists: today pnl uses previous day baseline if any
		prev, err := s.repo.GetDailyOnOrBefore(ctx, userID, now.AddDate(0, 0, -1).Format("2006-01-02"))
		if err != nil {
			return Overview{}, err
		}
		if prev != nil {
			todayWin = WindowPnL(v.TotalCny, prev.TotalValueCny, true)
		}
	} else if latest != nil {
		todayWin = WindowPnL(v.TotalCny, latest.TotalValueCny, true)
	}

	return Overview{
		TotalUsdt:      v.TotalUsdt,
		TotalCny:       RoundMoney(v.TotalCny),
		UsdtCny:        v.UsdtCny,
		UsdtPremiumPct: v.UsdtPremiumPct,
		RateFallback:   v.RateFallback,
		Today:          todayWin,
		D7:             WindowPnL(v.TotalCny, snapBaseline(snap7), snap7 != nil),
		D30:            WindowPnL(v.TotalCny, snapBaseline(snap30), snap30 != nil),
		AllTime:        AllTimePnL(v.TotalCny, settings.PrincipalCny),
		MissingSymbols: v.Missing,
	}, nil
}

func snapBaseline(s *Snapshot) float64 {
	if s == nil {
		return 0
	}
	return s.TotalValueCny
}

func (s *service) ListSnapshots(ctx context.Context, userID int64, q ListSnapshotsQuery) (ListSnapshotsResult, error) {
	if !s.Enabled() {
		return ListSnapshotsResult{}, ErrDisabled
	}
	items, total, err := s.repo.ListSnapshots(ctx, userID, q)
	if err != nil {
		return ListSnapshotsResult{}, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if items == nil {
		items = []Snapshot{}
	}
	return ListSnapshotsResult{Total: total, Page: page, PageSize: pageSize, Items: items}, nil
}

func (s *service) EligibleSymbols(_ context.Context) (EligibleSymbolsResult, error) {
	if !s.Enabled() {
		return EligibleSymbolsResult{}, ErrDisabled
	}
	snap := s.md.Snapshot()
	crypto := make([]EligibleSymbol, 0, len(snap.Quotes)+1)
	seen := map[string]struct{}{}
	for _, q := range snap.Quotes {
		sym := NormalizeCryptoSymbol(q.Symbol)
		if sym == "" || q.PriceUsdt <= 0 {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		crypto = append(crypto, EligibleSymbol{Symbol: sym, Name: sym, Type: AssetTypeCrypto})
	}
	if _, ok := seen["USDT"]; !ok {
		crypto = append(crypto, EligibleSymbol{Symbol: "USDT", Name: "USDT", Type: AssetTypeCrypto})
	}

	alpha := make([]EligibleSymbol, 0)
	alphaSeen := map[string]struct{}{}
	addAlpha := func(id, name string, price float64) {
		id = NormalizeAlphaSymbol(id)
		if id == "" || price <= 0 {
			return
		}
		if _, ok := alphaSeen[id]; ok {
			return
		}
		alphaSeen[id] = struct{}{}
		if name == "" {
			name = id
		}
		alpha = append(alpha, EligibleSymbol{Symbol: id, Name: name, Type: AssetTypeAlpha})
	}
	for _, row := range snap.Alpha.Indices {
		addAlpha(row.ID, row.Name, row.Price)
	}
	for _, row := range snap.Alpha.Stocks {
		addAlpha(row.ID, row.Name, row.Price)
	}
	return EligibleSymbolsResult{Crypto: crypto, Alpha: alpha}, nil
}

// RunDailySnapshot writes snapshots for forDate (YYYY-MM-DD). Empty => yesterday Shanghai.
func (s *service) RunDailySnapshot(ctx context.Context, forDate string) (int, error) {
	if !s.Enabled() {
		return 0, ErrDisabled
	}
	date := strings.TrimSpace(forDate)
	if date == "" {
		now := time.Now().In(s.tz)
		date = now.AddDate(0, 0, -1).Format("2006-01-02")
	}
	ids, err := s.repo.ListUserIDsWithHoldings(ctx)
	if err != nil {
		return 0, err
	}
	written := 0
	for _, uid := range ids {
		if err := s.snapshotUser(ctx, uid, date); err != nil {
			slog.Warn("portfolio daily snapshot skipped user", "user_id", uid, "date", date, "err", err)
			continue
		}
		written++
	}
	return written, nil
}

func (s *service) snapshotUser(ctx context.Context, userID int64, date string) error {
	holdings, err := s.repo.ListHoldings(ctx, userID)
	if err != nil {
		return err
	}
	if len(holdings) == 0 {
		return nil
	}
	v := ValueHoldings(s.resolver, holdings, s.cfg.DefaultUsdtCny)
	if len(v.Missing) > 0 {
		return fmt.Errorf("missing prices: %s", strings.Join(v.Missing, ","))
	}
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	prev, err := s.repo.GetDailyOnOrBefore(ctx, userID, prevDate(date))
	if err != nil {
		return err
	}
	prevCny := 0.0
	if prev != nil {
		prevCny = prev.TotalValueCny
	}
	dailyProfit, dailyRate, totalProfit, totalRate := DailyRatesFromPrev(v.TotalCny, prevCny, settings.PrincipalCny)
	detail, err := marshalAssetDetail(v.Details)
	if err != nil {
		return err
	}
	return s.repo.UpsertDailySnapshot(ctx, Snapshot{
		UserID:          userID,
		Date:            date,
		Kind:            SnapshotKindDaily,
		TotalValue:      v.TotalUsdt,
		TotalValueCny:   RoundMoney(v.TotalCny),
		DailyProfit:     RoundMoney(dailyProfit),
		DailyProfitRate: dailyRate,
		TotalProfit:     RoundMoney(totalProfit),
		TotalProfitRate: totalRate,
		AssetDetail:     detail,
		Source:          SourceSystem,
	})
}

func prevDate(date string) string {
	t, err := time.ParseInLocation("2006-01-02", date, time.UTC)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

func (s *service) loopDailyJob(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	var lastDate string
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().In(s.tz)
			// after 00:05, ensure yesterday snapshot exists
			if now.Hour() == 0 && now.Minute() < 5 {
				continue
			}
			y := now.AddDate(0, 0, -1).Format("2006-01-02")
			if y == lastDate {
				continue
			}
			n, err := s.RunDailySnapshot(ctx, y)
			if err != nil {
				slog.Error("portfolio daily job failed", "date", y, "err", err)
				continue
			}
			lastDate = y
			slog.Info("portfolio daily job done", "date", y, "users", n)
		}
	}
}
