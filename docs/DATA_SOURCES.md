# MarketPulse Data Sources

This document describes the current market data providers, how they are connected, and where each feed lands in the normalized read model.

The market data boundary is `internal/marketdata`. Other future modules should consume the normalized REST/WS contracts instead of calling these upstream APIs directly.

## Runtime Overview

| Area | Primary Source | Fallback / Secondary | Transport | Code |
| --- | --- | --- | --- | --- |
| Spot crypto quotes | Binance Spot miniTicker | None | WebSocket | `internal/marketdata/ingest/binance` |
| Spot crypto K lines | Binance Spot REST + WS | None | REST history, WS live | `internal/marketdata/binance`, `internal/marketdata/stream` |
| USDT/CNY | OKX C2C | None | REST poll | `internal/marketdata/ingest/otc` |
| USD/CNY | Frankfurter | None | REST poll | `internal/marketdata/ingest/forex` |
| Global indices and commodities | Baidu Finance | Tencent, Eastmoney | WS + REST poll | `internal/marketdata/ingest/baidu`, `internal/marketdata/ingest/equity` |
| Index K lines | Baidu Finance | Eastmoney, Tencent | REST | `internal/marketdata/ingest/baidu`, `internal/marketdata/ingest/equity` |
| Domestic gold | Eastmoney AU9999 | Sina gold T+D (`gds_AUTD`) | REST poll | `internal/marketdata/ingest/metals` |
| Macro | alternative.me, CoinGecko | Stablecoin market cap best-effort | REST poll | `internal/marketdata/ingest/macro` |
| Crypto metadata | CoinGecko | None | REST poll | `internal/marketdata/ingest/crypto` |
| Derivatives indicators | Binance USD-M Futures | None | REST poll | `internal/marketdata/ingest/derivatives` |
| Liquidations | Binance USD-M Futures | REST aggregate from memory | WS live + memory window | `internal/marketdata/ingest/derivatives` |
| US stock reference quotes | Bitget USDT-FUTURES | Binance Alpha | REST bootstrap + WS live, REST fallback | `internal/marketdata/ingest/bitget`, `internal/marketdata/ingest/alpha` |
| US stock reference K lines | Bitget USDT-FUTURES | Binance Alpha | REST history, WS/poll live | `internal/marketdata/service.go`, `internal/marketdata/stream/kline.go` |
| Market center (A/HK/US) | Baidu Finance | None | REST on-demand + TTL cache | `internal/marketdata/marketcenter` |

## Provider Health Names

Provider health is exposed through:

```text
GET /api/v1/market/providers/status
```

Current provider names:

| Name | Label | Role |
| --- | --- | --- |
| `binance_spot_ws` | Binance Spot WS | Crypto quote primary |
| `okx_c2c` | OKX C2C | USDT/CNY primary |
| `frankfurter_fx` | Frankfurter FX | USD/CNY primary |
| `baidu_index` | Baidu Finance | Index quote primary |
| `tencent_index` | Tencent | Index quote fallback |
| `eastmoney_index` | Eastmoney | Index quote fallback |
| `eastmoney_gold` | Eastmoney Gold | Domestic gold primary |
| `sina_gold` | Sina Gold | Domestic gold fallback |
| `coingecko_macro` | CoinGecko Macro | Macro primary |
| `coingecko_meta` | CoinGecko Metadata | Crypto metadata auxiliary |
| `binance_derivatives` | Binance Derivatives | Futures indicators primary |
| `binance_liquidations` | Binance Liquidations | Liquidation stream/aggregate |
| `bitget_alpha` | Bitget USDT-FUTURES | US stock reference primary |
| `binance_alpha` | Binance Alpha | US stock reference fallback |
| `baidu_market_center` | Baidu Market Center | Market center panel auxiliary |
| `baidu_expressnews` | Baidu Express News | 7×24 express news auxiliary |

The `alpha` category is a legacy internal/API name for the "US stock reference" panel. UI text should call it `美股参考`.

## Crypto Spot Quotes

### Binance Spot miniTicker

- Purpose: live crypto price table.
- Endpoint: `wss://stream.binance.com:9443/stream?streams=btcusdt@miniTicker/...`
- Config: `symbols`, `ingest.binance.ws_base`.
- Startup: one combined stream is built from configured symbols.
- Normalized output: `store.Quote`.
- Store field: `Snapshot.Quotes`.
- WebSocket channel: `quotes`.
- Health: `binance_spot_ws`.

### Binance Spot K Lines

- REST history endpoint: `GET https://api.binance.com/api/v3/klines`
- WS live endpoint: `wss://stream.binance.com:9443/ws/{symbol}usdt@kline_{interval}`
- REST API: `GET /api/v1/market/klines?symbol=BTC&interval=1h`
- WS API: `WS /ws/v1/market/kline?symbol=BTC&interval=1h`
- Normalized output: `binance.Candle`.

## FX and OTC

### OKX C2C USDT/CNY

- Purpose: USDT/CNY reference rate.
- Endpoint: `GET https://www.okx.com/v3/c2c/otc-ticker/quotedPrice`
- Poll interval: `ingest.otc.usdt_cny_interval`, default `30s`.
- Normalized output: `Rates.USDTCNY`.
- Health: `okx_c2c`.

### Frankfurter USD/CNY

- Purpose: fiat USD/CNY reference rate.
- Endpoint: `GET https://api.frankfurter.app/latest?from=USD&to=CNY`
- Poll interval: `ingest.forex.usd_cny_interval`, default `1h`.
- Normalized output: `Rates.USDCNY`.
- Health: `frankfurter_fx`.

## Indices and Commodities

### Watchlist

Configured by `ingest.equity.index_ids`.

Current default ids:

```text
sh000001, sz399001, sz399006, sh000300, sh000688,
hsi, n225, ks11,
dji, ixic, gspc,
gold, silver, crude
```

`sge-au9999` is appended separately by the domestic gold poller (Eastmoney → Sina).

### Baidu Finance Index Quotes

- Purpose: primary index and commodity quote source.
- HTTP endpoint: `GET https://finance.pae.baidu.com/vapi/v1/getquotation`
- WS endpoint: `wss://finance-ws.pae.baidu.com` (`product=snapshot`, `financeType=index`)
- Provider role: primary.
- Health: `baidu_index`.
- Fallback inside Baidu: WS disconnects fall back to the existing HTTP poller in `pollEquity`.

### Tencent Index Quotes

- Purpose: fallback index and commodity quote source.
- Endpoint: `GET https://qt.gtimg.cn/q={symbols}`
- Request shape: batched by Tencent symbol, for example `s_sh000001,s_usIXIC,hf_GC`.
- Provider role: fallback.
- Health: `tencent_index`.

### Eastmoney Index Quotes

- Purpose: fallback index quote source.
- Endpoint: `GET https://push2.eastmoney.com/api/qt/stock/get`
- Request shape: one request per `secid`, for example `1.000001`, `100.NDX`.
- Provider role: fallback.
- Health: `eastmoney_index`.

### Polling and Cache

- Poller: dynamic equity poller.
- Open-market interval: `equity.ActiveTTL`, currently `1m`.
- Closed-market interval: `equity.InactiveTTL`, currently `1h`.
- Market windows are in `internal/marketdata/ingest/equity/window.go`.
- Provider order comes from `ingest.equity.providers`, default `baidu`, then `tencent`, then `eastmoney`.
- A circuit breaker can temporarily skip a failing provider.

### Index K Lines

- Primary endpoint: `GET https://finance.pae.baidu.com/selfselect/getstockquotation`
- Fallback endpoint: `GET https://push2his.eastmoney.com/api/qt/stock/kline/get`
- REST API: `GET /api/v1/market/index-klines?id=sh000001&interval=1d`
- Supported intervals: Baidu handles `1h`, `1d`, `1w`; `15m` falls back to Eastmoney.
- K lines are cached by index/interval to avoid repeated upstream calls.

### Domestic Gold

- Purpose: domestic Au99.99 RMB/gram quote.
- Primary endpoint: `GET https://push2.eastmoney.com/api/qt/stock/get?fltt=2&secid=118.AU9999`
- Fallback endpoint: `GET https://hq.sinajs.cn/?list=gds_AUTD` (Referer: `https://finance.sina.com.cn`)
- Polling follows gold market window: active `1m`, inactive `1h`.
- Normalized id: `sge-au9999` (stable public id; UI label 国内金价).
- Health: `eastmoney_gold` (primary), `sina_gold` (fallback).
- Ingest status: `domestic_gold` (`ok` / `degraded` / `error`).

## Macro and Metadata

### alternative.me Fear and Greed

- Endpoint: `GET https://api.alternative.me/fng/?limit=1`
- Normalized output: `MacroSnapshot.FearGreed`.

### CoinGecko Global

- Endpoint: `GET https://api.coingecko.com/api/v3/global`
- Normalized output: total market cap, total volume, BTC/ETH dominance.
- Health: `coingecko_macro`.

### CoinGecko Categories

- Endpoint: `GET https://api.coingecko.com/api/v3/coins/categories`
- Purpose: stablecoin market cap and 24h change.
- Failure is best-effort: macro still succeeds without stablecoin fields when this call fails.

### CoinGecko Markets Metadata

- Endpoint: `GET https://api.coingecko.com/api/v3/coins/markets`
- Query: `vs_currency=usd`, `ids=bitcoin,ethereum,...`, `sparkline=false`.
- Purpose: rank, icon, market cap, and 24h volume for configured crypto symbols.
- Poll interval: `ingest.macro.interval`, default `5m`.
- Health: `coingecko_meta`.

## Derivatives

All derivatives data currently uses Binance USD-M Futures.

| Metric | Endpoint | Output |
| --- | --- | --- |
| Global long/short | `GET https://fapi.binance.com/futures/data/globalLongShortAccountRatio` | `MacroSnapshot.LongShort` |
| Top trader long/short | `GET https://fapi.binance.com/futures/data/topLongShortPositionRatio` | `MacroSnapshot.TopLongShort` |
| Funding / mark / index price | `GET https://fapi.binance.com/fapi/v1/premiumIndex` | `MacroSnapshot.Funding` |
| Open interest history | `GET https://fapi.binance.com/futures/data/openInterestHist` | `MacroSnapshot.OpenInterest` |
| Taker buy/sell ratio | `GET https://fapi.binance.com/futures/data/takerlongshortRatio` | `MacroSnapshot.TakerBuySell` |
| Liquidations | `wss://fstream.binance.com/market/ws/!forceOrder@arr` | in-memory liquidation window |

Derivatives polling currently follows `ingest.equity.interval` for several REST metrics. Liquidations maintain an in-memory one-hour rolling window and also have a minute poller to publish aggregate state.

## US Stock Reference

This area is still named `alpha` in backend structs and API contracts for compatibility, but product UI should call it `美股参考`.

### Config

```yaml
alpha:
  enabled: true
  provider: bitget
  product_type: USDT-FUTURES
  quote_asset: USDT
  poll_interval: 30s
  resolve_interval: 10m
  indices:
    - id: qqq
      name: 纳指ETF
      symbol: QQQUSDT
  stocks:
    - id: nvda
      name: 英伟达
      symbol: NVDAUSDT
```

Current default provider is Bitget, with Binance Alpha retained as fallback.

### Bitget USDT-FUTURES

- Product type: `USDT-FUTURES`.
- Ticker REST endpoint: `GET https://api.bitget.com/api/v2/mix/market/tickers?productType=USDT-FUTURES`
- K line REST endpoint: `GET https://api.bitget.com/api/v2/mix/market/history-candles`
- Public WS endpoint: `wss://ws.bitget.com/v2/ws/public`
- WS ticker subscription:
  - `instType=USDT-FUTURES`
  - `channel=ticker`
  - `instId={symbol}`, for example `AAPLUSDT`
- WS kline subscription:
  - `channel=candle1m`, `candle5m`, or `candle1H`
  - non-live intervals use REST snapshot only.

Ticker mapping:

| Bitget field | Normalized field |
| --- | --- |
| `lastPr` | `AlphaQuote.Price` |
| `change24h` | `AlphaQuote.Change24hPct` |
| `baseVolume`, `usdtVolume`, `quoteVolume` | `AlphaQuote.Volume` |
| `markPrice` | `AlphaQuote.MarkPrice` |
| `indexPrice` | `AlphaQuote.IndexPrice` |
| `fundingRate` | `AlphaQuote.FundingRate` |

`change24h` is normalized to percentage form when Bitget returns a decimal ratio.

### Binance Alpha Fallback

- REST ticker endpoint: `https://www.binance.com/bapi/defi/v1/public/alpha-trade/ticker`
- REST kline endpoint: `https://www.binance.com/bapi/defi/v1/public/alpha-trade/klines`
- Resolve endpoints:
  - `https://www.binance.com/bapi/defi/v1/public/alpha-trade/get-exchange-info`
  - `https://www.binance.com/bapi/defi/v1/public/wallet-direct/buw/wallet/cex/alpha/all/token/list`
- Legacy WS support remains in `internal/marketdata/ingest/alpha`, but the current Bitget-primary path uses Binance Alpha mainly as REST fallback.

Fallback behavior:

- Quotes: Bitget REST bootstrap, then Bitget WS live; resolve/poll/WS failure triggers one Binance Alpha REST fallback poll.
- REST K lines: Bitget history first; Binance Alpha is used when Bitget history is unavailable or sparse.
- Live K lines: Bitget WS for the latest candle when available; otherwise Bitget REST poll at `alpha.poll_interval`, then Binance Alpha REST poll as fallback.
- Provider health marks `bitget_alpha` and `binance_alpha` separately and marks the currently used one.

## Market Center

行情中心为按需 API，不纳入全局 snapshot 或 WS 推送。

- Package: `internal/marketdata/marketcenter`
- Upstream: Baidu Finance（经 CDN 代理）
- Cache: 服务端短 TTL 内存缓存（对齐 equity 轮询节奏）
- REST API:
  - `GET /api/v1/market/center?market=ab|hk|us`
  - `GET /api/v1/market/center/heatmap?market=&sortKey=amount|volume|marketValue`
- 前置条件: `ingest.baidu.enabled=true`
- 后台预热: `refresher.go` 在交易时段预热 ab/hk/us 缓存
- 前端: `MarketCenterPanel.vue`，60s 轮询

Baidu 上游端点:

| 模块 | 端点 |
| --- | --- |
| 涨跌分布 | `/sapi/v1/marketquote?bizType=chgdiagram` |
| 热力图 | `/vapi/v2/blocks?style=heatmap` |
| 主力净流入 | `/sapi/v1/marketquote?bizType=fundflow` |
| 热门板块 | `/vapi/v1/blocks/overview?hasTrend=1` |

市场参数映射:

| UI Tab | market | 热力图 typeCode |
| --- | --- | --- |
| A股 | `ab` | `HY` |
| 港股 | `hk` | `HSHY` |
| 美股 | `us` | `HY` |

## Normalized Output Contracts

External provider data should be converted before reaching other modules:

| Store type | Used by |
| --- | --- |
| `Quote` | Crypto spot table |
| `Rates` | FX/OTC display and CNY conversion |
| `IndexQuote` | Global overview and index K-line entry points |
| `AlphaQuote` / `AlphaSnapshot` | US stock reference panel, legacy `alpha` API field |
| `MacroSnapshot` | Market indicators |
| `binance.Candle` | All K-line APIs after normalization |
| `marketcenter.CenterResponse` | Market center panel (on-demand, not in snapshot) |

Do not expose raw provider payloads outside `internal/marketdata/ingest/*` or `internal/marketdata/binance`.

## Operational Notes

- All provider clients should accept an injected `*http.Client` for tests.
- Provider-specific parsing should stay unit-tested with local fixtures or test servers.
- New providers must be added to provider health before being surfaced in UI.
- If a source is only a fallback, set `Role: "fallback"` so status can show `standby` before first successful use.
- When changing API/WS fields, update `docs/RFC-002-api-contract.md` and frontend TypeScript types in the same change.
