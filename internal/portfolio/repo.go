package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type repository struct {
	db *sql.DB
}

func newRepository(db *sql.DB) *repository {
	return &repository{db: db}
}

func (r *repository) GetSettings(ctx context.Context, userID int64) (Settings, error) {
	var s Settings
	var principalUsdt sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
SELECT user_id, principal_cny, principal_usdt, created_at, updated_at
FROM portfolio_settings WHERE user_id = ?`, userID).Scan(
		&s.UserID, &s.PrincipalCny, &principalUsdt, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now()
		return Settings{UserID: userID, PrincipalCny: 0, CreatedAt: now, UpdatedAt: now}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("portfolio_settings get: %w", err)
	}
	if principalUsdt.Valid {
		v := principalUsdt.Float64
		s.PrincipalUsdt = &v
	}
	return s, nil
}

func (r *repository) UpsertSettings(ctx context.Context, userID int64, principalCny float64, principalUsdt *float64) (Settings, error) {
	now := time.Now()
	var usdt any
	if principalUsdt != nil {
		usdt = *principalUsdt
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO portfolio_settings (user_id, principal_cny, principal_usdt, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE principal_cny=VALUES(principal_cny), principal_usdt=VALUES(principal_usdt), updated_at=VALUES(updated_at)`,
		userID, principalCny, usdt, now, now,
	)
	if err != nil {
		return Settings{}, fmt.Errorf("portfolio_settings upsert: %w", err)
	}
	return r.GetSettings(ctx, userID)
}

func (r *repository) ListHoldings(ctx context.Context, userID int64) ([]Holding, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, asset_type, symbol, quantity, target_price, created_at, updated_at
FROM portfolio_holdings WHERE user_id = ? ORDER BY asset_type, symbol`, userID)
	if err != nil {
		return nil, fmt.Errorf("portfolio_holdings list: %w", err)
	}
	defer rows.Close()
	var out []Holding
	for rows.Next() {
		h, err := scanHolding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *repository) ReplaceHoldings(ctx context.Context, userID int64, holdings []Holding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM portfolio_holdings WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("portfolio_holdings delete: %w", err)
	}
	now := time.Now()
	for _, h := range holdings {
		var target any
		if h.TargetPrice != nil {
			target = *h.TargetPrice
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO portfolio_holdings (user_id, asset_type, symbol, quantity, target_price, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			userID, h.AssetType, h.Symbol, h.Quantity, target, now, now,
		); err != nil {
			return fmt.Errorf("portfolio_holdings insert: %w", err)
		}
	}
	return tx.Commit()
}

func (r *repository) ListUserIDsWithHoldings(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT DISTINCT user_id FROM portfolio_holdings WHERE quantity > 0 ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *repository) ListSnapshots(ctx context.Context, userID int64, q ListSnapshotsQuery) ([]Snapshot, int, error) {
	where := `WHERE user_id = ? AND kind = ?`
	args := []any{userID, SnapshotKindDaily}
	if from := strings.TrimSpace(q.From); from != "" {
		where += ` AND date >= ?`
		args = append(args, from)
	}
	if to := strings.TrimSpace(q.To); to != "" {
		where += ` AND date <= ?`
		args = append(args, to)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM portfolio_snapshots `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sortBy := "date"
	if strings.EqualFold(q.SortBy, "totalValueCny") {
		sortBy = "total_value_cny"
	} else if strings.EqualFold(q.SortBy, "dailyProfit") {
		sortBy = "daily_profit"
	}
	order := "DESC"
	if strings.EqualFold(q.SortOrder, "asc") {
		order = "ASC"
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
	offset := (page - 1) * pageSize

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
SELECT id, user_id, date, kind, total_value, total_value_cny, daily_profit, daily_profit_rate,
  total_profit, total_profit_rate, COALESCE(asset_detail, ''), source, created_at
FROM portfolio_snapshots %s ORDER BY %s %s LIMIT ? OFFSET ?`, where, sortBy, order),
		append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, s)
	}
	return items, total, rows.Err()
}

func (r *repository) GetLatestDaily(ctx context.Context, userID int64) (*Snapshot, error) {
	return r.getOneSnapshot(ctx, `
SELECT id, user_id, date, kind, total_value, total_value_cny, daily_profit, daily_profit_rate,
  total_profit, total_profit_rate, COALESCE(asset_detail, ''), source, created_at
FROM portfolio_snapshots WHERE user_id = ? AND kind = ? ORDER BY date DESC LIMIT 1`, userID, SnapshotKindDaily)
}

func (r *repository) GetDailyOnOrBefore(ctx context.Context, userID int64, date string) (*Snapshot, error) {
	return r.getOneSnapshot(ctx, `
SELECT id, user_id, date, kind, total_value, total_value_cny, daily_profit, daily_profit_rate,
  total_profit, total_profit_rate, COALESCE(asset_detail, ''), source, created_at
FROM portfolio_snapshots WHERE user_id = ? AND kind = ? AND date <= ? ORDER BY date DESC LIMIT 1`,
		userID, SnapshotKindDaily, date)
}

func (r *repository) GetDailyExact(ctx context.Context, userID int64, date string) (*Snapshot, error) {
	return r.getOneSnapshot(ctx, `
SELECT id, user_id, date, kind, total_value, total_value_cny, daily_profit, daily_profit_rate,
  total_profit, total_profit_rate, COALESCE(asset_detail, ''), source, created_at
FROM portfolio_snapshots WHERE user_id = ? AND kind = ? AND date = ? LIMIT 1`,
		userID, SnapshotKindDaily, date)
}

func (r *repository) UpsertDailySnapshot(ctx context.Context, s Snapshot) error {
	detail := s.AssetDetail
	if detail == "" {
		detail = "[]"
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO portfolio_snapshots (
  user_id, date, kind, total_value, total_value_cny, daily_profit, daily_profit_rate,
  total_profit, total_profit_rate, asset_detail, source, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  total_value=VALUES(total_value),
  total_value_cny=VALUES(total_value_cny),
  daily_profit=VALUES(daily_profit),
  daily_profit_rate=VALUES(daily_profit_rate),
  total_profit=VALUES(total_profit),
  total_profit_rate=VALUES(total_profit_rate),
  asset_detail=VALUES(asset_detail),
  source=VALUES(source)`,
		s.UserID, s.Date, SnapshotKindDaily, s.TotalValue, s.TotalValueCny,
		s.DailyProfit, s.DailyProfitRate, s.TotalProfit, s.TotalProfitRate,
		detail, s.Source, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("portfolio_snapshots upsert: %w", err)
	}
	return nil
}

func (r *repository) InsertSnapshotSkip(ctx context.Context, s Snapshot) (inserted bool, err error) {
	detail := s.AssetDetail
	if detail == "" {
		detail = "[]"
	}
	res, err := r.db.ExecContext(ctx, `
INSERT IGNORE INTO portfolio_snapshots (
  user_id, date, kind, total_value, total_value_cny, daily_profit, daily_profit_rate,
  total_profit, total_profit_rate, asset_detail, source, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.UserID, s.Date, s.Kind, s.TotalValue, s.TotalValueCny,
		s.DailyProfit, s.DailyProfitRate, s.TotalProfit, s.TotalProfitRate,
		detail, s.Source, s.CreatedAt,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *repository) getOneSnapshot(ctx context.Context, query string, args ...any) (*Snapshot, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	s, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHolding(rs rowScanner) (Holding, error) {
	var h Holding
	var target sql.NullFloat64
	if err := rs.Scan(&h.ID, &h.UserID, &h.AssetType, &h.Symbol, &h.Quantity, &target, &h.CreatedAt, &h.UpdatedAt); err != nil {
		return Holding{}, err
	}
	if target.Valid {
		v := target.Float64
		h.TargetPrice = &v
	}
	return h, nil
}

func scanSnapshot(rs rowScanner) (Snapshot, error) {
	var s Snapshot
	var date time.Time
	if err := rs.Scan(
		&s.ID, &s.UserID, &date, &s.Kind, &s.TotalValue, &s.TotalValueCny,
		&s.DailyProfit, &s.DailyProfitRate, &s.TotalProfit, &s.TotalProfitRate,
		&s.AssetDetail, &s.Source, &s.CreatedAt,
	); err != nil {
		return Snapshot{}, err
	}
	s.Date = date.Format("2006-01-02")
	return s, nil
}

func marshalAssetDetail(rows []AssetDetailRow) (string, error) {
	if rows == nil {
		rows = []AssetDetailRow{}
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
