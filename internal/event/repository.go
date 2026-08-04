package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errNotFound = errors.New("event not found")

// ErrNotFound is returned when an event id does not exist.
var ErrNotFound = errNotFound

type repository struct {
	db *sql.DB
}

func newRepository(db *sql.DB) *repository {
	return &repository{db: db}
}

func (r *repository) CreateEvent(ctx context.Context, ev *MarketEvent) error {
	symbols, err := json.Marshal(ev.Symbols)
	if err != nil {
		return err
	}
	markets, err := json.Marshal(ev.Markets)
	if err != nil {
		return err
	}
	ctxJSON, err := json.Marshal(ev.Context)
	if err != nil {
		return err
	}
	if ctxJSON == nil || string(ctxJSON) == "null" {
		ctxJSON = []byte("{}")
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO market_event (
  id, type, sub_type, title, description, severity, score, peak_score, status,
  start_time, end_time, symbols_json, markets_json, context_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, ev.Type, ev.SubType, ev.Title, ev.Description, ev.Severity, ev.Score, ev.PeakScore, ev.Status,
		mysqlUTC(ev.StartTime), nullTime(ev.EndTime), symbols, markets, ctxJSON, mysqlUTC(ev.CreatedAt), mysqlUTC(ev.UpdatedAt),
	)
	return err
}

func (r *repository) UpdateEvent(ctx context.Context, ev *MarketEvent) error {
	symbols, err := json.Marshal(ev.Symbols)
	if err != nil {
		return err
	}
	markets, err := json.Marshal(ev.Markets)
	if err != nil {
		return err
	}
	ctxJSON, err := json.Marshal(ev.Context)
	if err != nil {
		return err
	}
	if ctxJSON == nil || string(ctxJSON) == "null" {
		ctxJSON = []byte("{}")
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE market_event SET
  type=?, sub_type=?, title=?, description=?, severity=?, score=?, peak_score=?, status=?,
  start_time=?, end_time=?, symbols_json=?, markets_json=?, context_json=?, updated_at=?
WHERE id=?`,
		ev.Type, ev.SubType, ev.Title, ev.Description, ev.Severity, ev.Score, ev.PeakScore, ev.Status,
		mysqlUTC(ev.StartTime), nullTime(ev.EndTime), symbols, markets, ctxJSON, mysqlUTC(ev.UpdatedAt), ev.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNotFound
	}
	return nil
}

func (r *repository) InsertSignals(ctx context.Context, signals []EventSignal) error {
	if len(signals) == 0 {
		return nil
	}
	for i := range signals {
		sig := &signals[i]
		meta, err := json.Marshal(sig.Metadata)
		if err != nil {
			return err
		}
		if meta == nil || string(meta) == "null" {
			meta = []byte("{}")
		}
		_, err = r.db.ExecContext(ctx, `
INSERT INTO market_event_signal (
  id, event_id, signal_type, symbol, market, value, baseline, ratio, change_pct,
  window_label, direction, ts, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sig.ID, sig.EventID, sig.SignalType, sig.Symbol, sig.Market, sig.Value, sig.Baseline, sig.Ratio, sig.ChangePct,
			sig.Window, sig.Direction, mysqlUTC(sig.Timestamp), meta,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *repository) Get(ctx context.Context, id string) (*MarketEvent, error) {
	ev, err := r.scanEvent(r.db.QueryRowContext(ctx, `
SELECT id, type, sub_type, title, description, severity, score, peak_score, status,
       start_time, end_time, symbols_json, markets_json, context_json, created_at, updated_at
FROM market_event WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	sigs, err := r.ListSignals(ctx, id)
	if err != nil {
		return nil, err
	}
	ev.Signals = sigs
	return ev, nil
}

func (r *repository) ListSignals(ctx context.Context, eventID string) ([]EventSignal, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, event_id, signal_type, symbol, market, value, baseline, ratio, change_pct,
       window_label, direction, ts, metadata_json
FROM market_event_signal WHERE event_id=? ORDER BY ts ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EventSignal, 0)
	for rows.Next() {
		var s EventSignal
		var meta []byte
		var ts time.Time
		if err := rows.Scan(
			&s.ID, &s.EventID, &s.SignalType, &s.Symbol, &s.Market, &s.Value, &s.Baseline, &s.Ratio, &s.ChangePct,
			&s.Window, &s.Direction, &ts, &meta,
		); err != nil {
			return nil, err
		}
		s.Timestamp = asUTCWall(ts)
		_ = json.Unmarshal(meta, &s.Metadata)
		if s.Metadata == nil {
			s.Metadata = map[string]any{}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *repository) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	if f.Market != "" {
		where = append(where, "JSON_CONTAINS(markets_json, JSON_QUOTE(?))")
		args = append(args, strings.ToUpper(f.Market))
	}
	if f.Type != "" {
		where = append(where, "type=?")
		args = append(args, f.Type)
	}
	if f.SubType != "" {
		where = append(where, "sub_type=?")
		args = append(args, f.SubType)
	}
	if f.Severity != "" {
		where = append(where, "severity=?")
		args = append(args, strings.ToUpper(f.Severity))
	}
	if f.Status != "" {
		where = append(where, "status=?")
		args = append(args, strings.ToUpper(f.Status))
	}
	if f.Symbol != "" {
		where = append(where, "JSON_CONTAINS(symbols_json, JSON_QUOTE(?))")
		args = append(args, strings.ToUpper(f.Symbol))
	}
	if f.Start != nil {
		where = append(where, "start_time>=?")
		args = append(args, f.Start.UTC())
	}
	if f.End != nil {
		where = append(where, "start_time<=?")
		args = append(args, f.End.UTC())
	}
	q := fmt.Sprintf(`
SELECT id, type, sub_type, title, description, severity, score, peak_score, status,
       start_time, end_time, symbols_json, markets_json, context_json, created_at, updated_at
FROM market_event WHERE %s ORDER BY start_time DESC LIMIT ? OFFSET ?`, strings.Join(where, " AND "))
	args = append(args, f.Limit+1, f.Offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()
	items := make([]MarketEvent, 0, f.Limit)
	for rows.Next() {
		ev, err := r.scanEventRows(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, *ev)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	res := ListResult{Limit: f.Limit}
	if len(items) > f.Limit {
		items = items[:f.Limit]
		res.NextCursor = fmt.Sprintf("%d", f.Offset+f.Limit)
	}
	res.Items = items
	return res, nil
}

func (r *repository) ListOpen(ctx context.Context) ([]MarketEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, type, sub_type, title, description, severity, score, peak_score, status,
       start_time, end_time, symbols_json, markets_json, context_json, created_at, updated_at
FROM market_event WHERE status IN (?, ?, ?) ORDER BY start_time DESC LIMIT 200`,
		StatusDetected, StatusActive, StatusDeescalating)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MarketEvent, 0)
	for rows.Next() {
		ev, err := r.scanEventRows(rows)
		if err != nil {
			return nil, err
		}
		sigs, err := r.ListSignals(ctx, ev.ID)
		if err != nil {
			return nil, err
		}
		ev.Signals = sigs
		out = append(out, *ev)
	}
	return out, rows.Err()
}

// ResolveEventIDs marks the given event ids as RESOLVED.
func (r *repository) ResolveEventIDs(ctx context.Context, ids []string, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	now = now.UTC()
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+3)
	args = append(args, StatusResolved, mysqlUTC(now), mysqlUTC(now))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := fmt.Sprintf(`
UPDATE market_event SET status=?, end_time=?, updated_at=?, score=LEAST(score, 25)
WHERE id IN (%s) AND status IN ('DETECTED','ACTIVE','DEESCALATING')`, strings.Join(placeholders, ","))
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *repository) scanEvent(row rowScanner) (*MarketEvent, error) {
	var ev MarketEvent
	var end sql.NullTime
	var symbols, markets, ctxJSON []byte
	err := row.Scan(
		&ev.ID, &ev.Type, &ev.SubType, &ev.Title, &ev.Description, &ev.Severity, &ev.Score, &ev.PeakScore, &ev.Status,
		&ev.StartTime, &end, &symbols, &markets, &ctxJSON, &ev.CreatedAt, &ev.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if end.Valid {
		t := asUTCWall(end.Time)
		ev.EndTime = &t
	}
	ev.StartTime = asUTCWall(ev.StartTime)
	ev.CreatedAt = asUTCWall(ev.CreatedAt)
	ev.UpdatedAt = asUTCWall(ev.UpdatedAt)
	_ = json.Unmarshal(symbols, &ev.Symbols)
	_ = json.Unmarshal(markets, &ev.Markets)
	_ = json.Unmarshal(ctxJSON, &ev.Context)
	if ev.Context == nil {
		ev.Context = map[string]any{}
	}
	return &ev, nil
}

func (r *repository) scanEventRows(rows *sql.Rows) (*MarketEvent, error) {
	return r.scanEvent(rows)
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return mysqlUTC(*t)
}

// mysqlUTC stores UTC wall-clock digits. With DSN loc=Local, passing time.Time.UTC()
// would be shifted to local wall (+8h) and break merge/age checks.
func mysqlUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), u.Hour(), u.Minute(), u.Second(), u.Nanosecond(), time.Local)
}

// asUTCWall treats DATETIME digits as UTC regardless of driver location.
func asUTCWall(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}
