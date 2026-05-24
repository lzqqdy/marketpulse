# Vibe Coding Guide

This guide is for future AI-assisted development. Keep changes small, preserve module boundaries, and update contracts before changing behavior.

## Default Workflow

1. Read `docs/MODULES.md` before adding a new feature.
2. Identify the owning module.
3. Update the relevant RFC or contract document when API, WebSocket, or persisted data changes.
4. Keep changes scoped to the owning module and its public boundary.
5. Run focused tests first, then broader tests when shared code changes.

## Module Rules

- Market data is an independent service layer. Do not add users, alerts, portfolio, or AI state to the market data store.
- New modules must consume market data through API, WebSocket, events, or a stable service interface.
- Provider-specific logic belongs in market data ingestion only.
- Shared infrastructure belongs in platform-style packages such as config, logging, server, database, scheduler, or HTTP clients.
- API handlers should adapt HTTP/WS requests to services. They should not become the main business-logic layer.

## Backend Rules

- Prefer explicit module packages over generic names as the project grows.
- Keep provider clients small and testable with injected HTTP clients.
- Put persistence behind repository interfaces before business services depend on it.
- Keep route registration split by module.
- Keep legacy routes only as compatibility wrappers for canonical routes.
- Do not let planned modules import market data internals such as `ingest`, provider packages, or read-model implementation details.

## Frontend Rules

- New product areas should live under feature folders when introduced.
- Do not add alert, portfolio, user, or AI state to the market Pinia store.
- Keep API clients close to their feature.
- Split large Vue components before adding new responsibilities.
- Dashboard views should compose feature components rather than own feature logic.

## API and Contract Rules

- Canonical market data routes use `/api/v1/market/*` and `/ws/v1/market/*`.
- Legacy market routes are compatibility aliases only.
- New modules get their own namespaces.
- Keep Go response structs, TypeScript types, and docs synchronized.
- Prefer additive API changes. If a breaking change is unavoidable, document the migration path.

## Test Expectations

- Backend shared changes: run `go test -buildvcs=false ./...`.
- Frontend behavior or type changes: run `npm run build` from `web/`.
- Route changes: add or update handler/server tests for both canonical and compatibility paths.
- Provider changes: keep external calls behind test servers or mocked clients.

## Common Pitfalls

- Do not turn `store` into a global product state container.
- Do not call exchange APIs from alerts, portfolio, or AI.
- Do not put auth checks directly inside market data ingestion.
- Do not bury new API paths in unrelated handlers.
- Do not rely on README examples alone as the API contract.
