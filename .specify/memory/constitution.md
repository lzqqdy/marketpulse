# MarketPulse Constitution

## Core Principles

### I. Module Boundaries First

Market data is an independent service layer. Do not add users, alerts, portfolio, or AI state to the market data store. New modules consume market data through API, WebSocket, events, or stable service interfaces — never through provider packages or ingestion internals.

### II. Contract Before Code

API, WebSocket, or persisted data changes require updating the relevant RFC or contract document first (`docs/RFC-002-api-contract.md` for market routes). Keep Go response structs, TypeScript types, and docs synchronized. Prefer additive API changes; document migration paths for breaking changes.

### III. Small, Focused Changes

Keep changes scoped to the owning module and its public boundary. Read `docs/MODULES.md` before adding features. Split large Vue components before adding new responsibilities. Dashboard views compose feature components rather than owning feature logic.

### IV. Test What You Touch

- Backend shared changes: `go test -buildvcs=false ./...`
- Frontend behavior or type changes: `npm run build` from `web/`
- Route changes: add or update handler/server tests for canonical and compatibility paths
- Provider changes: keep external calls behind test servers or mocked clients

### V. Simplicity Over Abstraction

Prefer explicit module packages over generic names. Keep provider clients small and testable with injected HTTP clients. Put persistence behind repository interfaces before business services depend on it. Do not over-engineer — YAGNI applies.

## Technology Stack

| Layer | Stack |
|-------|-------|
| Backend | Go, Gin, in-memory store (market read model) |
| Frontend | Vue 3, Pinia, Vite 6, lightweight-charts, TypeScript |
| Deploy | Single-port `marketd`, `make ship` to HK VPS |
| Docs | RFC series in `docs/`, feature specs in `specs/` |

Canonical market routes: `/api/v1/market/*` and `/ws/v1/market/*`. Legacy routes are compatibility aliases only.

## Development Workflow

1. For new features: use Spec Kit flow (`/speckit-specify` → `/speckit-plan` → `/speckit-tasks` → `/speckit-implement`).
2. For incremental work on existing roadmap: follow `docs/RFC-004-implementation-roadmap.md` steps.
3. Before implementation: read `docs/MODULES.md`, `docs/VIBE_GUIDE.md`, and relevant RFCs.
4. After implementation: run focused tests, then broader tests when shared code changes.

## Quality Gates

- No exchange API calls from alerts, portfolio, or AI modules.
- No auth checks inside market data ingestion.
- No burying new API paths in unrelated handlers.
- Provider-specific logic belongs in `internal/marketdata/ingest/` only.
- New product areas live under `web/src/features/` when introduced.

## Governance

This constitution supersedes ad-hoc development practices. Amendments require updating this file and noting the change in commit messages. Runtime development guidance lives in `docs/VIBE_GUIDE.md` and `docs/SPEC_KIT_GUIDE.md`.

**Version**: 1.0.0 | **Ratified**: 2026-07-10 | **Last Amended**: 2026-07-11
