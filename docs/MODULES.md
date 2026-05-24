# MarketPulse Module Boundaries

MarketPulse is evolving from a single market dashboard into a modular product. The market data code should become an independent service layer that other modules consume without depending on its internals.

## Current Module Map

| Module | Purpose | Current Code | Owns |
| --- | --- | --- | --- |
| `marketdata` | Collect, normalize, cache, and stream market data | `internal/marketdata`, market routes in `internal/api` | Quotes, rates, indices, macro, derivatives, klines, provider health |
| `alerts` | Price and market-condition monitoring | Planned | Alert rules, trigger history, notification delivery |
| `portfolio` | Assets, positions, transactions, valuation | Planned | User holdings, cost basis, PnL, portfolio snapshots |
| `ai` | Market analysis and assistant workflows | Planned | Analysis jobs, prompts, model responses, cached insights |
| `users` | Identity and access control | Planned | Users, sessions, tokens, preferences |
| `platform` | Shared infrastructure | `internal/config`, `internal/logging`, `internal/server`, future DB/scheduler packages | Config, logging, HTTP server, persistence, background jobs |
| `web` | Browser UI | `web/src` | Feature views, client state, API clients |

## Dependency Direction

Allowed direction:

```text
platform
  <- marketdata
  <- alerts / portfolio / ai / users
  <- api
  <- web
```

Business modules may consume `marketdata` through stable service interfaces or HTTP/WS APIs. They must not call provider packages such as Binance, OKX, CoinGecko, Eastmoney, or Tencent directly.

## Market Data Contract

The market data module is responsible for:

- Connecting to external data sources.
- Normalizing source-specific payloads into MarketPulse types.
- Maintaining the in-memory market read model.
- Publishing REST snapshots and WebSocket streams.
- Reporting provider health.
- Serving kline data.

The market data module is not responsible for:

- User accounts or permissions.
- Alert rule storage.
- Portfolio or transaction records.
- AI prompts, model calls, or analysis history.
- Feature-specific UI state.

Future modules should consume market data through a narrow boundary similar to:

```go
type MarketDataService interface {
    Snapshot() Snapshot
    Quote(symbol string) (Quote, bool)
    ProviderStatus() ProviderStatusResponse
    Klines(ctx context.Context, req KlineRequest) (KlineResponse, error)
}
```

The exact interface can evolve, but callers should depend on an interface owned at the module boundary, not on `ingest`, `store`, or provider internals.

## Planned Module Responsibilities

### alerts

Owns price monitoring and notification workflow.

- Stores alert rules and trigger history in a repository.
- Evaluates rules from market data events or snapshots.
- Sends notifications through configurable channels.
- Does not fetch external prices itself.

### portfolio

Owns asset and holding state.

- Stores assets, positions, transactions, and valuation snapshots.
- Uses market data quotes for live valuation.
- Does not mutate market data state.

### ai

Owns analysis workflows.

- Reads market data, portfolio summaries, and user context through public module APIs.
- Stores prompts, analysis jobs, and generated insights.
- Does not embed provider-specific market fetching logic.

### users

Owns identity and preferences.

- Stores users, sessions, auth tokens, and user settings.
- Provides identity context to alerts, portfolio, and AI.
- Does not own market data access.

## API Namespace Plan

Canonical market endpoints use the `market` namespace:

```text
GET /api/v1/market/snapshot
GET /api/v1/market/providers/status
GET /api/v1/market/klines
GET /api/v1/market/index-klines
WS  /ws/v1/market/stream
WS  /ws/v1/market/kline
```

Legacy endpoints remain mounted for compatibility:

```text
GET /api/v1/snapshot
GET /api/v1/providers/status
GET /api/v1/klines
GET /api/v1/index-klines
WS  /ws/v1/stream
WS  /ws/v1/kline
```

New modules should use their own namespaces:

```text
/api/v1/alerts
/api/v1/portfolio
/api/v1/ai
/api/v1/users
```

## Frontend Module Plan

Keep feature code grouped by domain as the UI grows:

```text
web/src/features/
  market/
  alerts/
  portfolio/
  ai/
  auth/
web/src/shared/
```

Market files now live under `web/src/features/market`. New feature work should avoid adding unrelated business logic to `web/src/features/market/stores/market.ts` or `DashboardView.vue`.
