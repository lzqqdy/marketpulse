package equity

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/store"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const tencentQuoteBase = "https://qt.gtimg.cn/q="

var tencentQuotePattern = regexp.MustCompile(`v_([^=]+)="([^"]*)";`)

// FetchTencentQuotes loads index quotes from Tencent in one batched request.
func FetchTencentQuotes(client *http.Client, defs []IndexDef) (map[string]store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	symbols := make([]string, 0, len(defs))
	byTencent := make(map[string]IndexDef, len(defs))
	for _, def := range defs {
		if strings.TrimSpace(def.TencentSymbol) == "" {
			continue
		}
		symbols = append(symbols, def.TencentSymbol)
		byTencent[def.TencentSymbol] = def
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("tencent: no symbols")
	}

	req, err := http.NewRequest(http.MethodGet, tencentQuoteBase+strings.Join(symbols, ","), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("equity http request failed", "provider", "tencent", "symbols", len(symbols), "err", err)
		return nil, fmt.Errorf("tencent request: %w", err)
	}
	defer resp.Body.Close()
	slog.Info("equity http response", "provider", "tencent", "symbols", len(symbols), "status", resp.StatusCode)
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("tencent http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tencent http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(transform.NewReader(resp.Body, simplifiedchinese.GB18030.NewDecoder()))
	if err != nil {
		return nil, fmt.Errorf("tencent decode: %w", err)
	}
	return parseTencentQuotes(string(body), byTencent, time.Now().UTC())
}

func parseTencentQuotes(body string, byTencent map[string]IndexDef, now time.Time) (map[string]store.IndexQuote, error) {
	out := make(map[string]store.IndexQuote, len(byTencent))
	var firstErr error
	for _, m := range tencentQuotePattern.FindAllStringSubmatch(body, -1) {
		if len(m) != 3 {
			continue
		}
		def, ok := byTencent[m[1]]
		if !ok {
			continue
		}
		row, err := tencentRowToIndex(def, m[2], now)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out[row.ID] = row
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tencent: empty result")
	}
	if len(out) < len(byTencent) && firstErr == nil {
		firstErr = fmt.Errorf("tencent: partial result %d/%d", len(out), len(byTencent))
	}
	return out, firstErr
}

func tencentRowToIndex(def IndexDef, raw string, now time.Time) (store.IndexQuote, error) {
	var price, changePct float64
	var updatedAt time.Time
	var err error
	switch {
	case strings.HasPrefix(def.TencentSymbol, "hf_"):
		parts := strings.Split(raw, ",")
		if len(parts) < 2 {
			return store.IndexQuote{}, fmt.Errorf("tencent %s: empty quote", def.ID)
		}
		price, err = parseTencentFloat(parts, 0)
		if err != nil {
			return store.IndexQuote{}, fmt.Errorf("tencent %s price: %w", def.ID, err)
		}
		changePct, _ = parseTencentFloat(parts, 1)
	case strings.HasPrefix(def.TencentSymbol, "gz"):
		parts := strings.Split(raw, "~")
		if len(parts) < 6 {
			return store.IndexQuote{}, fmt.Errorf("tencent %s: empty quote", def.ID)
		}
		price, err = parseTencentFloat(parts, 3)
		if err != nil {
			return store.IndexQuote{}, fmt.Errorf("tencent %s price: %w", def.ID, err)
		}
		changePct, _ = parseTencentFloat(parts, 5)
		updatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(parts[2]), time.UTC)
	default:
		parts := strings.Split(raw, "~")
		if len(parts) < 6 {
			return store.IndexQuote{}, fmt.Errorf("tencent %s: empty quote", def.ID)
		}
		price, err = parseTencentFloat(parts, 3)
		if err != nil {
			return store.IndexQuote{}, fmt.Errorf("tencent %s price: %w", def.ID, err)
		}
		changePct, _ = parseTencentFloat(parts, 5)
	}
	if err := validatePrice(def, price); err != nil {
		return store.IndexQuote{}, fmt.Errorf("tencent %s: %w", def.ID, err)
	}
	if updatedAt.IsZero() {
		updatedAt = now
	}
	return store.IndexQuote{
		ID:        def.ID,
		Name:      def.Name,
		Price:     price,
		ChangePct: changePct,
		Source:    "tencent",
		FetchedAt: now,
		UpdatedAt: updatedAt,
	}, nil
}

func parseTencentFloat(parts []string, i int) (float64, error) {
	if i < 0 || i >= len(parts) {
		return 0, fmt.Errorf("missing field %d", i)
	}
	return strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
}
