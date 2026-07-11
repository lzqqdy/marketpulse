# Feature Specification: 7×24 财经快讯

**Feature Branch**: `003-express-news-panel`  
**Created**: 2026-07-11  
**Status**: Done

## 目标

在 Dashboard 最底部新增「7×24快讯」模块，对标百度财经快讯页：Tab 筛选、时间/关联/内容/来源列表、瀑布流加载更多。

## 数据源

百度 `GET /selfselect/expressnews`（稳定性 ⭐5）

| Tab | tag 参数 |
|-----|----------|
| 全部 | `""` |
| A股 | `A股` |
| 港股 | `港股` |
| 美股 | `美股` |
| 异动 | `异动` |

分页：`pn`（页码，从 0）、`rn`（每页条数，默认 20）

## 架构

- **不做 ingest 轮询**，按需 REST API + 服务端缓存
- 缓存策略：首页无新快讯时延长 TTL，历史页更长 TTL
- 复用 `baidu/client.go` CDN 回退

## API

`GET /api/v1/market/expressnews?tag=&pn=0&rn=20`

## 验收

1. 页面底部展示快讯列表，Tab 切换正常
2. 滚动到底自动加载下一页（瀑布流）
3. 关联标的显示 logo + 涨跌幅标签
4. 后端缓存命中时减少百度请求
