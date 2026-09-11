# Market data reliability

- Production frontend failures never switch to generated quotes or candles. The explicit market-store demo entry remains separate from live mode.
- Kline requests are scoped to a view generation. Changing symbol/interval or closing the drawer cancels REST work and invalidates WS callbacks/timers. Network failures reconnect without substituting prices.
- Event WS connections each have one writer and a 64-message queue. Heartbeat replies share that writer. A full queue or a 5-second write timeout disconnects only that client.
- `KlinesContext` and `IndexKlinesContext` propagate caller cancellation to provider HTTP requests, including fallback and symbol resolution. Legacy methods retain their signatures. Event `kline_timeout` applies to each query, not the whole sequential evaluation round.
- Equity quote polling and cache expiry use `ingest.equity.interval`; closed-market policy remains one hour, capped by the next opening. Index kline and market-center cache policies are unchanged. Derivatives use `ingest.derivatives.interval` (default one minute).

## Valuation quality

Crypto quotes older than two minutes and stock-reference quotes older than five minutes are marked stale. Unknown timestamps and timestamps more than one minute in the future are unverified. Reference feeds are still polled while the underlying stock market is closed; a closed stock exchange alone does not imply a failed reference feed. These conservative thresholds describe feed freshness, not an exchange holiday calendar.

Portfolio responses add `staleSymbols`; holding rows add `stale`, `priceUpdatedAt`, and `priceSource`. Stale positive prices remain in reference valuations rather than becoming zero. Daily snapshots reject stale/missing quotes and unverified conversion rates and retry deferred users. No historical or cash-flow accounting semantics are changed here.

USDT/CNY and USD/CNY carry separate acquisition timestamps. An update to one cannot make the other appear fresh. USDT/CNY older than five minutes retains the last rate with `rateFallback=true`; if no positive rate exists, the configured default is used with the same warning. Portfolio totals and allocation views display degraded-valuation warnings.

## Regression checks

Run `go test ./...`, focused `go test -race` for event/marketdata/portfolio, and `npm test`, `npm run typecheck`, `npm run build` in `web`. Tests use mock providers and fake timers, not live exchanges.
