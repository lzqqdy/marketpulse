# Quickstart: 008-crypto-event-engine

本地验证 Crypto Event Engine（实现完成后使用）。文档阶段仅定义路径。

## 前置

- 已能 `make dev-api` / `make dev-web` 跑通行情
- MySQL 可用（与 users/alerts 相同实例即可）
- Binance K 线与爆仓源基本健康（或具备测试注入手段）

## 配置

在 `config/config.yaml`：

```yaml
mysql:
  enabled: true
  # ... DSN 同现有模块

event:
  enabled: true
  evaluate_interval: 20s
  symbols: [BTC, ETH]
  aggregate_window: 10m
  price:
    "5m": { threshold_pct: 2 }
    "15m": { threshold_pct: 3 }
    "1h": { threshold_pct: 5 }
  volume:
    lookback_bars: 20
    ratio_medium: 2
    ratio_high: 3
    ratio_extreme: 5
  volatility:
    lookback_bars: 20
    ratio: 2
  liquidation:
    sample_interval: 1m
    baseline_samples: 60
    ratio: 3
```

启动后日志应出现 event 模块 migrate/bootstrap 成功；若 MySQL 缺失则 soft-skip。

## 冒烟

```bash
# 列表（无需 token）
curl -s 'http://127.0.0.1:8080/api/v1/events?limit=5' | jq .

# 详情（替换 ID）
curl -s 'http://127.0.0.1:8080/api/v1/events/evt_xxx' | jq .

# 时间线
curl -s 'http://127.0.0.1:8080/api/v1/events/evt_xxx/timeline' | jq .
```

WS（可用 `websocat` 或浏览器）：

```text
ws://127.0.0.1:8080/ws/v1/events
```

## UI

1. 打开 `http://localhost:5173`
2. 首页侧栏可见「市场异动」面板（未登录）
3. 有 ACTIVE 事件时可点开详情看 Signal / 时间线

## 验收场景（理想）

在测试环境注入或等待真实行情：

1. BTC 短时急跌 + 放量 + 爆仓放大 → 单一 `FLASH_SELLOFF`（或等价）Event
2. WS 收到 `event.created` / `event.updated`
3. `event.enabled: false` 重启后 API 503，行情 WS 仍正常

## 关闭

```yaml
event:
  enabled: false
```

无需改行情相关配置。
