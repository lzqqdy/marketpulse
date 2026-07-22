# Implementation Plan: 资产报告

**Branch**: `006-portfolio-asset-reports` | **Date**: 2026-07-22 | **Spec**: [spec.md](./spec.md)

## Summary

在 `portfolio` 模块增加报告只读 API；前端用 `lightweight-charts` + SVG donut 实现市面常见资产报告布局（区间切换 + 净值/收益/收益率/日盈亏 + 分布）。

## Architecture

```text
AssetReportsPanel
  ├ range tabs → GET /reports/series
  ├ summary strip
  ├ Area: 资产净值 (CNY)
  ├ Line: 累计收益 (CNY)
  ├ Line: 累计收益率 (%)
  ├ Histogram: 每日盈亏 (CNY)
  └ Donut: GET /reports/allocation（实时持仓）
```

## Backend

- `repository.ListDailySnapshotsRange(userID, from, to)` — ASC，上限 2500
- `Service.ReportSeries(userID, rangeKey)`
- `Service.ReportAllocation(userID)` — 复用 holdings 估值
- 路由挂在现有 `/api/v1/portfolio`

## Frontend

```text
web/src/features/portfolio/
  AssetReportsPanel.vue
  AllocationDonut.vue
  composables/usePortfolioLineChart.ts
  api.ts / types.ts 扩展
  AssetCenterPanel.vue  — 增加 总览|报告 切换
```

## 灰度 / 回滚

无新开关；随 `portfolio.enabled`。回滚：去掉报告路由与 UI Tab 即可，表结构无变更。
