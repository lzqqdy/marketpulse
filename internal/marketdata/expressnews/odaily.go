package expressnews

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	odailyProviderName = "odaily_expressnews"
	odailyBaseURL      = "https://api.odaily.news"
	odailyNewsPath     = "/api/v1/newsflash"
	odailyUserAgent    = "MarketPulse/1.0 (+expressnews)"
)

var htmlTagRe = regexp.MustCompile(`(?s)<[^>]*>`)

type odailyEnvelope struct {
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Success bool            `json:"success"`
	Data    odailyPageData  `json:"data"`
}

type odailyPageData struct {
	HasMore bool             `json:"hasMore"`
	List    []odailyNewsItem `json:"list"`
}

type odailyNewsItem struct {
	ID                int64    `json:"id"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	IsImportant       bool     `json:"isImportant"`
	PublishTimestamp  int64    `json:"publishTimestamp"`
	SourceURL         string   `json:"sourceUrl"`
	Link              string   `json:"link"`
}

func (c *Client) listOdaily(pn, rn int) (Response, error) {
	if pn < 0 {
		return Response{}, ErrInvalidPage
	}
	if rn <= 0 {
		rn = defaultRN
	}
	if rn > maxRN {
		rn = maxRN
	}

	key := cacheKey(TagCrypto, pn, rn, 0)
	if v, ok := c.cache.get(key); ok {
		c.reportOdailyCacheHit()
		return v, nil
	}

	start := time.Now()
	resp, latestID, err := c.fetchOdaily(pn, rn)
	if err != nil {
		c.reportOdailyResult(start, err)
		return Response{}, err
	}
	ttl := ttlForPage(TagCrypto, pn, latestID, c.fp)
	c.cache.set(key, resp, ttl)
	c.reportOdailyResult(start, nil)
	return resp, nil
}

func (c *Client) fetchOdaily(pn, rn int) (Response, string, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(pn+1)) // Odaily is 1-based
	q.Set("size", strconv.Itoa(rn))
	q.Set("lang", "zh-cn")

	reqURL := odailyBaseURL + odailyNewsPath + "?" + q.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return Response{}, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", odailyUserAgent)

	httpResp, err := c.http.Do(req)
	if err != nil {
		return Response{}, "", err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return Response{}, "", err
	}
	if httpResp.StatusCode != http.StatusOK {
		return Response{}, "", fmt.Errorf("odaily http %d: %s", httpResp.StatusCode, truncate(string(body), 180))
	}

	var env odailyEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Response{}, "", fmt.Errorf("odaily parse: %w", err)
	}
	if env.Code != 200 && !env.Success {
		msg := strings.TrimSpace(env.Msg)
		if msg == "" {
			msg = "upstream error"
		}
		return Response{}, "", fmt.Errorf("odaily: %s", msg)
	}

	items := make([]NewsItem, 0, len(env.Data.List))
	var latestID string
	for i, row := range env.Data.List {
		item := normalizeOdailyItem(row)
		if item.ID == "" {
			continue
		}
		items = append(items, item)
		if i == 0 && pn == 0 {
			latestID = item.ID
		}
	}

	out := Response{
		Tag:       TagCrypto,
		Pn:        pn,
		Rn:        rn,
		Source:    "odaily",
		FetchedAt: time.Now().Unix(),
		HasMore:   env.Data.HasMore,
		Items:     items,
	}
	return out, latestID, nil
}

func normalizeOdailyItem(row odailyNewsItem) NewsItem {
	id := ""
	if row.ID > 0 {
		id = "odaily:" + strconv.FormatInt(row.ID, 10)
	}
	third := strings.TrimSpace(row.Link)
	if third == "" {
		third = strings.TrimSpace(row.SourceURL)
	}
	return NewsItem{
		ID:          id,
		Title:       strings.TrimSpace(row.Title),
		Body:        stripHTML(row.Content),
		PublishTime: msToUnixSeconds(row.PublishTimestamp),
		Provider:    "Odaily",
		Tag:         TagCrypto,
		Important:   row.IsImportant,
		ThirdURL:    third,
	}
}

func msToUnixSeconds(ms int64) int64 {
	if ms <= 0 {
		return 0
	}
	// Guard against already-second timestamps.
	if ms < 1_000_000_000_000 {
		return ms
	}
	return ms / 1000
}

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (c *Client) reportOdailyCacheHit() {
	if c.health == nil {
		return
	}
	c.health.ReportSuccess(odailyProviderName, 0)
	c.health.ReportUsed(odailyProviderName, true)
}

func (c *Client) reportOdailyResult(start time.Time, err error) {
	if c.health == nil {
		return
	}
	if err != nil {
		c.health.ReportFailure(odailyProviderName, err)
		return
	}
	c.health.ReportSuccess(odailyProviderName, time.Since(start))
	c.health.ReportUsed(odailyProviderName, true)
}
