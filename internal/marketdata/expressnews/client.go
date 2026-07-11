package expressnews

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/baidu"
)

const providerName = "baidu_expressnews"

const (
	defaultRN   = 20
	maxRN       = 50
	expressPath = "/selfselect/expressnews"
)

var (
	ErrDisabled    = errors.New("baidu: disabled")
	ErrInvalidTag  = errors.New("invalid express news tag")
	ErrInvalidPage = errors.New("invalid page number")
)

var allowedTags = map[string]struct{}{
	"":     {},
	"A股":   {},
	"港股":   {},
	"美股":   {},
	"异动":   {},
}

// Client fetches and caches Baidu express news.
type Client struct {
	cfg    baidu.Config
	http   *http.Client
	cache  *responseCache
	fp     *fingerprintStore
	health ingest.ProviderReporter
	status atomic.Value // string
}

// NewClient creates an express news client.
func NewClient(cfg baidu.Config, health ingest.ProviderReporter) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = baidu.DefaultBaseURL
	}
	c := &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: 12 * time.Second},
		cache:  newResponseCache(),
		fp:     newFingerprintStore(),
		health: health,
	}
	if !cfg.Enabled {
		c.status.Store("disabled")
		if health != nil {
			health.ReportDisabled(providerName)
		}
	} else {
		c.status.Store("starting")
	}
	return c
}

// IngestStatus returns a coarse status string for /healthz.
func (c *Client) IngestStatus() string {
	if !c.cfg.Enabled {
		return "disabled"
	}
	if v, ok := c.status.Load().(string); ok && v != "" {
		return v
	}
	return "starting"
}

func (c *Client) reportCacheHit() {
	if c.health == nil || !c.cfg.Enabled {
		return
	}
	c.health.ReportSuccess(providerName, 0)
	c.health.ReportUsed(providerName, true)
	c.status.Store("ok")
}

func (c *Client) reportResult(start time.Time, err error) {
	if c.health == nil {
		if err != nil {
			c.status.Store("error")
		} else {
			c.status.Store("ok")
		}
		return
	}
	if !c.cfg.Enabled {
		c.health.ReportDisabled(providerName)
		c.status.Store("disabled")
		return
	}
	if err != nil {
		c.health.ReportFailure(providerName, err)
		c.status.Store("error")
		return
	}
	c.health.ReportSuccess(providerName, time.Since(start))
	c.health.ReportUsed(providerName, true)
	c.status.Store("ok")
}

// List loads a page of express news for the given tag.
func (c *Client) List(tag string, pn, rn, filterByUserStocks int) (Response, error) {
	if !c.cfg.Enabled {
		return Response{}, ErrDisabled
	}
	tag = NormalizeTag(tag)
	if _, ok := allowedTags[tag]; !ok {
		return Response{}, ErrInvalidTag
	}
	if pn < 0 {
		return Response{}, ErrInvalidPage
	}
	if rn <= 0 {
		rn = defaultRN
	}
	if rn > maxRN {
		rn = maxRN
	}

	key := cacheKey(tag, pn, rn, filterByUserStocks)
	if v, ok := c.cache.get(key); ok {
		c.reportCacheHit()
		return v, nil
	}

	start := time.Now()
	resp, latestID, err := c.fetch(tag, pn, rn, filterByUserStocks)
	if err != nil {
		c.reportResult(start, err)
		return Response{}, err
	}
	ttl := ttlForPage(tag, pn, latestID, c.fp)
	c.cache.set(key, resp, ttl)
	c.reportResult(start, nil)
	return resp, nil
}

func cacheKey(tag string, pn, rn, filterByUserStocks int) string {
	return fmt.Sprintf("expressnews:%s:%d:%d:%d", tag, pn, rn, filterByUserStocks)
}

func (c *Client) fetch(tag string, pn, rn, filterByUserStocks int) (Response, string, error) {
	q := url.Values{}
	q.Set("rn", strconv.Itoa(rn))
	q.Set("pn", strconv.Itoa(pn))
	q.Set("tag", tag)
	q.Set("filterByUserStocks", strconv.Itoa(filterByUserStocks))
	q.Set("finClientType", "pc")

	envelope, err := baidu.GetAPI(c.http, c.cfg, expressPath, q)
	if err != nil {
		return Response{}, "", err
	}

	var raw expressNewsResult
	if err := json.Unmarshal(envelope.Result, &raw); err != nil {
		return Response{}, "", fmt.Errorf("expressnews parse: %w", err)
	}

	items := make([]NewsItem, 0, len(raw.Content.List))
	var latestID string
	for i, row := range raw.Content.List {
		item := normalizeItem(row)
		if item.ID == "" {
			continue
		}
		items = append(items, item)
		if i == 0 && pn == 0 {
			latestID = item.ID
		}
	}

	out := Response{
		Tag:       tag,
		Pn:        pn,
		Rn:        rn,
		Source:    "baidu",
		FetchedAt: time.Now().Unix(),
		HasMore:   len(items) >= rn,
		Items:     items,
	}
	return out, latestID, nil
}

// NormalizeTag trims and validates the tag query value.
func NormalizeTag(tag string) string {
	return strings.TrimSpace(tag)
}

type expressNewsResult struct {
	Content struct {
		List []rawNewsItem `json:"list"`
	} `json:"content"`
}

type rawNewsItem struct {
	Loc         string `json:"loc"`
	Title       string `json:"title"`
	Content     struct {
		Items []struct {
			Type string `json:"type"`
			Data string `json:"data"`
		} `json:"items"`
	} `json:"content"`
	PublishTime json.Number `json:"publish_time"`
	ThirdURL    string      `json:"third_url"`
	Important   string      `json:"important"`
	Tag         string      `json:"tag"`
	Provider    string      `json:"provider"`
	Entity      []rawEntity `json:"entity"`
}

type rawEntity struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Market   string `json:"market"`
	Exchange string `json:"exchange"`
	Price    string `json:"price"`
	Ratio    string `json:"ratio"`
	Logo     struct {
		Logo string `json:"logo"`
	} `json:"logo"`
}

func normalizeItem(row rawNewsItem) NewsItem {
	id := strings.TrimSpace(row.Loc)
	if id == "" {
		id = strings.TrimSpace(row.ThirdURL)
	}
	body := extractBody(row.Content.Items)
	publishTime, _ := row.PublishTime.Int64()

	entities := make([]NewsEntity, 0, len(row.Entity))
	for _, e := range row.Entity {
		entities = append(entities, NewsEntity{
			Code:      strings.TrimSpace(e.Code),
			Name:      strings.TrimSpace(e.Name),
			Market:    strings.TrimSpace(e.Market),
			Exchange:  strings.TrimSpace(e.Exchange),
			Price:     strings.TrimSpace(e.Price),
			Ratio:     strings.TrimSpace(e.Ratio),
			ChangePct: parseRatioPct(e.Ratio),
			LogoURL:   strings.TrimSpace(e.Logo.Logo),
		})
	}

	return NewsItem{
		ID:          id,
		Title:       strings.TrimSpace(row.Title),
		Body:        body,
		PublishTime: publishTime,
		Provider:    strings.TrimSpace(row.Provider),
		Tag:         strings.TrimSpace(row.Tag),
		Important:   row.Important == "1",
		ThirdURL:    strings.TrimSpace(row.ThirdURL),
		Entities:    entities,
	}
}

func extractBody(items []struct {
	Type string `json:"type"`
	Data string `json:"data"`
}) string {
	var parts []string
	for _, it := range items {
		if strings.TrimSpace(it.Data) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(it.Data))
	}
	return strings.Join(parts, "")
}

func parseRatioPct(raw string) float64 {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "%")
	raw = strings.TrimPrefix(raw, "+")
	v, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
	if err != nil {
		return 0
	}
	return v
}
