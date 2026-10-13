# Market data reliability

- Production frontend failures never switch to generated quotes or candles. The explicit market-store demo entry remains separate from live mode.
- Kline requests are scoped to a view generation. Changing symbol/interval or closing the drawer cancels REST work and invalidates WS callbacks/timers. Network failures reconnect without substituting prices.
- The Kline Hub reserves one upstream task per symbol/interval under its lock. Each browser has one writer, a 64-message queue and a 5-second write timeout. Upstream failures retry after three seconds and refresh history on the existing browser connection. Subscription identity checks prevent an old task from modifying a replacement; cached snapshots preserve the actual provider and its error state. Binance streams have heartbeat/read deadlines and close promptly on cancellation.
- Event WS connections each have one writer and a 64-message queue. Heartbeat replies share that writer. A full queue or a 5-second write timeout disconnects only that client.
- `KlinesContext` and `IndexKlinesContext` propagate caller cancellation to provider HTTP requests, including fallback and symbol resolution. Legacy methods retain their signatures. Event `kline_timeout` applies to each query, not the whole sequential evaluation round.
- Equity quote polling and cache expiry use `ingest.equity.interval`; closed-market policy remains one hour, capped by the next opening. Index kline and market-center cache policies are unchanged. Derivatives use `ingest.derivatives.interval` (default one minute).

## Valuation quality

Crypto quotes older than two minutes and stock-reference quotes older than five minutes are marked stale. Unknown timestamps and timestamps more than one minute in the future are unverified. Reference feeds are still polled while the underlying stock market is closed; a closed stock exchange alone does not imply a failed reference feed. These conservative thresholds describe feed freshness, not an exchange holiday calendar.

Portfolio responses add `staleSymbols`; holding rows add `stale`, `priceUpdatedAt`, and `priceSource`. Stale positive prices remain in reference valuations rather than becoming zero. Daily snapshots reject stale/missing quotes and unverified conversion rates.

USDT/CNY and USD/CNY carry separate acquisition timestamps. An update to one cannot make the other appear fresh. USDT/CNY older than five minutes retains the last rate with `rateFallback=true`; if no positive rate exists, the configured default is used with the same warning. Portfolio totals and allocation views display degraded-valuation warnings.

Feed updates merge their value and timestamp atomically inside MarketStore. A late older response cannot overwrite a newer value, and USD/CNY cannot revert a concurrently refreshed USDT/CNY rate or its derived crypto CNY prices.

## Daily snapshots

The date belongs to the sampling day in `portfolio.daily_timezone` (default Asia/Shanghai). Samples run only during 23:59:00-23:59:59, retrying failures within that window. Starting the process during this window triggers an immediate attempt. Missing historical dates stay missing; current holdings cannot reconstruct a past portfolio and are never written under yesterday's date.

One frozen market snapshot is used for all users in an attempt. `valuedAt` records the actual capture instant. Holdings/principal changed after capture, degraded prices, and writes crossing midnight are rejected. Completed snapshots are inserted once, never overwritten by a later attempt. A missing immediately previous date produces `hasDailyBaseline=false`: the record establishes a baseline, daily profit/rate are stored as zero for contract compatibility, and the UI displays no daily baseline rather than claiming a daily return. Today's overview also requires an exact yesterday baseline. Cash-flow accounting is unchanged.

Migration version 7 (`internal/portfolio/migrate/002_snapshot_valuation.sql`) adds nullable `valued_at_ms` and `has_daily_baseline`. Existing records keep unknown metadata and are not rewritten. Normal startup with portfolio auto-migration applies this migration; deployments with auto-migration disabled must apply and register version 7 through their migration process before running this version.

## Alert reevaluation

Market changes and rule updates wake evaluation; a one-second timer also reevaluates fresh quotes so cooldown expiry and daily rollover do not depend on price changes. Redis cooldown still claims triggers atomically. Window sampling uses distinct feed timestamps, while timer checks only expire existing observations: polling does not invent new price samples or extend an obsolete five-minute spike.

## Regression checks

Run `go test ./...`, focused `go test -race` for event/marketdata/portfolio, and `npm test`, `npm run typecheck`, `npm run build` in `web`. Tests use mock providers and fake timers, not live exchanges.
