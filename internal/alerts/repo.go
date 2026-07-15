package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type repository struct {
	db *sql.DB
}

func newRepository(db *sql.DB) *repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, rule Rule) (Rule, error) {
	paramsJSON, err := json.Marshal(rule.Params)
	if err != nil {
		return Rule{}, err
	}
	chJSON, err := json.Marshal(rule.Channels)
	if err != nil {
		return Rule{}, err
	}
	now := time.Now().Unix()
	res, err := r.db.ExecContext(ctx, `
INSERT INTO alert_rules (
  user_id, asset_type, symbol, field, rule_type, params, channels, frequency,
  interval_minutes, set_price, status, trigger_count, created_at, updated_at, is_deleted
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, 0)`,
		rule.UserID, rule.AssetType, rule.Symbol, rule.Field, rule.RuleType,
		paramsJSON, chJSON, rule.Frequency, rule.IntervalMinutes, rule.SetPrice,
		rule.Status, now, now,
	)
	if err != nil {
		return Rule{}, fmt.Errorf("alert_rules insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Rule{}, err
	}
	return r.GetByID(ctx, rule.UserID, id)
}

func (r *repository) GetByID(ctx context.Context, userID, id int64) (Rule, error) {
	row, err := r.scanOne(ctx, `
SELECT id, user_id, asset_type, symbol, field, rule_type, params, channels, frequency,
  interval_minutes, set_price, status, last_triggered_at, trigger_count, created_at, updated_at
FROM alert_rules WHERE id = ? AND user_id = ? AND is_deleted = 0`, id, userID)
	if err != nil {
		return Rule{}, err
	}
	return row, nil
}

func (r *repository) ListByUser(ctx context.Context, userID int64, status string) ([]Rule, error) {
	q := `
SELECT id, user_id, asset_type, symbol, field, rule_type, params, channels, frequency,
  interval_minutes, set_price, status, last_triggered_at, trigger_count, created_at, updated_at
FROM alert_rules WHERE user_id = ? AND is_deleted = 0`
	args := []any{userID}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("alert_rules list: %w", err)
	}
	defer rows.Close()
	out := make([]Rule, 0)
	for rows.Next() {
		rule, err := scanRuleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *repository) ListActive(ctx context.Context) ([]Rule, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, asset_type, symbol, field, rule_type, params, channels, frequency,
  interval_minutes, set_price, status, last_triggered_at, trigger_count, created_at, updated_at
FROM alert_rules WHERE is_deleted = 0 AND status = 'active'`)
	if err != nil {
		return nil, fmt.Errorf("alert_rules list active: %w", err)
	}
	defer rows.Close()
	out := make([]Rule, 0)
	for rows.Next() {
		rule, err := scanRuleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *repository) Update(ctx context.Context, userID int64, id int64, rule Rule) (Rule, error) {
	paramsJSON, err := json.Marshal(rule.Params)
	if err != nil {
		return Rule{}, err
	}
	chJSON, err := json.Marshal(rule.Channels)
	if err != nil {
		return Rule{}, err
	}
	now := time.Now().Unix()
	res, err := r.db.ExecContext(ctx, `
UPDATE alert_rules SET params=?, channels=?, frequency=?, interval_minutes=?, status=?, updated_at=?
WHERE id=? AND user_id=? AND is_deleted=0`,
		paramsJSON, chJSON, rule.Frequency, rule.IntervalMinutes, rule.Status, now, id, userID,
	)
	if err != nil {
		return Rule{}, fmt.Errorf("alert_rules update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Rule{}, ErrNotFound
	}
	return r.GetByID(ctx, userID, id)
}

func (r *repository) SoftDelete(ctx context.Context, userID, id int64) error {
	now := time.Now().Unix()
	res, err := r.db.ExecContext(ctx, `
UPDATE alert_rules SET is_deleted=1, status='disabled', updated_at=? WHERE id=? AND user_id=? AND is_deleted=0`,
		now, id, userID,
	)
	if err != nil {
		return fmt.Errorf("alert_rules delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) Disable(ctx context.Context, ruleID int64) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
UPDATE alert_rules SET status='disabled', updated_at=? WHERE id=? AND is_deleted=0`, now, ruleID)
	return err
}

func (r *repository) RecordTrigger(ctx context.Context, ruleID int64) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
UPDATE alert_rules SET last_triggered_at=?, trigger_count=trigger_count+1, updated_at=? WHERE id=?`,
		now, now, ruleID,
	)
	return err
}

func (r *repository) InsertDelivery(ctx context.Context, d Delivery) (Delivery, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO alert_deliveries (
  rule_id, user_id, asset_type, symbol, rule_type, channel, trigger_value,
  title, body, status, error_msg, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.RuleID, d.UserID, d.AssetType, d.Symbol, d.RuleType, d.Channel, d.TriggerValue,
		d.Title, d.Body, d.Status, d.ErrorMsg, d.CreatedAt,
	)
	if err != nil {
		return Delivery{}, fmt.Errorf("alert_deliveries insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Delivery{}, err
	}
	d.ID = id
	return d, nil
}

func (r *repository) ListDeliveries(ctx context.Context, userID int64, q ListDeliveriesQuery) ([]Delivery, int, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	where := `WHERE user_id = ?`
	args := []any{userID}
	if q.RuleID > 0 {
		where += ` AND rule_id = ?`
		args = append(args, q.RuleID)
	}
	if ch := strings.TrimSpace(q.Channel); ch != "" {
		where += ` AND channel = ?`
		args = append(args, ch)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alert_deliveries `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (q.Page - 1) * q.PageSize
	listArgs := append(append([]any{}, args...), q.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
SELECT id, rule_id, user_id, asset_type, symbol, rule_type, channel, trigger_value,
  title, body, status, error_msg, created_at
FROM alert_deliveries `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("alert_deliveries list: %w", err)
	}
	defer rows.Close()
	out := make([]Delivery, 0)
	for rows.Next() {
		var d Delivery
		var trigger sql.NullString
		if err := rows.Scan(
			&d.ID, &d.RuleID, &d.UserID, &d.AssetType, &d.Symbol, &d.RuleType, &d.Channel,
			&trigger, &d.Title, &d.Body, &d.Status, &d.ErrorMsg, &d.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		d.TriggerValue = trigger.String
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func (r *repository) scanOne(ctx context.Context, query string, args ...any) (Rule, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	var rule Rule
	var paramsRaw, chRaw []byte
	var setPrice sql.NullString
	var lastTrig sql.NullInt64
	err := row.Scan(
		&rule.ID, &rule.UserID, &rule.AssetType, &rule.Symbol, &rule.Field, &rule.RuleType,
		&paramsRaw, &chRaw, &rule.Frequency, &rule.IntervalMinutes, &setPrice, &rule.Status,
		&lastTrig, &rule.TriggerCount, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Rule{}, ErrNotFound
	}
	if err != nil {
		return Rule{}, fmt.Errorf("alert_rules scan: %w", err)
	}
	if err := json.Unmarshal(paramsRaw, &rule.Params); err != nil {
		return Rule{}, err
	}
	if err := json.Unmarshal(chRaw, &rule.Channels); err != nil {
		return Rule{}, err
	}
	rule.SetPrice = setPrice.String
	if lastTrig.Valid {
		v := lastTrig.Int64
		rule.LastTriggeredAt = &v
	}
	return rule, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRuleRow(rows rowScanner) (Rule, error) {
	var rule Rule
	var paramsRaw, chRaw []byte
	var setPrice sql.NullString
	var lastTrig sql.NullInt64
	err := rows.Scan(
		&rule.ID, &rule.UserID, &rule.AssetType, &rule.Symbol, &rule.Field, &rule.RuleType,
		&paramsRaw, &chRaw, &rule.Frequency, &rule.IntervalMinutes, &setPrice, &rule.Status,
		&lastTrig, &rule.TriggerCount, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return Rule{}, err
	}
	if err := json.Unmarshal(paramsRaw, &rule.Params); err != nil {
		return Rule{}, err
	}
	if err := json.Unmarshal(chRaw, &rule.Channels); err != nil {
		return Rule{}, err
	}
	rule.SetPrice = setPrice.String
	if lastTrig.Valid {
		v := lastTrig.Int64
		rule.LastTriggeredAt = &v
	}
	return rule, nil
}

func formatDecimal(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
