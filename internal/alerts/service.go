package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
	"github.com/lzqqdy/marketpulse/internal/users"
	alertsmigrate "github.com/lzqqdy/marketpulse/internal/alerts/migrate"
)

// Service is the alerts module facade.
type Service interface {
	Enabled() bool
	Hub() *Hub
	ListRules(ctx context.Context, userID int64, status string) ([]Rule, error)
	CreateRule(ctx context.Context, userID int64, in CreateRuleInput) (Rule, error)
	UpdateRule(ctx context.Context, userID, id int64, in UpdateRuleInput) (Rule, error)
	DeleteRule(ctx context.Context, userID, id int64) error
	ListDeliveries(ctx context.Context, userID int64, q ListDeliveriesQuery) (ListDeliveriesResult, error)
	AckInbox(ctx context.Context, userID int64, deliveryIDs []int64) error
	InboxSnapshot(ctx context.Context, userID int64) ([]InboxItem, error)
}

type service struct {
	cfg        config.AlertsConfig
	repo       *repository
	inbox      *InboxStore
	hub        *Hub
	md         marketdata.MarketDataService
	evaluator  *Evaluator
	index      *ruleIndex
	dispatcher *Dispatcher
	cooldown   *CooldownStore
}

// BootstrapArgs bundles deps for the alerts module.
type BootstrapArgs struct {
	Alerts     config.AlertsConfig
	SMTP       config.SMTPConfig
	DB         *sql.DB
	Redis      *platformredis.Client
	MarketData marketdata.MarketDataService
	Users      users.Service
}

// Bootstrap opens migrations and starts evaluation when enabled.
func Bootstrap(ctx context.Context, args BootstrapArgs) (Service, error) {
	cfg := args.Alerts
	if !cfg.Enabled {
		return &service{cfg: cfg}, nil
	}
	if args.DB == nil {
		return nil, fmt.Errorf("alerts: mysql required when alerts.enabled")
	}
	if args.Redis == nil {
		return nil, fmt.Errorf("alerts: redis required when alerts.enabled")
	}
	if args.Users == nil || !args.Users.Enabled() {
		return nil, fmt.Errorf("alerts: users module required when alerts.enabled")
	}
	if args.MarketData == nil {
		return nil, fmt.Errorf("alerts: market data required when alerts.enabled")
	}
	if cfg.IsAutoMigrate() {
		if err := alertsmigrate.Run(ctx, args.DB); err != nil {
			return nil, err
		}
		slog.Info("alerts migrations applied")
	}

	tz, err := time.LoadLocation(cfg.DailyTimezone)
	if err != nil {
		tz = time.FixedZone("CST", 8*3600)
	}

	repo := newRepository(args.DB)
	inbox := NewInboxStore(args.Redis, cfg.InboxMaxLen)
	hub := NewHub()
	cooldown := NewCooldownStore(args.Redis, tz)
	windows := NewWindowTracker()
	index := newRuleIndex()

	svc := &service{
		cfg:      cfg,
		repo:     repo,
		inbox:    inbox,
		hub:      hub,
		md:       args.MarketData,
		index:    index,
		cooldown: cooldown,
	}

	onOnce := func(ctx context.Context, ruleID int64) {
		_ = repo.Disable(ctx, ruleID)
		if rules, err := repo.ListActive(ctx); err == nil {
			index.Rebuild(rules)
		}
	}

	svc.dispatcher = NewDispatcher(repo, inbox, hub, args.SMTP, args.Users, cooldown, onOnce)
	svc.evaluator = NewEvaluator(args.MarketData, svc.dispatcher, cooldown, windows, index)

	active, err := repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	index.Rebuild(active)
	slog.Info("alerts module enabled", "active_rules", len(active))
	return svc, nil
}

func (s *service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.repo != nil
}

func (s *service) Hub() *Hub {
	return s.hub
}

func (s *service) ListRules(ctx context.Context, userID int64, status string) ([]Rule, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	return s.repo.ListByUser(ctx, userID, strings.TrimSpace(status))
}

func (s *service) CreateRule(ctx context.Context, userID int64, in CreateRuleInput) (Rule, error) {
	if !s.Enabled() {
		return Rule{}, ErrDisabled
	}
	rule, err := s.buildRule(ctx, userID, in)
	if err != nil {
		return Rule{}, err
	}
	price, ok := s.currentPrice(in.AssetType, in.Symbol)
	if !ok {
		return Rule{}, ErrSymbolUnavailable
	}
	rule.SetPrice = formatDecimal(price)

	var params RuleParams
	if err := json.Unmarshal(in.Params, &params); err != nil {
		return Rule{}, ErrInvalidParams
	}
	if err := ValidateCreateParams(in.RuleType, params); err != nil {
		return Rule{}, err
	}
	params, err = BuildBounds(in.RuleType, price, params)
	if err != nil {
		return Rule{}, err
	}
	rule.Params = params

	key := indexKey(rule.AssetType, normalizeIndexSymbol(rule.AssetType, rule.Symbol))
	amp, ready := s.evaluator.windows.Update(key, price, time.Now())
	if ConditionAlreadyMetAtCreate(in.RuleType, price, params, amp, ready) {
		return Rule{}, ErrConditionAlreadyMet
	}

	created, err := s.repo.Create(ctx, rule)
	if err != nil {
		return Rule{}, err
	}
	if created.Status == StatusActive {
		s.index.Upsert(created)
	}
	return created, nil
}

func (s *service) UpdateRule(ctx context.Context, userID, id int64, in UpdateRuleInput) (Rule, error) {
	if !s.Enabled() {
		return Rule{}, ErrDisabled
	}
	cur, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return Rule{}, err
	}
	old := cur

	if in.Channels != nil {
		if err := validateChannels(in.Channels); err != nil {
			return Rule{}, err
		}
		cur.Channels = in.Channels
	}
	if in.Frequency != nil {
		if err := validateFrequency(*in.Frequency, cur.IntervalMinutes, s.cfg); err != nil {
			return Rule{}, err
		}
		cur.Frequency = *in.Frequency
	}
	if in.IntervalMinutes != nil {
		if err := validateFrequency(cur.Frequency, *in.IntervalMinutes, s.cfg); err != nil {
			return Rule{}, err
		}
		cur.IntervalMinutes = *in.IntervalMinutes
	}
	if in.Status != nil {
		st := strings.TrimSpace(*in.Status)
		if st != StatusActive && st != StatusDisabled {
			return Rule{}, ErrInvalidParams
		}
		cur.Status = st
	}
	if in.Params != nil {
		var params RuleParams
		if err := json.Unmarshal(*in.Params, &params); err != nil {
			return Rule{}, ErrInvalidParams
		}
		if err := ValidateCreateParams(cur.RuleType, params); err != nil {
			return Rule{}, err
		}
		setPrice, err := strconv.ParseFloat(cur.SetPrice, 64)
		if err != nil {
			return Rule{}, ErrInvalidParams
		}
		params, err = BuildBounds(cur.RuleType, setPrice, params)
		if err != nil {
			return Rule{}, err
		}
		price, ok := s.currentPrice(cur.AssetType, cur.Symbol)
		if !ok {
			return Rule{}, ErrSymbolUnavailable
		}
		key := indexKey(cur.AssetType, normalizeIndexSymbol(cur.AssetType, cur.Symbol))
		amp, ready := s.evaluator.WindowAmplitude(cur.AssetType, cur.Symbol)
		if amp == 0 && !ready {
			amp, ready = s.evaluator.windows.Update(key, price, time.Now())
		}
		if ConditionAlreadyMetAtCreate(cur.RuleType, price, params, amp, ready) {
			return Rule{}, ErrConditionAlreadyMet
		}
		cur.Params = params
	}

	updated, err := s.repo.Update(ctx, userID, id, cur)
	if err != nil {
		return Rule{}, err
	}
	s.index.Remove(old)
	if updated.Status == StatusActive {
		s.index.Upsert(updated)
	}
	return updated, nil
}

func (s *service) DeleteRule(ctx context.Context, userID, id int64) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	cur, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, userID, id); err != nil {
		return err
	}
	s.index.Remove(cur)
	_ = s.cooldown.Clear(ctx, id)
	return nil
}

func (s *service) ListDeliveries(ctx context.Context, userID int64, q ListDeliveriesQuery) (ListDeliveriesResult, error) {
	if !s.Enabled() {
		return ListDeliveriesResult{}, ErrDisabled
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	items, total, err := s.repo.ListDeliveries(ctx, userID, q)
	if err != nil {
		return ListDeliveriesResult{}, err
	}
	return ListDeliveriesResult{Items: items, Page: q.Page, PageSize: q.PageSize, Total: total}, nil
}

func (s *service) AckInbox(ctx context.Context, userID int64, deliveryIDs []int64) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	return s.inbox.Ack(ctx, userID, deliveryIDs)
}

func (s *service) InboxSnapshot(ctx context.Context, userID int64) ([]InboxItem, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	return s.inbox.List(ctx, userID)
}

func (s *service) buildRule(_ context.Context, userID int64, in CreateRuleInput) (Rule, error) {
	assetType := strings.ToLower(strings.TrimSpace(in.AssetType))
	if assetType != AssetSpot && assetType != AssetIndex {
		return Rule{}, ErrInvalidParams
	}
	symbol, err := normalizeSymbol(assetType, in.Symbol)
	if err != nil {
		return Rule{}, err
	}
	field := strings.TrimSpace(in.Field)
	if field == "" {
		field = "price"
	}
	if in.RuleType < 1 || in.RuleType > 5 {
		return Rule{}, ErrInvalidParams
	}
	if err := validateChannels(in.Channels); err != nil {
		return Rule{}, err
	}
	freq := strings.TrimSpace(in.Frequency)
	if freq == "" {
		freq = FrequencyLoop
	}
	interval := in.IntervalMinutes
	if interval <= 0 {
		interval = 10
	}
	if err := validateFrequency(freq, interval, s.cfg); err != nil {
		return Rule{}, err
	}
	return Rule{
		UserID:          userID,
		AssetType:       assetType,
		Symbol:          symbol,
		Field:           field,
		RuleType:        in.RuleType,
		Channels:        in.Channels,
		Frequency:       freq,
		IntervalMinutes: interval,
		Status:          StatusActive,
	}, nil
}

func (s *service) currentPrice(assetType, symbol string) (float64, bool) {
	switch assetType {
	case AssetSpot:
		base := normalizeSpotBase(symbol)
		q, ok := s.md.Quote(base)
		if !ok || q.PriceUsdt <= 0 {
			return 0, false
		}
		return q.PriceUsdt, true
	case AssetIndex:
		id := strings.ToLower(strings.TrimSpace(symbol))
		q, ok := s.md.IndexQuote(id)
		if !ok || q.Stale || q.Price <= 0 {
			return 0, false
		}
		return q.Price, true
	default:
		return 0, false
	}
}

func normalizeSymbol(assetType, symbol string) (string, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return "", ErrInvalidParams
	}
	if assetType == AssetIndex {
		return strings.ToLower(symbol), nil
	}
	base := normalizeSpotBase(symbol)
	if base == "" {
		return "", ErrInvalidParams
	}
	return base + "USDT", nil
}

func normalizeSpotBase(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	return strings.TrimSuffix(s, "USDT")
}

func validateChannels(channels []string) error {
	if len(channels) == 0 {
		return ErrInvalidParams
	}
	for _, ch := range channels {
		if _, ok := allowedChannels[strings.TrimSpace(ch)]; !ok {
			return ErrInvalidParams
		}
	}
	return nil
}

func validateFrequency(freq string, interval int, cfg config.AlertsConfig) error {
	switch freq {
	case FrequencyOnce, FrequencyDaily:
		return nil
	case FrequencyLoop:
		min := cfg.LoopIntervalMin
		max := cfg.LoopIntervalMax
		if min <= 0 {
			min = 1
		}
		if max <= 0 {
			max = 1440
		}
		if interval < min || interval > max {
			return ErrInvalidParams
		}
		return nil
	default:
		return ErrInvalidParams
	}
}
