# MarketPulse Module Boundaries

MarketPulse is evolving from a single market dashboard into a modular product. The market data code should become an independent service layer that other modules consume without depending on its internals.

## Current Module Map

| Module | Purpose | Current Code | Owns |
| --- | --- | --- | --- |
| `marketdata` | Collect, normalize, cache, and stream market data | `internal/marketdata`, market routes in `internal/api` | Quotes, rates, indices, macro, derivatives, klines, alpha, market center, provider health |
| `alerts` | Price and market-condition monitoring | Implemented | Alert rules, trigger history, in_app / email / PushPlus |
| `portfolio` | Assets, positions, valuation, daily snapshots, reports | Implemented（`specs/005` + `specs/006`） | User holdings, principal, live overview, daily snapshots, chart reports |
| `ai` | Market analysis and assistant workflows | Implemented（`specs/007-ai-assistant/`） | Conversations, messages, tool-grounded chat, daily quota |
| `users` | Identity and access control | `internal/users`, `/api/v1/users`, `web/src/features/auth` | Users, sessions (Redis), profile, password; seed account (no public register) |
| `platform` | Shared infrastructure | `internal/config`, `internal/logging`, `internal/server`, `internal/platform/mysql`, `internal/platform/redis` | Config, logging, HTTP server, MySQL/Redis clients, future scheduler/jobs |
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
- Serving market center data (on-demand, not in snapshot).

Provider details are documented in [`DATA_SOURCES.md`](./DATA_SOURCES.md). New providers should be added there before or alongside implementation.

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

Owns price monitoring and notification workflow (`alerts.enabled` 灰度开关).

- Stores alert rules and trigger history in MySQL; cooldown / inbox in Redis.
- Evaluates rules from MarketStore change events for `spot` / `index` / `alpha` (no direct exchange calls).
- Sends notifications through `in_app` / email / PushPlus.
- Frontend: 用户中心规则与记录面板 + 全局 `AlertToastHost` WS 站内提醒.

### portfolio

Owns asset and holding state (`portfolio.enabled` 灰度开关).

- Stores holdings, principal, and daily valuation snapshots in MySQL.
- Uses market data quotes (crypto + alpha) and OTC rate for live valuation.
- Serves asset report series (trend / PnL charts) and allocation from snapshots / live holdings.
- Daily snapshot job.
- Does not mutate market data state.

### ai

Owns analysis workflows（规格：`specs/007-ai-assistant/`，OpenBB Rita 对话范式）。

- Reads market data, portfolio summaries, and user context through public module APIs.
- Stores conversations, messages, and (future) analysis jobs / generated insights.
- Streams chat via SSE；tool calling 只读 `MarketDataService`（二期可读 portfolio）。
- Does not embed provider-specific market fetching logic; does not place trades.

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
GET /api/v1/market/center
GET /api/v1/market/center/heatmap
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
  market/          # QuoteTable, MacroGrid, IndexGrid, MarketCenterPanel, AlphaStockPanel, KlineDrawer
  alerts/          # rules / deliveries / toast WS
  portfolio/       # AssetCenter + reports charts
  ai/              # specs/007-ai-assistant（一期已实现）
  auth/            # login / user center
web/src/shared/    # (planned)
web/src/stores/
  theme.ts         # global theme (not market-specific)
```

Market files now live under `web/src/features/market`. New feature work should avoid adding unrelated business logic to `web/src/features/market/stores/market.ts` or `DashboardView.vue`.
