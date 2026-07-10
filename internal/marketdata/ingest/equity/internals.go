package equity

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

const (
	eastmoneyClistBase   = "https://push2.eastmoney.com/api/qt/clist/get"
	eastmoneyAShareFS    = "m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048"
	aShareFields         = "f12,f14,f2,f3,f5,f6,f8,f20,f21"
	internalsPageSize    = 100
	defaultInternalsPoll = 60 * time.Second
	defaultInternalsIdle = time.Hour
	internalsHTTPTimeout = 12 * time.Second
)

var eastmoneyClistHosts = []string{
	"https://push2delay.eastmoney.com",
	"https://48.push2.eastmoney.com",
	"https://80.push2.eastmoney.com",
	"https://81.push2.eastmoney.com",
	"https://82.push2.eastmoney.com",
	"https://push2.eastmoney.com",
	"https://16.push2.eastmoney.com",
	"https://17.push2.eastmoney.com",
}

var internalsHTTPClient = &http.Client{
	Timeout:   internalsHTTPTimeout,
	Transport: eastmoneyKlineTransport(),
}

type clistResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Total int        `json:"total"`
		Diff  []clistRow `json:"diff"`
	} `json:"data"`
}

type clistRow struct {
	F2   float64 `json:"f2"`
	F3   float64 `json:"f3"`
	F5   float64 `json:"f5"`
	F6   float64 `json:"f6"`
	F8   float64 `json:"f8"`
	F12  string  `json:"f12"`
	F14  string  `json:"f14"`
	F20  float64 `json:"f20"`
	F21  float64 `json:"f21"`
	F104 float64 `json:"f104"`
	F105 float64 `json:"f105"`
	F128 string  `json:"f128"`
	F136 float64 `json:"f136"`
}

// NextInternalsPollInterval returns the poll cadence for A-share internals.
func NextInternalsPollInterval(now time.Time, active, idle time.Duration) time.Duration {
	if active <= 0 {
		active = defaultInternalsPoll
	}
	if idle <= 0 {
		idle = defaultInternalsIdle
	}
	if IsMarketActive("sh000001", now) {
		return active
	}
	return idle
}

// FetchAShareBreadth loads A-share realtime rows and computes breadth.
func FetchAShareBreadth(client *http.Client) (store.MarketBreadth, error) {
	if client == nil {
		client = internalsHTTPClient
	}
	rows, err := fetchAShareRows(client)
	if err != nil {
		return store.MarketBreadth{}, err
	}
	return ComputeMarketBreadth(rows, time.Now().UTC(), "eastmoney"), nil
}

// FetchCNInternals loads breadth, sector boards, and wind summary for A-shares.
func FetchCNInternals(client *http.Client, indexChangePct float64) (store.CNInternals, error) {
	if client == nil {
		client = internalsHTTPClient
	}
	now := time.Now().UTC()

	breadth, err := FetchAShareBreadth(client)
	if err != nil {
		return store.CNInternals{}, err
	}

	industry, err := FetchIndustrySectors(client)
	if err != nil {
		return store.CNInternals{}, err
	}
	if err := validateSectorFetch(industry, "industry"); err != nil {
		return store.CNInternals{}, err
	}

	concept, err := FetchConceptSectors(client)
	if err != nil {
		return store.CNInternals{}, err
	}
	if err := validateSectorFetch(concept, "concept"); err != nil {
		return store.CNInternals{}, err
	}

	wind := BuildMarketWind(breadth, industry, concept, indexChangePct, now)
	return store.CNInternals{
		Breadth:   breadth,
		Industry:  industry,
		Concept:   concept,
		Wind:      wind,
		UpdatedAt: now,
	}, nil
}

func fetchAShareRows(client *http.Client) ([]AShareRow, error) {
	raw, err := fetchClistAll(client, eastmoneyAShareFS, aShareFields, internalsPageSize)
	if err != nil {
		return nil, err
	}
	out := make([]AShareRow, 0, len(raw))
	for _, row := range raw {
		if row.F12 == "" || row.F14 == "" {
			continue
		}
		out = append(out, AShareRow{
			Code:           row.F12,
			Name:           row.F14,
			Price:          row.F2,
			ChangePct:      roundPct(row.F3),
			Volume:         row.F5,
			Amount:         row.F6,
			TurnoverRate:   row.F8,
			MarketCap:      row.F20,
			FloatMarketCap: row.F21,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("eastmoney a-share breadth: empty result")
	}
	return out, nil
}

func fetchClistAll(client *http.Client, fs, fields string, pageSize int) ([]clistRow, error) {
	if client == nil {
		client = internalsHTTPClient
	}
	if pageSize <= 0 {
		pageSize = internalsPageSize
	}
	page := 1
	out := make([]clistRow, 0, pageSize*4)
	for {
		rows, total, err := fetchClistPage(client, fs, fields, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		out = append(out, rows...)
		if total > 0 && len(out) >= total {
			break
		}
		if len(rows) == 0 {
			break
		}
		if len(rows) < pageSize && (total == 0 || len(out) >= total) {
			break
		}
		page++
		time.Sleep(80 * time.Millisecond)
	}
	return out, nil
}

func fetchClistPage(client *http.Client, fs, fields string, page, pageSize int) ([]clistRow, int, error) {
	q := url.Values{}
	q.Set("pn", strconv.Itoa(page))
	q.Set("pz", strconv.Itoa(pageSize))
	q.Set("po", "1")
	q.Set("np", "1")
	q.Set("fltt", "2")
	q.Set("fid", "f3")
	q.Set("fs", fs)
	q.Set("fields", fields)
	q.Set("ut", eastmoneyUT)
	query := q.Encode()

	var lastErr error
	for _, host := range eastmoneyClistHosts {
		req, err := http.NewRequest(http.MethodGet, host+"/api/qt/clist/get?"+query, nil)
		if err != nil {
			return nil, 0, err
		}
		setClistHeaders(req)
		req.Host = clistRequestHost(host)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			slog.Warn("eastmoney clist request failed", "host", host, "fs", fs, "page", page, "err", err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("eastmoney clist http %d", resp.StatusCode)
			continue
		}
		var parsed clistResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			lastErr = fmt.Errorf("eastmoney clist parse: %w", err)
			continue
		}
		if parsed.RC != 0 {
			lastErr = fmt.Errorf("eastmoney clist rc %d", parsed.RC)
			continue
		}
		return parsed.Data.Diff, parsed.Data.Total, nil
	}
	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, fmt.Errorf("eastmoney clist: all hosts failed")
}

func setClistHeaders(req *http.Request) {
	req.Header.Set("Referer", "https://quote.eastmoney.com/center/gridlist.html")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Close = true
}

func clistRequestHost(apiHost string) string {
	if strings.Contains(apiHost, "push2delay") {
		return "push2delay.eastmoney.com"
	}
	return "push2.eastmoney.com"
}

// BuildMarketWind applies rule-based summaries for A-share internals.
func BuildMarketWind(breadth store.MarketBreadth, industry, concept []store.SectorQuote, indexChangePct float64, now time.Time) store.MarketWind {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tags := make([]string, 0, 4)
	summaryParts := make([]string, 0, 3)

	if breadth.UpPct > 65 {
		tags = append(tags, "宽度偏强")
		summaryParts = append(summaryParts, "市场宽度偏强，赚钱效应较好")
	}
	if indexChangePct > flatChangeThreshold && breadth.UpPct < 40 {
		tags = append(tags, "权重拉动")
		summaryParts = append(summaryParts, "指数上涨但宽度较差，可能是权重拉动")
	}
	if indexChangePct < -flatChangeThreshold && breadth.UpPct < 35 {
		tags = append(tags, "普跌")
		summaryParts = append(summaryParts, "指数走弱且上涨家数偏少，市场情绪偏弱")
	}

	industryTop := topNSectors(industry, 10, true)
	conceptTop := topNSectors(concept, 10, true)
	industryAvg := avgSectorChange(industryTop, 10)
	conceptAvg := avgSectorChange(conceptTop, 10)
	if conceptAvg > 0 && industryAvg > 0 && conceptAvg >= industryAvg*1.3 && conceptAvg-industryAvg >= 0.8 {
		tags = append(tags, "题材活跃")
		summaryParts = append(summaryParts, "题材活跃，概念风向较强")
	}
	if sectorDispersion(industryTop, 10) >= 4 || sectorDispersion(conceptTop, 10) >= 5 {
		tags = append(tags, "结构分化")
		if len(summaryParts) == 0 || !strings.Contains(strings.Join(summaryParts, "；"), "结构性行情") {
			summaryParts = append(summaryParts, "结构性行情，注意分化")
		}
	}

	summary := "市场结构中性，等待更多宽度信号"
	if len(summaryParts) > 0 {
		summary = strings.Join(summaryParts, "；")
	} else if breadth.UpPct >= 55 {
		summary = "上涨家数占优，短线情绪尚可"
	} else if breadth.DownPct >= 55 {
		summary = "下跌家数占优，短线情绪偏弱"
	}

	return store.MarketWind{
		Summary:   summary,
		Tags:      tags,
		UpdatedAt: now,
	}
}
