# Research: 旧资产中心与迁移

**Feature**: `005-portfolio-asset-center`  
**Date**: 2026-07-22

## 旧项目角色

| 项目 | 路径 | 职责 |
|------|------|------|
| mine-web-master | `F:\lzqqdy\mine-web-master` | 资产页 UI、设置持仓/本金、总览 API、快照列表、分析图 |
| go-coin-master | `F:\lzqqdy\go-coin-master` | 行情进 Redis；`@daily` 写 `assets_log` |
| MarketPulse | `F:\lzqqdy\marketpulse` | 新产品；portfolio 待建 |

## 关键代码

- 日终：`go-coin-master/crontab/job/assets.go`
- 模型：`go-coin-master/models/assets.go`、`assets_log.go`
- DDL：`go-coin-master/README.md`
- 总览：`mine-web-master/app/controller/Index.php::getAssetsWave`
- 设仓：`setAssets` / `editUser`（本金 + 勾选币）
- 列表：`assetLog` / `getAssetsLog`
- UI：`view/index/assets.html`、`asset.html`

## 旧总览字段（getAssetsWave）

- `total_usdt` / `total_cny`
- `diff_usdt` / `diff_cny`：相对开盘价的浮动（非今日快照差）
- `today_diff` / `today_diff_pro`：相对**最新一条** assets_log 的 CNY
- `total_profit` / `total_profit_rate`：相对 `user.balance`
- `total_profit_7` / `_30`：相对 date=今天-7/-30 的快照
- `diff_r`：U 溢价 `(refer_price - usd_price) / usd_price`

一期总览对齐：**总资产、U溢价、今日、7日、30日、历史**；「相对开盘浮动」可作为可选增强（有 start/open 价时再做）。

## 旧日终逻辑注意点

1. 快照日期写 **昨天**。  
2. 日收益相对 `GetLastByUid`（最新一条，可能含坑若乱序）。  
3. 累计相对 `GetFastByUid`（最早一条，常为本金行 date=1）。  
4. bug 风险：循环里 `append` 前 map 复用——迁移以 DB 落库值为准，不依赖重跑旧脚本。  
5. USDT-CNY 缺省 6.44。

## 收益率存储

- 库内多为 **小数**（`0.0127`）。  
- 展示 `*100 + '%'`。  
- `total_profit_rate` 旧数据常见 `2.32` 表示 232%。  
- 新 API 建议：JSON 同时给 `rate`（小数）与格式化由前端做；或明确文档「一律小数」。

## 迁移执行建议

1. 只读连接旧库或先 `mysqldump` 出 `assets_log` / `assets` / `user`。  
2. 准备 `uid_map.yaml`：`{ old_uid: new_user_id }`。  
3. Dry-run 统计行数、日期范围、date=1 行、JSON 解析失败行。  
4. 导入 snapshots（source=legacy）→ settings → holdings。  
5. 抽查：首末日金额、随机 3 日与旧后台一致。  
6. 打开资产中心列表人工验收。

## 不照搬项

| 旧行为 | 原因 |
|--------|------|
| VIP mobile 白名单 | 改为登录 + portfolio.enabled |
| 全局 Redis 改 USDT 价 | 污染行情；用 marketdata OTC |
| coin_list 与 assets 双轨勾选 | 合并为 holdings 一张表 |
| 分析页硬编码本金 33300 | 用 settings.principal |
| yfinance 美股 | 用 alpha / 指数行情 |

## 结论

一期以「表语义兼容 + 公式兼容 + 迁移工具」为目标，UI/模块边界按 MarketPulse 重做，不移植 PHP/Layui。
