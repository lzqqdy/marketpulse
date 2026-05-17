package equity

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/store"
)

const stooqDailyBase = "https://stooq.com/q/d/l/"
const stooqQuoteBase = "https://stooq.com/q/l/"

type stooqRow struct {
	date  time.Time
	close float64
}

// FetchStooqQuotes loads delayed daily closes from Stooq CSV as a final fallback.
func FetchStooqQuotes(client *http.Client, defs []IndexDef) (map[string]store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	now := time.Now().UTC()
	out := make(map[string]store.IndexQuote, len(defs))
	var firstErr error
	for i, def := range defs {
		if strings.TrimSpace(def.StooqSymbol) == "" {
			continue
		}
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		q, err := fetchStooqOne(client, def, now)
		if err != nil {
			slog.Warn("equity symbol fetch failed", "provider", "stooq", "id", def.ID, "symbol", def.StooqSymbol, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out[def.ID] = q
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, firstErr
}

func fetchStooqOne(client *http.Client, def IndexDef, now time.Time) (store.IndexQuote, error) {
	values := url.Values{}
	values.Set("s", def.StooqSymbol)
	values.Set("f", "sd2t2ohlcv")
	values.Set("h", "")
	values.Set("e", "csv")
	req, err := http.NewRequest(http.MethodGet, stooqQuoteBase+"?"+values.Encode(), nil)
	if err != nil {
		return store.IndexQuote{}, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("equity http request failed", "provider", "stooq", "id", def.ID, "symbol", def.StooqSymbol, "err", err)
		return store.IndexQuote{}, fmt.Errorf("%s stooq request: %w", def.ID, err)
	}
	defer resp.Body.Close()
	slog.Info("equity http response", "provider", "stooq", "id", def.ID, "symbol", def.StooqSymbol, "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return store.IndexQuote{}, fmt.Errorf("%s stooq http %d", def.ID, resp.StatusCode)
	}
	q, err := parseStooqQuoteCSV(resp.Body)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s stooq parse: %w", def.ID, err)
	}
	if err := validatePrice(def, q.close); err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s stooq: %w", def.ID, err)
	}
	changePct := 0.0
	if q.open > 0 {
		changePct = (q.close - q.open) / q.open * 100
	}
	return store.IndexQuote{
		ID:        def.ID,
		Name:      def.Name,
		Price:     q.close,
		ChangePct: changePct,
		Source:    "stooq",
		FetchedAt: now,
		UpdatedAt: q.updatedAt,
	}, nil
}

type stooqQuoteRow struct {
	open      float64
	close     float64
	updatedAt time.Time
}

func parseStooqQuoteCSV(r io.Reader) (stooqQuoteRow, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return stooqQuoteRow{}, err
	}
	if len(records) < 2 {
		return stooqQuoteRow{}, fmt.Errorf("empty quote csv")
	}
	rec := records[1]
	if len(rec) < 7 || strings.EqualFold(strings.TrimSpace(rec[1]), "N/D") {
		return stooqQuoteRow{}, fmt.Errorf("no data")
	}
	open, err := strconv.ParseFloat(strings.TrimSpace(rec[3]), 64)
	if err != nil {
		open = 0
	}
	closePrice, err := strconv.ParseFloat(strings.TrimSpace(rec[6]), 64)
	if err != nil || closePrice <= 0 {
		return stooqQuoteRow{}, fmt.Errorf("empty close")
	}
	updatedAt := time.Now().UTC()
	datePart := strings.TrimSpace(rec[1])
	timePart := strings.TrimSpace(rec[2])
	if datePart != "" && timePart != "" && !strings.EqualFold(timePart, "N/D") {
		if t, err := time.Parse("2006-01-02 15:04:05", datePart+" "+timePart); err == nil {
			updatedAt = t.UTC()
		}
	}
	return stooqQuoteRow{open: open, close: closePrice, updatedAt: updatedAt}, nil
}

func parseStooqCSV(r io.Reader) ([]stooqRow, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	rows := make([]stooqRow, 0, len(records))
	for i, rec := range records {
		if i == 0 || len(rec) < 5 {
			continue
		}
		if strings.EqualFold(rec[0], "No data") {
			continue
		}
		day, err := time.Parse("2006-01-02", strings.TrimSpace(rec[0]))
		if err != nil {
			continue
		}
		closePrice, err := strconv.ParseFloat(strings.TrimSpace(rec[4]), 64)
		if err != nil || closePrice <= 0 {
			continue
		}
		rows = append(rows, stooqRow{date: day, close: closePrice})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].date.Before(rows[j].date) })
	return rows, nil
}
