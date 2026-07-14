# internal/

后端私有实现，禁止被外部项目 import。

| 包 | 职责 |
|----|------|
| `config` | YAML + 环境变量 |
| `logging` | 日志目录初始化 |
| `platform/mysql` | 可选 MySQL 连接池（users/alerts/portfolio） |
| `platform/redis` | 可选 Redis 客户端（会话/缓存/告警） |
| `marketdata` | 行情服务门面（`MarketDataService` 接口） |
| `marketdata/ingest` | 数据采集编排（WS 长连接 + REST 轮询） |
| `marketdata/ingest/binance` | Binance 现货 miniTicker |
| `marketdata/ingest/baidu` | 百度财经指数行情 + WS |
| `marketdata/ingest/equity` | 腾讯/东财指数备用 + 指数 K 线 |
| `marketdata/ingest/otc` | OKX C2C USDT/CNY |
| `marketdata/ingest/forex` | Frankfurter USD/CNY |
| `marketdata/ingest/macro` | CoinGecko + alternative.me |
| `marketdata/ingest/crypto` | CoinGecko 币种元数据 |
| `marketdata/ingest/derivatives` | Binance 衍生品指标 + 爆仓 |
| `marketdata/ingest/bitget` | Bitget 美股参考行情/K 线 |
| `marketdata/ingest/alpha` | Binance Alpha 备用 |
| `marketdata/ingest/metals` | 上金所国内黄金 |
| `marketdata/store` | 行情内存读模型（线程安全） |
| `marketdata/stream` | 行情 WS 广播 + K 线 WS 订阅 |
| `marketdata/marketcenter` | 行情中心按需 API（Baidu，短 TTL 缓存） |
| `marketdata/binance` | Binance K 线 REST/WS 客户端 |
| `api` | Gin HTTP/WS handlers + 路由注册 |
| `server` | Gin 引擎、CORS、可选静态资源托管 |

## API 路由

规范命名空间 `/api/v1/market/*` 和 `/ws/v1/market/*`，旧路径保留兼容。详见 `docs/RFC-002-api-contract.md`。

## 已完成

- Phase A–B：配置、Store、Binance ingest、REST/WS API、stream hub
- Phase D：汇率、股指（百度主源）、宏观、衍生品、美股参考
- 行情中心：`marketcenter` 按需 API
- Provider Health：12+ 数据源健康监控

## 规划中

`alerts`、`portfolio`、`ai`、`users` 通过 `marketdata.MarketDataService` 消费行情能力，不直接调用 ingest 包。
