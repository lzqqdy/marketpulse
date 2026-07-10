# Data Model: 百度指数数据源

## 配置模型扩展

### `ingest.baidu`（新增）

```yaml
ingest:
  baidu:
    enabled: true
    base_url: "https://finance.pae.baidu.com"
    ws_url: "wss://finance-ws.pae.baidu.com"
    quote_interval: 5s        # HTTP 降级轮询间隔（交易时段）
    quote_interval_inactive: 30s
    ws_enabled: true
    ws_reconnect_max: 5
    ws_reconnect_delay: 3s
    ws_patch_interval: 60s      # patch 续订
    http_rate_limit: 10       # req/s
```

### `ingest.equity.providers`（变更默认值）

```yaml
providers:
  - baidu      # primary
  - tencent    # fallback 1
  - eastmoney  # fallback 2
```

## IndexDef 扩展

```go
type IndexDef struct {
    ID             string
    Name           string
    EastmoneySecID string
    TencentSymbol  string
  // 新增
    BaiduCode       string  // 如 "000001"
    BaiduMarket     string  // ab | hk | us | global
    BaiduFinanceType string // index | futures，默认 index
    MinPrice        float64
    MaxPrice        float64
}

func (d IndexDef) HasBaidu() bool {
    return d.BaiduCode != "" && d.BaiduMarket != ""
}
```

## 归一化映射

### 行情 → `store.IndexQuote`

| 百度字段 | IndexQuote 字段 |
| --- | --- |
| `cur.price` | `Price` |
| `cur.ratio`（去 `%`） | `ChangePercent` |
| `cur.increase` | `Change` |
| `def.ID` | `ID` |
| `def.Name` | `Name` |
| — | `Source = "baidu"` |
| `time.Now().UTC()` | `UpdatedAt` |

### K 线 → `binance.Candle`

| 百度 marketData 列 | Candle 字段 |
| --- | --- |
| timestamp | `Time` (ms) |
| open/close/high/low | OHLC |
| volume | `Volume` |
| amount | `QuoteVolume` |

## Provider Health

| Name | Label | Category | Role |
| --- | --- | --- | --- |
| `baidu_index` | Baidu Finance | index | primary |
| `tencent_index` | Tencent | index | fallback |
| `eastmoney_index` | Eastmoney | index | fallback |

`baidu_index` 子状态（ingest status key）：

- `equity_baidu` — 整体
- `equity_baidu_ws` — WS 连接状态（connected / degraded / disabled）

## WS 消息模型（订阅）

```json
{
  "source": "pc-web",
  "method": "subscribe",
  "product": "snapshot",
  "items": [
    {"code": "000001", "name": "上证指数", "market": "ab", "financeType": "index"}
  ]
}
```

每 60s 发送 `method: "patch"` 续订。

## 降级状态机

```text
[启动]
  → 尝试 WS 连接
    → 成功 → WS 推送更新 equityCache
    → 失败 → HTTP 轮询模式（getquotation）
      → 成功 → healthy
      → 失败 → circuit open → tencent → eastmoney
```

K 线链（每次请求）：

```text
baidu getstockquotation → eastmoney → tencent (1d race)
```
