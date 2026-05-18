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

const sinaQuoteBase = "https://hq.sinajs.cn/list="

var sinaQuotePattern = regexp.MustCompile(`var hq_str_([^=]+)="([^"]*)";`)

// FetchSinaQuotes loads the configured index watchlist from Sina in one batched request.
func FetchSinaQuotes(client *http.Client, defs []IndexDef) (map[string]store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	symbols := make([]string, 0, len(defs))
	bySina := make(map[string]IndexDef, len(defs))
	for _, def := range defs {
		if strings.TrimSpace(def.SinaSymbol) == "" {
			continue
		}
		symbols = append(symbols, def.SinaSymbol)
		bySina[def.SinaSymbol] = def
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("sina: no symbols")
	}

	req, err := http.NewRequest(http.MethodGet, sinaQuoteBase+strings.Join(symbols, ","), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("equity http request failed", "provider", "sina", "symbols", len(symbols), "err", err)
		return nil, fmt.Errorf("sina request: %w", err)
	}
	defer resp.Body.Close()
	slog.Info("equity http response", "provider", "sina", "symbols", len(symbols), "status", resp.StatusCode)
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("sina http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sina http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(transform.NewReader(resp.Body, simplifiedchinese.GB18030.NewDecoder()))
	if err != nil {
		return nil, fmt.Errorf("sina decode: %w", err)
	}
	return parseSinaQuotes(string(body), bySina, time.Now().UTC())
}

func parseSinaQuotes(body string, bySina map[string]IndexDef, now time.Time) (map[string]store.IndexQuote, error) {
	out := make(map[string]store.IndexQuote, len(bySina))
	var firstErr error
	for _, m := range sinaQuotePattern.FindAllStringSubmatch(body, -1) {
		if len(m) != 3 {
			continue
		}
		def, ok := bySina[m[1]]
		if !ok {
			continue
		}
		row, err := sinaRowToIndex(def, m[2], now)
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
		return nil, fmt.Errorf("sina: empty result")
	}
	if len(out) < len(bySina) && firstErr == nil {
		firstErr = fmt.Errorf("sina: partial result %d/%d", len(out), len(bySina))
	}
	return out, firstErr
}

func sinaRowToIndex(def IndexDef, raw string, now time.Time) (store.IndexQuote, error) {
	parts := strings.Split(raw, ",")
	if len(parts) < 4 || strings.TrimSpace(raw) == "" {
		return store.IndexQuote{}, fmt.Errorf("sina %s: empty quote", def.ID)
	}

	var price, changePct float64
	var err error
	switch {
	case strings.HasPrefix(def.SinaSymbol, "hf_"):
		price, err = parseFloat(parts, 0)
		if err != nil {
			return store.IndexQuote{}, fmt.Errorf("sina %s price: %w", def.ID, err)
		}
		prev, _ := parseFloat(parts, 7)
		if prev > 0 {
			changePct = (price - prev) / prev * 100
		}
	case strings.HasPrefix(def.SinaSymbol, "s_"), strings.HasPrefix(def.SinaSymbol, "int_"):
		price, err = parseFloat(parts, 1)
		if err != nil {
			return store.IndexQuote{}, fmt.Errorf("sina %s price: %w", def.ID, err)
		}
		changePct, _ = parseFloat(parts, 3)
	default:
		return store.IndexQuote{}, fmt.Errorf("sina %s: unsupported symbol %s", def.ID, def.SinaSymbol)
	}
	if err := validatePrice(def, price); err != nil {
		return store.IndexQuote{}, fmt.Errorf("sina %s: %w", def.ID, err)
	}
	return store.IndexQuote{
		ID:        def.ID,
		Name:      def.Name,
		Price:     price,
		ChangePct: changePct,
		Source:    "sina",
		FetchedAt: now,
		UpdatedAt: now,
	}, nil
}

func parseFloat(parts []string, i int) (float64, error) {
	if i < 0 || i >= len(parts) {
		return 0, fmt.Errorf("missing field %d", i)
	}
	return strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
}
