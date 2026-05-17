package equity

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/store"
)

const stooqDailyBase = "https://stooq.com/q/d/l/"

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
	values.Set("i", "d")
	req, err := http.NewRequest(http.MethodGet, stooqDailyBase+"?"+values.Encode(), nil)
	if err != nil {
		return store.IndexQuote{}, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s stooq request: %w", def.ID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return store.IndexQuote{}, fmt.Errorf("%s stooq http %d", def.ID, resp.StatusCode)
	}
	rows, err := parseStooqCSV(resp.Body)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s stooq parse: %w", def.ID, err)
	}
	if len(rows) < 1 {
		return store.IndexQuote{}, fmt.Errorf("%s stooq: empty result", def.ID)
	}
	latest := rows[len(rows)-1]
	changePct := 0.0
	if len(rows) >= 2 && rows[len(rows)-2].close > 0 {
		prev := rows[len(rows)-2].close
		changePct = (latest.close - prev) / prev * 100
	}
	return store.IndexQuote{
		ID:        def.ID,
		Name:      def.Name,
		Price:     latest.close,
		ChangePct: changePct,
		Source:    "stooq",
		FetchedAt: now,
		UpdatedAt: latest.date,
	}, nil
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
