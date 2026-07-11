# Implementation Plan: 7×24 财经快讯

## 后端

1. `internal/marketdata/expressnews/` — 百度转发、解析、指纹缓存
2. `GET /api/v1/market/expressnews` — Gin handler
3. `MarketDataService.ExpressNews` — service 接入

## 前端

1. `types/expressNews.ts` + `api/expressNews.ts`
2. `ExpressNewsPanel.vue` — Tab、列表、IntersectionObserver 瀑布流
3. `MarketDashboard.vue` — 底部挂载

## 缓存

| 场景 | TTL |
|------|-----|
| pn=0 且有新快讯 | 30s |
| pn=0 指纹未变 | 2min |
| pn>0 历史页 | 10min |

## 健康监测

- Provider：`baidu_expressnews`（category: `news`）
- `/healthz` ingest key：`expressnews`
