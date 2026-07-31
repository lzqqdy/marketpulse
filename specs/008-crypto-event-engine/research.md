# Research: 008-crypto-event-engine

**Date**: 2026-07-31  
**Status**: Draft  
**Spec**: [spec.md](./spec.md)

## 1. 问题陈述

MarketPulse 能回答「现在是什么价」，尚不能系统回答「刚刚发生了什么值得关注的事」。需要一层规则驱动的 Event Engine：把行情异常变成 Signal，再聚合成可展示、可查询、可广播的 MarketEvent。一期仅加密，且不做个人推送。

## 2. 与 alerts 的边界

| 维度 | alerts（已有） | event（本期） |
|------|----------------|---------------|
| 触发源 | 用户规则 | 系统规则 / 模板 |
| 受众 | 单用户 | 所有访客（公共事实） |
| WS | `/ws/v1/alerts/stream` + token | `/ws/v1/events` 无 token |
| 通道 | in_app / email / PushPlus | 仅公共 REST + 公共 WS |
| 数据 | 规则、投递、inbox | Event、Signal、生命周期 |

**结论**：新建 `internal/event`，一期**不**调用 alerts dispatcher。二期若做个人推送，优先「Event 产出 → 用户订阅 → 复用 alerts 投递」，避免两套邮件/PushPlus。

## 3. 数据原料可行性

| Detector | 原料 | 结论 |
|----------|------|------|
| Price | `Klines(symbol, interval, limit)` | **可行**；Snapshot 仅有日涨跌，不够短窗 |
| Volume | K 线 `Volume` 序列算 ratio | **可行** |
| Volatility | K 线算 realized vol / ATR / 分位 | **可行**（一期可用简化版：滚动 std 或 ATR vs 近 N 窗均值） |
| Liquidation | `Macro.Liquidations`（全市场 1h 滚动） | **部分可行**：无「BTC 15m 专属爆仓」；与价格 Signal **同窗关联**即可支撑 Flash Sell-off 叙事，文档与 UI 需诚实标注 |
| Fund Flow | marketcenter（A/HK/US） | **一期不做** |
| Cross-Market | 指数 + crypto | **一期不做**完整 RISK_OFF |

### 爆仓实现细节（现网）

- `ingest/liquidations.go`：Binance forceOrder WS → 内存 `liquidationWindow`（默认 lookback 1h）
- 写入 `store.Liquidations{Window:"1h", LongUsd, ShortUsd, TotalUsd}`
- Event 引擎需**自建**历史 `TotalUsd` 时间序列（环形缓冲）才能算 ratio；marketdata 不提供历史基线 API

### 基线策略（一期选定）

| 指标 | 基线 | 初值规则（可配置） |
|------|------|-------------------|
| Price | 窗口起止价算 changePct；可选近 20 窗 |absolute| 均值做相对判断 | 15m ≥ 3%、5m ≥ 2%、1h ≥ 5%（与需求草案对齐，可调） |
| Volume | 近 N 根同 interval 均量（N 默认 20） | ratio ≥ 2/3/5 → 中/高/极高分量 |
| Volatility | 近 N 窗 ATR 或 std | 当前 > 基线 × k 或 > 历史 P95（P95 需足够样本，不足则退回倍数阈值） |
| Liquidation | 引擎内过去 M 个采样点的均值 | ratio ≥ 3 触发（可配） |

一期**先落地绝对阈值 + ratio**；Z-Score / 严格分位数作为同模块可进化项，不阻塞 MVP。

## 4. 评测驱动模型

**选定**：仿 `alerts.Evaluator` —— `MarketData.AddListener` + 通道合并 + **固定节流**（建议 15–30s）跑检测轮。

备选（未选）：纯 cron 无视行情活跃度 —— 更简单但休市/冷市浪费与热市延迟不可调。

K 线拉取：检测轮内按需 `Klines`；对配置 symbol 列表限流；失败跳过本轮该标的。

## 5. 聚合策略

**选定**：基于**时间窗 + symbol（及 LIQUIDATION 市场级信号）+ 模板匹配**的规则 Aggregator。

示例模板：

| 模板 ID | 所需 Signal | Event type / subType |
|---------|-------------|----------------------|
| flash_selloff | PRICE_DROP + VOLUME_SPIKE + LIQUIDATION_SPIKE（同窗） | `CRYPTO_MOVE` / `FLASH_SELLOFF` |
| volume_move | PRICE_* + VOLUME_SPIKE | `CRYPTO_MOVE` / `VOLUME_MOVE` |
| price_only | PRICE_*  alone | `PRICE_ANOMALY` / `DROP` or `SPIKE` |

同一 ACTIVE Event（同模板 + 主 symbol）未 RESOLVED 前，新 Signal **merge**，更新 score 与 timeline。

**未选**：图数据库 / 复杂相关矩阵 —— 超出一期，且无历史图数据。

## 6. 公共 WS 模型

**选定**：内存广播 Hub（所有连接收同一类消息），对齐 `market/stream` 的公开性；**不**做 per-user Register。

与 alerts Hub 差异：alerts 按 `userID` 路由；event 无用户维度。

## 7. 存储

**选定**：MySQL 两张核心表 `market_event`、`market_event_signal`；`market_event_relation` 可建空表或一期省略（spec 允许预留）。

热路径：检测状态、liquidation 基线环形缓冲、ACTIVE 索引 —— **内存**；落库异步或同事务短路径，避免阻塞 listener 回调（listener 内只投递到内部 queue）。

## 8. 前端接入

**选定**：`web/src/features/events/` + 挂到 `MarketDashboard` 侧栏（与 `ExpressNewsPanel` 并列）。

不新增必须登录的用户中心 Tab（一期无订阅配置）。

## 9. 多市场扩展缺口（记录供二期）

| 市场 | 缺口 |
|------|------|
| 美股参考 | 有报价+K线；无真股资金流；需标明「参考」 |
| 指数 | 有 IndexKlines；量能弱；需交易时段 |
| A/港/美中心 | Fundflow 有，但是按需+TTL，非 tick |
| 外汇 | 仅汇率，无盘 |
| Cross-Market | 缺 VIX/DXY 专用高质量源与时段对齐 |

## 10. 决策汇总

| 议题 | 决策 | 理由 |
|------|------|------|
| 模块 | 新建 `event` | 与 alerts 产品不同 |
| 鉴权 | 公共只读 | 产品定位为市场事实；用户要求一期不做个人推送 |
| 检测 | 规则 + K 线 + 爆仓聚合 | 数据已具备；符合「LLM 不发现」 |
| 聚合 | 模板 + 时间窗 | 可验收 Flash Sell-off |
| 存储 | MySQL + 内存热状态 | 对齐现有业务模块 |
| 爆仓叙事 | 市场级关联 | 诚实反映现网能力 |
