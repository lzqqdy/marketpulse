package marketcenter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/baidu"
)

const providerName = "baidu_market_center"

const (
	defaultHeatmapRN   = 20
	defaultFundflowRN  = 12
	defaultHeatmapSort = "amount"
	maxTrendPoints     = 32
)

var sortKeyLabels = map[string]string{
	"amount":      "成交额",
	"volume":      "成交量",
	"marketValue": "市值",
}

// Client fetches and caches Baidu market center data.
type Client struct {
	cfg    baidu.Config
	http   *http.Client
	cache  *responseCache
	health ingest.ProviderReporter
	status atomic.Value // string
}

// NewClient creates a market center client.
func NewClient(cfg baidu.Config, health ingest.ProviderReporter) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = baidu.DefaultBaseURL
	}
	c := &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: 12 * time.Second},
		cache:  newResponseCache(),
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

// Center loads aggregated market center data (heatmap defaults to amount).
func (c *Client) Center(market string) (CenterResponse, error) {
	market, err := NormalizeMarket(market)
	if err != nil {
		return CenterResponse{}, err
	}
	key := "center:" + market
	if v, ok := c.cache.getIfFresh(key, CacheTTLForMarket(market, time.Now())); ok {
		c.reportCacheHit()
		return v.(CenterResponse), nil
	}
	start := time.Now()
	now := time.Now().UTC()
	var (
		chg      ChgDiagram
		heatmap  Heatmap
		fundflow Fundflow
		overview Overview
		wg       sync.WaitGroup
		errMu    sync.Mutex
		fetchErr error
	)
	recordErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if fetchErr == nil {
			fetchErr = err
		}
		errMu.Unlock()
	}
	wg.Add(4)
	go func() {
		defer wg.Done()
		v, err := c.fetchChgDiagram(market)
		if err != nil {
			recordErr(err)
			return
		}
		chg = v
	}()
	go func() {
		defer wg.Done()
		v, err := c.fetchHeatmap(market, defaultHeatmapSort)
		if err != nil {
			recordErr(err)
			return
		}
		heatmap = v
	}()
	go func() {
		defer wg.Done()
		v, err := c.fetchFundflow(market)
		if err != nil {
			recordErr(err)
			return
		}
		fundflow = v
	}()
	go func() {
		defer wg.Done()
		v, err := c.fetchOverview(market)
		if err != nil {
			recordErr(err)
			return
		}
		overview = v
	}()
	wg.Wait()
	if fetchErr != nil {
		c.reportResult(start, fetchErr)
		return CenterResponse{}, fetchErr
	}
	out := CenterResponse{
		Market:     market,
		Source:     "baidu",
		FetchedAt:  now.Unix(),
		ChgDiagram: chg,
		Heatmap:    heatmap,
		Fundflow:   fundflow,
		Overview:   overview,
	}
	c.cache.set(key, out)
	c.reportResult(start, nil)
	return out, nil
}

// Heatmap loads heatmap for a specific sort key.
func (c *Client) Heatmap(market, sortKey string) (Heatmap, error) {
	market, err := NormalizeMarket(market)
	if err != nil {
		return Heatmap{}, err
	}
	sortKey = normalizeSortKey(sortKey)
	key := fmt.Sprintf("heatmap:%s:%s", market, sortKey)
	if v, ok := c.cache.getIfFresh(key, CacheTTLForMarket(market, time.Now())); ok {
		c.reportCacheHit()
		return v.(Heatmap), nil
	}
	start := time.Now()
	heatmap, err := c.fetchHeatmap(market, sortKey)
	if err != nil {
		c.reportResult(start, err)
		return Heatmap{}, err
	}
	c.cache.set(key, heatmap)
	c.reportResult(start, nil)
	return heatmap, nil
}

func (c *Client) fetchChgDiagram(market string) (ChgDiagram, error) {
	q := url.Values{}
	q.Set("bizType", "chgdiagram")
	q.Set("market", market)
	q.Set("finClientType", "pc")
	env, err := baidu.GetAPI(c.http, c.cfg, "/sapi/v1/marketquote", q)
	if err != nil {
		return ChgDiagram{}, fmt.Errorf("chgdiagram: %w", err)
	}
	var wrapper struct {
		ChgDiagram struct {
			Total *struct {
				Title string `json:"title"`
				Price string `json:"price"`
			} `json:"total"`
			Ratio struct {
				Up      int `json:"up"`
				Down    int `json:"down"`
				Balance int `json:"balance"`
			} `json:"ratio"`
			Diagram []ChgDiagramBar `json:"diagram"`
		} `json:"chgdiagram"`
	}
	if err := json.Unmarshal(env.Result, &wrapper); err != nil {
		return ChgDiagram{}, fmt.Errorf("chgdiagram parse: %w", err)
	}
	out := ChgDiagram{
		Up:      wrapper.ChgDiagram.Ratio.Up,
		Down:    wrapper.ChgDiagram.Ratio.Down,
		Balance: wrapper.ChgDiagram.Ratio.Balance,
		Bars:    wrapper.ChgDiagram.Diagram,
	}
	if wrapper.ChgDiagram.Total != nil {
		out.TotalTitle = strings.TrimSpace(wrapper.ChgDiagram.Total.Title)
		out.TotalValue = strings.TrimSpace(wrapper.ChgDiagram.Total.Price)
	}
	return out, nil
}

func (c *Client) fetchHeatmap(market, sortKey string) (Heatmap, error) {
	sortKey = normalizeSortKey(sortKey)
	q := url.Values{}
	q.Set("style", "heatmap")
	q.Set("market", market)
	q.Set("typeCode", HeatmapTypeCode(market))
	q.Set("sortKey", sortKey)
	q.Set("sortType", "desc")
	q.Set("pn", "0")
	q.Set("rn", strconv.Itoa(defaultHeatmapRN))
	q.Set("finClientType", "pc")
	env, err := baidu.GetAPI(c.http, c.cfg, "/vapi/v2/blocks", q)
	if err != nil {
		return Heatmap{}, fmt.Errorf("heatmap: %w", err)
	}
	var wrapper struct {
		List struct {
			Body []struct {
				Code         string `json:"code"`
				Name         string `json:"name"`
				Market       string `json:"market"`
				Amount       string `json:"amount"`
				Volume       string `json:"volume"`
				MarketValue  string `json:"marketValue"`
				LastPx       string `json:"lastPx"`
				PxChangeRate string `json:"pxChangeRate"`
				Logo         struct {
					Logo string `json:"logo"`
				} `json:"logo"`
			} `json:"body"`
		} `json:"list"`
	}
	if err := json.Unmarshal(env.Result, &wrapper); err != nil {
		return Heatmap{}, fmt.Errorf("heatmap parse: %w", err)
	}
	items := make([]HeatmapItem, 0, len(wrapper.List.Body))
	for _, row := range wrapper.List.Body {
		metric := row.Amount
		switch sortKey {
		case "volume":
			metric = row.Volume
		case "marketValue":
			metric = row.MarketValue
		}
		items = append(items, HeatmapItem{
			Code:         row.Code,
			Name:         row.Name,
			Market:       row.Market,
			Amount:       row.Amount,
			Volume:       row.Volume,
			MarketValue:  row.MarketValue,
			LastPx:       row.LastPx,
			PxChangeRate: parsePercent(row.PxChangeRate),
			MetricValue:  metric,
			Logo:         row.Logo.Logo,
		})
	}
	return Heatmap{
		SortKey:  sortKey,
		TypeCode: HeatmapTypeCode(market),
		Items:    items,
	}, nil
}

func (c *Client) fetchFundflow(market string) (Fundflow, error) {
	q := url.Values{}
	q.Set("bizType", "fundflow")
	q.Set("rn", strconv.Itoa(defaultFundflowRN))
	q.Set("market", market)
	q.Set("finClientType", "pc")
	env, err := baidu.GetAPI(c.http, c.cfg, "/sapi/v1/marketquote", q)
	if err != nil {
		return Fundflow{}, fmt.Errorf("fundflow: %w", err)
	}
	var wrapper struct {
		Fundflow []struct {
			BlockType     string `json:"blockType"`
			BlockTypeName string `json:"blockTypeName"`
			Data          []struct {
				Code            string `json:"code"`
				Name            string `json:"name"`
				MainNetTurnover string `json:"mainNetTurnover"`
				RawData         struct {
					MainNetTurnover float64 `json:"mainNetTurnover"`
				} `json:"rawData"`
			} `json:"data"`
		} `json:"fundflow"`
	}
	if err := json.Unmarshal(env.Result, &wrapper); err != nil {
		return Fundflow{}, fmt.Errorf("fundflow parse: %w", err)
	}
	groups := make([]FundflowGroup, 0, len(wrapper.Fundflow))
	for _, g := range wrapper.Fundflow {
		items := make([]FundflowItem, 0, len(g.Data))
		for _, row := range g.Data {
			items = append(items, FundflowItem{
				Code:            row.Code,
				Name:            row.Name,
				MainNetTurnover: row.MainNetTurnover,
				NetAmount:       row.RawData.MainNetTurnover,
			})
		}
		name := strings.TrimSpace(g.BlockTypeName)
		if name == "" {
			name = OverviewTabName(market, g.BlockType)
		}
		groups = append(groups, FundflowGroup{
			BlockType:     g.BlockType,
			BlockTypeName: name,
			Items:         items,
		})
	}
	return Fundflow{Groups: groups}, nil
}

func (c *Client) fetchOverview(market string) (Overview, error) {
	q := url.Values{}
	q.Set("hasTrend", "1")
	q.Set("market", market)
	q.Set("finClientType", "pc")
	env, err := baidu.GetAPI(c.http, c.cfg, "/vapi/v1/blocks/overview", q)
	if err != nil {
		return Overview{}, fmt.Errorf("overview: %w", err)
	}
	var rows []struct {
		Blocks []struct {
			Type string `json:"type"`
			Name string `json:"name"`
			List []struct {
				Code  string  `json:"code"`
				Name  string  `json:"name"`
				Price float64 `json:"price"`
				Ratio struct {
					Value  string `json:"value"`
					Status string `json:"status"`
				} `json:"ratio"`
				RiseFirst []struct {
					Name  string `json:"name"`
					Ratio struct {
						Value string `json:"value"`
					} `json:"ratio"`
				} `json:"rise_first"`
				MinuteData json.RawMessage `json:"minuteData"`
			} `json:"list"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(env.Result, &rows); err != nil {
		return Overview{}, fmt.Errorf("overview parse: %w", err)
	}
	if len(rows) == 0 {
		return Overview{Tabs: nil}, nil
	}
	tabs := make([]OverviewTab, 0, len(rows[0].Blocks))
	for _, block := range rows[0].Blocks {
		name := strings.TrimSpace(block.Name)
		if name == "" {
			name = OverviewTabName(market, block.Type)
		}
		items := make([]OverviewItem, 0, len(block.List))
		for _, row := range block.List {
			item := OverviewItem{
				Code:         row.Code,
				Name:         row.Name,
				Price:        row.Price,
				ChangePct:    parsePercent(row.Ratio.Value),
				ChangeStatus: row.Ratio.Status,
				Trend:        downsampleTrend(parseMinuteTrendRaw(row.MinuteData)),
			}
			if len(row.RiseFirst) > 0 {
				item.LeadName = row.RiseFirst[0].Name
				item.LeadChangePct = parsePercent(row.RiseFirst[0].Ratio.Value)
			}
			items = append(items, item)
		}
		tabs = append(tabs, OverviewTab{
			Type:  block.Type,
			Name:  name,
			Items: items,
		})
	}
	return Overview{Tabs: tabs}, nil
}

func normalizeSortKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultHeatmapSort
	}
	if _, ok := sortKeyLabels[raw]; ok {
		return raw
	}
	return defaultHeatmapSort
}

func parsePercent(raw string) float64 {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "%")
	raw = strings.TrimPrefix(raw, "+")
	v, _ := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
	return v
}

func parseMinuteTrend(raw string) []float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

func parseMinuteTrendRaw(raw json.RawMessage) []float64 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return parseMinuteTrend(s)
	}
	var obj struct {
		PriceInfo []struct {
			Price float64 `json:"price"`
		} `json:"priceinfo"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && len(obj.PriceInfo) >= 2 {
		out := make([]float64, 0, len(obj.PriceInfo))
		for _, p := range obj.PriceInfo {
			out = append(out, p.Price)
		}
		return out
	}
	return nil
}

func downsampleTrend(points []float64) []float64 {
	if len(points) <= maxTrendPoints {
		return points
	}
	out := make([]float64, 0, maxTrendPoints)
	step := float64(len(points)-1) / float64(maxTrendPoints-1)
	for i := 0; i < maxTrendPoints; i++ {
		idx := int(float64(i) * step)
		if idx >= len(points) {
			idx = len(points) - 1
		}
		out = append(out, points[idx])
	}
	return out
}

// SortKeyLabel returns Chinese label for heatmap sort key.
func SortKeyLabel(key string) string {
	if label, ok := sortKeyLabels[normalizeSortKey(key)]; ok {
		return label
	}
	return sortKeyLabels[defaultHeatmapSort]
}

// SortKeys returns supported heatmap sort keys.
func SortKeys() []string {
	return []string{"amount", "volume", "marketValue"}
}
