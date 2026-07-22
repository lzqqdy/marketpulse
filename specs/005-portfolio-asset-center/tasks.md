# Tasks: 005-portfolio-asset-center

**Status**: 一期核心已落地（2026-07-22）

## Backend

- [x] DDL migrate version 3 + `portfolio.enabled` 灰度
- [x] holdings / settings / overview / snapshots / eligible-symbols API
- [x] valuation 纯函数 + 单测
- [x] 日终 snapshot job（Asia/Shanghai）
- [x] `cmd/migrate-assets-log` 旧库导入

## Frontend

- [x] 用户中心「资产中心」Tab
- [x] 持仓表 + 本金 + 总览卡 + 快照列表（参考旧 UI，MarketPulse 视觉）

## Docs

- [x] RFC-002 / MODULES 更新

## Follow-ups

- [ ] 生产打开 `portfolio.enabled` 并验收
- [ ] 按需跑 legacy 迁移 dry-run
- [ ] 二期：报告图（series / allocation）
