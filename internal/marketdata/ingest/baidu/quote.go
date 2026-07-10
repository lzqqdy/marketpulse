package baidu

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

const quotationPath = "/vapi/v1/getquotation"

// FetchQuotes loads index quotes from Baidu Finance HTTP API.
func FetchQuotes(client *http.Client, cfg Config, refs []IndexRef) (map[string]store.IndexQuote, error) {
	cfg = normalizeConfig(cfg)
	if !cfg.Enabled {
		return nil, fmt.Errorf("baidu: disabled")
	}
	now := time.Now().UTC()
	out := make(map[string]store.IndexQuote, len(refs))
	wanted := make(map[string]IndexRef, len(refs))
	for _, ref := range refs {
		if !ref.valid() {
			continue
		}
		wanted[bannerKey(ref)] = ref
	}
	if len(wanted) == 0 {
		return nil, fmt.Errorf("baidu: no mapped symbols")
	}

	if bannerRows, err := fetchIndexBannerQuotes(client, cfg, wanted, now); err == nil {
		for id, row := range bannerRows {
			out[id] = row
		}
	} else {
		slog.Warn("baidu indexbanner failed", "err", err)
	}

	var firstErr error
	for _, ref := range refs {
		if !ref.valid() {
			continue
		}
		if _, ok := out[ref.ID]; ok {
			continue
		}
		row, err := fetchQuote(client, cfg, ref, now)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			slog.Warn("baidu quote failed", "id", ref.ID, "err", err)
			continue
		}
		out[ref.ID] = row
	}
	if len(out) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("baidu: no quotes")
	}
	return out, nil
}

func fetchIndexBannerQuotes(client *http.Client, cfg Config, wanted map[string]IndexRef, now time.Time) (map[string]store.IndexQuote, error) {
	envelope, err := getJSON(client, cfg.BaseURL, "/api/indexbanner", nil)
	if err != nil {
		return nil, err
	}
	var items []indexBannerItem
	if err := json.Unmarshal(envelope.Result, &items); err != nil {
		return nil, fmt.Errorf("baidu indexbanner parse: %w", err)
	}
	out := make(map[string]store.IndexQuote, len(items))
	for _, item := range items {
		ref, ok := wanted[bannerKey(IndexRef{Code: item.Code, Market: item.Market})]
		if !ok {
			continue
		}
		price, err := strconvParse(item.Price)
		if err != nil {
			continue
		}
		if err := ref.validatePrice(price); err != nil {
			continue
		}
		name := ref.Name
		if strings.TrimSpace(item.Name) != "" {
			name = strings.TrimSpace(item.Name)
		}
		out[ref.ID] = store.IndexQuote{
			ID:        ref.ID,
			Name:      name,
			Price:     price,
			ChangePct: parseBaiduPercent(item.Ratio),
			Source:    "baidu",
			FetchedAt: now,
			UpdatedAt: now,
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("baidu indexbanner: no matched quotes")
	}
	return out, nil
}

func bannerKey(ref IndexRef) string {
	return lower(ref.Market) + ":" + upper(ref.Code)
}

func upper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

func fetchQuote(client *http.Client, cfg Config, ref IndexRef, now time.Time) (store.IndexQuote, error) {
	query, err := quoteParams(ref)
	if err != nil {
		return store.IndexQuote{}, err
	}
	envelope, err := getJSON(client, cfg.BaseURL, quotationPath, query)
	if err != nil {
		return store.IndexQuote{}, err
	}
	var result quotationResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s baidu quote parse: %w", ref.ID, err)
	}
	price, err := strconvParse(result.Cur.Price)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s baidu quote price: %w", ref.ID, err)
	}
	if err := ref.validatePrice(price); err != nil {
		return store.IndexQuote{}, err
	}
	return store.IndexQuote{
		ID:        ref.ID,
		Name:      ref.Name,
		Price:     price,
		ChangePct: parseBaiduPercent(result.Cur.Ratio),
		Source:    "baidu",
		FetchedAt: now,
		UpdatedAt: now,
	}, nil
}

func strconvParse(raw string) (float64, error) {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if raw == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseFloat(raw, 64)
}

// ParseQuotationResult converts a raw quotation payload into an index quote.
func ParseQuotationResult(ref IndexRef, raw []byte, now time.Time) (store.IndexQuote, error) {
	var result quotationResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return store.IndexQuote{}, err
	}
	price, err := strconvParse(result.Cur.Price)
	if err != nil {
		return store.IndexQuote{}, err
	}
	if err := ref.validatePrice(price); err != nil {
		return store.IndexQuote{}, err
	}
	return store.IndexQuote{
		ID:        ref.ID,
		Name:      ref.Name,
		Price:     price,
		ChangePct: parseBaiduPercent(result.Cur.Ratio),
		Source:    "baidu",
		FetchedAt: now,
		UpdatedAt: now,
	}, nil
}
