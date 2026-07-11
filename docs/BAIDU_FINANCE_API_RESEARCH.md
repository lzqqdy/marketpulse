# Baidu Finance API Research

> **注意**：本文档为 2026-07-10 早期调研版本。百度财经已成为指数主源（specs/001 已实现）。
> 最新调研报告见 [providers/baidu_finance.md](./providers/baidu_finance.md)，数据源接入见 [DATA_SOURCES.md](./DATA_SOURCES.md)。

Target page:

```text
https://finance.baidu.com/index/ab-000001?mainTab=%E7%AE%80%E5%86%B5
```

Research date: 2026-07-10.

## MarketPulse Fit

MarketPulse backend currently uses Go `net/http` plus stdlib JSON parsing for upstream market data. No extra SDK is needed for the usable Baidu Finance endpoints. The Vue frontend should continue consuming MarketPulse APIs instead of calling Baidu directly.

Recommended integration location:

```text
internal/marketdata/ingest/baidu/
```

> **已更新（2026-07-11）**：Baidu Finance 已成为指数行情与 K 线的主数据源（primary），腾讯/东财为 fallback。见 `specs/001-baidu-index-provider/`。

## Common Request Shape

Base URL:

```text
https://finance.pae.baidu.com
```

Useful headers:

```text
Accept: application/vnd.finance-web.v1+json
Referer: https://finance.baidu.com/index/ab-000001?mainTab=%E7%AE%80%E5%86%B5
User-Agent: normal browser UA
```

Common query parameter:

```text
finClientType=pc
```

Most successful responses use:

```json
{
  "ResultCode": 0,
  "Result": {}
}
```

## Usable Without Acs-Token

### Basic Info

```text
GET /sapi/v1/basicinfo?code=000001&market=ab&financeType=index&finClientType=pc
```

Returns index metadata: Chinese/English name, publish agency/date, base date, base point, description, index type, weighting method, constituent count.

### Constituents Heatmap/List

```text
GET /sapi/v1/constituents?financeType=index&market=ab&code=000001&style=heatmap&sortKey=pxChangeRate&rn=20&pn=0&finClientType=pc
```

Returns `Result.list.body[]` with constituent stock quote fields:

```text
code, name, exchange, financeType, market, lastPx, pxChange, pxChangeRate,
amount, volume, turnoverRatio, marketValue, rawData
```

Useful `sortKey` values observed in page code include `pxChangeRate`; table modules also use `mvWeight`, `marketValue`, and fund-flow variants depending on `bizType`.

### Up/Down Distribution

```text
GET /sapi/v1/constituents?financeType=index&market=ab&code=000001&graphType=simple&bizType=chgdiagram&finClientType=pc
```

Returns `Result.chgdiagram`:

```text
total.price
ratio.up / ratio.down / ratio.balance
diagram[]: title, status, count
updateDate
```

### Main Fund Flow by Block

```text
GET /sapi/v1/index/main-fundflow?code=000001&market=ab&finClientType=pc
```

Returns concept, industry, and region groups. Each group has block rows with:

```text
code, name, market, financeType, mainNetTurnover, rawData.mainNetTurnover
```

### Turnover / Metric Trend

```text
GET /sapi/v1/metrictrend?financeType=index&market=ab&code=000001&targetType=amount&metric=amount&finClientType=pc
```

Returns:

```text
overview[]: name, unit, value
trend[]: marketDate, data.amount, data.rawData.amount
```

### Related Indices

```text
GET /sapi/v1/index/related?financeType=index&market=ab&code=000001&style=tablelist&finClientType=pc
```

Returns table headers and related index rows with quote-like fields:

```text
code, name, market, exchange, financeType, lastPx, pxChange, pxChangeRate,
tradeStatus, tradeStatusCN, rawData
```

### Small Trend Sparkline

```text
POST /selfselect/gettrenddata
Content-Type: application/x-www-form-urlencoded

stock=[{"code":"000001","market":"ab","type":"index"}]&finClientType=pc
```

Returns `Result.trend.index_ab_000001.p` as a comma-separated price series, plus `lastPrice`, `preClosePx`, and `status`.

## Guarded / Not Recommended

### Real-Time Quote Chart

Page source calls:

```text
GET /vapi/v1/getquotation
```

Observed page params:

```text
srcid=5353
all=1
code=000001
query=000001
eprop=min
financeType=index
group=quotation_index_minute
market_type=ab
finClientType=pc
```

Direct backend `curl` returned HTTP 403 without `Acs-Token`.

The page initializes Baidu Paris anti-abuse signing:

```text
sid=2108
acsUrl=https://dlswbr.baidu.com/heicha/mm/2108/acs-2108.js
header=Acs-Token
```

Do not depend on this endpoint from MarketPulse backend unless we intentionally implement and maintain the browser-side signing flow. Existing Tencent/Eastmoney sources are more stable for quotes and K-lines.

## Implementation Notes

1. Add a small Baidu client with a shared `http.Client{Timeout: 10 * time.Second}`.
2. Keep Baidu-specific response structs in the Baidu package and normalize only fields used by MarketPulse UI.
3. Treat formatted fields like `"1.52万亿"` as display-only; prefer `rawData` numeric values when available.
4. Cache non-real-time modules for at least 1-5 minutes. Basic info can be cached much longer.
5. Mark provider health separately, for example `baidu_finance_index_detail`, if added to `/api/v1/market/providers/status`.
6. Keep Tencent/Eastmoney as quote/K-line providers; use Baidu for enrichment: index description, constituent heatmap, up/down distribution, related indices, and block fund-flow summaries.
