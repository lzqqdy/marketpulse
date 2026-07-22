# Feature Specification: 资产报告（Portfolio Reports）

**Feature Branch**: `006-portfolio-asset-reports`  
**Created**: 2026-07-22  
**Status**: Complete  
**Input**: 基于每日资产快照与当前持仓，在资产中心提供「资产报告」页：市面常见的净值走势、累计收益/收益率、每日盈亏柱状图、资产分布等图表；支持时间范围切换。

## 背景

- 一期（`005`）已提供持仓、本金、总览、日快照列表与日终任务。
- 旧 mine-web `assetAnalysis` 使用 ECharts：累计收益率、每日收益、资产走势、资产分布 + 近 N 天切换。
- MarketPulse 前端已有 `lightweight-charts`，报告时序图复用该库；饼/环图用轻量 SVG，不新增 ECharts 依赖。

## 已确认决策

| # | 议题 | 决策 |
|---|------|------|
| 1 | 入口 | 资产中心内二级切换：「总览」\|「报告」，不新开顶级用户 Tab |
| 2 | 图表集合（一期） | ①资产净值走势 ②累计收益(CNY) ③累计收益率(%) ④每日盈亏柱 ⑤当前资产分布 |
| 3 | 时间范围 | `7d` / `30d` / `90d` / `180d` / `1y` / `all`（默认 `30d`） |
| 4 | 分布数据源 | **当前持仓实时估值**（与总览一致）；缺价标的单独标注 |
| 5 | 时序数据源 | `portfolio_snapshots`（`kind=daily`）按日期升序 |
| 6 | 图表库 | 时序：`lightweight-charts`；分布：SVG donut |
| 7 | 不做（本期） | 月历热力图、回撤曲线、对照 BTC 基准、导出 PDF |

## User Scenarios & Testing

### User Story 1 - 查看资产报告图表 (Priority: P1)

作为已登录用户，我希望在资产中心打开「报告」，看到选定时间范围内的净值、收益与分布图，以便理解资产变化。

**Acceptance Scenarios**:

1. **Given** 用户有 ≥2 条日快照，**When** 打开报告并选 30d，**Then** 净值/累计收益/收益率/日盈亏图均有数据点。
2. **Given** 无快照，**When** 打开报告，**Then** 时序图空态提示「暂无快照数据」；若有持仓则分布图仍可展示。
3. **Given** 切换 7d→1y，**When** 请求完成，**Then** 图表与区间汇总指标随之更新。

### User Story 2 - 区间汇总 (Priority: P1)

作为用户，我希望在图表上方看到区间内起止净值、区间盈亏与收益率摘要。

**Acceptance Scenarios**:

1. **Given** 区间内有快照，**When** 加载报告 series，**Then** 返回 `summary.startCny/endCny/pnlCny/pnlPct`。
2. **Given** 仅 1 个点，**When** 计算 pnlPct，**Then** 不除零；可为 0 或 null。

### User Story 3 - 资产分布 (Priority: P1)

作为用户，我希望看到当前各标的占比环形图与图例（金额 + 百分比）。

**Acceptance Scenarios**:

1. **Given** 多持仓且有报价，**When** 请求 allocation，**Then** 各切片 valueCny 之和 ≈ 总 CNY；百分比之和 ≈ 100%。
2. **Given** 部分缺价，**When** 展示，**Then** 仅计入有价持仓，并提示 missing。

## Requirements

- **FR-001**: MUST 提供 `GET /api/v1/portfolio/reports/series?range=`
- **FR-002**: MUST 提供 `GET /api/v1/portfolio/reports/allocation`
- **FR-003**: MUST 在资产中心提供报告 UI：范围选择 + 5 类图表 + 空态
- **FR-004**: 涨跌色 MUST 跟随主题 `--up` / `--down`
- **FR-005**: 禁止报告模块直连交易所；只读 snapshots / holdings 估值路径

## Success Criteria

- 有历史快照时，报告页四类时序图可读、可切换区间。
- 有持仓时分布图正确。
- `portfolio.enabled=false` 时接口禁用，与一期一致。
