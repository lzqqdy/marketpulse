package metals

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/store"
)

var sgeDelayedQuotesURL = "https://en.sge.com.cn/data_DelayedQuotes"
var sgeDailyReportURL = "https://en.sge.com.cn/data/data_daily_international_new"

var au9999RowPattern = regexp.MustCompile(`(?i)Au99\.99\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)`)
var au9999DailyPattern = regexp.MustCompile(`(?i)\d{4}-\d{2}-\d{2}\s+Au99\.99\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)\s+([0-9]+(?:\.[0-9]+)?)\s+[-+0-9.]+\s+([-+0-9.]+)%`)

// FetchAu9999 loads Shanghai Gold Exchange delayed Au99.99 RMB/gram quote.
func FetchAu9999(client *http.Client) (store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, sgeDelayedQuotesURL, nil)
	if err != nil {
		return store.IndexQuote{}, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("sge au9999 request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.IndexQuote{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.IndexQuote{}, fmt.Errorf("sge au9999 http %d", resp.StatusCode)
	}

	latest, open, err := parseAu9999(body)
	if err != nil {
		return fetchRecentAu9999Daily(client)
	}
	changePct := 0.0
	if open > 0 {
		changePct = (latest - open) / open * 100
	}
	return store.IndexQuote{
		ID:        "sge-au9999",
		Name:      "国内金价",
		Price:     latest,
		ChangePct: changePct,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func fetchRecentAu9999Daily(client *http.Client) (store.IndexQuote, error) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	today := time.Now().In(loc)
	var firstErr error
	for i := 0; i < 14; i++ {
		day := today.AddDate(0, 0, -i).Format("2006-01-02")
		price, changePct, err := fetchAu9999Daily(client, day)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return store.IndexQuote{
			ID:        "sge-au9999",
			Name:      "国内金价",
			Price:     price,
			ChangePct: changePct,
			UpdatedAt: time.Now().UTC(),
		}, nil
	}
	return store.IndexQuote{}, firstErr
}

func fetchAu9999Daily(client *http.Client, day string) (price, changePct float64, err error) {
	u := sgeDailyReportURL + "?start_date=" + url.QueryEscape(day) + "&end_date=" + url.QueryEscape(day)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("sge au9999 daily request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("sge au9999 daily http %d", resp.StatusCode)
	}
	return parseAu9999Daily(body)
}

func parseAu9999(body []byte) (latest, open float64, err error) {
	text := normalizeSGEText(string(body))
	matches := au9999RowPattern.FindStringSubmatch(text)
	if len(matches) != 5 {
		return 0, 0, fmt.Errorf("sge au9999: row not found")
	}
	nums := make([]float64, 0, 4)
	for _, raw := range matches[1:] {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, 0, err
		}
		nums = append(nums, v)
	}
	latest = nums[0]
	open = nums[3]
	if latest <= 0 {
		return 0, 0, fmt.Errorf("sge au9999: empty latest")
	}
	if open <= 0 || math.Abs(open-latest)/latest > 0.2 {
		open = latest
	}
	return latest, open, nil
}

func parseAu9999Daily(body []byte) (price, changePct float64, err error) {
	text := normalizeSGEText(string(body))
	matches := au9999DailyPattern.FindStringSubmatch(text)
	if len(matches) != 6 {
		return 0, 0, fmt.Errorf("sge au9999 daily: row not found")
	}
	closePrice, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return 0, 0, err
	}
	if closePrice <= 0 {
		return 0, 0, fmt.Errorf("sge au9999 daily: empty close")
	}
	pct, err := strconv.ParseFloat(matches[5], 64)
	if err != nil {
		return 0, 0, err
	}
	return closePrice, pct, nil
}

func normalizeSGEText(raw string) string {
	replacer := strings.NewReplacer(
		"\r", " ",
		"\n", " ",
		"\t", " ",
		"&nbsp;", " ",
		"&#160;", " ",
		"&amp;", "&",
	)
	text := replacer.Replace(raw)
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
