# Tasks: 006 Portfolio Asset Reports

**Status**: Complete (2026-07-22)

- [x] `GET /api/v1/portfolio/reports/series` + range 解析
- [x] `GET /api/v1/portfolio/reports/allocation`
- [x] repo 区间查询 `ListDailySnapshotsRange`
- [x] 前端 `AssetReportsPanel`（区间切换 + 摘要）
- [x] 净值 / 累计收益 / 收益率 / 日盈亏图表（lightweight-charts）
- [x] 资产分布 SVG donut
- [x] 资产中心「总览 | 资产报告」二级切换
- [x] RFC-002 / MODULES / docs 索引更新
- [x] `go test` + `npm run build` 通过

## 验收

1. `portfolio.enabled: true`，登录 → 用户中心 → 资产中心 → **资产报告**
2. 有快照时切换 7d/30d/… 图表更新；有持仓时分布环图有数据
3. 无快照时空态提示；功能随 `portfolio.enabled` 灰度
